# Cleanup Trader Strategy Duplicates

This script cleans up duplicate strategy-related fields in the `traders` table for traders that have a `strategy_id` set.

## Purpose

When a trader has a `strategy_id`, all strategy-related settings should be loaded from the associated strategy (Strategy Studio is the single source of truth). This script removes any duplicate data that may have been stored in the trader record itself.

## What It Does

For all traders with a non-empty `strategy_id`, this script:
- Clears all strategy-related fields (sets them to default/empty values)
- Keeps only trader-specific fields (name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, followed_trader_id, show_in_competition, strategy_id)
- Creates a backup of the database before making changes

## Strategy-Related Fields That Are Cleared

- `btc_eth_leverage` → 0
- `altcoin_leverage` → 0
- `trading_symbols` → ''
- `custom_prompt` → ''
- `override_base_prompt` → 0
- `system_prompt_template` → ''
- `is_cross_margin` → 1 (default)
- `use_coin_pool` → 0
- `use_oi_top` → 0
- `use_tradingview` → 0
- `enable_raw_klines` → 1 (required field)
- `enable_ema` → 0
- `enable_macd` → 0
- `enable_rsi` → 0
- `enable_atr` → 0
- `enable_volume` → 1 (default)
- `enable_oi` → 1 (default)
- `enable_funding` → 1 (default)
- `indicator_timeframe` → ''
- `quant_data_url` → ''

## Usage

```bash
# Run with default database path (data/data.db)
go run scripts/cleanup_trader_strategy_duplicates/main.go

# Run with custom database path
go run scripts/cleanup_trader_strategy_duplicates/main.go /path/to/data.db
```

## Safety

- **Automatic Backup**: The script creates a backup file (`data.db.pre_cleanup_backup`) before making any changes
- **Transaction**: All updates are performed in a single transaction - if any error occurs, all changes are rolled back
- **Verification**: After running, verify that traders with `strategy_id` are correctly loading settings from their strategies

## After Running

1. Verify that traders with `strategy_id` are working correctly
2. Test that strategy settings are being loaded from Strategy Studio
3. Once verified, you can manually delete the backup file if desired

## Notes

- This script only affects traders that have a `strategy_id` set
- Traders without a `strategy_id` are not modified (they use custom configuration)
- The actual strategy values are loaded dynamically from the `strategies` table when `GetTraderConfig` is called

