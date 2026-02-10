package logger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "nofx/config"
)

// DecisionRecord Decision record
type DecisionRecord struct {
	Timestamp      time.Time          `json:"timestamp"`       // Decision timestamp
	CycleNumber    int                `json:"cycle_number"`    // Cycle number
	SystemPrompt   string             `json:"system_prompt"`   // System prompt (system prompt sent to AI)
	InputPrompt    string             `json:"input_prompt"`    // Input prompt sent to AI
	CoTTrace       string             `json:"cot_trace"`       // AI reasoning chain (output)
	DecisionJSON   string             `json:"decision_json"`   // Decision JSON
	AccountState   AccountSnapshot    `json:"account_state"`   // Account state snapshot
	Positions      []PositionSnapshot `json:"positions"`       // Position snapshots
	CandidateCoins []string           `json:"candidate_coins"` // Candidate coin list
	Decisions      []DecisionAction   `json:"decisions"`       // Executed decisions
	ExecutionLog   []string           `json:"execution_log"`   // Execution log
	Success        bool               `json:"success"`         // Whether successful
	ErrorMessage   string             `json:"error_message"`   // Error message (if any)
	// AIRequestDurationMs Records AI API call duration (milliseconds) for performance evaluation
	AIRequestDurationMs int64  `json:"ai_request_duration_ms,omitempty"`
	RawResponse         string `json:"raw_response,omitempty"` // Raw AI response for debugging parse failures
}

// AccountSnapshot Account state snapshot
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"` // Records the initial balance baseline at that time
}

// PositionSnapshot Position snapshot
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
}

// DecisionAction Decision action
type DecisionAction struct {
	Action     string    `json:"action"`                // open_long, open_short, close_long, close_short, update_stop_loss, update_take_profit, partial_close
	Symbol     string    `json:"symbol"`                // Trading symbol
	Quantity   float64   `json:"quantity"`              // Quantity (used for partial close)
	Leverage   int       `json:"leverage"`              // Leverage (when opening position)
	Price      float64   `json:"price"`                 // Execution price
	OrderID    int64     `json:"order_id"`              // Order ID
	Timestamp  time.Time `json:"timestamp"`             // Execution timestamp
	Success    bool      `json:"success"`               // Whether successful
	Error      string    `json:"error"`                 // Error message
	StopLoss   float64   `json:"stop_loss,omitempty"`   // Stop loss price
	TakeProfit float64   `json:"take_profit,omitempty"` // Take profit price
	Confidence int       `json:"confidence,omitempty"`  // Confidence level (0-100)
}

// IDecisionLogger Decision logger interface
type IDecisionLogger interface {
	// LogDecision Logs a decision
	LogDecision(record *DecisionRecord) error
	// GetLatestRecords Gets the latest N records (chronological order: oldest to newest)
	GetLatestRecords(n int) ([]*DecisionRecord, error)
	// GetRecordByDate Gets all records for a specific date
	GetRecordByDate(date time.Time) ([]*DecisionRecord, error)
	// CleanOldRecords Cleans old records older than N days
	CleanOldRecords(days int) error
	// GetStatistics Gets statistics
	GetStatistics() (*Statistics, error)
	// AnalyzePerformance Analyzes trading performance for the last N cycles
	// database can be nil or a database interface that provides GetPositionHistory method
	AnalyzePerformance(lookbackCycles int, traderID string, database interface{}) (*PerformanceAnalysis, error)
	// SetCycleNumber Allows restoring internal counter (for backtest recovery)
	SetCycleNumber(n int)
	// GetCycleNumber Gets the current cycle number
	GetCycleNumber() int
}

// DecisionLogger Decision logger
type DecisionLogger struct {
	logDir      string
	cycleNumber int
}

// NewDecisionLogger Creates a decision logger
func NewDecisionLogger(logDir string) IDecisionLogger {
	if logDir == "" {
		logDir = "data/decision_logs"
	}

	// Ensure log directory exists (using secure permissions: owner-only access)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Printf("⚠ Failed to create log directory: %v\n", err)
	}

	// Force set directory permissions (even if directory already exists) - ensure security
	if err := os.Chmod(logDir, 0700); err != nil {
		fmt.Printf("⚠ Failed to set log directory permissions: %v\n", err)
	}

	return &DecisionLogger{
		logDir:      logDir,
		cycleNumber: 0,
	}
}

// SetCycleNumber Allows external restoration of internal cycle counter (for backtest recovery)
func (l *DecisionLogger) SetCycleNumber(n int) {
	if n > 0 {
		l.cycleNumber = n
	}
}

// GetCycleNumber Gets the current cycle number
func (l *DecisionLogger) GetCycleNumber() int {
	return l.cycleNumber
}

