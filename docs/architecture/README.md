# NOFX Architecture Documentation

Technical documentation for developers who want to understand NOFX internals.

---

## Overview

NOFX is a full-stack AI trading platform with:
- **Backend:** Go (Gin framework, SQLite)
- **Frontend:** React/TypeScript (Vite, TailwindCSS)
- **Architecture:** Microservice-inspired modular design

---

## Project Structure

```
nofx/
├── main.go                          # Program entry (multi-trader manager)
├── config.json                      # Configuration (now via web interface)
├── trading.db                       # SQLite database
│
├── api/                            # HTTP API service
│   ├── server.go                   # Gin framework RESTful API
│   ├── strategy.go                 # Strategy management endpoints
│   ├── webhook.go                  # Webhook handling
│   └── backtest.go                 # Backtest API endpoints
│
├── trader/                         # Trading execution layer
│   ├── auto_trader.go              # Main trading orchestrator
│   ├── interface.go                # Unified trader interface
│   ├── binance_futures.go          # Binance API wrapper
│   ├── bybit_trader.go             # Bybit API wrapper
│   ├── okx_trader.go               # OKX API wrapper
│   ├── hyperliquid_trader.go       # Hyperliquid DEX wrapper
│   ├── aster_trader.go             # Aster DEX wrapper
│   ├── lighter_trader_v2.go        # Lighter DEX wrapper (V2)
│   └── position.go                 # Position management
│
├── manager/                        # Multi-trader management
│   └── trader_manager.go           # Manages multiple trader instances
│
├── decision/                       # AI decision engine
│   ├── engine.go                   # Decision logic with historical feedback
│   └── prompt_manager.go           # Prompt template system
│
├── market/                         # Market data system
│   ├── data.go                     # Market data & technical indicators
│   ├── api_client.go               # Market data API client
│   ├── websocket_client.go         # WebSocket data streaming
│   ├── combined_streams.go         # Combined streaming interface
│   └── monitor.go                  # Market data cache
│
├── mcp/                            # Model Context Protocol - AI communication
│   ├── client.go                   # AI API client interface
│   ├── deepseek_client.go          # DeepSeek client
│   ├── qwen_client.go              # Qwen client
│   ├── openai_client.go            # OpenAI client
│   ├── claude_client.go            # Claude client
│   ├── gemini_client.go            # Gemini client
│   ├── grok_client.go              # Grok client
│   └── kimi_client.go              # Kimi client
│
├── config/                         # Configuration & database
│   ├── database.go                 # SQLite operations and schema
│   └── config.go                  # Configuration management
│
├── auth/                           # Authentication
│   └── auth.go                     # JWT token management & 2FA
│
├── backtest/                       # Backtesting system
│   ├── runner.go                   # Backtest execution engine
│   ├── manager.go                  # Backtest lifecycle management
│   ├── storage.go                  # Backtest data storage
│   └── metrics.go                  # Performance metrics calculation
│
├── bootstrap/                      # Module initialization framework
│   ├── bootstrap.go                # Bootstrap system
│   └── context.go                  # Initialization context
│
├── crypto/                         # Cryptography & encryption
│   ├── encryption.go               # Data encryption utilities
│   └── secure_storage.go           # Secure storage interface
│
├── hook/                           # Event hooks system
│   ├── hooks.go                    # Hook registry
│   ├── trader_hook.go              # Trader lifecycle hooks
│   └── http_client_hook.go         # HTTP client hooks
│
├── logger/                         # Logging system
│   ├── logger.go                   # Logger interface
│   ├── decision_logger.go          # Decision recording
│   └── telegram_hook.go            # Telegram notification hooks
│
├── pool/                           # Coin pool management
│   └── coin_pool.go                # AI500 + OI Top merged pool
│
├── security/                       # Security utilities
│   └── url_validator.go            # URL validation
│
└── web/                            # React frontend
    ├── src/
    │   ├── components/             # React components
    │   ├── lib/api.ts              # API client
    │   ├── stores/                 # Zustand state management
    │   └── App.tsx                 # Main app
    └── package.json                # Frontend dependencies
```

---

## System Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      PRESENTATION LAYER                          │
│    React SPA (Vite + TypeScript + TailwindCSS)                  │
│    - Competition dashboard, trader management UI                 │
│    - Real-time charts, authentication pages                      │
└──────────────────────────────────────────────────────────────────┘
                             ↓ HTTP/JSON API
