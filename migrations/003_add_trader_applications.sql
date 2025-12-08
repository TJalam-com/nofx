-- ============================================================
-- nofx Trader Applications 支持迁移
-- Version: 1.0.0
-- Date: 2025-01-20
-- Description: 添加 trader_applications 表支持交易员申请功能
-- ============================================================

-- 开启 WAL 模式检查 (确保已启用)
PRAGMA journal_mode;
-- 预期输出: wal

-- 开始事务
BEGIN TRANSACTION;

-- ============================================================
-- Part 1: 创建 trader_applications 表
-- ============================================================

CREATE TABLE IF NOT EXISTS trader_applications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    description TEXT NOT NULL,
    trading_experience TEXT NOT NULL,
    strategy_overview TEXT NOT NULL,
    social_links TEXT, -- JSON string
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'rejected'
    admin_notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============================================================
-- Part 2: 创建索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_trader_applications_user_id ON trader_applications(user_id);
CREATE INDEX IF NOT EXISTS idx_trader_applications_status ON trader_applications(status);

-- 提交事务
COMMIT;

-- ============================================================
-- Part 3: 验证迁移
-- ============================================================

-- 查看 trader_applications 表结构
PRAGMA table_info(trader_applications);

-- 输出摘要
SELECT '✅ Trader applications table created successfully' AS status;