// LogDecision Logs a decision
func (l *DecisionLogger) LogDecision(record *DecisionRecord) error {
	l.cycleNumber++
	record.CycleNumber = l.cycleNumber
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	// Generate filename: decision_YYYYMMDD_HHMMSS_cycleN.json
	filename := fmt.Sprintf("decision_%s_cycle%d.json",
		record.Timestamp.Format("20060102_150405"),
		record.CycleNumber)

	filepath := filepath.Join(l.logDir, filename)

	// Debug: Log AccountState before serialization
	fmt.Printf("🔍 LogDecision - AccountState before serialization: TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%, InitialBalance=%.2f\n",
		record.AccountState.TotalBalance, record.AccountState.AvailableBalance,
		record.AccountState.PositionCount, record.AccountState.MarginUsedPct, record.AccountState.InitialBalance)

	// Serialize to JSON (with indentation for readability)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize decision record: %w", err)
	}

	// Write to file (using secure permissions: owner-only read/write)
	if err := ioutil.WriteFile(filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write decision record: %w", err)
	}

	fmt.Printf("📝 Decision record saved: %s\n", filename)
	return nil
}

// GetLatestRecords Gets the latest N records (chronological order: oldest to newest)
func (l *DecisionLogger) GetLatestRecords(n int) ([]*DecisionRecord, error) {
	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	// First collect in reverse modification time order (newest first)
	var records []*DecisionRecord
	count := 0
	for i := len(files) - 1; i >= 0 && count < n; i-- {
		file := files[i]
		if file.IsDir() {
			continue
		}

		filepath := filepath.Join(l.logDir, file.Name())
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		records = append(records, &record)
		count++
	}

	// Reverse array to arrange from oldest to newest (for chart display)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// convertPositionToTradeOutcome converts a PositionRecord from database to TradeOutcome
func convertPositionToTradeOutcome(pos *cfg.PositionRecord) *TradeOutcome {
	// Consider position closed if ClosedAt is set OR if we have realized PnL/exit price
	isClosed := pos.ClosedAt != nil || (pos.RealizedPnL != 0 && pos.ExitPrice > 0)
	if !isClosed {
		return nil // Only convert closed positions
	}

	// If ClosedAt is not set but we have realized PnL, create a synthetic closed time
	var closedTime time.Time
	if pos.ClosedAt != nil {
		closedTime = *pos.ClosedAt
	} else {
		// Use opened_at + 1 hour as fallback if ClosedAt is missing
		closedTime = pos.OpenedAt.Add(1 * time.Hour)
		log.Printf("⚠️ [convertPositionToTradeOutcome] Position %s %s missing ClosedAt, using opened_at+1h as fallback", 
			pos.Symbol, pos.Side)
	}

	// Validate that we have essential data: we have either ExitPrice or RealizedPnL
	// ExitPrice can be 0 in some edge cases, but if RealizedPnL is set, we can still process the trade
	if pos.RealizedPnL == 0 && pos.ExitPrice == 0 {
		// If both are zero, try to infer exit price from entry price (for break-even trades)
		log.Printf("⚠️ [convertPositionToTradeOutcome] Position %s %s has both ExitPrice=0 and RealizedPnL=0, using entry price as fallback", 
			pos.Symbol, pos.Side)
		// We'll use entry price as a fallback, but this is not ideal
	}

	positionValue := pos.Quantity * pos.EntryPrice
	marginUsed := positionValue / float64(pos.Leverage)
	pnlPct := 0.0
	if marginUsed > 0 {
		pnlPct = (pos.RealizedPnL / marginUsed) * 100
	}

	duration := closedTime.Sub(pos.OpenedAt).String()

	// Determine if this was a stop loss (if exit price matches stop loss price, or if PnL is negative and close was recent)
	wasStopLoss := false
	if pos.StopLossPrice > 0 && pos.ExitPrice > 0 {
		// Check if exit price is close to stop loss price (within 0.1% tolerance)
		priceDiff := absFloat(pos.ExitPrice - pos.StopLossPrice)
		if priceDiff/pos.StopLossPrice < 0.001 {
			wasStopLoss = true
		}
	}
	// If not determined by price match, infer from negative PnL
	if !wasStopLoss && pos.RealizedPnL < 0 {
		wasStopLoss = true
	}

	// Use entry price as fallback if exit price is 0 (for break-even or data quality issues)
	closePrice := pos.ExitPrice
	if closePrice == 0 && pos.EntryPrice > 0 {
		closePrice = pos.EntryPrice
		log.Printf("⚠️ [convertPositionToTradeOutcome] Using entry price %.8f as fallback for exit price (position: %s %s)", 
			pos.EntryPrice, pos.Symbol, pos.Side)
	}

	return &TradeOutcome{
		Symbol:        pos.Symbol,
		Side:          pos.Side,
		Quantity:      pos.Quantity,
		Leverage:      pos.Leverage,
		OpenPrice:     pos.EntryPrice,
		ClosePrice:    closePrice,
		PositionValue: positionValue,
		MarginUsed:    marginUsed,
		PnL:           pos.RealizedPnL,
		PnLPct:        pnlPct,
		Duration:      duration,
		OpenTime:      pos.OpenedAt,
		CloseTime:     closedTime,
		WasStopLoss:   wasStopLoss,
	}
}

// absFloat returns absolute value of float64
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// GetRecordByDate Gets all records for a specific date
func (l *DecisionLogger) GetRecordByDate(date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("20060102")
	pattern := filepath.Join(l.logDir, fmt.Sprintf("decision_%s_*.json", dateStr))

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find log files: %w", err)
	}

	var records []*DecisionRecord
	for _, filepath := range files {
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		records = append(records, &record)
	}

	return records, nil
}

