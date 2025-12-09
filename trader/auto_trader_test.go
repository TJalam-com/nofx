package trader

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/pool"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"
)

// ============================================================
// AutoTraderTestSuite - Using testify/suite for structured testing
// ============================================================

// AutoTraderTestSuite is the test suite for AutoTrader
// Uses testify/suite to organize tests, providing unified setup/teardown and mock management
type AutoTraderTestSuite struct {
	suite.Suite

	// Test object
	autoTrader *AutoTrader

	// Mock dependencies
	mockTrader *MockTrader
	mockDB     *MockDatabase
	mockLogger logger.IDecisionLogger

	// gomonkey patches
	patches *gomonkey.Patches

	// Test configuration
	config AutoTraderConfig
}

// SetupSuite executes once before the entire test suite starts
func (s *AutoTraderTestSuite) SetupSuite() {
	// Can initialize some global resources here
}

// TearDownSuite executes once after the entire test suite ends
func (s *AutoTraderTestSuite) TearDownSuite() {
	// Clean up global resources
}

// SetupTest executes before each test case starts
func (s *AutoTraderTestSuite) SetupTest() {
	// Initialize patches
	s.patches = gomonkey.NewPatches()

	// Create mock objects
	s.mockTrader = &MockTrader{
		balance: map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		},
		positions: []map[string]interface{}{},
	}

	s.mockDB = &MockDatabase{}

	// Create temporary decision logger
	s.mockLogger = logger.NewDecisionLogger("/tmp/test_decision_logs")

	// Set default configuration
	s.config = AutoTraderConfig{
		ID:                   "test_trader",
		Name:                 "Test Trader",
		AIModel:              "deepseek",
		Exchange:             "binance",
		InitialBalance:       10000.0,
		ScanInterval:         3 * time.Minute,
		SystemPromptTemplate: "adaptive",
		BTCETHLeverage:       10,
		AltcoinLeverage:      5,
		IsCrossMargin:        true,
	}

	// Create AutoTrader instance (direct construction, not calling NewAutoTrader to avoid external dependencies)
	s.autoTrader = &AutoTrader{
		id:                    s.config.ID,
		name:                  s.config.Name,
		aiModel:               s.config.AIModel,
		exchange:              s.config.Exchange,
		config:                s.config,
		trader:                s.mockTrader,
		mcpClient:             nil, // No actual MCP Client needed in tests
		decisionLogger:        s.mockLogger,
		initialBalance:        s.config.InitialBalance,
		systemPromptTemplate:  s.config.SystemPromptTemplate,
		defaultCoins:          []string{"BTC", "ETH"},
		tradingCoins:          []string{},
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		peakPnLCache:          make(map[string]float64),
		lastBalanceSyncTime:   time.Now(),
		database:              s.mockDB,
		userID:                "test_user",
	}
}

// TearDownTest executes after each test case ends
func (s *AutoTraderTestSuite) TearDownTest() {
	// Reset gomonkey patches
	if s.patches != nil {
		s.patches.Reset()
	}
}

// ============================================================
// Level 1: Utility function tests
// ============================================================

func (s *AutoTraderTestSuite) TestSortDecisionsByPriority() {
	tests := []struct {
		name  string
		input []decision.Decision
	}{
		{
			name: "Mixed decisions_verify priority sorting",
			input: []decision.Decision{
				{Action: "open_long", Symbol: "BTCUSDT"},
				{Action: "close_short", Symbol: "ETHUSDT"},
				{Action: "hold", Symbol: "BNBUSDT"},
				{Action: "update_stop_loss", Symbol: "SOLUSDT"},
				{Action: "open_short", Symbol: "ADAUSDT"},
				{Action: "partial_close", Symbol: "DOGEUSDT"},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := sortDecisionsByPriority(tt.input)

			s.Equal(len(tt.input), len(result), "Result length should be the same")

			// Verify priority is ascending
			getActionPriority := func(action string) int {
				switch action {
				case "close_long", "close_short", "partial_close":
					return 1
				case "update_stop_loss", "update_take_profit":
					return 2
				case "open_long", "open_short":
					return 3
				case "hold", "wait":
					return 4
				default:
					return 999
				}
			}

			for i := 0; i < len(result)-1; i++ {
				currentPriority := getActionPriority(result[i].Action)
				nextPriority := getActionPriority(result[i+1].Action)
				s.LessOrEqual(currentPriority, nextPriority, "Priority should be ascending")
			}
		})
	}
}

