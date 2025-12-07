# AI Agents and Implementations Guide

This guide explains the different AI agents available in NOFX and how they are implemented.

## Overview

NOFX supports multiple AI providers through a unified interface called **MCP (Model Client Protocol)**. The system uses a flexible architecture that allows you to:

1. Use built-in AI providers (DeepSeek, Qwen)
2. Use custom OpenAI-compatible APIs (OpenAI, Claude, Gemini, Grok, Kimi, etc.)
3. Use local models (Ollama, LM Studio)

## Architecture

### Core Components

#### 1. **MCP Interface** (`mcp/interface.go`)
The `AIClient` interface defines the contract for all AI clients:

```go
type AIClient interface {
    SetAPIKey(apiKey string, customURL string, customModel string)
    SetTimeout(timeout time.Duration)
    CallWithMessages(systemPrompt, userPrompt string) (string, error)
    CallWithRequest(req *Request) (string, error)
}
```

#### 2. **Base Client** (`mcp/client.go`)
The `Client` struct implements the base functionality using a **Template Method Pattern** with hooks:

- **Fixed workflow**: Retry logic, HTTP request handling
- **Customizable hooks**: Request building, URL construction, response parsing
- **Provider-agnostic**: Works with any OpenAI-compatible API

Key features:
- Automatic retry on network errors (up to 3 attempts)
- Configurable timeout (default: 120 seconds)
- Temperature control (default: 0.5)
- Max tokens limit (default: 2000)

#### 3. **Provider-Specific Clients**

##### **DeepSeek Client** (`mcp/deepseek_client.go`)
- **Provider ID**: `"deepseek"`
- **Default Base URL**: `https://api.deepseek.com/v1`
- **Default Model**: `deepseek-chat`
- **Usage**: Optimized for DeepSeek's API

```go
client := mcp.NewDeepSeekClientWithOptions()
client.SetAPIKey("sk-xxx", "", "")
```

##### **Qwen Client** (`mcp/qwen_client.go`)
- **Provider ID**: `"qwen"`
- **Default Base URL**: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **Default Model**: `qwen3-max`
- **Usage**: Optimized for Alibaba Cloud's Qwen API

```go
client := mcp.NewQwenClientWithOptions()
client.SetAPIKey("sk-xxx", "", "")
```

##### **Custom Client** (via base `Client`)
- **Provider ID**: `"custom"`
- **Base URL**: User-provided (any OpenAI-compatible endpoint)
- **Model**: User-provided
- **Usage**: Works with any OpenAI-compatible API

```go
client := mcp.NewClient()
client.SetAPIKey("sk-xxx", "https://api.openai.com/v1", "gpt-4o")
```

## Supported AI Models

According to the README, the following models are supported:

| AI Model | Provider | Status |
|----------|----------|--------|
| **DeepSeek** | `deepseek` | ✅ Supported |
| **Qwen** | `qwen` | ✅ Supported |
| **OpenAI (GPT)** | `custom` | ✅ Supported |
| **Claude** | `custom` | ✅ Supported |
| **Gemini** | `custom` | ✅ Supported |
| **Grok** | `custom` | ✅ Supported |
| **Kimi** | `custom` | ✅ Supported |

## Implementation Flow

### 1. **Model Registration** (`config/database.go`)

AI models are stored in the database with the following structure:

```go
type AIModelConfig struct {
    ID              string
    UserID          string
    Name            string
    Provider        string  // "deepseek", "qwen", or "custom"
    Enabled         bool
    APIKey          string  // Encrypted
    CustomAPIURL    string  // For custom provider
    CustomModelName string  // For custom provider
}
```

Models are retrieved via:
- `GetAIModels(userID)` - Get all models for a user
- `GetAIModel(userID, modelID)` - Get specific model (falls back to "default" user)

### 2. **Trader Creation** (`manager/trader_manager.go`)

When creating a trader:

1. **Load AI Model Config** from database
2. **Create appropriate client** based on provider:
   ```go
   if aiModelCfg.Provider == "qwen" {
       traderConfig.QwenKey = aiModelCfg.APIKey
   } else if aiModelCfg.Provider == "deepseek" {
       traderConfig.DeepSeekKey = aiModelCfg.APIKey
   }
   ```

3. **Initialize AutoTrader** (`trader/auto_trader.go`):
   ```go
   if config.AIModel == "custom" {
       mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
   } else if config.AIModel == "qwen" {
       mcpClient = mcp.NewQwenClient()
       mcpClient.SetAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
   } else {
       mcpClient = mcp.NewDeepSeekClient()
       mcpClient.SetAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
   }
   ```

### 3. **Decision Making** (`decision/engine.go`)

The decision engine uses the AI client to make trading decisions:

```go
func GetFullDecisionWithCustomPrompt(
    ctx *Context, 
    mcpClient mcp.AIClient, 
    customPrompt string, 
    overrideBase bool, 
    templateName string
) (*FullDecision, error)
```

The process:
1. **Build System Prompt** - From template files (`prompts/*.txt`)
2. **Build User Prompt** - Market data, positions, account info
3. **Call AI** - `mcpClient.CallWithMessages(systemPrompt, userPrompt)`
4. **Parse Response** - Extract JSON decisions from AI response
5. **Return Decisions** - Structured decision objects