// CleanOldRecords Cleans old records older than N days
func (l *DecisionLogger) CleanOldRecords(days int) error {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	removedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if file.ModTime().Before(cutoffTime) {
			filepath := filepath.Join(l.logDir, file.Name())
			if err := os.Remove(filepath); err != nil {
				fmt.Printf("⚠ Failed to delete old record %s: %v\n", file.Name(), err)
				continue
			}
			removedCount++
		}
	}

	if removedCount > 0 {
		fmt.Printf("🗑️ Cleaned %d old records (%d days ago)\n", removedCount, days)
	}

	return nil
}

// GetStatistics Gets statistics
func (l *DecisionLogger) GetStatistics() (*Statistics, error) {
	files, err := ioutil.ReadDir(l.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	stats := &Statistics{}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filepath := filepath.Join(l.logDir, file.Name())
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}

		var record DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		stats.TotalCycles++

		for _, action := range record.Decisions {
			if action.Success {
				switch action.Action {
				case "open_long", "open_short":
					stats.TotalOpenPositions++
				case "close_long", "close_short", "auto_close_long", "auto_close_short":
					stats.TotalClosePositions++
					// 🔧 BUG FIX: partial_close is not counted in TotalClosePositions to avoid double counting
					// case "partial_close": // Not counted, only full close counts as one
					// update_stop_loss and update_take_profit are not counted in statistics
				}
			}
		}

		if record.Success {
			stats.SuccessfulCycles++
		} else {
			stats.FailedCycles++
		}
	}

	return stats, nil
}

// Statistics Statistics information
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// TradeOutcome Single trade outcome
type TradeOutcome struct {
	Symbol        string    `json:"symbol"`         // Trading symbol
	Side          string    `json:"side"`           // long/short
	Quantity      float64   `json:"quantity"`       // Position quantity
	Leverage      int       `json:"leverage"`       // Leverage multiplier
	OpenPrice     float64   `json:"open_price"`     // Open price
	ClosePrice    float64   `json:"close_price"`    // Close price
	PositionValue float64   `json:"position_value"` // Position value (quantity × openPrice)
	MarginUsed    float64   `json:"margin_used"`    // Margin used (positionValue / leverage)
	PnL           float64   `json:"pn_l"`           // P&L (USDT)
	PnLPct        float64   `json:"pn_l_pct"`       // P&L percentage (relative to margin)
	Duration      string    `json:"duration"`       // Holding duration
	OpenTime      time.Time `json:"open_time"`      // Open time
	CloseTime     time.Time `json:"close_time"`     // Close time
	WasStopLoss   bool      `json:"was_stop_loss"`  // Whether stop loss
}

// PerformanceAnalysis Trading performance analysis
type PerformanceAnalysis struct {
	TotalTrades   int                           `json:"total_trades"`   // Total number of trades
	WinningTrades int                           `json:"winning_trades"` // Number of winning trades
	LosingTrades  int                           `json:"losing_trades"`  // Number of losing trades
	WinRate       float64                       `json:"win_rate"`       // Win rate
	AvgWin        float64                       `json:"avg_win"`        // Average win
	AvgLoss       float64                       `json:"avg_loss"`       // Average loss
	ProfitFactor  float64                       `json:"profit_factor"`  // Profit factor
	SharpeRatio   float64                       `json:"sharpe_ratio"`   // Sharpe ratio (risk-adjusted return)
	RecentTrades  []TradeOutcome                `json:"recent_trades"`  // Recent N trades
	SymbolStats   map[string]*SymbolPerformance `json:"symbol_stats"`   // Performance by symbol
	BestSymbol    string                        `json:"best_symbol"`    // Best performing symbol
	WorstSymbol   string                        `json:"worst_symbol"`   // Worst performing symbol
}