func (s *AutoTraderTestSuite) TestNormalizeSymbol() {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Already in standard format", "BTCUSDT", "BTCUSDT"},
		{"Lowercase to uppercase", "btcusdt", "BTCUSDT"},
		{"Only coin name_add USDT", "BTC", "BTCUSDT"},
		{"With spaces_remove spaces", " BTC ", "BTCUSDT"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := normalizeSymbol(tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

// ============================================================
// Level 2: Getter/Setter tests
// ============================================================

func (s *AutoTraderTestSuite) TestGettersAndSetters() {
	s.Run("GetID", func() {
		s.Equal("test_trader", s.autoTrader.GetID())
	})

	s.Run("GetName", func() {
		s.Equal("Test Trader", s.autoTrader.GetName())
	})

	s.Run("SetSystemPromptTemplate", func() {
		s.autoTrader.SetSystemPromptTemplate("aggressive")
		s.Equal("aggressive", s.autoTrader.GetSystemPromptTemplate())
	})

	s.Run("SetCustomPrompt", func() {
		s.autoTrader.SetCustomPrompt("custom prompt")
		s.Equal("custom prompt", s.autoTrader.customPrompt)
	})
}

// ============================================================
// Level 3: PeakPnL cache tests
// ============================================================

func (s *AutoTraderTestSuite) TestPeakPnLCache() {
	s.Run("UpdatePeakPnL_First record", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.5)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(10.5, cache["BTCUSDT_long"])
	})

	s.Run("UpdatePeakPnL_Update to higher value", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 15.0)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(15.0, cache["BTCUSDT_long"])
	})

	s.Run("UpdatePeakPnL_Do not update to lower value", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 12.0)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(15.0, cache["BTCUSDT_long"], "Peak value should remain unchanged")
	})

	s.Run("ClearPeakPnLCache", func() {
		s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long")
		cache := s.autoTrader.GetPeakPnLCache()
		_, exists := cache["BTCUSDT_long"]
		s.False(exists, "Should be cleared")
	})
}

// ============================================================
// Level 4: GetStatus tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetStatus() {
	s.autoTrader.isRunning = true
	s.autoTrader.callCount = 15

	status := s.autoTrader.GetStatus()

	s.Equal("test_trader", status["trader_id"])
	s.Equal("Test Trader", status["trader_name"])
	s.Equal("deepseek", status["ai_model"])
	s.Equal("binance", status["exchange"])
	s.True(status["is_running"].(bool))
	s.Equal(15, status["call_count"])
	s.Equal(10000.0, status["initial_balance"])
}

// ============================================================
// Level 5: GetAccountInfo tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetAccountInfo() {
	accountInfo, err := s.autoTrader.GetAccountInfo()

	s.NoError(err)
	s.NotNil(accountInfo)

	// Verify core fields and values
	s.Equal(10100.0, accountInfo["total_equity"]) // 10000 + 100
	s.Equal(8000.0, accountInfo["available_balance"])
	s.Equal(100.0, accountInfo["total_pnl"]) // 10100 - 10000
}

// ============================================================
// Level 6: GetPositions tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetPositions() {
	s.Run("Empty positions", func() {
		positions, err := s.autoTrader.GetPositions()

		s.NoError(err)
		// positions may be nil or empty array, both are valid
		if positions != nil {
			s.Equal(0, len(positions))
		}
	})

	s.Run("Has positions", func() {
		// Set mock positions
		s.mockTrader.positions = []map[string]interface{}{
			{
				"symbol":           "BTCUSDT",
				"side":             "long",
				"entryPrice":       50000.0,
				"markPrice":        51000.0,
				"positionAmt":      0.1,
				"unRealizedProfit": 100.0,
				"liquidationPrice": 45000.0,
				"leverage":         10.0,
			},
		}

		positions, err := s.autoTrader.GetPositions()

		s.NoError(err)
		s.Equal(1, len(positions))

		pos := positions[0]
		s.Equal("BTCUSDT", pos["symbol"])
		s.Equal("long", pos["side"])
		s.Equal(0.1, pos["quantity"])
		s.Equal(50000.0, pos["entry_price"])
	})
}

