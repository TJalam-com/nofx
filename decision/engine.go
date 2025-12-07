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

// 预编译正则表达式（性能优化：避免每次调用时重新编译）
var (
	// ✅ 安全的正則：精確匹配 ```json 代碼塊
	// 使用反引號 + 拼接避免轉義問題
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// 新增：XML标签提取（支持思维链中包含任何字符）
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // 历史最高收益率（百分比）
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // 未实现盈亏
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                             `json:"current_time"`
	RuntimeMinutes  int                                `json:"runtime_minutes"`
	CallCount       int                                `json:"call_count"`
	Account         AccountInfo                        `json:"account"`
	Positions       []PositionInfo                     `json:"positions"`
	CandidateCoins  []CandidateCoin                    `json:"candidate_coins"`
	PromptVariant   string                             `json:"prompt_variant,omitempty"`
	MarketDataMap   map[string]*market.Data            `json:"-"` // 不序列化，但内部使用
	MultiTFMarket   map[string]map[string]*market.Data `json:"-"`
	OITopDataMap    map[string]*OITopData              `json:"-"` // OI Top数据映射
	Performance     interface{}                        `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                                `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                                `json:"-"` // 山寨币杠杆倍数（从配置读取）
}

// Decision AI的交易决策
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // 用于 partial_close (0-100)

	// 通用参数
	Confidence int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning  string  `json:"reasoning"`

	// TradingView信号相关字段
	TradingViewSignalID string `json:"tradingview_signal_id,omitempty"` // TradingView警报ID
	SignalDecision      string `json:"signal_decision,omitempty"`       // "accept", "reject", "modify"
	
	// Parent trade signal相关字段
	ParentSignalID      string `json:"parent_signal_id,omitempty"`      // 父交易员信号ID
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
	// AIRequestDurationMs 记录 AI API 调用耗时（毫秒）方便排查延迟问题
	AIRequestDurationMs int64 `json:"ai_request_duration_ms,omitempty"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient mcp.AIClient, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// 1. 为所有币种获取市场数据（若上层已提供，则无需重复拉取）
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataForContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	} else if ctx.OITopDataMap == nil {
		// 确保 OI 数据映射已初始化，避免后续访问空指针
		ctx.OITopDataMap = make(map[string]*OITopData)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
		customPrompt,
		overrideBase,
		templateName,
		ctx.PromptVariant,
	)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("failed to call AI API: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)

	// 无论是否有错误，都要保存 SystemPrompt 和 UserPrompt（用于调试和决策未执行后的问题定位）
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // 保存系统prompt
		decision.UserPrompt = userPrompt     // 保存输入prompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // 保存系统prompt
	decision.UserPrompt = userPrompt     // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于阈值的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		// 💡 OI 門檻配置：用戶可根據風險偏好調整
		const minOIThresholdMillions = 15.0 // 可調整：15M(保守) / 10M(平衡) / 8M(寬鬆) / 5M(激進)

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < minOIThresholdMillions {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < %.1fM)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
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

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
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
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// 获取基础prompt（使用指定的模板）
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName, variant)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 Custom Trading Strategy\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("Note: The above custom strategy supplements the base rules and must not violate the fundamental risk control principles.\n")

	return sb.String()
}

// BuildSystemPromptWithTradingView 构建包含TradingView信号分析的 System Prompt
func BuildSystemPromptWithTradingView(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string) string {
	// 先构建基础prompt
	basePrompt := buildSystemPromptWithCustom(accountEquity, btcEthLeverage, altcoinLeverage, customPrompt, overrideBase, templateName, variant)

	// 添加TradingView信号分析部分
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📡 TradingView Signal Analysis Mode\n\n")
	sb.WriteString("You are analyzing a webhook signal from TradingView. The signal contains the following information:\n")
	sb.WriteString("- Symbol: Trading pair\n")
	sb.WriteString("- Action: Operation type (buy = open long, sell = open short)\n")
	sb.WriteString("- Entry: Suggested entry price\n")
	sb.WriteString("- Stop Loss (SL): Suggested stop loss price\n")
	sb.WriteString("- Take Profit (TP): Suggested take profit price\n")
	sb.WriteString("- Quantity: Suggested quantity\n\n")
	sb.WriteString("## Decision Options\n\n")
	sb.WriteString("You must make one of the following three decisions for each TradingView signal:\n\n")
	sb.WriteString("1. **accept**: The signal aligns with your trading strategy, execute the trade using the parameters provided by the signal\n")
	sb.WriteString("2. **reject**: The signal does not align with your trading strategy or risk control requirements, do not execute the trade\n")
	sb.WriteString("3. **modify**: The signal direction is correct, but parameters need adjustment, execute the trade using your optimized parameters\n\n")
	sb.WriteString("## Output Format Requirements\n\n")
	sb.WriteString("In the JSON decision output, you must include the following fields:\n")
	sb.WriteString("- `signal_decision`: Must be one of \"accept\", \"reject\", or \"modify\"\n")
	sb.WriteString("- `tradingview_signal_id`: TradingView alert ID (obtained from user prompt)\n")
	sb.WriteString("- If `signal_decision` is \"reject\", you do not need to provide other trading parameters\n")
	sb.WriteString("- If `signal_decision` is \"accept\" or \"modify\", you must provide complete trading parameters (symbol, action, leverage, position_size_usd, stop_loss, take_profit, etc.)\n\n")
	sb.WriteString("## Analysis Points\n\n")
	sb.WriteString("- Carefully analyze the consistency between current market data (price, indicators, OI, etc.) and the signal\n")
	sb.WriteString("- Check if signal parameters meet risk control requirements (stop loss/take profit ratio, position size, etc.)\n")
	sb.WriteString("- Consider current account status (balance, positions, margin usage rate, etc.)\n")
	sb.WriteString("- If signal direction is correct but parameters are unreasonable, use \"modify\" and provide optimized parameters\n")
	sb.WriteString("- If signal completely does not align with strategy or risk is too high, use \"reject\" and explain the reason\n\n")

	return sb.String()
}

// BuildSystemPromptWithParentSignal 构建包含父交易员信号分析的 System Prompt
func BuildSystemPromptWithParentSignal(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string) string {
	// 先构建基础prompt（使用risk_management模板）
	basePrompt := buildSystemPromptWithCustom(accountEquity, btcEthLeverage, altcoinLeverage, customPrompt, overrideBase, templateName, variant)
	
	// 添加父交易员信号分析部分
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📡 Parent Trade Signal Analysis Mode\n\n")
	sb.WriteString("You are analyzing a trade signal from a parent trader that you are following. The signal contains the following information:\n")
	sb.WriteString("- Parent Trader: Name and ID of the trader you are following\n")
	sb.WriteString("- Symbol: Trading pair\n")
	sb.WriteString("- Action: Operation type (open_long, open_short, close_long, close_short, etc.)\n")
	sb.WriteString("- Leverage: Suggested leverage\n")
	sb.WriteString("- Position Size: Suggested position size in USDT\n")
	sb.WriteString("- Stop Loss: Suggested stop loss price\n")
	sb.WriteString("- Take Profit: Suggested take profit price\n")
	sb.WriteString("- Reasoning: Parent trader's reasoning for this trade\n\n")
	sb.WriteString("## Decision Options\n\n")
	sb.WriteString("You must make one of the following three decisions for each parent trade signal:\n\n")
	sb.WriteString("1. **accept**: The signal aligns with your risk management strategy, execute the trade using parameters scaled to your account size\n")
	sb.WriteString("2. **reject**: The signal does not align with your risk control requirements or account state, do not execute the trade\n")
	sb.WriteString("3. **modify**: The signal direction is correct, but parameters need adjustment for your account size/risk tolerance, execute with your optimized parameters\n\n")
	sb.WriteString("## Output Format Requirements\n\n")
	sb.WriteString("In the JSON decision output, you must include the following fields:\n")
	sb.WriteString("- `signal_decision`: Must be one of \"accept\", \"reject\", or \"modify\"\n")
	sb.WriteString("- `parent_signal_id`: Parent signal ID (obtained from user prompt)\n")
	sb.WriteString("- If `signal_decision` is \"reject\", you do not need to provide other trading parameters\n")
	sb.WriteString("- If `signal_decision` is \"accept\" or \"modify\", you must provide complete trading parameters (symbol, action, leverage, position_size_usd, stop_loss, take_profit, etc.)\n\n")
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
	sb.WriteString("- Better to skip a trade (reject) than take excessive risk\n\n")

	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string, variant string) string {
	var sb strings.Builder

	// 1. 加载提示词模板（核心交易策略部分）
	if templateName == "" {
		templateName = "default" // 默认使用 default 模板
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// 如果模板不存在，记录错误并使用 default
		log.Printf("⚠️  提示词模板 '%s' 不存在，使用 default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// 如果连 default 都不存在，使用内置的简化版本
			log.Printf("❌ Unable to load any prompt template, using built-in simplified version")
			sb.WriteString("You are a professional cryptocurrency trading AI. Please make trading decisions based on market data.\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// 2. 交易模式变体
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥70\n- Higher positions allowed, but must strictly set stop loss and explain profit-to-loss ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals converge\n- Prioritize preserving cash, must pause for multiple cycles after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick execution\n- If price does not move as expected within two bars, immediately reduce position or stop loss\n\n")
	}

	// 3. 硬约束（风险控制）
	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("1. Risk-reward ratio: Must be ≥ 1:3 (risk 1% to earn 3%+ returns)\n")
	sb.WriteString("2. Maximum positions: 3 coins (quality > quantity)\n")
	sb.WriteString(fmt.Sprintf("3. Single coin position: Altcoins %.0f-%.0f USDT | BTC/ETH %.0f-%.0f USDT\n",
		accountEquity*0.8, accountEquity*1.5, accountEquity*5, accountEquity*10))
	sb.WriteString(fmt.Sprintf("4. Leverage limits: **Altcoins maximum %dx leverage** | **BTC/ETH maximum %dx leverage**\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString("5. Margin usage rate ≤ 90%\n")
	sb.WriteString("6. Opening amount: Recommended ≥12 USDT (exchange minimum notional value 10 USDT + safety margin)\n\n")

	// 4. 交易频率与信号质量
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
	sb.WriteString("Freely use any effective analysis methods, but **confidence ≥75** required to open positions; avoid single indicators, contradictory signals, sideways consolidation, immediately reopening after closing positions, and other low-quality behaviors.\n\n")

	// 5. 夏普比率驱动的自适应
	sb.WriteString("# 🧬 Sharpe Ratio Self-Evolution\n\n")
	sb.WriteString("- Sharpe < -0.5: Immediately stop trading, wait at least 6 cycles and conduct deep review\n")
	sb.WriteString("- -0.5 ~ 0: Only trade with confidence >80, and reduce frequency\n")
	sb.WriteString("- 0 ~ 0.7: Maintain current strategy\n")
	sb.WriteString("- >0.7: Allow moderate position increase, but still follow risk control\n\n")

	// 6. 决策流程提示
	sb.WriteString("# 📋 Decision Process\n\n")
	sb.WriteString("1. Review Sharpe ratio/P&L → Whether to reduce frequency or pause\n")
	sb.WriteString("2. Check positions → Whether to take profit/stop loss/adjust\n")
	sb.WriteString("3. Scan candidate coins + multiple timeframes → Whether strong signals exist\n")
	sb.WriteString("4. Write chain of thought first, then output structured JSON\n\n")

	// 7. 输出格式 - 动态生成
	sb.WriteString("# Output Format (Strict Compliance)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON to avoid parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your chain of thought analysis...\n")
	sb.WriteString("- Briefly analyze your thinking process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"Downtrend + MACD death cross\"},\n", btcEthLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"SOLUSDT\", \"action\": \"update_stop_loss\", \"new_stop_loss\": 155, \"reasoning\": \"Move stop loss to breakeven\"},\n")
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"Take profit exit\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Descriptions\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | update_stop_loss | update_take_profit | partial_close | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100 (recommended ≥75 for opening positions)\n")
	sb.WriteString("- Required when opening positions: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n")
	sb.WriteString("- Required for update_stop_loss: new_stop_loss (note: it's new_stop_loss, not stop_loss)\n")
	sb.WriteString("- Required for update_take_profit: new_take_profit (note: it's new_take_profit, not take_profit)\n")
	sb.WriteString("- Required for partial_close: close_percentage (0-100)\n\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("Time: %s | Cycle: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | P&L %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | Holding duration %d minutes", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | Holding duration %d hours %d minutes", durationHour, durationMinRemainder)
				}
			}

			// 计算仓位价值（用于 partial_close 检查）
			positionValue := math.Abs(pos.Quantity) * pos.MarkPrice

			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Quantity %.4f | Position Value %.2f USDT | P&L %+.2f%% | P&L Amount %+.2f USDT | Peak Return %.2f%% | Leverage %dx | Margin %.0f | Liquidation %.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("Current positions: None\n\n")
	}

	// 候选币种（完整市场数据）
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

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 夏普比率（直接传值，不要复杂格式化）
	if ctx.Performance != nil {
		// 直接从interface{}中提取SharpeRatio
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

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
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

// ParseFullDecisionResponse 解析AI的完整决策响应（公共包装函数）
// 从Context中提取账户权益和杠杆参数，调用私有parseFullDecisionResponse函数
func ParseFullDecisionResponse(ctx *Context, aiResponse string) (*FullDecision, error) {
	return parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
	)
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 方法1: 优先尝试提取 <reasoning> 标签内容
	if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
		log.Printf("✓ 使用 <reasoning> 标签提取思维链")
		return strings.TrimSpace(match[1])
	}

	// 方法2: 如果没有 <reasoning> 标签，但有 <decision> 标签，提取 <decision> 之前的内容
	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		log.Printf("✓ 提取 <decision> 标签之前的内容作为思维链")
		return strings.TrimSpace(response[:decisionIdx])
	}

	// 方法3: 后备方案 - 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		log.Printf("⚠️  使用旧版格式（[ 字符分离）提取思维链")
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到任何标记，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 预清洗：去零宽/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 关键修复 (Critical Fix)：在正则匹配之前就先修复全角字符！
	// 否则正则表达式 \[ 无法匹配全角的 ［
	s = fixMissingQuotes(s)

	// 方法1: 优先尝试从 <decision> 标签中提取
	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		log.Printf("✓ 使用 <decision> 标签提取JSON")
	} else {
		// 后备方案：使用整个响应
		jsonPart = s
		log.Printf("⚠️  未找到 <decision> 标签，使用全文搜索JSON")
	}

	// 修复 jsonPart 中的全角字符
	jsonPart = fixMissingQuotes(jsonPart)

	// 1) 优先从 ```json 代码块中提取
	if m := reJSONFence.FindStringSubmatch(jsonPart); len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // 把 "[ {" 规整为 "[{"
		jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	// 2) 退而求其次 (Fallback)：全文寻找首个对象数组
	// 注意：此时 jsonPart 已经过 fixMissingQuotes()，全角字符已转换为半角
	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		// 🔧 安全回退 (Safe Fallback)：当AI只输出思维链没有JSON时，生成保底决策（避免系统崩溃）
		log.Printf("⚠️  [SafeFallback] AI未输出JSON决策，进入安全等待模式 (AI response without JSON, entering safe wait mode)")

		// 提取思维链摘要（最多 240 字符）
		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// 生成保底决策：所有币种进入 wait 状态
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("model did not output structured JSON decision, entering safe wait mode; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	// 🔧 规整格式（此时全角字符已在前面修复过）
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）

	// 🔧 验证 JSON 格式（检测常见错误）
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
	}

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// removeThousandsSeparators 移除JSON数字中的千位分隔符逗号
// 只移除数字之间的逗号，不影响JSON结构中的逗号
func removeThousandsSeparators(jsonStr string) string {
	// 使用正则表达式匹配数字中的千位分隔符
	// 模式: 匹配数字序列中的逗号（如 3,426 或 1,234,567）
	// 匹配一个或多个数字，后跟逗号和3位数字（可以重复多次）
	// 例如: 3,426 或 1,234,567 或 98,000
	re := regexp.MustCompile(`(\d{1,3})(,\d{3})+`)
	result := re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// 移除所有逗号
		return strings.ReplaceAll(match, ",", "")
	})
	return result
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	// 🔧 移除数字中的千位分隔符（如 3,426 或 98,000）
	jsonStr = removeThousandsSeparators(jsonStr)

	return jsonStr
}