// SymbolPerformance Symbol performance statistics
type SymbolPerformance struct {
	Symbol        string  `json:"symbol"`         // Trading symbol
	TotalTrades   int     `json:"total_trades"`   // Number of trades
	WinningTrades int     `json:"winning_trades"` // Number of wins
	LosingTrades  int     `json:"losing_trades"`  // Number of losses
	WinRate       float64 `json:"win_rate"`       // Win rate
	TotalPnL      float64 `json:"total_pn_l"`     // Total P&L
	AvgPnL        float64 `json:"avg_pn_l"`       // Average P&L
}

// AnalyzePerformance Analyzes trading performance for the last N cycles
// database can be nil or a database interface that provides GetPositionHistory method
func (l *DecisionLogger) AnalyzePerformance(lookbackCycles int, traderID string, database interface{}) (*PerformanceAnalysis, error) {
	records, err := l.GetLatestRecords(lookbackCycles)
	if err != nil {
		return nil, fmt.Errorf("failed to read historical records: %w", err)
	}

	if len(records) == 0 {
		return &PerformanceAnalysis{
			RecentTrades: []TradeOutcome{},
			SymbolStats:  make(map[string]*SymbolPerformance),
		}, nil
	}

	analysis := &PerformanceAnalysis{
		RecentTrades: []TradeOutcome{},
		SymbolStats:  make(map[string]*SymbolPerformance),
	}

	// Track position state: symbol_side -> {side, openPrice, openTime, quantity, leverage}
	openPositions := make(map[string]map[string]interface{})

	// To avoid matching failures when open records are outside the window, first find all open positions from all historical records
	// Get more historical records to build complete position state (use larger window)
	allRecords, err := l.GetLatestRecords(lookbackCycles * 3) // Expand window by 3x
	if err == nil && len(allRecords) > len(records) {
		// First collect all open records from the expanded window
		for _, record := range allRecords {
			for _, action := range record.Decisions {
				if !action.Success {
					continue
				}

				symbol := action.Symbol
				side := ""
				if action.Action == "open_long" || action.Action == "close_long" || action.Action == "partial_close" || action.Action == "auto_close_long" {
					side = "long"
				} else if action.Action == "open_short" || action.Action == "close_short" || action.Action == "auto_close_short" {
					side = "short"
				}

				// partial_close needs to determine direction based on position
				if action.Action == "partial_close" && side == "" {
					for key, pos := range openPositions {
						if posSymbol, _ := pos["side"].(string); key == symbol+"_"+posSymbol {
							side = posSymbol
							break
						}
					}
				}

				posKey := symbol + "_" + side

				switch action.Action {
				case "open_long", "open_short":
					// Record open position
					openPositions[posKey] = map[string]interface{}{
						"side":      side,
						"openPrice": action.Price,
						"openTime":  action.Timestamp,
						"quantity":  action.Quantity,
						"leverage":  action.Leverage,
					}
				case "close_long", "close_short", "auto_close_long", "auto_close_short":
					// Remove closed position record
					delete(openPositions, posKey)
					// partial_close not processed, keep position record
				}
			}
		}
	}

	// Iterate through records in analysis window to generate trade outcomes
	for _, record := range records {
		for _, action := range record.Decisions {
			if !action.Success {
				continue
			}

			symbol := action.Symbol
			side := ""
			if action.Action == "open_long" || action.Action == "close_long" || action.Action == "partial_close" || action.Action == "auto_close_long" {
				side = "long"
			} else if action.Action == "open_short" || action.Action == "close_short" || action.Action == "auto_close_short" {
				side = "short"
			}

			// partial_close needs to determine direction based on position
			if action.Action == "partial_close" {
				// Find position direction from openPositions
				for key, pos := range openPositions {
					if posSymbol, _ := pos["side"].(string); key == symbol+"_"+posSymbol {
						side = posSymbol
						break
					}
				}
			}

			posKey := symbol + "_" + side // Use symbol_side as key to distinguish long/short positions

			switch action.Action {
			case "open_long", "open_short":
				// Update open position record (may have been recorded during pre-fill)
				openPositions[posKey] = map[string]interface{}{
					"side":               side,
					"openPrice":          action.Price,
					"openTime":           action.Timestamp,
					"quantity":           action.Quantity,
					"leverage":           action.Leverage,
					"remainingQuantity":  action.Quantity, // 🔧 BUG FIX: Track remaining quantity
					"accumulatedPnL":     0.0,             // 🔧 BUG FIX: Accumulate partial close P&L
					"partialCloseCount":  0,               // 🔧 BUG FIX: Partial close count
					"partialCloseVolume": 0.0,             // 🔧 BUG FIX: Partial close total volume
				}

			case "close_long", "close_short", "partial_close", "auto_close_long", "auto_close_short":
				// Find corresponding open position record (may come from pre-fill or current window)
				if openPos, exists := openPositions[posKey]; exists {
					openPrice := openPos["openPrice"].(float64)
					openTime := openPos["openTime"].(time.Time)
					side := openPos["side"].(string)
					quantity := openPos["quantity"].(float64)
					leverage := openPos["leverage"].(int)

					// 🔧 BUG FIX: Get tracking fields (initialize if not exists)
					remainingQty, _ := openPos["remainingQuantity"].(float64)
					if remainingQty == 0 {
						remainingQty = quantity // Compatible with old data (no remainingQuantity field)
					}
					accumulatedPnL, _ := openPos["accumulatedPnL"].(float64)
					partialCloseCount, _ := openPos["partialCloseCount"].(int)
					partialCloseVolume, _ := openPos["partialCloseVolume"].(float64)

					// For partial_close, use actual close quantity; otherwise use remaining position quantity
					actualQuantity := remainingQty
					if action.Action == "partial_close" {
						actualQuantity = action.Quantity
					}

					// Calculate P&L for this close (USDT)
					var pnl float64
					if side == "long" {
						pnl = actualQuantity * (action.Price - openPrice)
					} else {
						pnl = actualQuantity * (openPrice - action.Price)
					}

					// 🔧 BUG FIX: Handle partial_close aggregation logic
					if action.Action == "partial_close" {
						// Accumulate P&L and quantity
						accumulatedPnL += pnl
						remainingQty -= actualQuantity
						partialCloseCount++
						partialCloseVolume += actualQuantity

						// Update openPositions (keep position record but update tracking data)
						openPos["remainingQuantity"] = remainingQty
						openPos["accumulatedPnL"] = accumulatedPnL
						openPos["partialCloseCount"] = partialCloseCount
						openPos["partialCloseVolume"] = partialCloseVolume

						// Check if fully closed
						if remainingQty <= 0.0001 { // Use small threshold to avoid floating point errors
							// ✅ Fully closed: record as one complete trade
							positionValue := quantity * openPrice
							marginUsed := positionValue / float64(leverage)
							pnlPct := 0.0
							if marginUsed > 0 {
								pnlPct = (accumulatedPnL / marginUsed) * 100
							}

							// Partial close doesn't typically involve SL/TP, but check anyway
							wasStopLoss := false
							if accumulatedPnL < 0 {
								// Negative PnL from partial close might indicate SL, but usually partial closes are manual
								wasStopLoss = false // Partial closes are typically manual, not SL/TP
							}

							outcome := TradeOutcome{
								Symbol:        symbol,
								Side:          side,
								Quantity:      quantity, // Use original total quantity
								Leverage:      leverage,
								OpenPrice:     openPrice,
								ClosePrice:    action.Price, // Last close price
								PositionValue: positionValue,
								MarginUsed:    marginUsed,
								PnL:           accumulatedPnL, // 🔧 Use accumulated P&L
								PnLPct:        pnlPct,
								Duration:      action.Timestamp.Sub(openTime).String(),
								OpenTime:      openTime,
								CloseTime:     action.Timestamp,
								WasStopLoss:   wasStopLoss,
							}

							analysis.RecentTrades = append(analysis.RecentTrades, outcome)
							analysis.TotalTrades++ // 🔧 Only count when fully closed

							// Categorize trade
							if accumulatedPnL > 0 {
								analysis.WinningTrades++
								analysis.AvgWin += accumulatedPnL
							} else if accumulatedPnL < 0 {
								analysis.LosingTrades++
								analysis.AvgLoss += accumulatedPnL
							}

							// Update symbol statistics
							if _, exists := analysis.SymbolStats[symbol]; !exists {
								analysis.SymbolStats[symbol] = &SymbolPerformance{
									Symbol: symbol,
								}
							}
							stats := analysis.SymbolStats[symbol]
							stats.TotalTrades++
							stats.TotalPnL += accumulatedPnL
							if accumulatedPnL > 0 {
								stats.WinningTrades++
							} else if accumulatedPnL < 0 {
								stats.LosingTrades++
							}

							// Delete position record
							delete(openPositions, posKey)
						}
						// ⚠️ Otherwise do nothing (wait for subsequent partial_close or full close)

					} else {
						// 🔧 Full close (close_long/close_short/auto_close)
						// If there was a previous partial close, add accumulated P&L
						totalPnL := accumulatedPnL + pnl

						positionValue := quantity * openPrice
						marginUsed := positionValue / float64(leverage)
						pnlPct := 0.0
						if marginUsed > 0 {
							pnlPct = (totalPnL / marginUsed) * 100
						}

						// Determine if this was a stop loss by checking execution log
						wasStopLoss := false
						if action.Action == "auto_close_long" || action.Action == "auto_close_short" {
							// Check execution log for stop loss indication
							for _, logMsg := range record.ExecutionLog {
								if strings.Contains(strings.ToLower(logMsg), "stop loss") {
									wasStopLoss = true
									break
								}
							}
							// If not found in execution log, infer from PnL (negative = likely SL)
							if !wasStopLoss && totalPnL < 0 {
								wasStopLoss = true
							}
						}

						outcome := TradeOutcome{
							Symbol:        symbol,
							Side:          side,
							Quantity:      quantity, // Use original total quantity
							Leverage:      leverage,
							OpenPrice:     openPrice,
							ClosePrice:    action.Price,
							PositionValue: positionValue,
							MarginUsed:    marginUsed,
							PnL:           totalPnL, // 🔧 Include previous partial close P&L
							PnLPct:        pnlPct,
							Duration:      action.Timestamp.Sub(openTime).String(),
							OpenTime:      openTime,
							CloseTime:     action.Timestamp,
							WasStopLoss:   wasStopLoss,
						}

						analysis.RecentTrades = append(analysis.RecentTrades, outcome)
						analysis.TotalTrades++

						// Categorize trade
						if totalPnL > 0 {
							analysis.WinningTrades++
							analysis.AvgWin += totalPnL
						} else if totalPnL < 0 {
							analysis.LosingTrades++
							analysis.AvgLoss += totalPnL
						}

						// Update symbol statistics
						if _, exists := analysis.SymbolStats[symbol]; !exists {
							analysis.SymbolStats[symbol] = &SymbolPerformance{
								Symbol: symbol,
							}
						}
						stats := analysis.SymbolStats[symbol]
						stats.TotalTrades++
						stats.TotalPnL += totalPnL
						if totalPnL > 0 {
							stats.WinningTrades++
						} else if totalPnL < 0 {
							stats.LosingTrades++
						}

						// Delete position record
						delete(openPositions, posKey)
					}
				}
			}
		}
	}

	// Calculate statistical metrics
	if analysis.TotalTrades > 0 {
		analysis.WinRate = (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100

		// Calculate total profit and total loss
		totalWinAmount := analysis.AvgWin   // Currently accumulated sum
		totalLossAmount := analysis.AvgLoss // Currently accumulated sum (negative)

		if analysis.WinningTrades > 0 {
			analysis.AvgWin /= float64(analysis.WinningTrades)
		}
		if analysis.LosingTrades > 0 {
			analysis.AvgLoss /= float64(analysis.LosingTrades)
		}

		// Profit Factor = Total Profit / Total Loss (absolute value)
		// Note: totalLossAmount is negative, so take negative to get absolute value
		if totalLossAmount != 0 {
			analysis.ProfitFactor = totalWinAmount / (-totalLossAmount)
		} else if totalWinAmount > 0 {
			// Only profit no loss case, set to a large value to represent perfect strategy
			analysis.ProfitFactor = 999.0
		}
	}

	// Calculate win rate and average P&L for each symbol
	bestPnL := -999999.0
	worstPnL := 999999.0
	for symbol, stats := range analysis.SymbolStats {
		if stats.TotalTrades > 0 {
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)

			if stats.TotalPnL > bestPnL {
				bestPnL = stats.TotalPnL
				analysis.BestSymbol = symbol
			}
			if stats.TotalPnL < worstPnL {
				worstPnL = stats.TotalPnL
				analysis.WorstSymbol = symbol
			}
		}
	}

	// Query database for closed positions as fallback/supplement
	dbTrades := make([]*TradeOutcome, 0)
	if database != nil && traderID != "" {
		// Try to cast database to *cfg.Database
		if db, ok := database.(*cfg.Database); ok && db != nil {
			log.Printf("📊 [AnalyzePerformance] Querying database for closed positions (traderID: %s)", traderID)
			// Get closed positions from database (last 200 to cover extended time window)
			positionHistory, err := db.GetPositionHistory(traderID, 200, 0)
			if err == nil {
				log.Printf("📊 [AnalyzePerformance] Retrieved %d positions from database", len(positionHistory))
				
				// Create a map to track trades from decision records (for deduplication)
				// Use more precise key: symbol_side_entryPrice_quantity_closeTime to avoid false duplicates
				recordTradeKeys := make(map[string]bool)
				for _, trade := range analysis.RecentTrades {
					// Include entry price and quantity in key for better deduplication
					key := fmt.Sprintf("%s_%s_%.8f_%.8f_%d", trade.Symbol, trade.Side, trade.OpenPrice, trade.Quantity, trade.CloseTime.Unix())
					recordTradeKeys[key] = true
				}
				log.Printf("📊 [AnalyzePerformance] Found %d trades from decision records for deduplication", len(recordTradeKeys))

				closedCount := 0
				skippedOpen := 0
				skippedDuplicate := 0
				skippedConversion := 0
				
				// Convert database positions to trades
				for _, pos := range positionHistory {
					// Consider a position closed if:
					// 1. ClosedAt is set, OR
					// 2. RealizedPnL is non-zero (indicates position was closed and PnL was realized)
					// 3. ExitPrice is set (indicates position was closed)
					isClosed := pos.ClosedAt != nil || (pos.RealizedPnL != 0 && pos.ExitPrice > 0)
					
					if !isClosed {
						skippedOpen++
						log.Printf("📊 [AnalyzePerformance] Skipping open position: %s %s (closed_at: %v, exit_price: %.8f, realized_pnl: %.2f)", 
							pos.Symbol, pos.Side, pos.ClosedAt != nil, pos.ExitPrice, pos.RealizedPnL)
						continue // Skip open positions
					}
					closedCount++

					// For deduplication, use ClosedAt timestamp if available, otherwise use a fallback
					var closeTimeUnix int64
					if pos.ClosedAt != nil {
						closeTimeUnix = pos.ClosedAt.Unix()
					} else {
						// Use opened_at + a small offset if ClosedAt is not set but position is closed
						// This helps avoid false duplicates while still allowing the position to be processed
						closeTimeUnix = pos.OpenedAt.Unix() + 1
						log.Printf("⚠️ [AnalyzePerformance] Position %s %s has no ClosedAt, using opened_at+1 for deduplication key", 
							pos.Symbol, pos.Side)
					}

					// Check if this trade is already in decision records (deduplicate)
					// Use more precise key: symbol_side_entryPrice_quantity_closeTime
					key := fmt.Sprintf("%s_%s_%.8f_%.8f_%d", pos.Symbol, pos.Side, pos.EntryPrice, pos.Quantity, closeTimeUnix)
					if recordTradeKeys[key] {
						skippedDuplicate++
						closedAtStr := "N/A"
						if pos.ClosedAt != nil {
							closedAtStr = pos.ClosedAt.Format("2006-01-02 15:04:05")
						}
						log.Printf("📊 [AnalyzePerformance] Skipping duplicate trade: %s %s (entry: %.8f, qty: %.8f, closed: %s)", 
							pos.Symbol, pos.Side, pos.EntryPrice, pos.Quantity, closedAtStr)
						continue // Skip if already in decision records
					}

					// Convert to TradeOutcome
					trade := convertPositionToTradeOutcome(pos)
					if trade != nil {
						dbTrades = append(dbTrades, trade)
						log.Printf("📊 [AnalyzePerformance] Added database trade: %s %s (PnL: %.2f, exit: %.8f, closed_at: %v)", 
							trade.Symbol, trade.Side, trade.PnL, trade.ClosePrice, pos.ClosedAt != nil)
					} else {
						skippedConversion++
						log.Printf("⚠️ [AnalyzePerformance] Failed to convert position to trade: %s %s (closed_at: %v, exit_price: %.8f, realized_pnl: %.2f)", 
							pos.Symbol, pos.Side, pos.ClosedAt != nil, pos.ExitPrice, pos.RealizedPnL)
					}
				}
				
				log.Printf("📊 [AnalyzePerformance] Database processing summary: total=%d, closed=%d, skipped_open=%d, skipped_duplicate=%d, skipped_conversion=%d, added=%d", 
					len(positionHistory), closedCount, skippedOpen, skippedDuplicate, skippedConversion, len(dbTrades))
			} else {
				log.Printf("⚠️ [AnalyzePerformance] Failed to get position history from database: %v", err)
			}
		} else {
			log.Printf("⚠️ [AnalyzePerformance] Database type assertion failed or database is nil (traderID: %s)", traderID)
		}
	} else {
		if database == nil {
			log.Printf("⚠️ [AnalyzePerformance] Database parameter is nil (traderID: %s)", traderID)
		}
		if traderID == "" {
			log.Printf("⚠️ [AnalyzePerformance] TraderID is empty")
		}
	}

	// Merge database trades with decision record trades
	log.Printf("📊 [AnalyzePerformance] Before merge: decision_record_trades=%d, database_trades=%d, current_total_trades=%d", 
		len(analysis.RecentTrades), len(dbTrades), analysis.TotalTrades)
	
	allTrades := make([]TradeOutcome, 0, len(analysis.RecentTrades)+len(dbTrades))
	allTrades = append(allTrades, analysis.RecentTrades...)
	for _, trade := range dbTrades {
		allTrades = append(allTrades, *trade)

		// Update analysis metrics for database trades
		analysis.TotalTrades++
		
		if trade.PnL > 0 {
			analysis.WinningTrades++
			analysis.AvgWin += trade.PnL
		} else if trade.PnL < 0 {
			analysis.LosingTrades++
			analysis.AvgLoss += trade.PnL
		}

		// Update symbol statistics
		if _, exists := analysis.SymbolStats[trade.Symbol]; !exists {
			analysis.SymbolStats[trade.Symbol] = &SymbolPerformance{
				Symbol: trade.Symbol,
			}
		}
		stats := analysis.SymbolStats[trade.Symbol]
		stats.TotalTrades++
		stats.TotalPnL += trade.PnL
		if trade.PnL > 0 {
			stats.WinningTrades++
		} else if trade.PnL < 0 {
			stats.LosingTrades++
		}
	}
	
	log.Printf("📊 [AnalyzePerformance] After merge: total_trades=%d, winning=%d, losing=%d, all_trades_count=%d", 
		analysis.TotalTrades, analysis.WinningTrades, analysis.LosingTrades, len(allTrades))

	// Recalculate metrics with merged data
	if analysis.TotalTrades > 0 {
		analysis.WinRate = (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100

		// Recalculate total profit and total loss
		totalWinAmount := analysis.AvgWin   // Currently accumulated sum
		totalLossAmount := analysis.AvgLoss // Currently accumulated sum (negative)

		if analysis.WinningTrades > 0 {
			analysis.AvgWin /= float64(analysis.WinningTrades)
		}
		if analysis.LosingTrades > 0 {
			analysis.AvgLoss /= float64(analysis.LosingTrades)
		}

		// Profit Factor = Total Profit / Total Loss (absolute value)
		if totalLossAmount != 0 {
			analysis.ProfitFactor = totalWinAmount / (-totalLossAmount)
		} else if totalWinAmount > 0 {
			analysis.ProfitFactor = 999.0
		}

		// Recalculate symbol statistics
		bestPnL := -999999.0
		worstPnL := 999999.0
		for symbol, stats := range analysis.SymbolStats {
			if stats.TotalTrades > 0 {
				stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
				stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)

				if stats.TotalPnL > bestPnL {
					bestPnL = stats.TotalPnL
					analysis.BestSymbol = symbol
				}
				if stats.TotalPnL < worstPnL {
					worstPnL = stats.TotalPnL
					analysis.WorstSymbol = symbol
				}
			}
		}
	}

	// Sort all trades by close time (newest first) and keep top 10
	if len(allTrades) > 0 {
		// Sort by CloseTime descending (newest first)
		for i := 0; i < len(allTrades)-1; i++ {
			for j := i + 1; j < len(allTrades); j++ {
				if allTrades[i].CloseTime.Before(allTrades[j].CloseTime) {
					allTrades[i], allTrades[j] = allTrades[j], allTrades[i]
				}
			}
		}

		// Keep only top 10
		if len(allTrades) > 10 {
			analysis.RecentTrades = allTrades[:10]
		} else {
			analysis.RecentTrades = allTrades
		}
	}

	// Calculate Sharpe ratio (requires at least 2 data points)
	analysis.SharpeRatio = l.calculateSharpeRatio(records)

	return analysis, nil
}

