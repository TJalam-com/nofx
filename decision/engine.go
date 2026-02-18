package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// Pre-compiled regular expressions (performance optimization: avoid recompiling on each call)
var (
	// ✅ Safe regex: exactly matches ```json code block
	// Use backticks + concatenation to avoid escaping issues
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// New: XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)

	// Enhanced JSON parsing for inconsistent AI outputs
	reJSONCodeBlock  = regexp.MustCompile(`(?is)\x60\x60\x60(?:json)?\s*(\{.*?\})\s*\x60\x60\x60|\x60\x60\x60json\s*(\{.*?\})\s*\x60\x60\x60|\x60\x60\x60json\s*(\[.*?\])\s*\x60\x60\x60|\x60\x60\x60(?:json)?\s*(\[.*?\])\s*\x60\x60\x60`)
	reJSONObject     = regexp.MustCompile(`(?s)\{[^{}]*"symbol"[^}]*\}`)
	reJSONArrayAlt   = regexp.MustCompile(`(?s)\[\s*\{[^{}]*(?:"symbol"[^{}]*|[^{}]*"action"[^{}]*|[^{}]*"signal_decision"[^{}]*)+[^{}]*\}\s*\]`)
	reSignalDecision = regexp.MustCompile(`(?s)"signal_decision"\s*:\s*"([^"]+)"`)
)

/*
JSON Response Format Documentation for AI Models

The system supports multiple JSON response formats for different trading scenarios.
AI models should use the appropriate format based on the prompt instructions.

1. STANDARD TRADING DECISIONS (Regular autonomous trading)
   Required tags: <reasoning> and <decision>
   Format:
   <reasoning>
   Your step-by-step analysis and reasoning...
   </reasoning>

   <decision>
   [
     {
       "symbol": "BTCUSDT",
       "action": "open_long",
       "leverage": 5,
       "position_size_usd": 2200,
       "stop_loss": 95000,
       "take_profit": 105000,
       "confidence": 85,
       "risk_usd": 300,
       "reasoning": "Strong bullish signal with support at 95000"
     }
   ]
   </decision>

2. TRADINGVIEW SIGNAL DECISIONS (Webhook-based trading)
   Required tags: <reasoning> and <decision>
   Format:
   <reasoning>
   Analysis of the TradingView signal...
   </reasoning>

   <decision>
   {
     "symbol": "SOLUSDT",
     "action": "open_short",
     "signal_decision": "accept",
     "tradingview_signal_id": "alert_123",
     "leverage": 3,
     "position_size_usd": 1500,
     "stop_loss": 130.00,
     "take_profit": 120.00,
     "reasoning": "Signal parameters meet risk criteria"
   }
   </decision>

   Alternative for rejection:
   <decision>
   {
     "symbol": "SOLUSDT",
     "signal_decision": "reject",
     "tradingview_signal_id": "alert_123",
     "reasoning": "Signal parameters outside risk tolerance"
   }
   </decision>

3. FLEXIBLE JSON FORMATS (Auto-detected)
   The system can parse JSON in multiple formats:
   - Standard tagged format: <decision>[{"symbol": "..."}]</decision>
   - Code blocks: ```json [{"symbol": "..."}] ```
   - Raw JSON arrays: [{"symbol": "..."}]
   - Single objects: {"symbol": "...", "action": "..."}

4. COMMON AI MISTAKES AND FIXES
   - Use straight quotes (") not smart quotes (", ")
   - Use standard brackets [] {} not full-width ［］｛｝
   - Use standard colon (:) not full-width (：)
   - Remove thousands separators: 50000 not 50,000
   - Remove range symbols: 100 not 95~100
   - Ensure all strings are quoted
   - Add missing commas between fields

5. FALLBACK BEHAVIOR
   If no valid JSON is found, the system enters "safe wait mode" and generates:
   {
     "symbol": "ALL",
     "action": "wait",
     "reasoning": "AI did not output structured JSON decision..."
   }
*/

// PositionInfo Position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // Historical peak return rate (percentage)
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
}

// AccountInfo Account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized P&L
	TotalPnL         float64 `json:"total_pnl"`         // Total P&L
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total P&L percentage
	MarginUsed       float64 `json:"margin_used"`       // Margin used
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Position count
}

// CandidateCoin Candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData Open interest growth Top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
	NetLong           float64 // Net long position
	NetShort          float64 // Net short position
}

// CompletedTrade Completed trade information
type CompletedTrade struct {
	Symbol      string    // Trading symbol
	Side        string    // Position side (long/short)
	EntryPrice  float64   // Entry price
	ExitPrice   float64   // Exit price
	Quantity    float64   // Trade quantity
	RealizedPnL float64   // Realized profit/loss
	ClosedAt    time.Time // Close time
}

// Context Trading context (complete information passed to AI)
type Context struct {
	CurrentTime     string                             `json:"current_time"`
	RuntimeMinutes  int                                `json:"runtime_minutes"`
	CallCount       int                                `json:"call_count"`
	Account         AccountInfo                        `json:"account"`
	Positions       []PositionInfo                     `json:"positions"`
	CompletedTrades []CompletedTrade                   `json:"completed_trades"`
	CandidateCoins  []CandidateCoin                    `json:"candidate_coins"`
	PromptVariant   string                             `json:"prompt_variant,omitempty"`
	MarketDataMap   map[string]*market.Data            `json:"-"` // Not serialized, but used internally
	MultiTFMarket   map[string]map[string]*market.Data `json:"-"`
	OITopDataMap    map[string]*OITopData              `json:"-"` // OI Top data mapping
	Performance     interface{}                        `json:"-"` // Historical performance analysis (logger.PerformanceAnalysis)
	BTCETHLeverage  int                                `json:"-"` // BTC/ETH leverage multiplier (read from config)
	AltcoinLeverage int                                `json:"-"` // Altcoin leverage multiplier (read from config)
	StrategyID      string                             `json:"-"` // Strategy ID (optional, for loading strategy config)
}

// StrategyConfig holds configurable strategy parameters
type StrategyConfig struct {
	MinRiskRewardRatio        float64
	MaxPositions              int
	MarginUsageLimit          float64
	MinOpeningAmount          float64
	MinOpeningAmountBTCETH    float64
	AltcoinPositionMin        float64
	AltcoinPositionMax        float64
	BTCETHPositionMin         float64
	BTCETHPositionMax         float64
	AvailableMarginMultiplier float64
	MinConfidenceForEntry     int
	MinHoldingTimeMinutes     int
	SharpeRatioConfig         map[string]interface{} // Parsed from JSON
}

// GetDefaultStrategyConfig returns default strategy configuration
func GetDefaultStrategyConfig() StrategyConfig {
	return StrategyConfig{
		MinRiskRewardRatio:        3.0,
		MaxPositions:              3,
		MarginUsageLimit:          90.0,
		MinOpeningAmount:          12.0,
		MinOpeningAmountBTCETH:    60.0,
		AltcoinPositionMin:        0.8,
		AltcoinPositionMax:        1.5,
		BTCETHPositionMin:         5.0,
		BTCETHPositionMax:         10.0,
		AvailableMarginMultiplier: 0.88,
		MinConfidenceForEntry:     75,
		MinHoldingTimeMinutes:     30,
		SharpeRatioConfig:         make(map[string]interface{}),
	}
}