┌──────────────────────────────────────────────────────────────────┐
│                      API LAYER (Gin Router)                      │
│    /api/traders, /api/status, /api/positions, /api/decisions    │
│    Authentication middleware (JWT), CORS handling                │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                      BUSINESS LOGIC LAYER                        │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │ TraderManager    │  │ DecisionEngine   │  │ MarketData     │ │
│  │ - Multi-trader   │  │ - AI reasoning   │  │ - K-lines      │ │
│  │   orchestration  │  │ - Risk control   │  │ - Indicators   │ │
│  └──────────────────┘  └──────────────────┘  └────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                      DATA ACCESS LAYER                           │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │ SQLite DB    │  │ File Logger  │  │ External APIs      │     │
│  │ - Traders    │  │ - Decisions  │  │ - Binance          │     │
│  │ - Models     │  │ - Performance│  │ - Hyperliquid      │     │
│  │ - Exchanges  │  │   analysis   │  │ - Aster/Lighter    │     │
│  └──────────────┘  └──────────────┘  └────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

---

## Core Modules

### Trader System (`trader/`)

**Purpose:** Trading execution layer with multi-exchange support

**Key Files:**
- `auto_trader.go` - Main trading orchestrator
- `interface.go` - Unified trader interface
- `binance_futures.go`, `bybit_trader.go`, `okx_trader.go` - CEX wrappers
- `hyperliquid_trader.go`, `aster_trader.go`, `lighter_trader_v2.go` - DEX wrappers
- `position.go` - Position management

**Design Pattern:** Strategy pattern with interface-based abstraction

---

### Decision Engine (`decision/`)

**Purpose:** AI-powered trading decision making

**Key Files:**
- `engine.go` - Decision logic with historical feedback
- `prompt_manager.go` - Template system for AI prompts

**Features:**
- Chain-of-Thought reasoning
- Historical performance analysis
- Risk-aware decision making
- Multi-model support

**Documentation:** See [Strategy Module Technical Documentation](STRATEGY_MODULE.md) for complete data flow, prompt construction, AI response parsing, and risk control enforcement.

---

### Market Data System (`market/`)

**Purpose:** Fetch and analyze market data

**Key Files:**
- `data.go` - Market data fetching and technical indicators
- `api_client.go` - Market data API client
- `websocket_client.go` - WebSocket streaming
- `combined_streams.go` - Combined streaming interface
- `monitor.go` - Market data cache

**Features:**
- Multi-timeframe K-line data
- Technical indicators (EMA, MACD, RSI, ATR)
- Open Interest tracking
- Real-time data streaming

---

### MCP Client (`mcp/`)

**Purpose:** AI model communication layer

**Key Files:**
- `client.go` - AI API client interface
- `deepseek_client.go`, `qwen_client.go` - Primary AI clients
- `openai_client.go`, `claude_client.go`, `gemini_client.go` - Additional AI clients
- `grok_client.go`, `kimi_client.go` - Extended AI clients

**Supported Models:** DeepSeek, Qwen, OpenAI, Claude, Gemini, Grok, Kimi

---

### API Server (`api/`)

**Purpose:** HTTP API for frontend communication

**Key Files:**
- `server.go` - Gin framework RESTful API
- `strategy.go` - Strategy management endpoints
- `webhook.go` - Webhook handling
- `backtest.go` - Backtest API endpoints

**Endpoints:** `/api/traders`, `/api/status`, `/api/positions`, `/api/decisions`, `/api/strategies`, `/api/backtests`

---

### Database Layer (`config/`)

**Purpose:** SQLite data persistence

**Key Files:**
- `database.go` - Database operations and schema
- `config.go` - Configuration management

**Tables:** `users`, `ai_models`, `exchanges`, `traders`, `strategies`, `equity_history`, `decision_logs`, `backtest_runs`

---

### Manager (`manager/`)

**Purpose:** Multi-trader orchestration

**Key Files:**
- `trader_manager.go` - Manages multiple trader instances

**Responsibilities:**
- Trader lifecycle (start, stop, restart)
- Resource allocation
- Concurrent execution coordination

---

### Authentication (`auth/`)

**Purpose:** User authentication and authorization

**Key Files:**
- `auth.go` - JWT token management & 2FA

**Features:**
- JWT token-based auth
- 2FA with TOTP (Google Authenticator)
- Bcrypt password hashing
- Role-based access control

---

### Backtest System (`backtest/`)

**Purpose:** Historical strategy backtesting

**Key Files:**
- `runner.go` - Backtest execution engine
- `manager.go` - Backtest lifecycle management
- `storage.go` - Backtest data storage
- `metrics.go` - Performance metrics calculation

**Features:**
- Historical data replay
- Strategy performance analysis
- Equity curve generation
- Trade-by-trade analysis

---

### Bootstrap (`bootstrap/`)

**Purpose:** Module initialization framework

**Key Files:**
- `bootstrap.go` - Bootstrap system
- `context.go` - Initialization context
- `hook_builder.go` - Hook registration

