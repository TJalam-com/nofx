package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FundingRateCache is the funding rate cache structure
// Binance Funding Rate only updates every 8 hours, using 1-hour cache can significantly reduce API calls
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
	klineCount     = 10 // Default number of klines to include in AI prompts
)

// SetKlineCount sets the number of klines to include in AI prompts
func SetKlineCount(count int) {
	if count > 0 {
		klineCount = count
	}
}

// getKlineCount returns the configured kline count (default: 10)
func getKlineCount() int {
	return klineCount
}

// IndicatorConfig holds configuration for which indicators to include in formatted output
type IndicatorConfig struct {
	EnableRawKlines    bool   // Raw OHLCV klines (always true, required)
	EnableEMA          bool   // Enable EMA indicator
	EnableMACD         bool   // Enable MACD indicator
	EnableRSI          bool   // Enable RSI indicator
	EnableATR          bool   // Enable ATR indicator
	EnableVolume       bool   // Enable volume data
	EnableOI           bool   // Enable open interest data
	EnableFunding      bool   // Enable funding rate data
	IndicatorTimeframe string // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
}

// Get retrieves market data for the specified token
func Get(symbol string) (*Data, error) {
	var klines3m, klines4h []Kline
	var err error
	// Normalize symbol
	symbol = Normalize(symbol)
	// Get 3-minute K-line data (latest 10)
	klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m") // Get more for calculation
	if err != nil {
		return nil, fmt.Errorf("Failed to get 3-minute K-line: %v", err)
	}

	// Data staleness detection: Prevent DOGEUSDT-style price freeze issues
	if isStaleData(klines3m, symbol) {
		log.Printf("⚠️  WARNING: %s detected stale data (consecutive price freeze), skipping symbol", symbol)
		return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol)
	}

	// Get 4-hour K-line data (latest 10)
	klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h") // Get more for indicator calculation
	if err != nil {
		return nil, fmt.Errorf("Failed to get 4-hour K-line: %v", err)
	}

	// Check if data is empty
	if len(klines3m) == 0 {
		return nil, fmt.Errorf("3-minute K-line data is empty")
	}
	if len(klines4h) == 0 {
		return nil, fmt.Errorf("4-hour K-line data is empty")
	}

	// Calculate current indicators (based on 3-minute latest data)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// Calculate price change percentage
	// 1-hour price change = price from 20 3-minute K-lines ago
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // Need at least 21 K-lines (current + 20 previous)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4-hour price change = price from 1 4-hour K-line ago
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// Get OI data
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI failure doesn't affect overall result, use default values
		oiData = &OIData{Latest: 0, Average: 0}
	}

	// Get Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// Calculate intraday series data
	// Use default kline count of 10 if not configured
	klineCount := getKlineCount()
	intradayData := calculateIntradaySeries(klines3m, klineCount)

	// Calculate longer-term data
	longerTermData := calculateLongerTermData(klines4h, klineCount)

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

// calculateEMA calculates EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// Calculate SMA as initial EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// Calculate EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD calculates MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// Calculate 12-period and 26-period EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI calculates RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain/loss
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Use Wilder smoothing method to calculate subsequent RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR calculates ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Calculate initial ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder smoothing
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries calculates intraday series data
func calculateIntradaySeries(klines []Kline, klineCount int) *IntradayData {
	if klineCount <= 0 {
		klineCount = 10 // Default to 10 if invalid
	}

	data := &IntradayData{
		MidPrices:   make([]float64, 0, klineCount),
		EMA20Values: make([]float64, 0, klineCount),
		MACDValues:  make([]float64, 0, klineCount),
		RSI7Values:  make([]float64, 0, klineCount),
		RSI14Values: make([]float64, 0, klineCount),
		Volume:      make([]float64, 0, klineCount),
		Klines:      make([]KlineBar, 0, klineCount),
	}

	// Get latest klineCount data points
	start := len(klines) - klineCount
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		k := klines[i]
		data.MidPrices = append(data.MidPrices, k.Close)
		data.Volume = append(data.Volume, k.Volume)

		// Store complete OHLCV data as KlineBar
		data.Klines = append(data.Klines, KlineBar{
			Time:   time.Unix(k.CloseTime/1000, 0),
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		})

		// Calculate EMA20 for each point
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// Calculate MACD for each point
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// Calculate RSI for each point
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// Calculate 3m ATR14
	data.ATR14 = calculateATR(klines, 14)

	return data
}