// ============================================================
// Level 7: getCandidateCoins tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetCandidateCoins() {
	s.Run("Use database default coins", func() {
		s.autoTrader.defaultCoins = []string{"BTC", "ETH", "BNB"}
		s.autoTrader.tradingCoins = []string{} // Empty custom coins

		coins, err := s.autoTrader.getCandidateCoins()

		s.NoError(err)
		s.Equal(3, len(coins))
		s.Equal("BTCUSDT", coins[0].Symbol)
		s.Equal("ETHUSDT", coins[1].Symbol)
		s.Equal("BNBUSDT", coins[2].Symbol)
		s.Contains(coins[0].Sources, "default")
	})

	s.Run("Use custom coins", func() {
		s.autoTrader.tradingCoins = []string{"SOL", "AVAX"}

		coins, err := s.autoTrader.getCandidateCoins()

		s.NoError(err)
		s.Equal(2, len(coins))
		s.Equal("SOLUSDT", coins[0].Symbol)
		s.Equal("AVAXUSDT", coins[1].Symbol)
		s.Contains(coins[0].Sources, "custom")
	})

	s.Run("Use AI500+OI as fallback", func() {
		s.autoTrader.defaultCoins = []string{} // Empty default coins
		s.autoTrader.tradingCoins = []string{} // Empty custom coins

		// Mock pool.GetMergedCoinPool
		s.patches.ApplyFunc(pool.GetMergedCoinPool, func(ai500Limit int) (*pool.MergedCoinPool, error) {
			return &pool.MergedCoinPool{
				AllSymbols: []string{"BTCUSDT", "ETHUSDT"},
				SymbolSources: map[string][]string{
					"BTCUSDT": {"ai500", "oi_top"},
					"ETHUSDT": {"ai500"},
				},
			}, nil
		})

		coins, err := s.autoTrader.getCandidateCoins()

		s.NoError(err)
		s.Equal(2, len(coins))
	})
}

// ============================================================
// Level 8: buildTradingContext tests
// ============================================================

func (s *AutoTraderTestSuite) TestBuildTradingContext() {
	// Mock market.Get
	s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: 50000.0}, nil
	})

	ctx, err := s.autoTrader.buildTradingContext()

	s.NoError(err)
	s.NotNil(ctx)

	// Verify core fields
	s.Equal(10100.0, ctx.Account.TotalEquity) // 10000 + 100
	s.Equal(8000.0, ctx.Account.AvailableBalance)
	s.Equal(10, ctx.BTCETHLeverage)
	s.Equal(5, ctx.AltcoinLeverage)
}

// ============================================================
// Level 9: Trade execution tests
// ============================================================

// TestExecuteOpenPosition test open position operations (long/short common)
func (s *AutoTraderTestSuite) TestExecuteOpenPosition() {
	tests := []struct {
		name          string
		action        string
		expectedOrder int64
		existingSide  string
		availBalance  float64
		expectedErr   string
		executeFn     func(*decision.Decision, *logger.DecisionAction) error
	}{
		{
			name:          "Successfully open long position",
			action:        "open_long",
			expectedOrder: 123456,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:          "Successfully open short position",
			action:        "open_short",
			expectedOrder: 123457,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long position_insufficient margin",
			action:       "open_long",
			availBalance: 0.0,
			expectedErr:  "insufficient margin",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short position_insufficient margin",
			action:       "open_short",
			availBalance: 0.0,
			expectedErr:  "insufficient margin",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long position_already has same direction position",
			action:       "open_long",
			existingSide: "long",
			availBalance: 8000.0,
			expectedErr:  "already has long position",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short position_already has same direction position",
			action:       "open_short",
			existingSide: "short",
			availBalance: 8000.0,
			expectedErr:  "already has short position",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
				return &market.Data{Symbol: symbol, CurrentPrice: 50000.0}, nil
			})

			s.mockTrader.balance["availableBalance"] = tt.availBalance
			if tt.existingSide != "" {
				s.mockTrader.positions = []map[string]interface{}{{"symbol": "BTCUSDT", "side": tt.existingSide}}
			} else {
				s.mockTrader.positions = []map[string]interface{}{}
			}

			decision := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT", PositionSizeUSD: 1000.0, Leverage: 10}
			actionRecord := &logger.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(decision, actionRecord)

			if tt.expectedErr != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedErr)
			} else {
				s.NoError(err)
				s.Equal(tt.expectedOrder, actionRecord.OrderID)
				s.Greater(actionRecord.Quantity, 0.0)
				s.Equal(50000.0, actionRecord.Price)
			}

			// Restore default state
			s.mockTrader.balance["availableBalance"] = 8000.0
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

