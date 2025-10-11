# Changelog

All notable changes to this project will be documented in this file.

## [0.9.4] - 2025-09-24
- **Feature(coinbase):** Added a new `exchange coinbase history` command that iterates through all tradable products and fetches their complete 1-minute candle history. This automates the process of backfilling data for the entire exchange, using the same robust gap-filling logic as the `fetch` command.

## [0.9.3] - 2025-09-19
- **Fix(coinbase):** Corrected an off-by-one error in the `data fetch` command that could cause it to request more than the maximum of 350 candles from the Coinbase API. The window-sizing logic is now stricter, ensuring all batch requests respect the API's limit and preventing `INVALID_ARGUMENT` errors.

## [0.9.2] - 2025-09-19
- **Perf(coinbase):** Implemented a worker pool in the `data fetch` command to parallelize the process of marking data gaps. After a bulk fetch, any remaining missing timestamps are now concurrently inserted as "fake" candles by a pool of goroutines, significantly speeding up the process when many gaps are found.

## [0.9.1] - 2025-09-19
- **Perf(coinbase):** Implemented a worker pool in the `data fetch` command to parallelize the fetching and processing of missing candles. This significantly speeds up the process of filling large data gaps by concurrently handling multiple API requests.

## [0.9.0] - 2025-09-19
- **Refactor(coinbase):** Overhauled the `data fetch` command's gap-filling logic for precision and efficiency. Instead of fetching large time windows and inferring gaps, the command now first queries the local database to get a precise list of missing timestamps. It then makes targeted, single-candle API requests for each of those timestamps. This surgical approach resolves several issues: it prevents the incorrect marking of real data, it is more efficient with the API, and it correctly handles empty responses, which finally resolves the long-standing issue of the command getting stuck in a loop.

## [0.8.6] - 2025-09-19
- **Fix(coinbase):** Resolved an issue where the `data fetch` command would get stuck in a loop, repeatedly trying to fetch data that was already present in the database. The gap-detection logic has been corrected to accurately identify only the timestamps that are genuinely missing or need to be retried.

## [0.8.5] - 2025-09-19
- **Fix(coinbase):** Corrected an off-by-one error in the `data fetch` command that caused it to request an extra candle from the Coinbase API. This fix aligns the application's time window with the API's behavior, preventing unnecessary fetches and inaccurate log messages.

## [0.8.4] - 2025-09-19
- **Fix(coinbase):** Prevented `data fetch` from corrupting real candle data. The gap-filling logic was incorrectly overwriting existing candles with "fake" gap markers. The fix ensures that a candle is only marked as a gap if it is already a fake candle, preserving data integrity.

## [0.8.3] - 2025-09-19
- **Perf(coinbase):** Optimized gap detection in `data fetch` by using a dedicated SQL query (`CountGapsToFill`) to identify only actionable gaps. This avoids fetching and processing data for time ranges that are already complete or have been permanently marked as empty (`fake_fill_count >= 5`), significantly improving efficiency and reducing unnecessary API calls.

## [0.8.2] - 2025-09-19
- **Fix(coinbase):** Robustly fill data gaps in `data fetch` command. The previous implementation failed to create placeholder "fake" candles when the Coinbase API returned a partial dataset for a requested time window. The logic now iterates through the expected time range, compares it against the received candles, and inserts placeholders for any missing timestamps. This ensures that intermittent data gaps from the source are correctly tracked and can be re-queried later.

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

## [0.8.1] - 2025-09-18
- **Fix**: Corrected an issue where the `sync-products` command only fetched the first page of results from the Coinbase API, leading to an incomplete list of products in the local database. The command now properly paginates through all results, ensuring all tradeable products are synced.

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