// StrategyConfigFromFields creates StrategyConfig from individual fields (for use when StrategyRecord is not available)
func StrategyConfigFromFields(
	minRiskRewardRatio, marginUsageLimit, minOpeningAmount, minOpeningAmountBTCETH float64,
	altcoinPositionMin, altcoinPositionMax, btcEthPositionMin, btcEthPositionMax, availableMarginMultiplier float64,
	maxPositions, minConfidenceForEntry, minHoldingTimeMinutes int,
	sharpeRatioConfigJSON string,
) StrategyConfig {
	config := GetDefaultStrategyConfig()

	// Always set minRiskRewardRatio if provided (even if 0, to allow explicit 0 values)
	// Only skip if the value is negative (invalid)
	if minRiskRewardRatio >= 0 {
		config.MinRiskRewardRatio = minRiskRewardRatio
	}
	if maxPositions > 0 {
		config.MaxPositions = maxPositions
	}
	if marginUsageLimit > 0 {
		config.MarginUsageLimit = marginUsageLimit
	}
	if minOpeningAmount > 0 {
		config.MinOpeningAmount = minOpeningAmount
	}
	if minOpeningAmountBTCETH > 0 {
		config.MinOpeningAmountBTCETH = minOpeningAmountBTCETH
	}
	if altcoinPositionMin > 0 {
		config.AltcoinPositionMin = altcoinPositionMin
	}
	if altcoinPositionMax > 0 {
		config.AltcoinPositionMax = altcoinPositionMax
	}
	if btcEthPositionMin > 0 {
		config.BTCETHPositionMin = btcEthPositionMin
	}
	if btcEthPositionMax > 0 {
		config.BTCETHPositionMax = btcEthPositionMax
	}
	if availableMarginMultiplier > 0 {
		config.AvailableMarginMultiplier = availableMarginMultiplier
	}
	if minConfidenceForEntry > 0 {
		config.MinConfidenceForEntry = minConfidenceForEntry
	}
	if minHoldingTimeMinutes > 0 {
		config.MinHoldingTimeMinutes = minHoldingTimeMinutes
	}
	if sharpeRatioConfigJSON != "" {
		if err := json.Unmarshal([]byte(sharpeRatioConfigJSON), &config.SharpeRatioConfig); err != nil {
			log.Printf("⚠️ Failed to parse sharpe_ratio_config JSON: %v, using empty config", err)
		}
	}

	return config
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// Position opening parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	EntryPrice      float64 `json:"entry_price,omitempty"` // Optional; used for RR calculation when set (e.g. from webhook)

	// Adjustment parameters (new)
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // For update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // For update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // For partial_close (0-100)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`

	// TradingView signal related fields
	TradingViewSignalID string `json:"tradingview_signal_id,omitempty"` // TradingView alert ID
	SignalDecision      string `json:"signal_decision,omitempty"`       // "accept", "reject", "modify"

	// Parent trade signal related fields
	ParentSignalID string `json:"parent_signal_id,omitempty"` // Parent trader signal ID
}

// FullDecision AI's complete decision (includes reasoning chain)
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // System prompt (system prompt sent to AI)
	UserPrompt   string     `json:"user_prompt"`   // Input prompt sent to AI
	CoTTrace     string     `json:"cot_trace"`     // Reasoning chain analysis (AI output)
	Decisions    []Decision `json:"decisions"`     // Specific decision list
	Timestamp    time.Time  `json:"timestamp"`
	// AIRequestDurationMs Records AI API call duration (milliseconds) for troubleshooting latency issues
	AIRequestDurationMs int64  `json:"ai_request_duration_ms,omitempty"`
	RawResponse         string `json:"raw_response,omitempty"` // Raw AI response for debugging parse failures
}

// GetFullDecision Gets AI's complete trading decision (batch analysis of all coins and positions)
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "", GetDefaultStrategyConfig())
}

// GetFullDecisionWithCustomPrompt Gets AI's complete trading decision (supports custom prompt and template selection)
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient mcp.AIClient, customPrompt string, overrideBase bool, templateName string, config StrategyConfig) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// 1. Fetch market data for all coins (if already provided by upper layer, no need to fetch again)
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataForContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	} else if ctx.OITopDataMap == nil {
		// Ensure OI data mapping is initialized to avoid null pointer access later
		ctx.OITopDataMap = make(map[string]*OITopData)
	}

	// 2. Build System Prompt (fixed rules) and User Prompt (dynamic data)
	systemPrompt, err := buildSystemPromptWithCustom(
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
		customPrompt,
		overrideBase,
		templateName,
		ctx.PromptVariant,
		PromptTypeStandard,
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build system prompt: %w", err)
	}
	userPrompt := buildUserPrompt(ctx)

	// 3. Call AI API (using system + user prompt)
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("failed to call AI API: %w", err)
	}

	// 4. Parse AI response
	decision, err := parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
		config,
		ctx.Account.PositionCount,
		ctx.Account.MarginUsedPct,
		ctx.Positions,
	)

	// Save SystemPrompt and UserPrompt regardless of errors (for debugging and troubleshooting when decisions are not executed)
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // Save system prompt
		decision.UserPrompt = userPrompt     // Save input prompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse // Save raw AI response for debugging parse failures
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // Save system prompt
	decision.UserPrompt = userPrompt     // Save input prompt
	decision.RawResponse = aiResponse    // Save raw AI response for debugging parse failures
	return decision, nil
}

// fetchMarketDataForContext Fetches market data and OI data for all coins in the context
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// Collect all coins that need data fetching
	symbolSet := make(map[string]bool)

	// 1. Prioritize fetching data for position coins (this is required)
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. Candidate coin count is dynamically adjusted based on account status
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// Fetch market data concurrently
	// Position coin set (used to determine whether to skip OI check)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// Single coin failure does not affect the whole, only log error
			continue
		}

		// ⚠️ Liquidity filtering: skip coins with position value below threshold (both long and short)
		// Position value = position size × current price
		// But existing positions must be retained (need to decide whether to close)
		// 💡 OI threshold configuration: users can adjust based on risk preference
		const minOIThresholdMillions = 15.0 // Adjustable: 15M(conservative) / 10M(balanced) / 8M(loose) / 5M(aggressive)

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// Calculate position value (USD) = position size × current price
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // Convert to millions USD
			if oiValueInMillions < minOIThresholdMillions {
				log.Printf("⚠️  %s position value too low (%.2fM USD < %.1fM), skipping this coin [position size:%.0f × price:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// Load OI Top data (does not affect main flow)
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// Normalize symbol matching
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates Calculates the number of candidate coins to analyze based on account status
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ Important: limit candidate coin count to avoid Prompt being too large
	// Dynamically adjust based on position count: fewer positions allow analyzing more candidate coins
	const (
		maxCandidatesWhenEmpty    = 30 // When no positions, analyze up to 30 candidate coins
		maxCandidatesWhenHolding1 = 25 // When holding 1 position, analyze up to 25 candidate coins
		maxCandidatesWhenHolding2 = 20 // When holding 2 positions, analyze up to 20 candidate coins
		maxCandidatesWhenHolding3 = 15 // When holding 3 positions, analyze up to 15 candidate coins (avoid Prompt being too large)
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ positions
		maxCandidates = maxCandidatesWhenHolding3
	}

	// Return the smaller value between actual candidate coin count and upper limit
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom Builds System Prompt with custom content
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string, promptType PromptType, config StrategyConfig) (string, error) {
	// If override base prompt and have custom prompt, use only custom prompt
	if overrideBase && customPrompt != "" {
		// Ensure format section is included when overriding base prompt
		enhancedPrompt := EnsureFormatSection(customPrompt, promptType, accountEquity, btcEthLeverage, altcoinLeverage)
		// Validate and warn if format is incomplete
		ValidateAndWarn(enhancedPrompt, promptType, accountEquity, btcEthLeverage, altcoinLeverage)
		return enhancedPrompt, nil
	}

	// Get base prompt (using specified template)
	basePrompt, err := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName, variant, config)
	if err != nil {
		// If template loading fails, return error
		return "", err
	}

	// If no custom prompt, return base prompt directly (which already includes format)
	if customPrompt == "" {
		return basePrompt, nil
	}

	// Add custom prompt section to base prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 Custom Trading Strategy\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("Note: The above custom strategy supplements the base rules and must not violate the fundamental risk control principles.\n")

	return sb.String(), nil
}