// TestExecuteClosePosition test close position operations (long/short common)
func (s *AutoTraderTestSuite) TestExecuteClosePosition() {
	tests := []struct {
		name          string
		action        string
		currentPrice  float64
		expectedOrder int64
		executeFn     func(*decision.Decision, *logger.DecisionAction) error
	}{
		{
			name:          "Successfully close long position",
			action:        "close_long",
			currentPrice:  51000.0,
			expectedOrder: 123458,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeCloseLongWithRecord(d, a)
			},
		},
		{
			name:          "Successfully close short position",
			action:        "close_short",
			currentPrice:  49000.0,
			expectedOrder: 123459,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeCloseShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
				return &market.Data{Symbol: symbol, CurrentPrice: tt.currentPrice}, nil
			})

			decision := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT"}
			actionRecord := &logger.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(decision, actionRecord)

			s.NoError(err)
			s.Equal(tt.expectedOrder, actionRecord.OrderID)
			s.Equal(tt.currentPrice, actionRecord.Price)
		})
	}
}

// TestExecuteUpdateStopOrTakeProfit test updating stop loss/take profit (long/short common)
func (s *AutoTraderTestSuite) TestExecuteUpdateStopOrTakeProfit() {
	// Use pointer variable to control market.Get return value
	var testPrice *float64
	s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		price := 50000.0
		if testPrice != nil {
			price = *testPrice
		}
		return &market.Data{Symbol: symbol, CurrentPrice: price}, nil
	})

	tests := []struct {
		name         string
		action       string
		symbol       string
		side         string
		currentPrice float64
		newPrice     float64
		hasPosition  bool
		expectedErr  string
		executeFn    func(*decision.Decision, *logger.DecisionAction) error
	}{
		{
			name:         "Successfully update long stop loss",
			action:       "update_stop_loss",
			symbol:       "BTCUSDT",
			side:         "long",
			currentPrice: 52000.0,
			newPrice:     51000.0,
			hasPosition:  true,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateStopLossWithRecord(d, a)
			},
		},
		{
			name:         "Successfully update short stop loss",
			action:       "update_stop_loss",
			symbol:       "ETHUSDT",
			side:         "short",
			currentPrice: 2900.0,
			newPrice:     2950.0,
			hasPosition:  true,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateStopLossWithRecord(d, a)
			},
		},
		{
			name:         "Successfully update long take profit",
			action:       "update_take_profit",
			symbol:       "BTCUSDT",
			side:         "long",
			currentPrice: 52000.0,
			newPrice:     55000.0,
			hasPosition:  true,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateTakeProfitWithRecord(d, a)
			},
		},
		{
			name:         "Successfully update short take profit",
			action:       "update_take_profit",
			symbol:       "ETHUSDT",
			side:         "short",
			currentPrice: 2900.0,
			newPrice:     2800.0,
			hasPosition:  true,
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateTakeProfitWithRecord(d, a)
			},
		},
		{
			name:         "Long stop loss price unreasonable",
			action:       "update_stop_loss",
			symbol:       "BTCUSDT",
			side:         "long",
			currentPrice: 50000.0,
			newPrice:     51000.0,
			hasPosition:  true,
			expectedErr:  "long stop loss must be below current price",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateStopLossWithRecord(d, a)
			},
		},
		{
			name:         "Long take profit price unreasonable",
			action:       "update_take_profit",
			symbol:       "BTCUSDT",
			side:         "long",
			currentPrice: 50000.0,
			newPrice:     49000.0,
			hasPosition:  true,
			expectedErr:  "long take profit must be above current price",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateTakeProfitWithRecord(d, a)
			},
		},
		{
			name:         "Stop loss_position does not exist",
			action:       "update_stop_loss",
			symbol:       "BTCUSDT",
			currentPrice: 50000.0,
			newPrice:     49000.0,
			hasPosition:  false,
			expectedErr:  "position does not exist",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateStopLossWithRecord(d, a)
			},
		},
		{
			name:         "Take profit_position does not exist",
			action:       "update_take_profit",
			symbol:       "BTCUSDT",
			currentPrice: 50000.0,
			newPrice:     55000.0,
			hasPosition:  false,
			expectedErr:  "position does not exist",
			executeFn: func(d *decision.Decision, a *logger.DecisionAction) error {
				return s.autoTrader.executeUpdateTakeProfitWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			// Set price for current test case
			testPrice = &tt.currentPrice

			if tt.hasPosition {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": tt.symbol, "side": tt.side, "positionAmt": 0.1},
				}
			} else {
				s.mockTrader.positions = []map[string]interface{}{}
			}

			decision := &decision.Decision{Action: tt.action, Symbol: tt.symbol}
			if tt.action == "update_stop_loss" {
				decision.NewStopLoss = tt.newPrice
			} else {
				decision.NewTakeProfit = tt.newPrice
			}
			actionRecord := &logger.DecisionAction{Action: tt.action, Symbol: tt.symbol}

			err := tt.executeFn(decision, actionRecord)

			if tt.expectedErr != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedErr)
			} else {
				s.NoError(err)
				s.Equal(tt.currentPrice, actionRecord.Price)
			}

			// Restore default state
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

