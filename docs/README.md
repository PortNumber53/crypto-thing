# cryptool

`cryptool` is a command-line tool for managing cryptocurrency data, including database migrations and fetching historical data from Coinbase.

## Running the Tool

You can run the tool directly from the source code using `go run`:

```bash
go run cryptool.go [command]
```

Alternatively, you can build and install the binary to your system's `GOPATH`:

```bash
go install .
```

After installation, you can run the tool directly:

```bash
cryptool [command]
```

*The examples below use `go run`. If you have installed the tool, you can replace `go run cryptool.go` with `cryptool`.*

## Configuration

`cryptool` requires a configuration file located at `~/.config/crypto-thing/config.ini`. You can also specify a different path using the global `--config` flag.

The configuration can be structured in multiple ways for flexibility. The tool supports a `[default]` section for common key-value pairs, which is useful for environments that share settings with other tools.

### Example `config.ini`

Here is an example using a `[default]` section, which is the recommended approach:

```ini
[default]
# Database Connection
DB_HOST=localhost
DB_PORT=5432
DB_NAME=cryptool
DB_USER=myuser
DB_PASSWORD=mypassword
DB_SSLMODE=disable

# Coinbase Advanced Trade API Credentials (JWT)
# These are the preferred credentials for authentication.
COINBASE_CLOUD_API_KEY_NAME="organizations/<org_id>/apiKeys/<api_key_id>"
COINBASE_CLOUD_API_SECRET="-----BEGIN EC PRIVATE KEY-----\n<your_private_key>\n-----END EC PRIVATE KEY-----"

# Rate Limiting (optional)
COINBASE_RPM = 30
COINBASE_MAX_RETRIES = 5
COINBASE_BACKOFF_MS = 1000
```

### Alternative Configuration

The tool also supports older configuration formats with `[database]` and `[coinbase]` sections for backward compatibility. However, using the `[default]` section is encouraged.

### JWT Credentials File

For enhanced security, you can store your Coinbase JWT credentials in a separate JSON file (e.g., `cdp_api_key.json`) and provide the path via the `--coinbase-creds` flag on the command line. This method overrides any credentials set in `config.ini`.

**Example `cdp_api_key.json`:**
```json
{
  "name": "organizations/<org_id>/apiKeys/<api_key_id>",
  "privateKey": "-----BEGIN EC PRIVATE KEY-----\n<your_private_key>\n-----END EC PRIVATE KEY-----"
}
```

**Usage with credentials file:**
```bash
go run cryptool.go exchange coinbase data fetch --coinbase-creds /path/to/cdp_api_key.json
```

## Usage

### Database Migrations

The `migrate` command manages the database schema.

**Show migration status:**

```bash
go run cryptool.go migrate status
```

**Apply all pending migrations:**

```bash
go run cryptool.go migrate up
```

**Rollback the last migration:**

```bash
go run cryptool.go migrate down
```

**Rollback a specific number of migrations:**

```bash
go run cryptool.go migrate down --step 2
```

**Reset the database (rolls back all migrations, then applies them again):**

```bash
go run cryptool.go migrate reset
```

### Fetch Coinbase Data

The `exchange coinbase data fetch` command fetches historical candle data from Coinbase and stores it in the database.

**Smart Fetch (Recommended):**

To automatically fetch all missing data for a product, you can run the command without date arguments. This will fill all gaps from the product's launch date to the current time.

```bash
go run cryptool.go exchange coinbase data fetch --product BTC-USD
```

**Fetch data for a specific date range:**

```bash
go run cryptool.go exchange coinbase data fetch 2023-01-01 2023-01-31 --product BTC-USD
```

**Arguments:**

*   `[start-date]` (optional): The start date in `YYYY-MM-DD` or RFC3339 format. Defaults to the product's launch date.
*   `[end-date]` (optional): The end date in `YYYY-MM-DD` or RFC3339 format. Defaults to the current time.

**Flags:**

*   `--product` (required): The product ID (e.g., `BTC-USD`).
*   `--granularity` (optional): The candle granularity. Can be `1m`, `5m`, `15m`, `30m`, `1h`, `2h`, `6h`, or `1d`. Defaults to `1h`.
