# cryptool

`cryptool` is a command-line tool for managing cryptocurrency data, including database migrations and fetching historical data from Coinbase.

## Installation

To build and install the tool, run the following command from the project root:

```bash
go install ./cmd/cryptool
```

## Configuration

`cryptool` requires a configuration file located at `~/.config/crypto-thing/config.ini`. You can also specify a different path using the `--config` flag.

Here is an example `config.ini` file:

```ini
[database]
url = postgres://user:password@host:port/dbname?sslmode=disable

[coinbase]
# HMAC credentials
api_key = <your_api_key>
api_secret = <your_api_secret>
passphrase = <your_passphrase>

# JWT credentials (preferred)
api_key_name = <your_api_key_name>
api_private_key = <your_api_private_key>

# Rate limiting (optional)
rpm = 30
max_retries = 5
backoff_ms = 1000

[app]
products = BTC-USD,ETH-USD
```

## Usage

### Database Migrations

The `migrate` command manages the database schema.

**Show migration status:**

```bash
cryptool migrate status
```

**Apply all pending migrations:**

```bash
cryptool migrate up
```

**Rollback the last migration:**

```bash
cryptool migrate down
```

**Rollback a specific number of migrations:**

```bash
cryptool migrate down --step 2
```

**Reset the database (rolls back all migrations, then applies them again):**

```bash
cryptool migrate reset
```

### Fetch Coinbase Data

The `exchange coinbase data fetch` command fetches historical candle data from Coinbase and stores it in the database.

**Fetch data for a specific product and date range:**

```bash
cryptool exchange coinbase data fetch 2023-01-01 2023-01-31 --product BTC-USD
```

**Arguments:**

*   `<start-date>`: The start date in `YYYY-MM-DD` or RFC3339 format.
*   `<end-date>`: The end date in `YYYY-MM-DD` or RFC3339 format.

**Flags:**

*   `--product` (required): The product ID (e.g., `BTC-USD`).
*   `--granularity` (optional): The candle granularity. Can be `1m`, `5m`, `15m`, `30m`, `1h`, `2h`, `6h`, or `1d`. Defaults to `1h`.
