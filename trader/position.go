package trader

import "time"

// TraderStats represents trader statistics
type TraderStats struct {
	TotalTrades      int     // Total number of trades
	WinningTrades    int     // Number of winning trades
	LosingTrades     int     // Number of losing trades
	TotalPnL         float64 // Total profit and loss
	TotalPnLPercent  float64 // Total PnL percentage
	AverageWin       float64 // Average winning trade amount
	AverageLoss      float64 // Average losing trade amount
	LargestWin       float64 // Largest winning trade
	LargestLoss      float64 // Largest losing trade
	WinRate          float64 // Win rate percentage
	ProfitFactor     float64 // Profit factor (total wins / total losses)
	MaxDrawdown      float64 // Maximum drawdown
	MaxDrawdownPct   float64 // Maximum drawdown percentage
	SharpeRatio      float64 // Sharpe ratio
	StartDate        time.Time // Start date of statistics
	EndDate          time.Time // End date of statistics
}

// CalculateWinRate calculates win rate from winning and losing trades
func CalculateWinRate(winningTrades, losingTrades int) float64 {
	total := winningTrades + losingTrades
	if total == 0 {
		return 0.0
	}
	return (float64(winningTrades) / float64(total)) * 100.0
}

// CalculateProfitFactor calculates profit factor from total wins and losses
func CalculateProfitFactor(totalWins, totalLosses float64) float64 {
	if totalLosses == 0 {
		if totalWins > 0 {
			return 999.0 // Infinite profit factor
		}
		return 0.0
	}
	return totalWins / totalLosses
}