// calculateLongerTermData calculates longer-term data
func calculateLongerTermData(klines []Kline, klineCount int) *LongerTermData {
	if klineCount <= 0 {
		klineCount = 10 // Default to 10 if invalid
	}

	data := &LongerTermData{
		MACDValues:  make([]float64, 0, klineCount),
		RSI14Values: make([]float64, 0, klineCount),
		Klines:      make([]KlineBar, 0, klineCount),
	}

	// Calculate EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// Calculate ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// Calculate volume
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// Calculate average volume
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// Calculate MACD and RSI sequences
	// Get latest klineCount data points
	start := len(klines) - klineCount
	if start < 0 {
		start = 0
	}

	// Store complete OHLCV data as KlineBar
	for i := start; i < len(klines); i++ {
		k := klines[i]
		data.Klines = append(data.Klines, KlineBar{
			Time:   time.Unix(k.CloseTime/1000, 0),
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		})
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// getOpenInterestData gets OI data
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}

// getFundingRate gets funding rate (optimized: uses 1-hour cache)
func getFundingRate(symbol string) (float64, error) {
	// Check cache (valid for 1 hour)
	// Funding Rate only updates every 8 hours, 1-hour cache is very reasonable
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// Cache hit, return directly
			return cache.Rate, nil
		}
	}

	// Cache expired or doesn't exist, call API
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)

	// Update cache
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// Format formats and outputs market data
func Format(data *Data) string {
	var sb strings.Builder

	// Use dynamic precision to format price
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	sb.WriteString(fmt.Sprintf("current_price = %s, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		priceStr, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		// Use dynamic precision to format OI data
		oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
		oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
			oiLatestStr, oiAverageStr))
	}

	// Display multi-timeframe OI delta if quant data is available
	if data.QuantData != nil && data.QuantData.OIDelta != nil && len(data.QuantData.OIDelta) > 0 {
		oiDeltaParts := []string{}
		// Order timeframes: 1m, 5m, 15m, 30m, 1h, 4h, 8h, 12h, 24h, 2d, 3d
		timeframeOrder := []string{"1m", "5m", "15m", "30m", "1h", "4h", "8h", "12h", "24h", "2d", "3d"}
		for _, tf := range timeframeOrder {
			if delta, exists := data.QuantData.OIDelta[tf]; exists {
				sign := "+"
				if delta.OIDelta < 0 {
					sign = ""
				}
				oiDeltaParts = append(oiDeltaParts, fmt.Sprintf("%s: %s%.0f", tf, sign, delta.OIDelta))
			}
		}
		if len(oiDeltaParts) > 0 {
			sb.WriteString(fmt.Sprintf("OI Delta (%s)\n\n", strings.Join(oiDeltaParts, ", ")))
		}
	}

	// Display multi-timeframe Netflow if quant data is available
	if data.QuantData != nil && data.QuantData.Netflow != nil && len(data.QuantData.Netflow) > 0 {
		netflowParts := []string{}
		// Order timeframes: 1m, 5m, 15m, 30m, 1h, 4h, 8h, 12h, 24h, 2d, 3d
		timeframeOrder := []string{"1m", "5m", "15m", "30m", "1h", "4h", "8h", "12h", "24h", "2d", "3d"}
		for _, tf := range timeframeOrder {
			if netflow, exists := data.QuantData.Netflow[tf]; exists {
				total := netflow.Total
				// Format large numbers with K/M/B suffixes
				var formatted string
				if total >= 1000000000 {
					formatted = fmt.Sprintf("%.2fB", total/1000000000)
				} else if total >= 1000000 {
					formatted = fmt.Sprintf("%.2fM", total/1000000)
				} else if total >= 1000 {
					formatted = fmt.Sprintf("%.2fK", total/1000)
				} else {
					formatted = fmt.Sprintf("%.0f", total)
				}
				sign := "+"
				if total < 0 {
					sign = ""
				}
				netflowParts = append(netflowParts, fmt.Sprintf("%s: %s%s", tf, sign, formatted))
			}
		}
		if len(netflowParts) > 0 {
			sb.WriteString(fmt.Sprintf("Netflow (%s USDT)\n\n", strings.Join(netflowParts, ", ")))
		}
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		// Format klines as OHLCV table if available
		if len(data.IntradaySeries.Klines) > 0 {
			sb.WriteString("Kline data (OHLCV):\n")
			sb.WriteString(formatKlineTable(data.IntradaySeries.Klines))
			sb.WriteString("\n")
		}

		// Keep backward compatibility: show MidPrices array
		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}

		if len(data.IntradaySeries.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
		}

		sb.WriteString(fmt.Sprintf("3m ATR (14‑period): %.3f\n\n", data.IntradaySeries.ATR14))
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		// Format klines as OHLCV table if available
		if len(data.LongerTermContext.Klines) > 0 {
			sb.WriteString("Kline data (OHLCV):\n")
			sb.WriteString(formatKlineTable(data.LongerTermContext.Klines))
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	return sb.String()
}

