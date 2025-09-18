package coinbase

import (
	"context"
	"crypto/hmac"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"strconv"
	jwt "github.com/golang-jwt/jwt/v5"
	"math/rand"
	"sync"
)

const baseURL = "https://api.coinbase.com"

// Client minimal Advanced Trade API client

type Client struct {
	apiKey     string
	apiSecret  string
	passphrase string
	httpClient *http.Client
	jwtKeyName    string
	jwtPrivateKey *ecdsa.PrivateKey
	// rate limiting and retry
	rpm         int
	interval    time.Duration
	mu          sync.Mutex
	lastReqAt   time.Time
	maxRetries  int
	backoffBase time.Duration
}

// GetCandlesOnce fetches candles for a single sub-range [start,end] with an optional limit (max 350 per API docs).
// It does not iterate over the full requested range.
func (c *Client) GetCandlesOnce(ctx context.Context, productID string, start, end time.Time, granularity string, limit int64) ([]Candle, error) {
    path := fmt.Sprintf("/api/v3/brokerage/market/products/%s/candles", url.PathEscape(productID))
    if limit <= 0 || limit > 350 {
        limit = 350
    }
    q := url.Values{}
    q.Set("start", strconv.FormatInt(start.UTC().Unix(), 10))
    q.Set("end", strconv.FormatInt(end.UTC().Unix(), 10))
    q.Set("granularity", mapGranularity(granularity))
    q.Set("limit", strconv.FormatInt(limit, 10))
    resp, err := c.do(ctx, http.MethodGet, path, q, "")
    if err != nil {
        return nil, err
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        return nil, fmt.Errorf("coinbase http %d: %s", resp.StatusCode, string(b))
    }
    var payload struct {
        Candles []struct {
            Start   string  `json:"start"`
            Low     string  `json:"low"`
            High    string  `json:"high"`
            Open    string  `json:"open"`
            Close   string  `json:"close"`
            Volume  string  `json:"volume"`
        } `json:"candles"`
    }
    dec := json.NewDecoder(resp.Body)
    if err := dec.Decode(&payload); err != nil {
        resp.Body.Close()
        return nil, fmt.Errorf("decode candles: %w", err)
    }
    resp.Body.Close()
    // Ensure ascending order
    out := make([]Candle, 0, len(payload.Candles))
    for i := len(payload.Candles) - 1; i >= 0; i-- {
        cnd := payload.Candles[i]
        ts := parseStartTime(cnd.Start)
        out = append(out, Candle{
            Time:   ts,
            Open:   parseFloat(cnd.Open),
            High:   parseFloat(cnd.High),
            Low:    parseFloat(cnd.Low),
            Close:  parseFloat(cnd.Close),
            Volume: parseFloat(cnd.Volume),
        })
    }
    return out, nil
}

// Configure sets rate limiting and retry/backoff settings.
// rpm <= 0 disables rate limiting. maxRetries defaults to 3 when <= 0. backoffMs defaults to 500 when <= 0.
func (c *Client) Configure(rpm, maxRetries, backoffMs int) {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if backoffMs <= 0 {
		backoffMs = 500
	}
	c.maxRetries = maxRetries
	c.backoffBase = time.Duration(backoffMs) * time.Millisecond
	c.rpm = rpm
	if rpm > 0 {
		// minimum interval between requests
		c.interval = time.Minute / time.Duration(rpm)
	} else {
		c.interval = 0
	}
}

