package api

import (
	"encoding/json"
)

// PromptSectionsConfig Configuration for prompt sections
type PromptSectionsConfig struct {
	RoleDefinition  string `json:"role_definition"`
	TradingFrequency string `json:"trading_frequency"`
	EntryStandards   string `json:"entry_standards"`
	DecisionProcess  string `json:"decision_process"`
}

// DefaultStrategyConfigResponse Response structure for default strategy configuration
type DefaultStrategyConfigResponse struct {
	PromptTemplate string `json:"prompt_template"` // Always "default"
	CustomPrompt   string `json:"custom_prompt"`    // JSON string of PromptSectionsConfig
	OITopAPIURL    string `json:"oi_top_api_url"`   // Default OI Top API URL
}

// GetDefaultStrategyConfig returns default strategy configuration based on language
// Supports "zh" (Chinese) and "en" (English, default)
func GetDefaultStrategyConfig(lang string) DefaultStrategyConfigResponse {
	var config PromptSectionsConfig

	if lang == "zh" {
		// Chinese prompt templates
		config = PromptSectionsConfig{
			RoleDefinition: `# 你是一名专业的加密货币交易AI

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长技术分析和风险管理。`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟
如果你发现自己每个周期都在交易 → 标准太低；如果持仓不到30分钟就平仓 → 太冲动。`,
			EntryStandards: `# 🎯 入场标准（严格）

只在多个信号共振时入场。自由使用任何有效的分析方法，避免单一指标、信号矛盾、横盘震荡、或平仓后立即重新开仓等低质量行为。`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否止盈/止损
2. 扫描候选币种 + 多时间框架 → 是否存在强信号
3. 先写思维链，再输出结构化JSON`,
		}
	} else {
		// English prompt templates (default)
		config = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in technical analysis and risk management.`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent traders: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- More than 2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes
If you find yourself trading every cycle → standards too low; if closing positions in less than 30 minutes → too impulsive.`,
			EntryStandards: `# 🎯 Entry Standards (Strict)

Only enter positions when multiple signals resonate. Freely use any effective analysis methods, avoid single indicators, conflicting signals, sideways consolidation, or immediately reopening positions after closing - these are low-quality behaviors.`,
			DecisionProcess: `# 📋 Decision Process

1. Check positions → whether to take profit/stop loss
2. Scan candidate coins + multi-timeframe → whether strong signals exist
3. Write chain of thought first, then output structured JSON`,
		}
	}

	// Convert config to JSON string
	configJSON, err := json.Marshal(config)
	if err != nil {
		// If marshaling fails, return empty custom prompt
		return DefaultStrategyConfigResponse{
			PromptTemplate: "default",
			CustomPrompt:   "",
			OITopAPIURL:    "http://nofxaios.com:30006/api/oi/top",
		}
	}

	return DefaultStrategyConfigResponse{
		PromptTemplate: "default",
		CustomPrompt:   string(configJSON),
		OITopAPIURL:    "http://nofxaios.com:30006/api/oi/top",
	}
}