// BuildSystemPromptWithTradingView Builds System Prompt with TradingView signal analysis
func BuildSystemPromptWithTradingView(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string, config StrategyConfig) (string, error) {
	// Build base prompt first
	basePrompt, err := buildSystemPromptWithCustom(accountEquity, btcEthLeverage, altcoinLeverage, customPrompt, overrideBase, templateName, variant, PromptTypeTradingView, config)
	if err != nil {
		return "", err
	}

	// Add TradingView signal analysis section
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📡 TradingView Signal Analysis Mode\n\n")
	sb.WriteString("You are analyzing a webhook signal from TradingView. The signal contains the following information:\n")
	sb.WriteString("- Symbol: Trading pair\n")
	sb.WriteString("- Action: Operation type (buy = open long, sell = open short)\n")
	sb.WriteString("- Entry: Suggested entry price\n")
	sb.WriteString("- Stop Loss (SL): Stop loss price specified by the signal (must not be changed)\n")
	sb.WriteString("- Take Profit (TP): Take profit price specified by the signal (must not be changed)\n")
	sb.WriteString("- Quantity: Suggested quantity\n\n")
	sb.WriteString("## Decision Options\n\n")
	sb.WriteString("You must make one of the following three decisions for each TradingView signal:\n\n")
	sb.WriteString("1. **accept**: The signal aligns with your trading strategy, execute the trade using ALL parameters provided by the signal, including SL and TP\n")
	sb.WriteString("2. **reject**: The signal does not align with your trading strategy or risk control requirements, do not execute the trade\n")
	sb.WriteString("3. **modify**: The signal direction is correct, but leverage and/or position size need adjustment. You may ONLY modify `leverage` and `position_size_usd`. You MUST keep `stop_loss` and `take_profit` exactly equal to the values from the TradingView signal.\n\n")
	sb.WriteString("**Hard Rule**: In TradingView signal mode, `stop_loss` and `take_profit` are FINAL risk parameters provided by the webhook. You are NOT allowed to change them under any circumstance. If they are unsafe, you must choose `signal_decision` = \"reject\" instead of modifying SL/TP.\n\n")

	// Use modular JSON format template
	sb.WriteString(GetTradingViewJSONFormat())

	sb.WriteString("## Analysis Points\n\n")
	sb.WriteString("- Carefully analyze the consistency between current market data (price, indicators, OI, etc.) and the signal\n")
	sb.WriteString("- Check if signal parameters meet risk control requirements (stop loss/take profit ratio, position size, etc.)\n")
	sb.WriteString("- Consider current account status (balance, positions, margin usage rate, etc.)\n")
	sb.WriteString("- If signal direction is correct but parameters are unreasonable, use \"modify\" and provide optimized parameters\n")
	sb.WriteString("- If signal completely does not align with strategy or risk is too high, use \"reject\" and explain the reason\n\n")
	sb.WriteString("## Confidence Score Calculation (CRITICAL)\n\n")
	sb.WriteString("The TradingView signal only provides trade parameters (entry, SL, TP). **YOU must independently calculate the `confidence` score (0-100)** based on your own analysis.\n")
	sb.WriteString("Do NOT default confidence to 0 or omit it — a missing/zero confidence will cause the trade to be rejected by the system.\n\n")
	sb.WriteString("Evaluate confidence based on:\n")
	sb.WriteString("1. **Trend alignment**: Does the signal direction match the current trend on multiple timeframes?\n")
	sb.WriteString("2. **Indicator confluence**: Do MACD, RSI, EMA, volume, and OI support the signal direction?\n")
	sb.WriteString("3. **Risk-reward quality**: Is the SL/TP ratio favorable?\n")
	sb.WriteString("4. **Market conditions**: Volatility, funding rate, overall market sentiment\n")
	sb.WriteString("5. **Account health**: Current margin usage, existing positions, drawdown status\n\n")
	sb.WriteString("Confidence guidelines: 90+ = exceptional setup, 75-89 = strong setup, 60-74 = acceptable setup, <60 = weak/reject\n\n")

	return sb.String(), nil
}

// BuildSystemPromptWithParentSignal Builds System Prompt with parent trader signal analysis
func BuildSystemPromptWithParentSignal(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string, config StrategyConfig) (string, error) {
	// Build base prompt first (using risk_management template)
	basePrompt, err := buildSystemPromptWithCustom(accountEquity, btcEthLeverage, altcoinLeverage, customPrompt, overrideBase, templateName, variant, PromptTypeParent, config)
	if err != nil {
		return "", err
	}

	// Add parent trader signal analysis section
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📡 Parent Trade Signal Analysis Mode\n\n")
	sb.WriteString("You are analyzing a trade signal from a parent trader that you are following.\n\n")
	sb.WriteString("## Signal Types\n\n")
	sb.WriteString("Signals can come from two sources:\n")
	sb.WriteString("1. **TradingView Instant Forwarding**: Raw TradingView alerts forwarded immediately to followers before parent AI processing. These signals may have basic parameters (symbol, direction) and require your AI to determine optimal execution parameters.\n")
	sb.WriteString("2. **Post-Execution Replication**: Signals sent after parent trader's AI has analyzed and executed a trade. These signals include parent's AI reasoning and parameters.\n\n")
	sb.WriteString("The signal contains the following information:\n")
	sb.WriteString("- Parent Trader: Name and ID of the trader you are following\n")
	sb.WriteString("- Symbol: Trading pair\n")
	sb.WriteString("- Action: Operation type (open_long, open_short, close_long, close_short, etc.)\n")
	sb.WriteString("- Leverage: Suggested leverage (may be 0 for TradingView signals - use your own leverage settings)\n")
	sb.WriteString("- Position Size: Suggested position size in USDT (may be 0 for TradingView signals - scale based on your account)\n")
	sb.WriteString("- Stop Loss: Suggested stop loss price (may be 0 - determine based on your risk management)\n")
	sb.WriteString("- Take Profit: Suggested take profit price (may be 0 - determine based on your risk management)\n")
	sb.WriteString("- Reasoning: Parent trader's reasoning or TradingView alert context\n\n")
	sb.WriteString("**Important**: For TradingView-originated signals, you have full autonomy to determine optimal parameters (leverage, position size, stop loss, take profit) based on your account size and risk tolerance. The signal provides direction and symbol only.\n\n")
	sb.WriteString("## Decision Options\n\n")
	sb.WriteString("You must make one of the following three decisions for each parent trade signal:\n\n")
	sb.WriteString("1. **accept**: The signal aligns with your risk management strategy, execute the trade using parameters scaled to your account size\n")
	sb.WriteString("2. **reject**: The signal does not align with your risk control requirements or account state, do not execute the trade\n")
	sb.WriteString("3. **modify**: The signal direction is correct, but parameters need adjustment for your account size/risk tolerance, execute with your optimized parameters\n\n")

	// Use modular JSON format template
	sb.WriteString(GetParentSignalJSONFormat())

	sb.WriteString("## Analysis Points\n\n")
	sb.WriteString("- Carefully analyze if the parent's trade makes sense for your account size and risk tolerance\n")
	sb.WriteString("- Scale position sizes appropriately based on your available balance (not the parent's balance)\n")
	sb.WriteString("- Check if signal parameters meet your risk control requirements (stop loss/take profit ratio, position size, leverage, etc.)\n")
	sb.WriteString("- Consider your current account status (balance, positions, margin usage rate, drawdown, etc.)\n")
	sb.WriteString("- If signal direction is correct but position size/leverage is too large for your account, use \"modify\" and scale down appropriately\n")
	sb.WriteString("- If signal completely does not align with your risk management or account state is unhealthy, use \"reject\" and explain the reason\n\n")
	sb.WriteString("## Important Reminders\n\n")
	sb.WriteString("- Always scale position sizes based on YOUR account equity, not the parent's equity\n")
	sb.WriteString("- Never risk more than 2-3% of your account equity on a single trade\n")
	sb.WriteString("- Use conservative leverage, especially for smaller accounts\n")
	sb.WriteString("- You are responsible for your own risk management - the parent's decision is a signal, not a command\n")
	sb.WriteString("- Better to skip a trade (reject) than take excessive risk\n")
	sb.WriteString("- For TradingView instant signals: prioritize speed while maintaining safety - make efficient decisions without over-analyzing\n\n")

	return sb.String(), nil
}