func (c *Client) beforeRequest() {
	if c.interval <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.lastReqAt.IsZero() {
		c.lastReqAt = now
		return
	}
	elapsed := now.Sub(c.lastReqAt)
	if elapsed < c.interval {
		time.Sleep(c.interval - elapsed)
		c.lastReqAt = time.Now()
	} else {
		c.lastReqAt = now
	}
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	// Retry on 429 and 5xx, and on transient network errors
	attempts := c.maxRetries
	if attempts <= 0 {
		attempts = 3
	}
	var resp *http.Response
	var err error
	for i := 0; i < attempts; i++ {
		c.beforeRequest()
		resp, err = c.httpClient.Do(req)
		if err != nil {
			// network error -> backoff and retry
			if i < attempts-1 {
				c.sleepBackoff(i)
				continue
			}
			return nil, err
		}
		// If success or non-retriable
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}
		// Retriable status codes
		if i < attempts-1 {
			// Drain and close body before retry to avoid leaks
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			c.sleepBackoff(i)
			continue
		}
		// Out of retries
		return resp, nil
	}
	// should not reach
	return resp, err
}

func (c *Client) sleepBackoff(attempt int) {
	base := c.backoffBase
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	// Exponential backoff with jitter
	d := base * time.Duration(1<<attempt)
	jitter := time.Duration(rand.Int63n(int64(base / 2)))
	time.Sleep(d + jitter)
}

// doPublic performs a request without any auth headers. Use for public endpoints.
func (c *Client) doPublic(ctx context.Context, method, path string, query url.Values, body string) (*http.Response, error) {
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	return c.doRequest(req)
}