// calculateSharpeRatio Calculates Sharpe ratio
// Calculates risk-adjusted return based on account equity changes
func (l *DecisionLogger) calculateSharpeRatio(records []*DecisionRecord) float64 {
	if len(records) < 2 {
		return 0.0
	}

	// Extract account equity for each cycle
	// Note: TotalBalance field actually stores TotalEquity (total account equity)
	// TotalUnrealizedProfit field actually stores TotalPnL (P&L relative to initial balance)
	var equities []float64
	for _, record := range records {
		// Directly use TotalBalance as it is already complete account equity
		equity := record.AccountState.TotalBalance
		if equity > 0 {
			equities = append(equities, equity)
		}
	}

	if len(equities) < 2 {
		return 0.0
	}

	// Calculate period returns
	var returns []float64
	for i := 1; i < len(equities); i++ {
		if equities[i-1] > 0 {
			periodReturn := (equities[i] - equities[i-1]) / equities[i-1]
			returns = append(returns, periodReturn)
		}
	}

	if len(returns) == 0 {
		return 0.0
	}

	// Calculate average return
	sumReturns := 0.0
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	// Calculate return standard deviation
	sumSquaredDiff := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(returns))
	stdDev := math.Sqrt(variance)

	// Avoid division by zero
	if stdDev == 0 {
		if meanReturn > 0 {
			return 999.0 // Positive return with no volatility
		} else if meanReturn < 0 {
			return -999.0 // Negative return with no volatility
		}
		return 0.0
	}

	// Calculate Sharpe ratio (assuming risk-free rate is 0)
	// Note: Returns cycle-level Sharpe ratio (not annualized), normal range -2 to +2
	sharpeRatio := meanReturn / stdDev
	return sharpeRatio
}

