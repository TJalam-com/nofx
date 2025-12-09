# Plan: Complete Remaining Batch 9 Files

## Overview
Complete translation of Chinese comments and strings to English for the remaining 5 files from Batch 9, following the translation patterns from commit `a12c0ae8c9a5aad6e6f28edbf1a292045959da89`.

## Files to Translate

### 1. trader/auto_trader_test.go (150 Chinese characters)
- Translate test suite comments and descriptions
- Translate test function names and comments
- Translate mock setup comments
- Examples:
  - "使用 testify/suite 进行结构化测试" → "Using testify/suite for structured testing"
  - "测试对象" → "Test object"
  - "Mock 依赖" → "Mock dependencies"
  - "测试配置" → "Test configuration"

### 2. trader/order_sync.go (9 Chinese characters)
- Translate struct and function comments
- Translate error messages
- Examples:
  - "订单同步管理器" → "Order sync manager"
  - "创建订单同步管理器" → "Create order sync manager"
  - "不支持的交易所" → "Unsupported exchange"

### 3. trader/position_sync.go (8 Chinese characters)
- Translate struct and function comments
- Translate error messages
- Examples:
  - "持仓同步管理器" → "Position sync manager"
  - "创建持仓同步管理器" → "Create position sync manager"
  - "不支持的交易所" → "Unsupported exchange"

### 4. trader/bybit_trader.go (123 Chinese characters)
- Translate struct comments and field descriptions
- Translate function comments
- Translate log messages and error messages
- Examples:
  - "Bybit USDT 永續合約交易器" → "Bybit USDT Perpetual Futures Trader"
  - "创建 Bybit 交易器" → "Create Bybit trader"
  - "余额缓存" → "Balance cache"
  - "持仓缓存" → "Position cache"
  - "获取账户余额" → "Get account balance"
  - "交易器已初始化" → "Trader initialized"

### 5. trader/bybit_trader_test.go (52 Chinese characters)
- Translate test suite comments
- Translate test function names and descriptions
- Translate mock setup comments
- Examples:
  - "Bybit交易器测试套件" → "Bybit trader test suite"
  - "创建 Bybit 测试套件" → "Create Bybit test suite"

## Translation Guidelines

1. Follow patterns from commit `a12c0ae8c9a5aad6e6f28edbf1a292045959da89`
2. Preserve all functionality - only translate text
3. Maintain consistent terminology:
   - "交易器" → "trader"
   - "交易所" → "exchange"
   - "余额" → "balance"
   - "持仓" → "position"
   - "订单" → "order"
   - "缓存" → "cache"
   - "获取" → "get"
   - "创建" → "create"
4. For test files: Translate test names, comments, and assertion messages
5. For error messages: Use clear, descriptive English error messages

## Implementation Order

1. Start with smaller files (order_sync.go, position_sync.go) - 17 chars total
2. Then bybit_trader.go (123 chars)
3. Then test files (auto_trader_test.go, bybit_trader_test.go) - 202 chars total

## Expected Outcome

- All 5 files fully translated
- No Chinese characters remaining in these files
- All functionality preserved
- No linter errors introduced
- Consistent with previous translation batches

