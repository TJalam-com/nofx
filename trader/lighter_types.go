package trader

// AccountBalance represents account balance information from Lighter API
// JSON tags match the actual API response field names
type AccountBalance struct {
	TotalEquity       float64 `json:"total_equity"`       // Total equity
	AvailableBalance  float64 `json:"available_balance"`  // Available balance
	MarginUsed        float64 `json:"margin_used"`        // Margin used
	UnrealizedPnL     float64 `json:"unrealized_pnl"`     // Unrealized profit/loss
	MaintenanceMargin float64 `json:"maintenance_margin"` // Maintenance margin
}

// Position represents position information from Lighter API
// JSON tags match the actual API response field names
// Note: API returns "position" (not "size"), "avg_entry_price" (not "entry_price"), "sign" (not "side")
type Position struct {
	Symbol           string  `json:"symbol"`            // Trading pair symbol
	Side             string  `json:"sign"`              // "long" or "short" (API field is "sign")
	Size             float64 `json:"position"`          // Position size (API field is "position")
	EntryPrice       float64 `json:"avg_entry_price"`   // Average entry price (API field is "avg_entry_price")
	MarkPrice        float64 `json:"mark_price"`        // Mark price
	LiquidationPrice float64 `json:"liquidation_price"` // Liquidation price
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	Leverage         float64 `json:"leverage"`          // Leverage multiplier
	MarginUsed       float64 `json:"margin_used"`       // Margin used
}