// FormatWithIndicators formats market data based on indicator configuration
func FormatWithIndicators(data *Data, config *IndicatorConfig) string {
	var sb strings.Builder

	// Use dynamic precision to format price
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)

	// Build current price line with enabled indicators
	currentParts := []string{fmt.Sprintf("current_price = %s", priceStr)}

	if config.EnableEMA {
		currentParts = append(currentParts, fmt.Sprintf("current_ema20 = %.3f", data.CurrentEMA20))
	}
	if config.EnableMACD {
		currentParts = append(currentParts, fmt.Sprintf("current_macd = %.3f", data.CurrentMACD))
	}
	if config.EnableRSI {
		currentParts = append(currentParts, fmt.Sprintf("current_rsi (7 period) = %.3f", data.CurrentRSI7))
	}

	sb.WriteString(strings.Join(currentParts, ", "))
	sb.WriteString("\n\n")

	// Open interest and funding rate (only if enabled)
	if config.EnableOI || config.EnableFunding {
		sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
			data.Symbol))

		if config.EnableOI && data.OpenInterest != nil {
			// Use dynamic precision to format OI data
			oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
			oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
				oiLatestStr, oiAverageStr))
		}

		// Display multi-timeframe OI delta if quant data is available and OI is enabled
		if config.EnableOI && data.QuantData != nil && data.QuantData.OIDelta != nil && len(data.QuantData.OIDelta) > 0 {
			oiDeltaParts := []string{}
			// Order timeframes: 1m, 5m, 15m, 30m, 1h, 4h, 8h, 12h, 24h, 2d, 3d
			timeframeOrder := []string{"1m", "5m", "15m", "30m", "1h", "4h", "8h", "12h", "24h", "2d", "3d"}
			for _, tf := range timeframeOrder {
				if delta, exists := data.QuantData.OIDelta[tf]; exists {
					sign := "+"
					if delta.OIDelta < 0 {
						sign = ""
					}
					oiDeltaParts = append(oiDeltaParts, fmt.Sprintf("%s: %s%.0f", tf, sign, delta.OIDelta))
				}
			}
			if len(oiDeltaParts) > 0 {
				sb.WriteString(fmt.Sprintf("OI Delta (%s)\n\n", strings.Join(oiDeltaParts, ", ")))
			}
		}

		// Display multi-timeframe Netflow if quant data is available and OI is enabled
		if config.EnableOI && data.QuantData != nil && data.QuantData.Netflow != nil && len(data.QuantData.Netflow) > 0 {
			netflowParts := []string{}
			// Order timeframes: 1m, 5m, 15m, 30m, 1h, 4h, 8h, 12h, 24h, 2d, 3d
			timeframeOrder := []string{"1m", "5m", "15m", "30m", "1h", "4h", "8h", "12h", "24h", "2d", "3d"}
			for _, tf := range timeframeOrder {
				if netflow, exists := data.QuantData.Netflow[tf]; exists {
					total := netflow.Total
					// Format large numbers with K/M/B suffixes
					var formatted string
					if total >= 1000000000 {
						formatted = fmt.Sprintf("%.2fB", total/1000000000)
					} else if total >= 1000000 {
						formatted = fmt.Sprintf("%.2fM", total/1000000)
					} else if total >= 1000 {
						formatted = fmt.Sprintf("%.2fK", total/1000)
					} else {
						formatted = fmt.Sprintf("%.0f", total)
					}
					sign := "+"
					if total < 0 {
						sign = ""
					}
					netflowParts = append(netflowParts, fmt.Sprintf("%s: %s%s", tf, sign, formatted))
				}
			}
			if len(netflowParts) > 0 {
				sb.WriteString(fmt.Sprintf("Netflow (%s USDT)\n\n", strings.Join(netflowParts, ", ")))
			}
		}

		if config.EnableFunding {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		// Format klines as OHLCV table if available and raw klines are enabled
		if config.EnableRawKlines && len(data.IntradaySeries.Klines) > 0 {
			sb.WriteString("Kline data (OHLCV):\n")
			sb.WriteString(formatKlineTable(data.IntradaySeries.Klines))
			sb.WriteString("\n")
		}

		// Keep backward compatibility: show MidPrices array if raw klines enabled
		if config.EnableRawKlines && len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if config.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if config.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if config.EnableRSI && len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if config.EnableRSI && len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}

		if config.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
		}

		if config.EnableATR && data.IntradaySeries.ATR14 > 0 {
			sb.WriteString(fmt.Sprintf("3m ATR (14‑period): %.3f\n\n", data.IntradaySeries.ATR14))
		}
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		// Format klines as OHLCV table if available and raw klines are enabled
		if config.EnableRawKlines && len(data.LongerTermContext.Klines) > 0 {
			sb.WriteString("Kline data (OHLCV):\n")
			sb.WriteString(formatKlineTable(data.LongerTermContext.Klines))
			sb.WriteString("\n")
		}

		if config.EnableEMA {
			sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
				data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
		}

		if config.EnableATR {
			sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
				data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
		}

		if config.EnableVolume {
			sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
				data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
		}

		if config.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if config.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	return sb.String()
}