// buildSystemPrompt Builds System Prompt (using template + dynamic parts)
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string, variant string, config StrategyConfig) (string, error) {
	var sb strings.Builder

	// 1. Load prompt template (core trading strategy part)
	if templateName == "" {
		templateName = "default" // Default to default template
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// Template must exist in database - fail if not found (no hardcoded fallback)
		log.Printf("❌ Prompt template '%s' does not exist in database: %v", templateName, err)
		// Try default template as last resort
		template, err = GetPromptTemplate("default")
		if err != nil {
			// If even default does not exist, this is a critical error
			log.Printf("❌ CRITICAL: Default prompt template does not exist in database. Please ensure templates are loaded from strategy-studio.")
			return "", fmt.Errorf("prompt template '%s' and 'default' template not found in database. Templates must be loaded from strategy-studio (http://localhost:3000/strategy-studio). Error: %w", templateName, err)
		}
		log.Printf("⚠️  Using 'default' template as fallback for '%s'", templateName)
	}

	sb.WriteString(template.Content)
	sb.WriteString("\n\n")

	// 2. Trading mode variants
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥70\n- Higher positions allowed, but must strictly set stop loss and explain profit-to-loss ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals converge\n- Prioritize preserving cash, must pause for multiple cycles after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick execution\n- If price does not move as expected within two bars, immediately reduce position or stop loss\n\n")
	}

	// 3. Hard constraints (risk control)
	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString(fmt.Sprintf("1. Risk-reward ratio: Must be ≥ 1:%.0f (risk 1%% to earn %.0f%%+ returns)\n", config.MinRiskRewardRatio, config.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("2. Maximum positions: %d coins (quality > quantity)\n", config.MaxPositions))
	sb.WriteString(fmt.Sprintf("3. Single coin position: Altcoins %.0f-%.0f USDT | BTC/ETH %.0f-%.0f USDT\n",
		accountEquity*config.AltcoinPositionMin, accountEquity*config.AltcoinPositionMax, accountEquity*config.BTCETHPositionMin, accountEquity*config.BTCETHPositionMax))
	sb.WriteString(fmt.Sprintf("4. Leverage limits: **Altcoins maximum %dx leverage** | **BTC/ETH maximum %dx leverage**\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString(fmt.Sprintf("5. Margin usage rate ≤ %.0f%%\n", config.MarginUsageLimit))
	sb.WriteString(fmt.Sprintf("6. Opening amount: Recommended ≥%.0f USDT (exchange minimum notional value 10 USDT + safety margin)\n\n", config.MinOpeningAmount))

	// Position sizing guidance
	sb.WriteString("# Position Sizing Guidance\n\n")
	sb.WriteString("**Important**: `position_size_usd` is the **notional value** (including leverage), not margin requirement.\n\n")
	sb.WriteString("**Calculation Steps**:\n")
	sb.WriteString(fmt.Sprintf("1. **Available Margin** = Available Cash × %.2f (reserve %.0f%% for fees, slippage, and liquidation margin buffer)\n", config.AvailableMarginMultiplier, (1-config.AvailableMarginMultiplier)*100))
	sb.WriteString("2. **Notional Value** = Available Margin × Leverage\n")
	sb.WriteString("3. **position_size_usd** = Notional Value (fill this value in JSON)\n")
	sb.WriteString("4. **Actual Coin Amount** = position_size_usd / Current Price\n\n")
	sb.WriteString("**Example**: Available cash $500, leverage 5x\n")
	sb.WriteString(fmt.Sprintf("- Available Margin = $500 × %.2f = $%.0f\n", config.AvailableMarginMultiplier, 500*config.AvailableMarginMultiplier))
	sb.WriteString(fmt.Sprintf("- position_size_usd = $%.0f × 5 = **$%.0f** ← Fill this value in JSON\n", 500*config.AvailableMarginMultiplier, 500*config.AvailableMarginMultiplier*5))
	sb.WriteString(fmt.Sprintf("- Actual margin used = $%.0f, remaining $%.0f for fees, slippage, and liquidation protection\n\n", 500*config.AvailableMarginMultiplier, 500*(1-config.AvailableMarginMultiplier)))
	sb.WriteString("**Sizing Considerations**:\n")
	sb.WriteString("- Only use available cash (not account equity)\n")
	sb.WriteString("- Consider current margin usage\n")
	sb.WriteString(fmt.Sprintf("- Ensure position size fits within single coin limits (Altcoins %.0f-%.0f USDT, BTC/ETH %.0f-%.0f USDT)\n", accountEquity*config.AltcoinPositionMin, accountEquity*config.AltcoinPositionMax, accountEquity*config.BTCETHPositionMin, accountEquity*config.BTCETHPositionMax))
	sb.WriteString("- Account for fees and slippage in calculations\n\n")

	// 4. Trading frequency and signal quality
	sb.WriteString("# ⏱️ Trading Frequency Understanding\n\n")
	sb.WriteString("- Excellent traders: 2-4 trades per day ≈ 0.1-0.2 trades per hour\n")
	sb.WriteString("- >2 trades per hour = overtrading\n")
	sb.WriteString("- Single position holding time ≥30-60 minutes\n")
	sb.WriteString("If you find yourself trading every cycle → standards too low; if closing positions <30 minutes → too impatient.\n\n")

	sb.WriteString("# 🎯 Opening Criteria (Strict)\n\n")
	sb.WriteString("Only open positions when multiple signals converge. You have:\n")
	sb.WriteString("- 3-minute price sequence + 4-hour K-line sequence\n")
	sb.WriteString("- EMA20 / MACD / RSI7 / RSI14 indicator sequences\n")
	sb.WriteString("- Volume, Open Interest (OI), funding rate capital flow sequences\n")
	sb.WriteString("- AI500 / OI_Top filter tags (if available)\n\n")
	sb.WriteString(fmt.Sprintf("Freely use any effective analysis methods, but **confidence ≥%d** required to open positions; avoid single indicators, contradictory signals, sideways consolidation, immediately reopening after closing positions, and other low-quality behaviors.\n\n", config.MinConfidenceForEntry))

	// 5. Sharpe ratio-driven adaptation
	sb.WriteString("# 🧬 Sharpe Ratio Self-Evolution\n\n")
	sb.WriteString("- Sharpe < -0.5: Immediately stop trading, wait at least 6 cycles and conduct deep review\n")
	sb.WriteString("- -0.5 ~ 0: Only trade with confidence >80, and reduce frequency\n")
	sb.WriteString("- 0 ~ 0.7: Maintain current strategy\n")
	sb.WriteString("- >0.7: Allow moderate position increase, but still follow risk control\n\n")

	// 6. Decision process hints
	sb.WriteString("# 📋 Decision Process\n\n")
	sb.WriteString("1. Review Sharpe ratio/P&L → Whether to reduce frequency or pause\n")
	sb.WriteString("2. Check positions → Whether to take profit/stop loss/adjust\n")
	sb.WriteString("3. Scan candidate coins + multiple timeframes → Whether strong signals exist\n")
	sb.WriteString("4. Write chain of thought first, then output structured JSON\n\n")

	// 7. Output format - use modular template
	sb.WriteString(GetStandardJSONFormat(accountEquity, btcEthLeverage, altcoinLeverage))

	return sb.String(), nil
}

// buildUserPrompt Builds User Prompt (dynamic data)
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Cycle: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Account
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | P&L %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Recent Completed Trades (before Current Positions)
	if len(ctx.CompletedTrades) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		// Display last 5-10 completed trades
		displayCount := len(ctx.CompletedTrades)
		if displayCount > 10 {
			displayCount = 10
		}
		for i := 0; i < displayCount; i++ {
			trade := ctx.CompletedTrades[i]
			// Calculate time since close
			timeSinceClose := time.Since(trade.ClosedAt)
			timeAgo := ""
			if timeSinceClose.Hours() < 1 {
				timeAgo = fmt.Sprintf("%.0f minutes ago", timeSinceClose.Minutes())
			} else if timeSinceClose.Hours() < 24 {
				timeAgo = fmt.Sprintf("%.1f hours ago", timeSinceClose.Hours())
			} else {
				timeAgo = fmt.Sprintf("%.1f days ago", timeSinceClose.Hours()/24)
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | Quantity %.4f | P&L %+.2f USDT | Closed %s\n\n",
				i+1, trade.Symbol, strings.ToUpper(trade.Side),
				trade.EntryPrice, trade.ExitPrice, trade.Quantity, trade.RealizedPnL, timeAgo))
		}
		sb.WriteString("\n")
	}

	// Positions (complete market data)
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			// Calculate holding duration
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // Convert to minutes
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | Holding duration %d minutes", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | Holding duration %d hours %d minutes", durationHour, durationMinRemainder)
				}
			}

			// Calculate position value (for partial_close check)
			positionValue := math.Abs(pos.Quantity) * pos.MarkPrice

			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Quantity %.4f | Position Value %.2f USDT | P&L %+.2f%% | P&L Amount %+.2f USDT | Peak Return %.2f%% | Leverage %dx | Margin %.0f | Liquidation %.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// Use FormatMarketData to output complete market data
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("Current positions: None\n\n")
	}

	// Candidate coins (complete market data)
	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top dual signal)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top position growth)"
		}

		// Use FormatMarketData to output complete market data
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// OI Ranking Data (Top Open Interest Growth)
	if len(ctx.OITopDataMap) > 0 {
		sb.WriteString("## OI Ranking Data (Top Open Interest Growth)\n\n")
		sb.WriteString("The following coins show significant open interest growth and may indicate strong market momentum:\n\n")

		// Collect and sort OI data by rank
		type rankedOIData struct {
			Symbol string
			Data   *OITopData
		}
		rankedData := make([]rankedOIData, 0, len(ctx.OITopDataMap))
		for symbol, oiData := range ctx.OITopDataMap {
			rankedData = append(rankedData, rankedOIData{Symbol: symbol, Data: oiData})
		}

		// Sort by rank (ascending, rank 1 is best)
		for i := 0; i < len(rankedData)-1; i++ {
			for j := i + 1; j < len(rankedData); j++ {
				if rankedData[i].Data.Rank > rankedData[j].Data.Rank {
					rankedData[i], rankedData[j] = rankedData[j], rankedData[i]
				}
			}
		}

		// Display OI ranking data
		for _, item := range rankedData {
			oiData := item.Data
			sb.WriteString(fmt.Sprintf("Rank #%d: %s | OI Delta: %+.2f%% (%+.0f) | Price Delta: %+.2f%% | Net Long: %.0f | Net Short: %.0f\n",
				oiData.Rank, item.Symbol,
				oiData.OIDeltaPercent, oiData.OIDeltaValue,
				oiData.PriceDeltaPercent,
				oiData.NetLong, oiData.NetShort))
		}
		sb.WriteString("\n")
	}

	// Sharpe ratio (pass value directly, no complex formatting)
	if ctx.Performance != nil {
		// Extract SharpeRatio directly from interface{}
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 Sharpe Ratio: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (chain of thought + JSON)\n")

	return sb.String()
}

