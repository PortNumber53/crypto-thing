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