// formatPriceWithDynamicPrecision dynamically selects precision based on price range
// This perfectly supports all coins from ultra-low price meme coins (< 0.0001) to BTC/ETH
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// Ultra-low price meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8 decimal places)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// Low price meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6 decimal places)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// Mid-low price coin: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6 decimal places)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// Low price coin: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4 decimal places)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// Mid price coin: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4 decimal places)
		return fmt.Sprintf("%.4f", price)
	default:
		// High price coin: BTC, ETH (save tokens)
		// 45678.9123 → "45678.91" (2 decimal places)
		return fmt.Sprintf("%.2f", price)
	}
}

// formatFloatSlice formats float64 slice to string (using dynamic precision)
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// formatKlineTable formats klines as a readable table with Time, Open, High, Low, Close, Volume
// Marks the current (latest) bar for clarity
func formatKlineTable(klines []KlineBar) string {
	if len(klines) == 0 {
		return ""
	}

	var sb strings.Builder

	// Table header
	sb.WriteString("┌─────────────────────┬──────────────┬──────────────┬──────────────┬──────────────┬──────────────┐\n")
	sb.WriteString("│ Time                 │ Open         │ High         │ Low          │ Close        │ Volume       │\n")
	sb.WriteString("├─────────────────────┼──────────────┼──────────────┼──────────────┼──────────────┼──────────────┤\n")

	// Table rows (oldest to latest)
	for i, k := range klines {
		timeStr := k.Time.Format("2006-01-02 15:04:05")
		openStr := formatPriceWithDynamicPrecision(k.Open)
		highStr := formatPriceWithDynamicPrecision(k.High)
		lowStr := formatPriceWithDynamicPrecision(k.Low)
		closeStr := formatPriceWithDynamicPrecision(k.Close)
		volumeStr := formatPriceWithDynamicPrecision(k.Volume)

		// Mark current (latest) bar
		isCurrent := i == len(klines)-1
		marker := ""
		if isCurrent {
			marker = " ← (current)"
		}

		sb.WriteString(fmt.Sprintf("│ %-19s │ %-12s │ %-12s │ %-12s │ %-12s │ %-12s │%s\n",
			timeStr, openStr, highStr, lowStr, closeStr, volumeStr, marker))
	}

	// Table footer
	sb.WriteString("└─────────────────────┴──────────────┴──────────────┴──────────────┴──────────────┴──────────────┘")

	return sb.String()
}