func (s *AutoTraderTestSuite) TestExecutePartialCloseWithRecord() {
	s.Run("Successfully partial close", func() {
		// Set positions
		s.mockTrader.positions = []map[string]interface{}{
			{
				"symbol":      "BTCUSDT",
				"side":        "long",
				"positionAmt": 0.1,
				"entryPrice":  50000.0,
				"markPrice":   52000.0,
			},
		}

		// Mock market.Get
		s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
			return &market.Data{
				Symbol:       symbol,
				CurrentPrice: 52000.0,
			}, nil
		})

		decision := &decision.Decision{
			Action:          "partial_close",
			Symbol:          "BTCUSDT",
			ClosePercentage: 50.0,
		}

		actionRecord := &logger.DecisionAction{
			Action: "partial_close",
			Symbol: "BTCUSDT",
		}

		err := s.autoTrader.executePartialCloseWithRecord(decision, actionRecord)

		s.NoError(err)
		s.Equal(0.05, actionRecord.Quantity) // 50% of 0.1
	})

	s.Run("Invalid close percentage", func() {
		decision := &decision.Decision{
			Action:          "partial_close",
			Symbol:          "BTCUSDT",
			ClosePercentage: 150.0, // Invalid
		}

		actionRecord := &logger.DecisionAction{}

		err := s.autoTrader.executePartialCloseWithRecord(decision, actionRecord)

		s.Error(err)
		s.Contains(err.Error(), "close percentage must be between 0-100")
	})
}

// ============================================================
// Level 10: executeDecisionWithRecord routing tests
// ============================================================

func (s *AutoTraderTestSuite) TestExecuteDecisionWithRecord() {
	// Mock market.Get
	s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{
			Symbol:       symbol,
			CurrentPrice: 50000.0,
		}, nil
	})

	s.Run("Route to open_long", func() {
		decision := &decision.Decision{
			Action:          "open_long",
			Symbol:          "BTCUSDT",
			PositionSizeUSD: 1000.0,
			Leverage:        10,
		}
		actionRecord := &logger.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to close_long", func() {
		decision := &decision.Decision{
			Action: "close_long",
			Symbol: "BTCUSDT",
		}
		actionRecord := &logger.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to hold_no execution", func() {
		decision := &decision.Decision{
			Action: "hold",
			Symbol: "BTCUSDT",
		}
		actionRecord := &logger.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Unknown action returns error", func() {
		decision := &decision.Decision{
			Action: "unknown_action",
			Symbol: "BTCUSDT",
		}
		actionRecord := &logger.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.Error(err)
		s.Contains(err.Error(), "unknown action")
	})
}

