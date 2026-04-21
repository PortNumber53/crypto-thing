# RabbitMQ Job Architecture Design for CRYPTHING-1

## Overview

This document outlines the design for introducing RabbitMQ-backed job processing to split long-running history fetches into per-day jobs per product at 1-minute granularity.

## Current Implementation

The current `exchange coinbase history` command:
1. Fetches all products from the database
2. Iterates through days from today to the earliest product start date
3. For each day, iterates through all products
4. Uses recursive splitting to handle large time ranges
5. Processes everything synchronously in a single process

**Problems:**
- Long-running single process (can take hours/days)
- No fault tolerance - if process crashes, work is lost
- No parallelization across products/days
- No way to monitor progress or retry failed segments
- Resource intensive - holds database connections for extended periods

## Proposed Architecture

### 1. Producer/Consumer Topology

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Producer      │    │   RabbitMQ      │    │   Consumer      │
│   (CLI Command) │───>│   Exchange      │───>│   Workers       │
│                 │    │   + Queues      │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 2. Queue Design

#### Option 1: Single Queue with Routing Keys
- **Exchange:** `cryptool.history` (topic exchange)
- **Queue:** `cryptool.history.jobs`
- **Routing Key Pattern:** `{exchange}.{product}.{granularity}.{date}`
  - Example: `coinbase.BTC-USD.1m.2024-01-15`

#### Option 2: Per-Product Queues
- **Exchange:** `cryptool.history` (topic exchange)
- **Queues:** `cryptool.history.{product}` (e.g., `cryptool.history.BTC-USD`)
- **Routing Key:** `{exchange}.{granularity}.{date}`

**Recommendation:** Option 1 (single queue with routing keys) for:
- Simpler management
- Better load balancing across workers
- Easier monitoring
- No risk of queue buildup for popular products

### 3. Job Payload Contract

```json
{
  "job_id": "uuid-v4",
  "exchange": "coinbase",
  "product_id": "BTC-USD",
  "granularity": "1m",
  "day_start": "2024-01-15T00:00:00Z",
  "day_end": "2024-01-15T23:59:59Z",
  "created_at": "2024-01-16T10:30:00Z",
  "priority": 5,
  "retry_count": 0,
  "max_retries": 3,
  "correlation_id": "parent-job-uuid",
  "metadata": {
    "triggered_by": "cli",
    "user": "system",
    "reason": "backfill"
  }
}
```

### 4. Message Format

**Format:** JSON (for simplicity and debuggability)
**Alternative:** Protocol Buffers (for performance if needed later)

### 5. Queue Configuration

```yaml
exchange:
  name: cryptool.history
  type: topic
  durable: true
  auto_delete: false

queue:
  name: cryptool.history.jobs
  durable: true
  auto_delete: false
  arguments:
    x-message-ttl: 86400000  # 24 hours
    x-max-length: 100000     # 100k jobs max
    x-dead-letter-exchange: cryptool.history.dlx

bindings:
  - routing_key: "coinbase.*.1m.*"
    queue: cryptool.history.jobs
```

### 6. Dead Letter Queue (DLQ) Strategy

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Main Queue    │───>│   DLX           │───>│   DLQ           │
│                 │    │ (Dead Letter    │    │                 │
└─────────────────┘    │  Exchange)      │    └─────────────────┘
                       └─────────────────┘
```

- **DLX:** `cryptool.history.dlx` (fanout exchange)
- **DLQ:** `cryptool.history.dlq`
- **Triggers:** Message TTL expired, max retries exceeded, queue length exceeded
- **Monitoring:** Alert on DLQ growth, inspect failed jobs

### 7. Idempotency & Deduplication

#### Job Keys
- **Primary Key:** `{exchange}:{product_id}:{granularity}:{day_start}`
- **Database:** Create `history_jobs` table with unique constraint
- **Redis:** Optional caching layer for fast duplicate detection

#### Implementation
```sql
CREATE TABLE history_jobs (
    job_id UUID PRIMARY KEY,
    exchange VARCHAR(50) NOT NULL,
    product_id VARCHAR(50) NOT NULL,
    granularity VARCHAR(10) NOT NULL,
    day_start TIMESTAMPTZ NOT NULL,
    day_end TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    UNIQUE(exchange, product_id, granularity, day_start)
);
```

### 8. Retry & Backoff Strategy

- **Max Retries:** 3 per job
- **Backoff:** Exponential with jitter
  - 1st retry: 5 seconds + random(0-2s)
  - 2nd retry: 25 seconds + random(0-5s)
  - 3rd retry: 125 seconds + random(0-10s)
- **Poison Message Handling:** After max retries, move to DLQ

### 9. Observability

#### Metrics (Prometheus)
- `cryptool_history_jobs_total` - Total jobs processed
- `cryptool_history_jobs_failed_total` - Failed jobs
- `cryptool_history_jobs_retried_total` - Retried jobs
- `cryptool_history_job_duration_seconds` - Job processing time
- `cryptool_history_queue_depth` - Current queue depth
- `cryptool_history_candles_inserted_total` - Candles inserted per job

#### Logging
- Structured JSON logs
- Correlation IDs across producer → queue → consumer
- Log levels: DEBUG, INFO, WARN, ERROR

#### Tracing
- OpenTelemetry integration
- Trace job lifecycle: creation → queue → processing → completion
- Database query tracing

### 10. Migration Plan

#### Phase 1: Infrastructure Setup
1. Add RabbitMQ to Docker Compose / deployment
2. Create exchanges, queues, bindings
3. Deploy monitoring (Prometheus, Grafana)

#### Phase 2: Producer Implementation
1. Create `JobProducer` service
2. Modify `history` command to produce jobs instead of processing
3. Add feature flag: `USE_RABBITMQ_JOBS=false` (default)

#### Phase 3: Consumer Implementation
1. Create `JobConsumer` service
2. Implement job processing logic (reuse existing code)
3. Add health checks and metrics

#### Phase 4: Testing & Rollout
1. Test with small product set
2. Enable feature flag for 10% of traffic
3. Monitor metrics and error rates
4. Gradual rollout to 100%

#### Phase 5: Cleanup
1. Remove old synchronous code
2. Remove feature flag
3. Update documentation

## Implementation Details

### Producer Code Structure

```go
// cmd/cryptool/root/coinbase_history.go
func newCoinbaseHistoryCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "history",
        Short: "Queue 1m candle fetch jobs for all products",
        RunE: func(cmd *cobra.Command, args []string) error {
            if config.UseRabbitMQ {
                return queueHistoryJobs(cmd.Context())
            }
            return legacyHistoryFetch(cmd.Context())
        },
    }
}