// parseFullDecisionResponse Parses AI's complete decision response
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, config StrategyConfig, positionCount int, marginUsedPct float64, positions []PositionInfo) (*FullDecision, error) {
	// 1. Extract reasoning chain
	cotTrace := extractCoTTrace(aiResponse)

	// 2. Extract JSON decision list
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	// 3. Validate decisions
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, config, positionCount, marginUsedPct, positions); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// ParseFullDecisionResponse Parses AI's complete decision response (public wrapper function)
// Extracts account equity and leverage parameters from Context, calls private parseFullDecisionResponse function
func ParseFullDecisionResponse(ctx *Context, aiResponse string, config StrategyConfig) (*FullDecision, error) {
	return parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
		config,
		ctx.Account.PositionCount,
		ctx.Account.MarginUsedPct,
		ctx.Positions,
	)
}

// extractCoTTrace Extracts reasoning chain analysis
func extractCoTTrace(response string) string {
	// Method 1: Prioritize extracting <reasoning> tag content
	if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
		log.Printf("✓ Using <reasoning> tag to extract reasoning chain")
		return strings.TrimSpace(match[1])
	}

	// Method 2: If no <reasoning> tag but has <decision> tag, extract content before <decision>
	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		log.Printf("✓ Extracting content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	// Method 3: Fallback - find JSON array start position
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		log.Printf("⚠️  Using legacy format ([ character separation) to extract reasoning chain")
		return strings.TrimSpace(response[:jsonStart])
	}

	// If no markers found, entire response is reasoning chain
	return strings.TrimSpace(response)
}

