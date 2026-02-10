package decision

import (
	"testing"
)

// TestLeverageFallback tests automatic correction when leverage exceeds limit
func TestLeverageFallback(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantLeverage    int // Expected leverage after correction
		wantError       bool
	}{
		{
			name: "Altcoin leverage exceeded - auto-correct to limit",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // Limit 5x
			wantLeverage:    5, // Should be corrected to 5
			wantError:       false,
		},
		{
			name: "BTC leverage exceeded - auto-correct to limit",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 1000,
				StopLoss:        90000,
				TakeProfit:      110000,
			},
			accountEquity:   100,
			btcEthLeverage:  10, // Limit 10x
			altcoinLeverage: 5,
			wantLeverage:    10, // Should be corrected to 10
			wantError:       false,
		},
		{
			name: "Leverage within limit - no correction",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // Not exceeded
				PositionSizeUSD: 500,
				StopLoss:        4000,
				TakeProfit:      3000,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    5, // Stays unchanged
			wantError:       false,
		},
		{
			name: "Leverage is 0 - should error",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // Invalid
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    0,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, tt.accountEquity, tt.btcEthLeverage, tt.altcoinLeverage, GetDefaultStrategyConfig(), nil)

			// Check error status
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// If should not error, check if leverage is correctly corrected
			if !tt.wantError && tt.decision.Leverage != tt.wantLeverage {
				t.Errorf("Leverage not corrected: got %d, want %d", tt.decision.Leverage, tt.wantLeverage)
			}
		})
	}
}

// TestUpdateStopLossValidation tests field validation for update_stop_loss action
func TestUpdateStopLossValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "Correctly use new_stop_loss field",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: 155.5,
				Reasoning:   "Move stop loss to breakeven",
			},
			wantError: false,
		},
		{
			name: "new_stop_loss is 0 should error",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: 0,
				Reasoning:   "Test error case",
			},
			wantError: true,
			errorMsg:  "new stop loss price must be greater than 0",
		},
		{
			name: "new_stop_loss is negative should error",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: -100,
				Reasoning:   "Test error case",
			},
			wantError: true,
			errorMsg:  "new stop loss price must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5, GetDefaultStrategyConfig(), nil)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Error message mismatch: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestUpdateTakeProfitValidation tests field validation for update_take_profit action
func TestUpdateTakeProfitValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "Correctly use new_take_profit field",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: 98000,
				Reasoning:     "Adjust take profit to key resistance level",
			},
			wantError: false,
		},
		{
			name: "new_take_profit is 0 should error",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: 0,
				Reasoning:     "Test error case",
			},
			wantError: true,
			errorMsg:  "new take profit price must be greater than 0",
		},
		{
			name: "new_take_profit is negative should error",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: -1000,
				Reasoning:     "Test error case",
			},
			wantError: true,
			errorMsg:  "new take profit price must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5, GetDefaultStrategyConfig(), nil)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Error message mismatch: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestPartialCloseValidation tests field validation for partial_close action
func TestPartialCloseValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "Correctly use close_percentage field",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 50.0,
				Reasoning:       "Lock in half profit",
			},
			wantError: false,
		},
		{
			name: "close_percentage is 0 should error",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 0,
				Reasoning:       "Test error case",
			},
			wantError: true,
			errorMsg:  "close percentage must be between 0-100",
		},
		{
			name: "close_percentage exceeds 100 should error",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 150,
				Reasoning:       "Test error case",
			},
			wantError: true,
			errorMsg:  "close percentage must be between 0-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5, GetDefaultStrategyConfig(), nil)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Error message mismatch: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// contains checks if string contains substring (helper function)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