func (s *AutoTraderTestSuite) TestCheckPositionDrawdown() {
	tests := []struct {
		name             string
		setupPositions   func()
		setupPeakPnL     func()
		setupFailures    func()
		cleanupFailures  func()
		expectedCacheKey string
		shouldClearCache bool
		skipCacheCheck   bool
	}{
		{
			name:            "Get positions failed_no panic",
			setupFailures:   func() { s.mockTrader.shouldFailPositions = true },
			cleanupFailures: func() { s.mockTrader.shouldFailPositions = false },
			skipCacheCheck:  true,
		},
		{
			name:           "No positions_no panic",
			setupPositions: func() { s.mockTrader.positions = []map[string]interface{}{} },
			skipCacheCheck: true,
		},
		{
			name: "Profit less than 5%_do not trigger close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50150.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long") },
			skipCacheCheck: true,
		},
		{
			name: "Drawdown less than 40%_do not trigger close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50400.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			skipCacheCheck: true,
		},
		{
			name: "Long position_trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: true,
		},
		{
			name: "Short position_trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: true,
		},
		{
			name: "Long position_close failed_keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseLong = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseLong = false },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: false,
		},
		{
			name: "Short position_close failed_keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseShort = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseShort = false },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupPositions != nil {
				tt.setupPositions()
			}
			if tt.setupPeakPnL != nil {
				tt.setupPeakPnL()
			}
			if tt.setupFailures != nil {
				tt.setupFailures()
			}
			if tt.cleanupFailures != nil {
				defer tt.cleanupFailures()
			}

			s.autoTrader.checkPositionDrawdown()

			if !tt.skipCacheCheck {
				cache := s.autoTrader.GetPeakPnLCache()
				_, exists := cache[tt.expectedCacheKey]
				if tt.shouldClearCache {
					s.False(exists, "Peak cache should be cleared")
				} else {
					s.True(exists, "Peak cache should not be cleared")
				}
			}

			// Cleanup state
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

// ============================================================
// Mock implementations
// ============================================================

// MockDatabase mock database
type MockDatabase struct {
	shouldFail bool
}

func (m *MockDatabase) UpdateTraderInitialBalance(userID, traderID string, newBalance float64) error {
	if m.shouldFail {
		return errors.New("database error")
	}
	return nil
}

// MockTrader enhanced version (with error control)
type MockTrader struct {
	balance              map[string]interface{}
	positions            []map[string]interface{}
	shouldFailBalance    bool
	shouldFailPositions  bool
	shouldFailOpenLong   bool
	shouldFailCloseLong  bool
	shouldFailCloseShort bool
}

func (m *MockTrader) GetBalance() (map[string]interface{}, error) {
	if m.shouldFailBalance {
		return nil, errors.New("failed to get balance")
	}
	if m.balance == nil {
		return map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		}, nil
	}
	return m.balance, nil
}

func (m *MockTrader) GetPositions() ([]map[string]interface{}, error) {
	if m.shouldFailPositions {
		return nil, errors.New("failed to get positions")
	}
	if m.positions == nil {
		return []map[string]interface{}{}, nil
	}
	return m.positions, nil
}