// extractDecisions Extracts JSON decision list with enhanced robustness for inconsistent AI outputs
func extractDecisions(response string) ([]Decision, error) {
	// Pre-clean: remove zero-width/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 Critical Fix: Fix full-width characters before regex matching!
	// Otherwise regex \[ cannot match full-width ［
	s = fixMissingQuotes(s)

	// Method 1: Prioritize extracting from <decision> tag
	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		log.Printf("✓ Using <decision> tag to extract JSON")
	} else {
		// Fallback: use entire response
		jsonPart = s
		log.Printf("⚠️  <decision> tag not found, using full-text JSON search")
	}

	// Fix full-width characters in jsonPart
	jsonPart = fixMissingQuotes(jsonPart)

	// Try multiple parsing methods in order of preference

	// 1) Prioritize extracting from ```json code block
	if m := reJSONFence.FindStringSubmatch(jsonPart); len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // Normalize "[ {" to "[{"
		jsonContent = fixMissingQuotes(jsonContent) // Second fix (prevent remaining full-width after regex extraction)
		if decisions, err := tryParseJSON(jsonContent); err == nil {
			log.Printf("✓ Successfully parsed JSON from ```json code block")
			return decisions, nil
		}
		log.Printf("⚠️  Failed to parse JSON from ```json code block, trying alternatives")
	}

	// 2) Try enhanced JSON code block regex (more flexible)
	if m := reJSONCodeBlock.FindStringSubmatch(jsonPart); len(m) > 1 {
		for i := 1; i < len(m); i++ {
			if m[i] != "" {
				jsonContent := strings.TrimSpace(m[i])
				jsonContent = compactArrayOpen(jsonContent)
				jsonContent = fixMissingQuotes(jsonContent)
				// Check if it's a single object or an array
				trimmedContent := strings.TrimSpace(jsonContent)
				if strings.HasPrefix(trimmedContent, "{") {
					// Single object - use tryParseJSONObject
					if decisions, err := tryParseJSONObject(jsonContent); err == nil {
						log.Printf("✓ Successfully parsed single JSON object from enhanced code block regex")
						return decisions, nil
					}
				} else {
					// Array - use tryParseJSON
					if decisions, err := tryParseJSON(jsonContent); err == nil {
						log.Printf("✓ Successfully parsed JSON array from enhanced code block regex")
						return decisions, nil
					}
				}
			}
		}
	}

	// 3) Try alternative array regex (more permissive)
	if jsonContent := strings.TrimSpace(reJSONArrayAlt.FindString(jsonPart)); jsonContent != "" {
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)
		if decisions, err := tryParseJSON(jsonContent); err == nil {
			log.Printf("✓ Successfully parsed JSON from alternative array regex")
			return decisions, nil
		}
	}

	// 4) Fallback: Search entire text for first object array
	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent != "" {
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)
		if decisions, err := tryParseJSON(jsonContent); err == nil {
			log.Printf("✓ Successfully parsed JSON from standard array regex")
			return decisions, nil
		}
	}

	// 5) Try to extract single object (for TradingView signals that might output single object)
	if jsonContent := strings.TrimSpace(reJSONObject.FindString(jsonPart)); jsonContent != "" {
		jsonContent = fixMissingQuotes(jsonContent)
		if decisions, err := tryParseJSONObject(jsonContent); err == nil {
			log.Printf("✓ Successfully parsed single JSON object")
			return decisions, nil
		}
	}

	// 6) Last resort: Try to repair broken JSON
	if repairedJSON, err := tryRepairJSON(jsonPart); err == nil && repairedJSON != "" {
		// Check if it's a single object or an array
		trimmedRepaired := strings.TrimSpace(repairedJSON)
		if strings.HasPrefix(trimmedRepaired, "{") {
			// Single object - use tryParseJSONObject first
			if decisions, err := tryParseJSONObject(repairedJSON); err == nil {
				log.Printf("✓ Successfully parsed repaired JSON object")
				return decisions, nil
			}
		} else if strings.HasPrefix(trimmedRepaired, "[") {
			// Array - use tryParseJSON
			if decisions, err := tryParseJSON(repairedJSON); err == nil {
				log.Printf("✓ Successfully parsed repaired JSON array")
				return decisions, nil
			}
		}
	}

	// 🔧 Safe Fallback: When AI only outputs reasoning chain without JSON, generate fallback decision (avoid system crash)
	log.Printf("⚠️  [SafeFallback] AI did not output any parseable JSON decision, entering safe wait mode (all parsing methods failed)")

	// Extract reasoning chain summary (max 240 characters)
	cotSummary := jsonPart
	if len(cotSummary) > 240 {
		cotSummary = cotSummary[:240] + "..."
	}

	// Generate fallback decision: all coins enter wait state
	fallbackDecision := Decision{
		Symbol:    "ALL",
		Action:    "wait",
		Reasoning: fmt.Sprintf("model did not output structured JSON decision, entering safe wait mode; summary: %s", cotSummary),
	}

	return []Decision{fallbackDecision}, nil
}

// tryParseJSON attempts to parse JSON content and return decisions with enhanced error reporting
func tryParseJSON(jsonContent string) ([]Decision, error) {
	// First try standard validation
	if err := validateJSONFormat(jsonContent); err != nil {
		// If standard validation fails, try enhanced validation with suggestions
		if valid, suggestion, validateErr := validateJSONWithSuggestions(jsonContent); !valid {
			// Try auto-repair
			if repaired, fixes, repairErr := repairJSONCommonMistakes(jsonContent); repairErr == nil && len(fixes) > 0 {
				log.Printf("✓ Auto-repaired JSON: %v", fixes)
				jsonContent = repaired
			} else {
				return nil, fmt.Errorf("JSON validation failed: %w\nSuggestion: %s\nJSON content: %s", validateErr, suggestion, jsonContent)
			}
		}
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		// Provide more helpful error message
		return nil, fmt.Errorf("JSON parsing failed: %w\nThis usually means missing quotes, commas, or brackets. Check for: unquoted strings, missing commas between fields, unmatched brackets\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

// tryParseJSONObject attempts to parse a single JSON object (for TradingView signals)
func tryParseJSONObject(jsonContent string) ([]Decision, error) {
	// Validate that it's a single object (not an array)
	trimmed := strings.TrimSpace(jsonContent)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, fmt.Errorf("JSON object must start with {, actual: %s", trimmed[:min(20, len(trimmed))])
	}

	var decision Decision
	if err := json.Unmarshal([]byte(jsonContent), &decision); err != nil {
		return nil, fmt.Errorf("JSON object parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return []Decision{decision}, nil
}

// tryRepairJSON attempts to repair common JSON formatting issues
func tryRepairJSON(content string) (string, error) {
	content = strings.TrimSpace(content)

	// Try to extract JSON-like content between various markers
	patterns := []string{
		`\{[^{}]*"symbol"[^{}]*\}`,          // Single object with symbol
		`\{[^{}]*"signal_decision"[^{}]*\}`, // TradingView signal object
		`\{[^{}]*"action"[^{}]*\}`,          // Action-based object
		`\[[\s\S]*?\]`,                      // Array content
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?s)` + pattern)
		if match := re.FindString(content); match != "" {
			// Try to fix common issues
			fixed := fixMissingQuotes(match)
			fixed = strings.ReplaceAll(fixed, "”", "\"") // Fix smart quotes
			fixed = strings.ReplaceAll(fixed, `""`, `"`) // Fix other quote issues
			fixed = strings.ReplaceAll(fixed, "'", "\"") // Fix single quotes

			// Quick validation
			if strings.Contains(fixed, "{") && strings.Contains(fixed, "}") {
				return fixed, nil
			}
		}
	}

	return "", fmt.Errorf("could not repair JSON")
}

// removeThousandsSeparators Removes thousands separator commas in JSON numbers
// Only removes commas between digits, does not affect commas in JSON structure
func removeThousandsSeparators(jsonStr string) string {
	// Use regex to match thousands separators in numbers
	// Pattern: match commas in digit sequences (e.g., 3,426 or 1,234,567)
	// Match one or more digits, followed by comma and 3 digits (can repeat multiple times)
	// Example: 3,426 or 1,234,567 or 98,000
	re := regexp.MustCompile(`(\d{1,3})(,\d{3})+`)
	result := re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// Remove all commas
		return strings.ReplaceAll(match, ",", "")
	})
	return result
}