func NewClient(apiKey, apiSecret, passphrase string) *Client {
	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		passphrase: passphrase,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewClientWithJWT creates a client that uses JWT bearer tokens.
// keyName is the COINBASE_API_KEY_NAME (e.g., organizations/.../apiKeys/...).
// privateKeyPEM is the EC private key in PEM format. It may contain literal \n sequences; they will be converted.
func NewClientWithJWT(keyName, privateKeyPEM string) (*Client, error) {
	if privateKeyPEM == "" || keyName == "" {
		return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}}, nil
	}
	// Normalize escaped newlines
	normalized := strings.ReplaceAll(privateKeyPEM, "\\n", "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("invalid EC private key PEM")
	}
	pk, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse EC private key: %w", err)
	}
	return &Client{jwtKeyName: keyName, jwtPrivateKey: pk, httpClient: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) bearerToken() (string, error) {
	if c.jwtPrivateKey == nil || c.jwtKeyName == "" {
		return "", nil
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss": c.jwtKeyName,
		"sub": c.jwtKeyName,
		"aud": baseURL,
		"iat": now.Unix(),
		"exp": now.Add(2 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	// Set kid header to key name per Coinbase JWT examples
	token.Header["kid"] = c.jwtKeyName
	return token.SignedString(c.jwtPrivateKey)
}

func (c *Client) sign(ts, method, path, body string) (string, error) {
	// Coinbase Advanced Trade uses base64-decoded secret, HMAC SHA256 over prehash
	secret, err := base64.StdEncoding.DecodeString(c.apiSecret)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	prehash := ts + method + path + body
	h := hmac.New(sha256.New, secret)
	_, _ = io.WriteString(h, prehash)
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body string) (*http.Response, error) {
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Prefer JWT if configured
	if tok, err := c.bearerToken(); err == nil && tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	} else if c.apiKey != "" && c.apiSecret != "" {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		sig, err := c.sign(ts, method, path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("CB-ACCESS-KEY", c.apiKey)
		req.Header.Set("CB-ACCESS-SIGN", sig)
		req.Header.Set("CB-ACCESS-TIMESTAMP", ts)
		if c.passphrase != "" {
			req.Header.Set("CB-ACCESS-PASSPHRASE", c.passphrase)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	return c.doRequest(req)
}

// GET /api/v3/brokerage/market/products/{product_id}/candles
func (c *Client) GetCandles(ctx context.Context, productID string, start, end time.Time, granularity string) ([]Candle, error) {
    path := fmt.Sprintf("/api/v3/brokerage/market/products/%s/candles", url.PathEscape(productID))
    secPerBucket := bucketSeconds(granularity)
    maxBuckets := int64(350)
    var out []Candle
    var lastTime time.Time
    // Iterate over the requested time window in chunks of up to 350 buckets
    cursor := start.UTC()
    endUTC := end.UTC()
    for cursor.Before(endUTC) {
        // compute sub-window end
        windowEnd := cursor.Add(time.Duration(secPerBucket*maxBuckets) * time.Second)
        if windowEnd.After(endUTC) {
            windowEnd = endUTC
        }
        q := url.Values{}
        q.Set("start", strconv.FormatInt(cursor.Unix(), 10))
        q.Set("end", strconv.FormatInt(windowEnd.Unix(), 10))
        q.Set("granularity", mapGranularity(granularity))
        q.Set("limit", strconv.FormatInt(maxBuckets, 10))
        // Authenticated request (JWT if configured) as per Coinbase docs
        resp, err := c.do(ctx, http.MethodGet, path, q, "")
        if err != nil {
            return nil, err
        }
        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            b, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            return nil, fmt.Errorf("coinbase http %d: %s", resp.StatusCode, string(b))
        }
        var payload struct {
            Candles []struct {
                Start   string  `json:"start"`
                Low     string  `json:"low"`
                High    string  `json:"high"`
                Open    string  `json:"open"`
                Close   string  `json:"close"`
                Volume  string  `json:"volume"`
            } `json:"candles"`
        }
        dec := json.NewDecoder(resp.Body)
        if err := dec.Decode(&payload); err != nil {
            resp.Body.Close()
            return nil, fmt.Errorf("decode candles: %w", err)
        }
        resp.Body.Close()
        // Append, parsing start which may be epoch seconds
        if len(payload.Candles) == 0 {
            // No more data; break to avoid tight loop
            break
        }
        // The API may return candles in reverse chronological order; process ascending
        for i := len(payload.Candles) - 1; i >= 0; i-- {
            cnd := payload.Candles[i]
            ts := parseStartTime(cnd.Start)
            if !lastTime.IsZero() && (ts.Equal(lastTime) || ts.Before(lastTime)) {
                continue
            }
            open := parseFloat(cnd.Open)
            high := parseFloat(cnd.High)
            low := parseFloat(cnd.Low)
            closep := parseFloat(cnd.Close)
            vol := parseFloat(cnd.Volume)
            out = append(out, Candle{Time: ts, Open: open, High: high, Low: low, Close: closep, Volume: vol})
            lastTime = ts
        }
        // Move cursor forward by the sub-window size
        cursor = windowEnd
    }
    return out, nil
}

func mapGranularity(g string) string {
    switch strings.ToLower(g) {
    case "1m", "1min", "one_minute":
        return "ONE_MINUTE"
    case "5m", "5min", "five_minute":
        return "FIVE_MINUTE"
    case "15m", "15min", "fifteen_minute":
        return "FIFTEEN_MINUTE"
    case "30m", "thirty_minute":
        return "THIRTY_MINUTE"
    case "1h", "60m", "one_hour":
        return "ONE_HOUR"
    case "2h", "two_hour":
        return "TWO_HOUR"
    case "6h", "six_hour":
        return "SIX_HOUR"
    case "1d", "one_day", "24h":
        return "ONE_DAY"
    default:
        return "ONE_HOUR"
    }
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func bucketSeconds(g string) int64 {
	switch strings.ToLower(g) {
	case "1m", "1min", "one_minute":
		return 60
	case "5m", "5min", "five_minute":
		return 5 * 60
	case "15m", "15min", "fifteen_minute":
		return 15 * 60
	case "30m", "thirty_minute":
		return 30 * 60
	case "1h", "60m", "one_hour":
		return 60 * 60
	case "2h", "two_hour":
		return 2 * 60 * 60
	case "6h", "six_hour":
		return 6 * 60 * 60
	case "1d", "one_day", "24h":
		return 24 * 60 * 60
	default:
		return 60 * 60
	}
}

func parseStartTime(s string) time.Time {
	// Try UNIX seconds first
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0).UTC()
	}
	// Fallback to RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