**Features:**
- Priority-based initialization
- Hook system for module dependencies
- Context sharing between modules

**Documentation:** See `bootstrap/README.md` for detailed usage.

---

### Cryptography (`crypto/`)

**Purpose:** Data encryption and secure storage

**Key Files:**
- `encryption.go` - Data encryption utilities
- `secure_storage.go` - Secure storage interface

**Features:**
- AES-256 encryption
- RSA key management
- Secure credential storage

---

### Hook System (`hook/`)

**Purpose:** Event hooks for extensibility

**Key Files:**
- `hooks.go` - Hook registry
- `trader_hook.go` - Trader lifecycle hooks
- `http_client_hook.go` - HTTP client hooks

**Features:**
- Trader lifecycle events
- HTTP request/response hooks
- IP validation hooks

**Documentation:** See `hook/README.md` for detailed usage.

---

### Logger (`logger/`)

**Purpose:** Logging and notification system

**Key Files:**
- `logger.go` - Logger interface
- `decision_logger.go` - Decision recording
- `telegram_hook.go` - Telegram notifications

**Features:**
- Structured logging
- Decision log persistence
- Telegram notifications
- Performance tracking

---

### Coin Pool (`pool/`)

**Purpose:** Coin selection and pool management

**Key Files:**
- `coin_pool.go` - AI500 + OI Top merged pool

**Features:**
- AI500 coin pool integration
- OI Top ranking integration
- Coin deduplication and merging

---

### Security (`security/`)

**Purpose:** Security utilities and validation

**Key Files:**
- `url_validator.go` - URL validation

**Features:**
- URL validation
- Security policy enforcement

---

## Request Flow

### Trading Decision Cycle

```
AutoTrader (every 3-5 min)
    ↓
1. FetchAccountStatus()
    ↓
2. GetOpenPositions()
    ↓
3. FetchMarketData() → Technical indicators
    ↓
4. AnalyzeHistory() → Last 20 trades
    ↓
5. GeneratePrompt() → Full context
    ↓
6. CallAI() → DeepSeek/Qwen/Claude/etc
    ↓
7. ParseDecision() → Structured output
    ↓
8. ValidateRisk() → Position limits, margin
    ↓
9. ExecuteOrders() → Exchange API
    ↓
10. LogDecision() → Database + JSON files
```

**Documentation:** See [Strategy Module Documentation](STRATEGY_MODULE.md#7-decision-execution-execution) for complete execution flow, risk control enforcement, and order processing details.

---

## Related Documentation

### Architecture Documentation

- **[Strategy Module](STRATEGY_MODULE.md)** - Complete strategy module technical documentation
  - Data flow from coin selection to execution
  - System/user prompt construction details
  - AI response parsing and validation
  - Risk control enforcement
  - Code file references with line numbers

### Module-Specific Documentation

- **Bootstrap:** `bootstrap/README.md` - Module initialization framework
- **Hook System:** `hook/README.md` - Event hooks and extensibility
- **MCP Client:** `mcp/intro/README.md` - Model Context Protocol implementation

### Other Documentation

- [Getting Started](../getting-started/README.md) - Setup and deployment
- [Contributing](../../CONTRIBUTING.md) - How to contribute
- [Community](../community/README.md) - Bounties and recognition
- [Prompt Guide](../prompt-guide.md) - AI prompt writing guide

---

## Dependencies

**Backend:** See `go.mod` for complete dependency list

**Key Packages:**
- `github.com/gin-gonic/gin` - HTTP API framework
- `github.com/adshao/go-binance/v2` - Binance API client
- `modernc.org/sqlite` - SQLite database driver
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/sonirico/go-hyperliquid` - Hyperliquid DEX client

**Frontend:** See `web/package.json` for complete dependency list

**Key Packages:**
- `react` + `react-dom` - UI framework
- `vite` - Build tool
- `recharts` - Charts library
- `swr` - Data fetching & caching
- `zustand` - State management

---

## Development

**Build & Run:**
```bash
# Backend
go build -o nofx
./nofx

# Frontend
cd web && npm run dev

# Docker
docker compose up --build
```

**Code Quality:**
```bash
go fmt ./...
cd web && npm run build
```

---

## For Developers

**Want to contribute?**
- Read [Contributing Guide](../../CONTRIBUTING.md)
- Check [Open Issues](https://github.com/tinkle-community/nofx/issues)
- Join [Telegram Community](https://t.me/nofx_dev_community)

**Need clarification?**
- Open a [GitHub Discussion](https://github.com/tinkle-community/nofx/discussions)
- Ask in Telegram

---

[← Back to Documentation Home](../README.md)
