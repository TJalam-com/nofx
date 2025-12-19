package decision

import (
	"fmt"
	"strings"
)

// JSONFormatTemplate represents a JSON format template structure
type JSONFormatTemplate struct {
	FormatType   string // "standard", "tradingview", "parent"
	Requirements string // Format requirements text
	Example      string // Complete example with tags
	FieldDesc    string // Field descriptions
}

// GetStandardJSONFormat returns the standard trading decision JSON format template
func GetStandardJSONFormat(accountEquity float64, btcEthLeverage, altcoinLeverage int) string {
	var sb strings.Builder

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
	// Use configured position ratio: BTC/ETH max 10x account equity (matches validation limit)
	btcEthPositionSize := accountEquity * 10
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"Downtrend + MACD death cross\"},\n", btcEthLeverage, btcEthPositionSize))
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
	sb.WriteString("**IMPORTANT: All numeric values in your JSON decision output must be calculated numbers, not formulas or expressions.**\n")
	sb.WriteString("- ✅ Correct: `\"position_size_usd\": 2200`, `\"stop_loss\": 95000`, `\"leverage\": 5`\n")
	sb.WriteString("- ❌ Wrong: `\"position_size_usd\": \"available_margin * 5\"`, `\"stop_loss\": \"entry_price * 0.95\"`, `\"leverage\": \"max_leverage\"`\n\n")

	return sb.String()
}

// GetTradingViewJSONFormat returns the TradingView signal JSON format template
func GetTradingViewJSONFormat() string {
	var sb strings.Builder

	sb.WriteString("## Output Format Requirements\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON to avoid parsing errors**\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your analysis of the TradingView signal...\n")
	sb.WriteString("- Analyze market conditions and signal parameters\n")
	sb.WriteString("- Evaluate risk-reward ratio\n")
	sb.WriteString("- Consider account status and risk tolerance\n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"symbol\": \"SOLUSDT\",\n")
	sb.WriteString("  \"action\": \"open_short\",\n")
	sb.WriteString("  \"signal_decision\": \"accept\",\n")
	sb.WriteString("  \"tradingview_signal_id\": \"alert_123\",\n")
	sb.WriteString("  \"leverage\": 3,\n")
	sb.WriteString("  \"position_size_usd\": 1500,\n")
	sb.WriteString("  \"stop_loss\": 130.00,\n")
	sb.WriteString("  \"take_profit\": 120.00,\n")
	sb.WriteString("  \"reasoning\": \"Signal parameters meet risk criteria\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Requirements\n\n")
	sb.WriteString("In the JSON decision output, you must include the following fields:\n")
	sb.WriteString("- `signal_decision`: Must be one of \"accept\", \"reject\", or \"modify\"\n")
	sb.WriteString("- `tradingview_signal_id`: TradingView alert ID (obtained from user prompt)\n")
	sb.WriteString("- If `signal_decision` is \"reject\", you do not need to provide other trading parameters\n")
	sb.WriteString("- If `signal_decision` is \"accept\" or \"modify\", you must provide complete trading parameters (symbol, action, leverage, position_size_usd, stop_loss, take_profit, etc.)\n\n")
	sb.WriteString("**IMPORTANT: All numeric values in your JSON decision output must be calculated numbers, not formulas or expressions.**\n")
	sb.WriteString("- ✅ Correct: `\"position_size_usd\": 1500`, `\"stop_loss\": 130.00`, `\"leverage\": 3`\n")
	sb.WriteString("- ❌ Wrong: `\"position_size_usd\": \"available_margin * 3\"`, `\"stop_loss\": \"entry * 1.05\"`\n\n")

	return sb.String()
}

// GetParentSignalJSONFormat returns the parent signal JSON format template
func GetParentSignalJSONFormat() string {
	var sb strings.Builder

	sb.WriteString("## Output Format Requirements\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON to avoid parsing errors**\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your analysis of the parent trader signal...\n")
	sb.WriteString("- Evaluate if signal fits your account size\n")
	sb.WriteString("- Check risk-reward ratio for your account\n")
	sb.WriteString("- Consider scaling position size appropriately\n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"symbol\": \"BTCUSDT\",\n")
	sb.WriteString("  \"action\": \"open_long\",\n")
	sb.WriteString("  \"signal_decision\": \"modify\",\n")
	sb.WriteString("  \"parent_signal_id\": \"signal_456\",\n")
	sb.WriteString("  \"leverage\": 5,\n")
	sb.WriteString("  \"position_size_usd\": 2200,\n")
	sb.WriteString("  \"stop_loss\": 95000,\n")
	sb.WriteString("  \"take_profit\": 105000,\n")
	sb.WriteString("  \"reasoning\": \"Scaling down position size for my account\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Requirements\n\n")
	sb.WriteString("In the JSON decision output, you must include the following fields:\n")
	sb.WriteString("- `signal_decision`: Must be one of \"accept\", \"reject\", or \"modify\"\n")
	sb.WriteString("- `parent_signal_id`: Parent signal ID (obtained from user prompt)\n")
	sb.WriteString("- If `signal_decision` is \"reject\", you do not need to provide other trading parameters\n")
	sb.WriteString("- If `signal_decision` is \"accept\" or \"modify\", you must provide complete trading parameters (symbol, action, leverage, position_size_usd, stop_loss, take_profit, etc.)\n\n")
	sb.WriteString("**IMPORTANT: All numeric values in your JSON decision output must be calculated numbers, not formulas or expressions.**\n")
	sb.WriteString("- ✅ Correct: `\"position_size_usd\": 2200`, `\"stop_loss\": 95000`, `\"leverage\": 5`\n")
	sb.WriteString("- ❌ Wrong: `\"position_size_usd\": \"parent_size * 0.5\"`, `\"stop_loss\": \"parent_sl\"`\n\n")

	return sb.String()
}

// HasJSONFormatSection checks if a prompt already contains a JSON format section
func HasJSONFormatSection(prompt string) bool {
	lowerPrompt := strings.ToLower(prompt)
	// Check for common format section indicators
	indicators := []string{
		"output format",
		"<decision>",
		"```json",
		"signal_decision",
		"format requirements",
	}

	for _, indicator := range indicators {
		if strings.Contains(lowerPrompt, indicator) {
			return true
		}
	}

	return false
}
