package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetBalance gets account balance (implements Trader interface)
// Returns standard field names: totalWalletBalance, totalUnrealizedProfit
func (t *LighterTraderV2) GetBalance() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	// Calculate totalWalletBalance (equity minus unrealized PnL)
	totalWalletBalance := balance.TotalEquity - balance.UnrealizedPnL

	return map[string]interface{}{
		"totalWalletBalance":    totalWalletBalance,    // Wallet balance (excluding unrealized profit/loss)
		"totalUnrealizedProfit": balance.UnrealizedPnL, // Unrealized profit/loss
		"available_balance":     balance.AvailableBalance,
		"margin_used":           balance.MarginUsed,
		"maintenance_margin":    balance.MaintenanceMargin,
	}, nil
}

// GetAccountBalance gets detailed account balance information
func (t *LighterTraderV2) GetAccountBalance() (*AccountBalance, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("authentication token invalid: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/balance", t.baseURL, accountIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication header
	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get balance (status %d): %s", resp.StatusCode, string(body))
	}

	var balance AccountBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("failed to parse balance response: %w", err)
	}

	return &balance, nil
}

// GetPositions gets all positions (implements Trader interface)
func (t *LighterTraderV2) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		result = append(result, map[string]interface{}{
			"symbol":            pos.Symbol,
			"side":              pos.Side,
			"size":              pos.Size,
			"entry_price":       pos.EntryPrice,
			"mark_price":        pos.MarkPrice,
			"liquidation_price": pos.LiquidationPrice,
			"unrealized_pnl":    pos.UnrealizedPnL,
			"leverage":          pos.Leverage,
			"margin_used":       pos.MarginUsed,
		})
	}

	return result, nil
}

// GetPositionsRaw gets all positions (returns raw Position type)
func (t *LighterTraderV2) GetPositionsRaw(symbol string) ([]Position, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("authentication token invalid: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/positions", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("?symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get positions (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	return positions, nil
}

// GetPosition gets position for specified symbol
func (t *LighterTraderV2) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		if pos.Symbol == symbol && pos.Size > 0 {
			return &pos, nil
		}
	}

	return nil, nil // No position
}

// GetMarketPrice gets market price (implements Trader interface)
func (t *LighterTraderV2) GetMarketPrice(symbol string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/market/ticker?symbol=%s", t.baseURL, symbol)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get market price (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("failed to parse price response: %w", err)
	}

	price, err := SafeFloat64(ticker, "last_price")
	if err != nil {
		return 0, fmt.Errorf("unable to get price: %w", err)
	}

	return price, nil
}

// FormatQuantity formats quantity to correct precision (implements Trader interface)
func (t *LighterTraderV2) FormatQuantity(symbol string, quantity float64) (string, error) {
	// TODO: Get symbol precision from API
	// Temporarily use default precision
	return fmt.Sprintf("%.4f", quantity), nil
}