func (m *MockTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if m.shouldFailOpenLong {
		return nil, errors.New("failed to open long")
	}
	return map[string]interface{}{
		"orderId": int64(123456),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"orderId": int64(123457),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseLong {
		return nil, errors.New("failed to close long")
	}
	return map[string]interface{}{
		"orderId": int64(123458),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseShort {
		return nil, errors.New("failed to close short")
	}
	return map[string]interface{}{
		"orderId": int64(123459),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *MockTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (m *MockTrader) GetMarketPrice(symbol string) (float64, error) {
	return 50000.0, nil
}

func (m *MockTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (m *MockTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (m *MockTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (m *MockTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

// ============================================================
// Test suite entry point
// ============================================================

// TestAutoTraderTestSuite run AutoTrader test suite
func TestAutoTraderTestSuite(t *testing.T) {
	suite.Run(t, new(AutoTraderTestSuite))
}

// ============================================================
// Independent unit tests - calculatePnLPercentage function tests
// ============================================================

func TestCalculatePnLPercentage(t *testing.T) {
	tests := []struct {
		name          string
		unrealizedPnl float64
		marginUsed    float64
		expected      float64
	}{
		{
			name:          "Normal profit - 10x leverage",
			unrealizedPnl: 100.0,  // Profit 100 USDT
			marginUsed:    1000.0, // Margin 1000 USDT
			expected:      10.0,   // 10% return rate
		},
		{
			name:          "Normal loss - 10x leverage",
			unrealizedPnl: -50.0,  // Loss 50 USDT
			marginUsed:    1000.0, // Margin 1000 USDT
			expected:      -5.0,   // -5% return rate
		},
		{
			name:          "High leverage profit - price up 1%, 20x leverage",
			unrealizedPnl: 200.0,  // Profit 200 USDT
			marginUsed:    1000.0, // Margin 1000 USDT
			expected:      20.0,   // 20% return rate
		},
		{
			name:          "Margin is 0 - edge case",
			unrealizedPnl: 100.0,
			marginUsed:    0.0,
			expected:      0.0, // Should return 0 instead of divide by zero error
		},
		{
			name:          "Negative margin - edge case",
			unrealizedPnl: 100.0,
			marginUsed:    -1000.0,
			expected:      0.0, // Should return 0 (abnormal case)
		},
		{
			name:          "PnL is 0",
			unrealizedPnl: 0.0,
			marginUsed:    1000.0,
			expected:      0.0,
		},
		{
			name:          "Small trade",
			unrealizedPnl: 0.5,
			marginUsed:    10.0,
			expected:      5.0,
		},
		{
			name:          "Large profit",
			unrealizedPnl: 5000.0,
			marginUsed:    10000.0,
			expected:      50.0,
		},
		{
			name:          "Very small margin",
			unrealizedPnl: 1.0,
			marginUsed:    0.01,
			expected:      10000.0, // 100x return rate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercentage(tt.unrealizedPnl, tt.marginUsed)

			// Use precision comparison to avoid floating point errors
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("calculatePnLPercentage(%v, %v) = %v, want %v",
					tt.unrealizedPnl, tt.marginUsed, result, tt.expected)
			}
		})
	}
}

// TestCalculatePnLPercentage_RealWorldScenarios real-world scenario tests
func TestCalculatePnLPercentage_RealWorldScenarios(t *testing.T) {
	t.Run("BTC 10x leverage, price up 2%", func(t *testing.T) {
		// Open position: 1000 USDT margin, 10x leverage = 10000 USDT position
		// Price up 2% = 200 USDT profit
		// Return rate = 200 / 1000 = 20%
		result := calculatePnLPercentage(200.0, 1000.0)
		expected := 20.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("BTC scenario: got %v, want %v", result, expected)
		}
	})

	t.Run("ETH 5x leverage, price down 3%", func(t *testing.T) {
		// Open position: 2000 USDT margin, 5x leverage = 10000 USDT position
		// Price down 3% = -300 USDT loss
		// Return rate = -300 / 2000 = -15%
		result := calculatePnLPercentage(-300.0, 2000.0)
		expected := -15.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("ETH scenario: got %v, want %v", result, expected)
		}
	})

	t.Run("SOL 20x leverage, price up 0.5%", func(t *testing.T) {
		// Open position: 500 USDT margin, 20x leverage = 10000 USDT position
		// Price up 0.5% = 50 USDT profit
		// Return rate = 50 / 500 = 10%
		result := calculatePnLPercentage(50.0, 500.0)
		expected := 10.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("SOL scenario: got %v, want %v", result, expected)
		}
	})
}