// validateJSONFormat 验证 JSON 格式，检测常见错误
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// 允许 [ 和 { 之间存在任意空白（含零宽）
	if !reArrayHead.MatchString(trimmed) {
		// 检查是否是纯数字/范围数组（常见错误）
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// 检查是否包含范围符号 ~（LLM 常见错误）
	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be exact single values")
	}

	// 检查是否包含千位分隔符（如 98,000）
	// 如果发现，自动清理（因为已经在 fixMissingQuotes 中清理过，这里作为额外安全检查）
	// 使用简单的模式匹配：数字+逗号+3位数字
	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			// 如果仍然发现千位分隔符，记录警告（理论上不应该发生，因为已在 fixMissingQuotes 中清理）
			log.Printf("⚠️  检测到千位分隔符（应在清理阶段已处理），位置: %s", jsonStr[i:min(i+10, len(jsonStr))])
			// 不返回错误，让后续的 JSON 解析来处理（如果格式仍然无效，解析会失败）
		}
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 🔧 TradingView信号拒绝决策：如果signal_decision为"reject"（不区分大小写），跳过所有验证
	// 根据prompt说明，reject决策不需要提供其他交易参数（包括action）
	signalDecisionLower := strings.ToLower(strings.TrimSpace(d.SignalDecision))
	if signalDecisionLower == "reject" {
		return nil
	}

	// 🔧 检测是否为TradingView决策（通过TradingViewSignalID字段）
	isTradingViewDecision := d.TradingViewSignalID != ""

	// 验证action
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
		// 🔧 提供更详细的错误信息，特别是对于TradingView决策
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

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		// ✅ Fallback 机制：杠杆超限时自动修正为上限值（而不是直接拒绝决策）
		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			log.Printf("⚠️  [Leverage Fallback] %s 杠杆超限 (%dx > %dx)，自动调整为上限值 %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage // 自动修正为上限值
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		// ✅ 验证最小开仓金额（防止数量格式化为 0 的错误）
		// Binance 最小名义价值 10 USDT + 安全边际
		const minPositionSizeGeneral = 12.0 // 10 + 20% 安全边际
		const minPositionSizeBTCETH = 60.0  // BTC/ETH 因价格高和精度限制需要更大金额（更灵活）

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT (due to high price and precision limits, to avoid quantity rounding to 0)", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT (Binance minimum notional value requirement)", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
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

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("when going long, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("when going short, stop loss price must be greater than take profit price")
			}
		}

		// 验证风险回报比（必须≥1:3）
		// 计算入场价（假设当前市价）
		var entryPrice float64
		if d.Action == "open_long" {
			// 做多：入场价在止损和止盈之间
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // 假设在20%位置入场
		} else {
			// 做空：入场价在止损和止盈之间
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // 假设在20%位置入场
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

		// 硬约束：风险回报比必须≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("risk-reward ratio too low (%.2f:1), must be ≥3.0:1 [risk:%.2f%% reward:%.2f%%] [stop loss:%.2f take profit:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	// 动态调整止损验证
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("new stop loss price must be greater than 0: %.2f", d.NewStopLoss)
		}
	}

	// 动态调整止盈验证
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("new take profit price must be greater than 0: %.2f", d.NewTakeProfit)
		}
	}

	// 部分平仓验证
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("close percentage must be between 0-100: %.1f", d.ClosePercentage)
		}
	}

	return nil
}