// Normalize normalizes symbol, ensures it's a USDT trading pair
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat parses float value
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// BuildDataFromKlines constructs market data snapshot from preloaded kline series (for backtesting/simulation).
func BuildDataFromKlines(symbol string, primary []Kline, longer []Kline) (*Data, error) {
	if len(primary) == 0 {
		return nil, fmt.Errorf("primary series is empty")
	}

	symbol = Normalize(symbol)
	current := primary[len(primary)-1]
	currentPrice := current.Close

	data := &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		CurrentEMA20:      calculateEMA(primary, 20),
		CurrentMACD:       calculateMACD(primary),
		CurrentRSI7:       calculateRSI(primary, 7),
		PriceChange1h:     priceChangeFromSeries(primary, time.Hour),
		PriceChange4h:     priceChangeFromSeries(primary, 4*time.Hour),
		OpenInterest:      &OIData{Latest: 0, Average: 0},
		FundingRate:       0,
		IntradaySeries:    calculateIntradaySeries(primary, getKlineCount()),
		LongerTermContext: nil,
	}

	if len(longer) > 0 {
		data.LongerTermContext = calculateLongerTermData(longer, getKlineCount())
	}

	return data, nil
}

func priceChangeFromSeries(series []Kline, duration time.Duration) float64 {
	if len(series) == 0 || duration <= 0 {
		return 0
	}
	last := series[len(series)-1]
	target := last.CloseTime - duration.Milliseconds()
	for i := len(series) - 1; i >= 0; i-- {
		if series[i].CloseTime <= target {
			price := series[i].Close
			if price > 0 {
				return ((last.Close - price) / price) * 100
			}
			break
		}
	}
	return 0
}

// isStaleData detects stale data (consecutive price freeze)
// Fix DOGEUSDT-style issue: consecutive N periods with completely unchanged prices indicate data source anomaly
func isStaleData(klines []Kline, symbol string) bool {
	if len(klines) < 5 {
		return false // Insufficient data to determine
	}

	// Detection threshold: 5 consecutive 3-minute periods with unchanged price (15 minutes without fluctuation)
	const stalePriceThreshold = 5
	const priceTolerancePct = 0.0001 // 0.01% fluctuation tolerance (avoid false positives)

	// Take the last stalePriceThreshold K-lines
	recentKlines := klines[len(klines)-stalePriceThreshold:]
	firstPrice := recentKlines[0].Close

	// Check if all prices are within tolerance
	for i := 1; i < len(recentKlines); i++ {
		priceDiff := math.Abs(recentKlines[i].Close-firstPrice) / firstPrice
		if priceDiff > priceTolerancePct {
			return false // Price fluctuation exists, data is normal
		}
	}

	// Additional check: MACD and volume
	// If price is unchanged but MACD/volume shows normal fluctuation, it might be a real market situation (extremely low volatility)
	// Check if volume is also 0 (data completely frozen)
	allVolumeZero := true
	for _, k := range recentKlines {
		if k.Volume > 0 {
			allVolumeZero = false
			break
		}
	}

	if allVolumeZero {
		log.Printf("⚠️  %s stale data confirmed: price freeze + zero volume", symbol)
		return true
	}

	// Price frozen but has volume: might be extremely low volatility market, allow but log warning
	log.Printf("⚠️  %s detected extreme price stability (no fluctuation for %d consecutive periods), but volume is normal", symbol, stalePriceThreshold)
	return false
}