func queueHistoryJobs(ctx context.Context) error {
    producer := rabbitmq.NewJobProducer(config.RabbitMQ.URL)
    defer producer.Close()
    
    products := store.GetAllProducts(ctx, "coinbase")
    now := time.Now().UTC()
    
    for _, product := range products {
        startDate := store.GetProductStart(ctx, "coinbase", product)
        for day := now; day.After(startDate); day = day.AddDate(0, 0, -1) {
            job := HistoryJob{
                ID:          uuid.New(),
                Exchange:    "coinbase",
                ProductID:   product,
                Granularity: "1m",
                DayStart:    day.Truncate(24 * time.Hour),
                DayEnd:      day.Truncate(24 * time.Hour).Add(24 * time.Hour),
            }
            if err := producer.Publish(ctx, job); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Consumer Code Structure

```go
// internal/worker/history_worker.go
type HistoryWorker struct {
    consumer *rabbitmq.Consumer
    store    *ingest.Store
    client   *coinbase.Client
    metrics  *prometheus.Metrics
}

func (w *HistoryWorker) Process(ctx context.Context, job HistoryJob) error {
    start := time.Now()
    w.metrics.JobStarted(job.ProductID)
    
    // Check idempotency
    if w.store.JobExists(ctx, job.Key()) {
        w.metrics.JobSkipped(job.ProductID)
        return nil
    }
    
    // Mark job as started
    w.store.MarkJobStarted(ctx, job.ID)
    
    // Fetch candles
    candles, err := w.client.GetCandlesOnce(ctx, job.ProductID, job.DayStart, job.DayEnd, job.Granularity, 350)
    if err != nil {
        w.metrics.JobFailed(job.ProductID, "fetch_error")
        return err
    }
    
    // Insert candles
    inserted, err := w.store.InsertCandles(ctx, job.Exchange, job.ProductID, candles)
    if err != nil {
        w.metrics.JobFailed(job.ProductID, "insert_error")
        return err
    }
    
    // Mark job as completed
    w.store.MarkJobCompleted(ctx, job.ID)
    w.metrics.JobCompleted(job.ProductID, inserted, time.Since(start))
    
    return nil
}
```

## Configuration

### Environment Variables

```bash
# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=cryptool.history
RABBITMQ_QUEUE=cryptool.history.jobs
RABBITMQ_DLX=cryptool.history.dlx
RABBITMQ_DLQ=cryptool.history.dlq

# Feature Flag
USE_RABBITMQ_JOBS=false

# Worker Settings
WORKER_CONCURRENCY=10
WORKER_PREFETCH_COUNT=5
WORKER_ACK_TIMEOUT=300s

# Retry Settings
MAX_RETRIES=3
RETRY_BACKOFF_BASE=5s
RETRY_BACKOFF_MAX=300s
```

## Benefits

1. **Fault Tolerance:** Jobs survive process crashes
2. **Scalability:** Add more workers to increase throughput
3. **Monitoring:** Real-time visibility into job progress
4. **Resource Efficiency:** Short-lived connections, better DB connection pooling
5. **Flexibility:** Pause/resume, priority queues, scheduled jobs
6. **Debugging:** Inspect failed jobs, replay from DLQ

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| RabbitMQ downtime | High | Implement fallback to synchronous processing |
| Message loss | Medium | Use persistent messages + publisher confirms |
| Queue buildup | Medium | Monitor queue depth, auto-scale workers |
| Duplicate processing | Low | Idempotency keys + database constraints |
| Memory pressure | Low | Set TTL, max queue length, DLQ |

## Success Criteria

1. **Reliability:** 99.9% job completion rate
2. **Performance:** 10x throughput improvement over synchronous processing
3. **Observability:** Real-time dashboards for job metrics
4. **Maintainability:** Clean separation of concerns, testable code
5. **Backward Compatibility:** Feature flag allows gradual rollout

## Next Steps

1. Review and approve this design
2. Create Jira subtasks for implementation
3. Set up RabbitMQ infrastructure
4. Implement producer (Phase 2)
5. Implement consumer (Phase 3)
6. Testing and rollout (Phase 4)

---

**Document Version:** 1.0  
**Author:** AI Assistant  
**Date:** $(date)  
**Status:** Draft - Pending Review