// fixMissingQuotes Replaces Chinese quotes and full-width characters with English quotes and half-width characters (avoid AI outputting full-width JSON characters causing parsing failure)
func fixMissingQuotes(jsonStr string) string {
	// Replace Chinese quotes
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ Replace full-width brackets, colons, commas (prevent AI from outputting full-width JSON characters)
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B Full-width left square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D Full-width right square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B Full-width left curly brace
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D Full-width right curly brace
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A Full-width colon
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C Full-width comma

	// ⚠️ Replace CJK punctuation marks (AI may output these in Chinese context)
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK left square head bracket U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK right square head bracket U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK left tortoise shell bracket U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK right tortoise shell bracket U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK ideographic comma U+3001

	// ⚠️ Replace full-width spaces with half-width spaces (JSON should not have full-width spaces)
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 Full-width space

	// 🔧 Remove thousands separators in numbers (e.g., 3,426 or 98,000)
	jsonStr = removeThousandsSeparators(jsonStr)

	return jsonStr
}

// validateJSONFormat Validates JSON format, detects common errors
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// Allow arbitrary whitespace (including zero-width) between [ and {
	if !reArrayHead.MatchString(trimmed) {
		// Check if it's a pure number/range array (common error)
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// Check if contains range symbol ~ in numeric contexts (not in string values)
	// Allow ~ in string values (e.g., "~58% discrepancy" in reasoning field)
	// Only flag ~ when it appears outside quoted strings and adjacent to digits
	insideQuotes := false
	escapeNext := false
	for i := 0; i < len(jsonStr); i++ {
		char := jsonStr[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			insideQuotes = !insideQuotes
			continue
		}

		// Only check for ~ when outside quotes
		if !insideQuotes && char == '~' {
			// Check if ~ is adjacent to a digit (numeric range pattern)
			if (i > 0 && jsonStr[i-1] >= '0' && jsonStr[i-1] <= '9') ||
				(i < len(jsonStr)-1 && jsonStr[i+1] >= '0' && jsonStr[i+1] <= '9') {
				return fmt.Errorf("JSON cannot contain range symbol ~ in numeric values, all numbers must be exact single values")
			}
		}
	}

	// Check if contains thousands separators (e.g., 98,000)
	// If found, auto-clean (already cleaned in fixMissingQuotes, this is additional safety check)
	// Use simple pattern matching: digit+comma+3 digits
	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			// If thousands separator still found, log warning (should not happen in theory, already cleaned in fixMissingQuotes)
			log.Printf("⚠️  Thousands separator detected (should have been handled in cleanup phase), position: %s", jsonStr[i:min(i+10, len(jsonStr))])
			// Do not return error, let subsequent JSON parsing handle it (if format still invalid, parsing will fail)
		}
	}

	return nil
}

// validateJSONWithSuggestions Validates JSON and provides repair suggestions for common AI mistakes
func validateJSONWithSuggestions(jsonStr string) (bool, string, error) {
	trimmed := strings.TrimSpace(jsonStr)

	// Basic structure checks
	if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
		suggestion := "JSON must start with '[' (array) or '{' (object). Try wrapping your decisions in square brackets: [...]"
		return false, suggestion, fmt.Errorf("invalid JSON structure: must start with [ or {, got: %s", trimmed[:min(20, len(trimmed))])
	}

	if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed, "{") {
		suggestion := "Array must contain objects. Each decision should be an object like: {\"symbol\": \"BTCUSDT\", \"action\": \"open_long\"}"
		return false, suggestion, fmt.Errorf("array does not contain objects")
	}

	// Check for common AI mistakes
	mistakes := []struct {
		pattern     string
		suggestion  string
		description string
	}{
		{`~\d`, "Remove range symbols (~) from numbers. Use exact values: 100 instead of 95~100", "range symbol in numeric value"},
		{`[\u201c\u201d]`, `Use straight quotes (") instead of smart quotes (")`, "smart quotes detected"},
		{`[\u2018\u2019]`, "Use straight quotes (\") instead of smart quotes (\u2018\u2019)", "smart single quotes detected"},
		{`[\uff1a]`, "Use colon (:) instead of full-width colon (：)", "full-width colon detected"},
		{`[\uff0c]`, "Use comma (,) instead of full-width comma (，)", "full-width comma detected"},
		{`[\uff3b\uff3d]`, "Use square brackets [] instead of full-width brackets ［］", "full-width brackets detected"},
		{`[\uff5b\uff5d]`, "Use curly braces {} instead of full-width braces ｛｝", "full-width braces detected"},
		{`\d{1,3},\d{3}`, "Remove thousands separators from numbers: 50000 instead of 50,000", "thousands separator detected"},
	}

	var foundMistakes []string
	for _, mistake := range mistakes {
		if matched, _ := regexp.MatchString(mistake.pattern, jsonStr); matched {
			foundMistakes = append(foundMistakes, mistake.description)
		}
	}

	if len(foundMistakes) > 0 {
		suggestion := "Common issues found: " + strings.Join(foundMistakes, "; ") + ". Try using the repair function."
		return false, suggestion, fmt.Errorf("JSON contains formatting errors: %v", foundMistakes)
	}

	// Try to parse as JSON
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		suggestion := "Try these fixes: 1) Ensure all quotes are straight (\") 2) Check for missing commas 3) Verify brackets are balanced 4) Remove any non-JSON text"
		return false, suggestion, err
	}

	return true, "", nil
}

// repairJSONCommonMistakes Attempts to automatically fix common JSON formatting mistakes
func repairJSONCommonMistakes(jsonStr string) (string, []string, error) {
	original := jsonStr
	var fixes []string

	// Fix smart quotes
	if strings.Contains(jsonStr, "\u201c") || strings.Contains(jsonStr, "\u201d") {
		jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
		jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
		fixes = append(fixes, "replaced smart quotes with straight quotes")
	}

	// Fix smart single quotes
	if strings.Contains(jsonStr, "\u2018") || strings.Contains(jsonStr, "\u2019") {
		jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
		jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")
		fixes = append(fixes, "replaced smart single quotes")
	}

	// Fix full-width characters
	replacements := map[string]string{
		"［": "[", "］": "]", "｛": "{", "｝": "}", "：": ":", "，": ",",
	}
	for old, new := range replacements {
		if strings.Contains(jsonStr, old) {
			jsonStr = strings.ReplaceAll(jsonStr, old, new)
			fixes = append(fixes, fmt.Sprintf("replaced %s with %s", old, new))
		}
	}

	// Fix thousands separators in numbers
	if matched, _ := regexp.MatchString(`\d{1,3},\d{3}`, jsonStr); matched {
		re := regexp.MustCompile(`(\d{1,3}),(\d{3})`)
		jsonStr = re.ReplaceAllString(jsonStr, "$1$2")
		fixes = append(fixes, "removed thousands separators from numbers")
	}

	// Try to validate the repaired JSON
	if valid, _, err := validateJSONWithSuggestions(jsonStr); valid {
		return jsonStr, fixes, nil
	} else {
		return original, []string{}, fmt.Errorf("auto-repair failed: %w", err)
	}
}

