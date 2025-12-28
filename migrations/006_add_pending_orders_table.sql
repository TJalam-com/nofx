-- Migration: Add pending_orders table for separate storage of unfilled orders
-- This prevents SL/TP/limit orders from appearing in closed positions or affecting realized PnL

-- Create pending_orders table
CREATE TABLE IF NOT EXISTS pending_orders (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    order_type TEXT NOT NULL,
    trigger_price REAL,
    quantity REAL NOT NULL,
    filled_quantity REAL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    parent_position_id TEXT,
    exchange_order_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    filled_at DATETIME,
    cancelled_at DATETIME,
    FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
);

-- Create indexes for pending_orders
CREATE INDEX IF NOT EXISTS idx_pending_orders_trader ON pending_orders(trader_id);
CREATE INDEX IF NOT EXISTS idx_pending_orders_symbol ON pending_orders(symbol);
CREATE INDEX IF NOT EXISTS idx_pending_orders_status ON pending_orders(status);
CREATE INDEX IF NOT EXISTS idx_pending_orders_parent ON pending_orders(parent_position_id);

-- Cleanup invalid equity history records that cause -100% PnL in competition chart
-- These are records where total_equity is 0 or negative, or PnL is <= -100%
DELETE FROM trader_equity_history 
WHERE total_equity <= 0 
   OR total_pnl_pct <= -100
   OR available_balance < 0;

-- Add comment: Order types
-- 'stop_loss' - Stop loss order
-- 'take_profit' - Take profit order  
-- 'limit' - Limit order
-- 'market' - Market order (typically filled immediately)

-- Add comment: Order statuses
-- 'pending' - Order is active and waiting to be filled
-- 'filled' - Order has been completely filled
-- 'partially_filled' - Order has been partially filled
-- 'cancelled' - Order was cancelled
-- 'rejected' - Order was rejected by exchange
