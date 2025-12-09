package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// LeverageConfig Leverage configuration
type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"` // BTC and ETH leverage multiplier (main account recommended 5-50, sub-account ≤5)
	AltcoinLeverage int `json:"altcoin_leverage"` // Altcoin leverage multiplier (main account recommended 5-20, sub-account ≤5)
}

// LogConfig Logging configuration
type LogConfig struct {
	Level    string          `json:"level"`    // Log level: debug, info, warn, error (default: info)
	Telegram *TelegramConfig `json:"telegram"` // Telegram push configuration (optional)
}

// TelegramConfig Telegram push configuration (simplified version, only essential fields retained)
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`   // Whether enabled (default: false)
	BotToken string `json:"bot_token"` // Bot Token
	ChatID   int64  `json:"chat_id"`   // Chat ID
	MinLevel string `json:"min_level"` // Minimum log level, logs at this level and above will be pushed to Telegram (optional, default: error)
}

// Config Overall configuration
type Config struct {
	BetaMode           bool           `json:"beta_mode"`
	APIServerPort      int            `json:"api_server_port"`
	UseDefaultCoins    bool           `json:"use_default_coins"`
	DefaultCoins       []string       `json:"default_coins"`
	CoinPoolAPIURL     string         `json:"coin_pool_api_url"`
	OITopAPIURL        string         `json:"oi_top_api_url"`
	MaxDailyLoss       float64        `json:"max_daily_loss"`
	MaxDrawdown        float64        `json:"max_drawdown"`
	StopTradingMinutes int            `json:"stop_trading_minutes"`
	Leverage           LeverageConfig `json:"leverage"`
	JWTSecret          string         `json:"jwt_secret"`
	DataKLineTime      string         `json:"data_k_line_time"`
	Log                *LogConfig     `json:"log"` // Logging configuration
}

// LoadConfig Loads configuration from file
func LoadConfig(filename string) (*Config, error) {
	// Check if filename exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("📄 %s does not exist, using default configuration", filename)
		return &Config{}, nil
	}

	// Read filename
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// Parse JSON
	var configFile Config
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	return &configFile, nil
}