## Configuration via Web UI

### Frontend (`web/src/components/TraderConfigModal.tsx`)

The `TraderConfigModal` component allows users to:

1. **Select AI Model** from available models:
   ```typescript
   <select value={formData.ai_model}>
     {availableModels.map((model) => (
       <option key={model.id} value={model.id}>
         {getShortName(model.name || model.id).toUpperCase()}
       </option>
     ))}
   </select>
   ```

2. **Configure Custom Prompt**:
   - System prompt template selection
   - Custom prompt textarea (append or override)
   - Override base prompt option

### Backend API (`api/server.go`)

The API endpoints handle model configuration:

- `GET /api/models` - Get all AI model configurations
- `PUT /api/models` - Update AI model configurations
- `GET /api/supported-models` - Get list of supported model types

## Advanced Features

### 1. **Request Builder Pattern** (`mcp/request_builder.go`)

For advanced use cases, you can use the builder pattern:

```go
request := mcp.NewRequestBuilder().
    WithSystemPrompt("You are a trading assistant").
    WithUserPrompt("Analyze BTCUSDT").
    WithTemperature(0.8).
    WithMaxTokens(4000).
    Build()

result, err := client.CallWithRequest(request)
```

This supports:
- Multi-turn conversations
- Fine-grained parameter control
- Function calling / tools (future)
- Streaming responses (future)

### 2. **Backtest Integration** (`backtest/ai_client.go`)

Backtests can use different AI configurations:

```go
func configureMCPClient(cfg BacktestConfig, base mcp.AIClient) (mcp.AIClient, error) {
    switch cfg.AICfg.Provider {
    case "deepseek":
        ds := mcp.NewDeepSeekClientWithOptions()
        ds.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
        return ds, nil
    case "qwen":
        qc := mcp.NewQwenClientWithOptions()
        qc.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
        return qc, nil
    case "custom":
        client := cloneBaseClient(base)
        client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
        return client, nil
    }
}
```

### 3. **Custom API URL Handling**

Special URL handling:
- If URL ends with `#`, use full URL (no `/chat/completions` appended)
- Otherwise, automatically append `/chat/completions`

Example:
- `https://api.openai.com/v1` → `https://api.openai.com/v1/chat/completions`
- `https://custom-api.com/endpoint#` → `https://custom-api.com/endpoint` (no append)

## Code Examples

### Example 1: Using DeepSeek

```go
// Create DeepSeek client
client := mcp.NewDeepSeekClientWithOptions()
client.SetAPIKey("sk-xxx", "", "")

// Make a call
response, err := client.CallWithMessages(
    "You are a trading assistant",
    "Analyze BTCUSDT market",
)
```

### Example 2: Using Custom OpenAI API

```go
// Create custom client
client := mcp.NewClient()
client.SetAPIKey(
    "sk-proj-xxx",
    "https://api.openai.com/v1",
    "gpt-4o",
)

// Make a call
response, err := client.CallWithMessages(
    "You are a trading assistant",
    "Analyze BTCUSDT market",
)
```

### Example 3: Using Local Ollama

```go
// Create custom client for local Ollama
client := mcp.NewClient()
client.SetAPIKey(
    "ollama", // Ollama doesn't require real key
    "http://localhost:11434/v1",
    "llama2",
)

// Make a call
response, err := client.CallWithMessages(
    "You are a trading assistant",
    "Analyze BTCUSDT market",
)
```

## Key Design Patterns

### 1. **Template Method Pattern**
- Base `Client` defines the algorithm skeleton
- Subclasses (DeepSeek, Qwen) override specific steps via hooks

### 2. **Strategy Pattern**
- Different AI providers are interchangeable strategies
- All implement the same `AIClient` interface

### 3. **Builder Pattern**
- `RequestBuilder` for complex request construction
- Fluent API for configuration

### 4. **Dependency Injection**
- Configurable logger, HTTP client, timeout
- Options pattern for flexible initialization

## Error Handling

The system includes robust error handling:

1. **Retry Logic**: Automatic retry on network errors (up to 3 times)
2. **Retryable Errors**: Network timeouts, connection resets, EOF
3. **Non-Retryable Errors**: Invalid API keys, authentication failures
4. **Error Logging**: Detailed logging for debugging

## Performance Considerations

1. **Connection Pooling**: HTTP client reuse
2. **Timeout Management**: Configurable per-client
3. **Token Limits**: Configurable max tokens
4. **Caching**: AI response caching in backtests (optional)

## Future Enhancements

Based on the code structure, future enhancements may include:

1. **Streaming Responses**: Real-time token streaming
2. **Function Calling**: Tool/function support
3. **Multi-Model Routing**: Automatic failover between models
4. **Response Caching**: Cache AI responses for identical inputs
5. **Rate Limiting**: Per-provider rate limit management

## Summary

NOFX's AI agent system is designed for:

- **Flexibility**: Support multiple AI providers
- **Extensibility**: Easy to add new providers
- **Reliability**: Robust error handling and retry logic
- **Usability**: Simple interface for common use cases
- **Advanced Features**: Builder pattern for complex scenarios

The architecture allows traders to use any AI model that supports OpenAI's API format, making it easy to experiment with different models and find the best one for trading strategies.

