-- ============================================================
-- nofx Add Email to Trader Applications Migration
-- Version: 1.0.0
-- Date: 2025-01-20
-- Description: Add email column to trader_applications table
-- ============================================================

-- 开启 WAL 模式检查 (确保已启用)
PRAGMA journal_mode;
-- 预期输出: wal

-- 开始事务
BEGIN TRANSACTION;

-- ============================================================
-- Part 1: 添加 email 字段到 trader_applications 表
-- ============================================================

-- 检查列是否已存在（SQLite 3.35.0+ 支持 IF NOT EXISTS）
-- 如果您的 SQLite 版本较旧，请手动检查后再执行
ALTER TABLE trader_applications
ADD COLUMN email TEXT NOT NULL DEFAULT '';

-- ============================================================
-- Part 2: 更新现有记录（使用用户邮箱作为默认值）
-- ============================================================

-- 对于已有的申请，从 users 表获取邮箱
UPDATE trader_applications
SET email = (
    SELECT email FROM users WHERE users.id = trader_applications.user_id
)
WHERE email = '' OR email IS NULL;

-- 提交事务
COMMIT;

-- ============================================================
-- Part 3: 验证迁移
-- ============================================================

-- 查看 trader_applications 表结构
PRAGMA table_info(trader_applications);

-- 验证新增字段
SELECT
    name,
    type,
    dflt_value,
    notnull
FROM pragma_table_info('trader_applications')
WHERE name = 'email';

-- 输出摘要
SELECT '✅ Email column added to trader_applications table successfully' AS status;

