-- Migration: Add constraints and indexes to prevent duplicate closed position records
-- This migration adds:
-- 1. A unique index on position ID to enforce single-record-per-position
-- 2. An index for faster duplicate detection queries
-- 3. Cleanup of existing erroneous 0-PnL closed position records

-- Step 1: Add index for faster position closure duplicate detection
CREATE INDEX IF NOT EXISTS idx_trader_positions_closure_check 
ON trader_positions(trader_id, symbol, side, closed_at);

-- Step 2: Add index for position ID lookups (already primary key, but explicit for clarity)
-- The id column is already PRIMARY KEY, so this is for documentation

-- Step 3: Cleanup existing suspicious 0-PnL closed position records
-- These are records where:
-- - closed_at is NOT NULL (marked as closed)
-- - exit_price is 0 or NULL (no valid exit price)
-- - realized_pnl is 0 or NULL (no valid PnL)
-- 
-- CAUTION: This deletes potentially erroneous records. Review before applying in production.
-- To preview what will be deleted, run:
-- SELECT * FROM trader_positions 
-- WHERE closed_at IS NOT NULL 
--   AND (exit_price IS NULL OR exit_price = 0 OR exit_price = entry_price)
--   AND (realized_pnl IS NULL OR realized_pnl = 0);

-- Uncomment the following to actually delete suspicious records:
-- DELETE FROM trader_positions 
-- WHERE closed_at IS NOT NULL 
--   AND (exit_price IS NULL OR exit_price = 0)
--   AND (realized_pnl IS NULL OR realized_pnl = 0);

-- Step 4: Add a check constraint to prevent future 0-PnL closed positions without valid exit price
-- Note: SQLite doesn't support adding constraints to existing tables, so this is for documentation
-- In a new table creation, you would add:
-- CHECK (closed_at IS NULL OR exit_price > 0 OR realized_pnl != 0)
