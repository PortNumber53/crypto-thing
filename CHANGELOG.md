# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2025-09-03
- Initialize Go module and CLI `cryptool` with Cobra
- Add configuration loader reading `~/.config/crypto-thing/config.ini`
- Add PostgreSQL integration and embedded migrations using `goose`
- Create initial schema with `candles` table
- Add `migrate` commands: `status`, `up`, `down`, `reset`
- Add Coinbase Advanced Trade client scaffolding and `exchange coinbase data fetch` command

## [0.2.0] - 2025-09-04
- Coinbase client: add configurable rate limiting (RPM) and retry with exponential backoff (MaxRetries, BackoffMS)
- Candle fetch: implement batching across long date ranges using `limit` (up to 350) and sub-range iteration, with boundary deduplication and ascending ordering
- Prefer JWT auth when `COINBASE_API_KEY_NAME` and `COINBASE_API_PRIVATE_KEY` provided; fallback to HMAC or public

## [0.3.0] - 2025-09-17
- Add comprehensive `README.md` with installation, configuration, and usage instructions.

## [0.4.0] - 2025-09-18
- **Feature**: Add `exchange coinbase wallet syncdown` command to fetch and display account balances.
- **Feature**: Add `--coinbase-creds` flag to load API credentials directly from a JSON file.
- **Feature**: Add persistent `--verbose` (`-v`) flag for detailed debug output.
- **Fix**: Corrected JWT generation for Coinbase Advanced Trade API to resolve authentication errors.

## [0.5.0] - 2025-09-18
- **Refactor**: Refactored `exchange coinbase data fetch` command for robustness and efficiency.
- **Feature**: The `data fetch` command now validates product IDs against the Coinbase API before fetching.
- **Feature**: Restored gap-filling logic to `data fetch`, which now intelligently skips already-downloaded time windows, improving efficiency.
- **Fix**: Corrected an issue where `data fetch` would stop prematurely if a time window had no trading activity, ensuring the full date range is processed.

## [0.5.1] - 2025-09-18
- **Fix**: Resolved mixed package names (`root` vs `rootroot`) in `cmd/cryptool/root/` that caused build failures and misleading "missing metadata for import" errors.
- **Fix**: Removed invalid import of `cryptool/cmd/cryptool/root/subcmds` and referenced local constructors (`NewMigrateCmd`, `NewExchangeCmd`) directly in `cmd/cryptool/root/root.go`.

## [0.5.2] - 2025-09-18
- **Feature**: The `data fetch` command now reports the number of new candles inserted after each batch, providing better feedback on data ingestion.

## [0.8.0] - 2025-09-18
- **Feature**: Re-implemented the `data fetch` command with a highly efficient, recursive binary-search strategy. Instead of scanning linearly, the command now quickly identifies large gaps in historical data and fills them with the maximum number of candles per request (350), dramatically reducing the number of API calls and speeding up backfills.

## [0.7.3] - 2025-09-18
- **Fix**: Corrected a bug in the `data fetch` command where the time window for batch requests was calculated incorrectly, causing data to be fetched one candle at a time. The command now correctly fetches large batches (up to 350 candles), significantly improving performance.
- **Docs**: Improved the help text for the `--granularity` flag to be more descriptive.

## [0.7.2] - 2025-09-18
- **Docs**: Improved the help summaries (`--help`) for all commands to be more descriptive and accurately reflect their functionality.

## [0.7.1] - 2025-09-18
- **Refactor**: Removed unused `products` key from the application configuration (`config.ini`) and corresponding code to reduce confusion and simplify the setup.

## [0.7.0] - 2025-09-18
- **Feature**: Implemented a smart gap-filling feature for the `data fetch` command. The `start-date` and `end-date` arguments are now optional, defaulting to the product's creation date and the current time, respectively. The command now intelligently identifies and fills any gaps in the historical data.

## [0.6.0] - 2025-09-18
- **Feature**: Added `exchange coinbase data sync-products` command to fetch and store detailed product information from Coinbase.
- **Feature**: Added a new database migration (`0004_add_product_details_to_products_table.sql`) to expand the `products` table to store the full product data from the Coinbase API.
- **Refactor**: Updated the `Product` struct in `internal/coinbase/models.go` to match the full API response.
- **Refactor**: Added `UpsertProducts` function to `internal/ingest/store.go` to handle the database logic for syncing products.
