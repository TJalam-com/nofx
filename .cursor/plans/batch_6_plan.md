# Plan: Standardize Code Comments Batch 6

## Progress Summary

**Completed: 24 files**
- Batch 1: api/backtest.go, api/crypto_handler.go, api/register_otp_test.go, api/server.go (partial), api/server_test.go
- Batch 2: api/traderid_test.go, api/utils.go, api/utils_test.go, auth/auth.go
- Batch 3: backtest/account.go, backtest/ai_client.go, backtest/aicache.go, backtest/config.go, backtest/datafeed.go
- Batch 4: backtest/equity.go, backtest/lock.go, backtest/manager.go, backtest/metrics.go, backtest/runner.go
- Batch 5: backtest/storage.go, backtest/types.go, config/config.go, crypto/crypto.go, decision/engine.go

**Total files in commit: 103 files**
**Remaining: 79 files** (excluding api/strategy.go and decision/strategy_engine.go which don't exist)

## Files to Translate (Batch 6 - Next 10 files)

1. **[decision/prompt_manager.go](decision/prompt_manager.go)** - 50 Chinese occurrences
   - Prompt template management functions and documentation

2. **[decision/prompt_manager_test.go](decision/prompt_manager_test.go)** - 77 Chinese occurrences
   - Test names, comments, and messages for prompt manager tests

3. **[decision/prompt_reload_integration_test.go](decision/prompt_reload_integration_test.go)** - 77 Chinese occurrences
   - Integration test names, comments, and messages for prompt reload functionality

4. **[decision/prompt_test.go](decision/prompt_test.go)** - 8 Chinese occurrences
   - Test names and comments for prompt tests

5. **[decision/validate_test.go](decision/validate_test.go)** - 48 Chinese occurrences
   - Test names, comments, and messages for decision validation tests

6. **[hook/http_client_hook.go](hook/http_client_hook.go)** - 1 Chinese occurrence
   - HTTP client hook documentation

7. **[hook/ip_hook.go](hook/ip_hook.go)** - 1 Chinese occurrence
   - IP hook documentation

8. **[hook/trader_hook.go](hook/trader_hook.go)** - 2 Chinese occurrences
   - Trader hook documentation

9. **[logger/config.go](logger/config.go)** - 15 Chinese occurrences
   - Logger configuration comments and documentation

10. **[logger/logger.go](logger/logger.go)** - 26 Chinese occurrences
    - Logger implementation comments and documentation

## Implementation Approach

1. Translate all Chinese text to English following patterns from commit `a12c0ae8c9a5aad6e6f28edbf1a292045959da89`
2. Preserve code structure and functionality
3. Maintain consistent terminology across files
4. For test files, translate test names, comments, and error messages

## Notes

- Test files (prompt_manager_test.go, prompt_reload_integration_test.go, prompt_test.go, validate_test.go) will have many Chinese test names and comments
- Hook files are small with minimal Chinese text
- Logger files have moderate amounts of Chinese comments
- All translations should match the exact English equivalents from the reference commit

