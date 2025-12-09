# Plan: Standardize Code Comments Batch 9

## Progress Summary

- Completed: 54 files (Batches 1–8)
- Remaining with Chinese text: api/server.go (in-progress earlier but not finished), mcp/examples_test.go, config/db, multiple trader files, and various web/docs (out of scope for this batch).

## Files to Translate (Batch 9 - Next 10 files)

1. **[api/server.go](api/server.go)** - Remaining Chinese comments/logs in trader handlers and config endpoints.
2. **[mcp/examples_test.go](mcp/examples_test.go)** - Chinese prompts/messages in examples; align with reference commit phrasing.
3. **[config/database.go](config/database.go)** - Chinese comments/errors in DB setup.
4. **[config/database_test.go](config/database_test.go)** - Chinese test names/messages.
5. **[trader/auto_trader.go](trader/auto_trader.go)** - Chinese logs/comments across trader orchestration.
6. **[trader/auto_trader_test.go](trader/auto_trader_test.go)** - Chinese test names/messages.
7. **[trader/order_sync.go](trader/order_sync.go)** - Chinese logs/comments in order syncing.
8. **[trader/position_sync.go](trader/position_sync.go)** - Chinese logs/comments in position syncing.
9. **[trader/bybit_trader.go](trader/bybit_trader.go)** - Chinese logs/comments in Bybit trader implementation.
10. **[trader/bybit_trader_test.go](trader/bybit_trader_test.go)** - Chinese test names/messages.

## Implementation Approach

1. Translate all Chinese comments/strings to English following commit `a12c0ae8c9a5aad6e6f28edbf1a292045959da89`.
2. Preserve functionality; only change text.
3. Keep terminology consistent with previous batches (trader, exchange, AI model, prompt).
4. For tests, update names/messages to English equivalents; for logs/errors, mirror reference phrasing.

## Notes

- Start with `api/server.go` (largest surface), then proceed in listed order.
- If a file has no Chinese text upon inspection, mark as reviewed and continue.