// min Returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes Removes zero-width characters and BOM to avoid invisible prefixes breaking validation
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen Normalizes opening "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions Validates all decisions (requires account information, leverage configuration, and position context)
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, config StrategyConfig, positionCount int, marginUsedPct float64, positions []PositionInfo) error {
	// Enforce maximum position count at batch level (strategy-studio is single source of truth)
	newOpens := 0
	for _, d := range decisions {
		signalDecisionLower := strings.ToLower(strings.TrimSpace(d.SignalDecision))
		if signalDecisionLower == "reject" {
			continue
		}
		if d.Action == "open_long" || d.Action == "open_short" {
			newOpens++
		}
	}
	if newOpens > 0 && config.MaxPositions > 0 && positionCount+newOpens > config.MaxPositions {
		return fmt.Errorf("opening too many positions: existing=%d, new=%d, max allowed=%d (strategy-studio limit)",
			positionCount, newOpens, config.MaxPositions)
	}
	// Enforce margin usage limit (strategy-studio single source of truth)
	if config.MarginUsageLimit > 0 && marginUsedPct >= config.MarginUsageLimit {
		return fmt.Errorf("margin usage %.1f%% >= limit %.1f%% (strategy-studio MarginUsageLimit)", marginUsedPct, config.MarginUsageLimit)
	}

	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, config, positions); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

// validateDecision Validates validity of a single decision
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, config StrategyConfig, positions []PositionInfo) error {
	// 🔧 TradingView signal reject decision: if signal_decision is "reject" (case-insensitive), skip all validation
	// According to prompt instructions, reject decisions do not need to provide other trading parameters (including action)
	signalDecisionLower := strings.ToLower(strings.TrimSpace(d.SignalDecision))
	if signalDecisionLower == "reject" {
		return nil
	}

	// 🔧 Detect if it's a TradingView decision (via TradingViewSignalID field)
	isTradingViewDecision := d.TradingViewSignalID != ""

	// Validate action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		// 🔧 Provide more detailed error information, especially for TradingView decisions
		if isTradingViewDecision {
			if d.Action == "" {
				if signalDecisionLower != "" {
					return fmt.Errorf("TradingView signal decision %s must provide action field (except reject decisions)", d.SignalDecision)
				}
				return fmt.Errorf("TradingView decision must provide action field (or signal_decision is reject)")
			}
			return fmt.Errorf("TradingView decision action is invalid: %s (valid values: open_long, open_short, close_long, close_short, update_stop_loss, update_take_profit, partial_close, hold, wait)", d.Action)
		}
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	// Position opening operations must provide complete parameters
	if d.Action == "open_long" || d.Action == "open_short" {
		// Enforce minimum confidence for entries (strategy-studio setting)
		if d.Confidence < config.MinConfidenceForEntry {
			return fmt.Errorf("confidence too low for opening position: %d, must be ≥%d (strategy-studio MinConfidenceForEntry)",
				d.Confidence, config.MinConfidenceForEntry)
		}

		// Use configured leverage limits based on coin type
		maxLeverage := altcoinLeverage                                // Altcoins use configured leverage
		maxPositionValue := accountEquity * config.AltcoinPositionMax // Altcoins max from config
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage                                // BTC and ETH use configured leverage
			maxPositionValue = accountEquity * config.BTCETHPositionMax // BTC/ETH max from config
		}

		// ✅ Fallback mechanism: automatically correct to upper limit when leverage exceeds limit (instead of directly rejecting decision)
		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			log.Printf("⚠️  [Leverage Fallback] %s leverage exceeds limit (%dx > %dx), automatically adjusted to upper limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage // Automatically correct to upper limit
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		// ✅ Validate minimum opening amount (prevent quantity formatting to 0 error)
		// Use configurable minimum opening amounts
		minPositionSizeGeneral := config.MinOpeningAmount
		minPositionSizeBTCETH := config.MinOpeningAmountBTCETH

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT (due to high price and precision limits, to avoid quantity rounding to 0)", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT (Binance minimum notional value requirement)", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		// Validate position value upper limit (add 1% tolerance to avoid floating point precision issues)
		tolerance := maxPositionValue * 0.01 // 1% tolerance
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH single coin position value cannot exceed %.0f USDT (10x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("altcoin single coin position value cannot exceed %.0f USDT (1.5x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		// Validate stop loss and take profit reasonableness
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("when going long, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("when going short, stop loss price must be greater than take profit price")
			}
		}

		// Validate risk-reward ratio (must be ≥1:3)
		// Calculate entry price (assume current market price)
		var entryPrice float64
		if d.Action == "open_long" {
			// Long: entry price between stop loss and take profit
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // Assume entry at 20% position
		} else {
			// Short: entry price between stop loss and take profit
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // Assume entry at 20% position
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// Hard constraint: risk-reward ratio must be ≥ config.MinRiskRewardRatio
		if riskRewardRatio < config.MinRiskRewardRatio {
			return fmt.Errorf("risk-reward ratio too low (%.2f:1), must be ≥%.2f:1 [risk:%.2f%% reward:%.2f%%] [stop loss:%.2f take profit:%.2f]",
				riskRewardRatio, config.MinRiskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	// Closing / partial close operations must respect minimum holding time (if we can determine it)
	if d.Action == "close_long" || d.Action == "close_short" || d.Action == "partial_close" {
		if config.MinHoldingTimeMinutes > 0 && d.Symbol != "" {
			var targetSide string
			switch d.Action {
			case "close_long":
				targetSide = "long"
			case "close_short":
				targetSide = "short"
			default: // partial_close: allow either side, match by symbol
				targetSide = ""
			}

			var matched *PositionInfo
			for i := range positions {
				pos := &positions[i]
				if !strings.EqualFold(pos.Symbol, d.Symbol) {
					continue
				}
				if targetSide != "" && !strings.EqualFold(pos.Side, targetSide) {
					continue
				}
				matched = pos
				break
			}

			if matched != nil {
				if matched.UpdateTime == 0 {
					// We don't know the holding time, so warn but allow to avoid false blocks
					log.Printf("⚠️  Unable to enforce MinHoldingTimeMinutes for %s (missing UpdateTime), allowing close action", matched.Symbol)
				} else {
					nowMs := time.Now().UnixMilli()
					heldMs := nowMs - matched.UpdateTime
					if heldMs > 0 {
						heldMinutes := int(heldMs / (1000 * 60))
						if heldMinutes < config.MinHoldingTimeMinutes {
							return fmt.Errorf("position %s held for only %d minutes, must be ≥%d minutes before closing (strategy-studio MinHoldingTimeMinutes)",
								matched.Symbol, heldMinutes, config.MinHoldingTimeMinutes)
						}
					}
				}
			}
		}
	}

	// Dynamic stop loss adjustment validation
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("new stop loss price must be greater than 0: %.2f", d.NewStopLoss)
		}
	}

	// Dynamic take profit adjustment validation
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("new take profit price must be greater than 0: %.2f", d.NewTakeProfit)
		}
	}

	// Partial close validation
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("close percentage must be between 0-100: %.1f", d.ClosePercentage)
		}
	}

	return nil
}
