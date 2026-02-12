package api

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"nofx/decision"
	"nofx/trader"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// extractTraderIDs extracts and normalizes trader IDs from payload
// Supports both trader_id (string) and trader_ids (array) for backward compatibility
func (s *Server) extractTraderIDs(payload map[string]interface{}, userID string) ([]string, error) {
	var traderIDs []string

	// Debug: Log what we received
	if traderIDsRaw, exists := payload["trader_ids"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_ids field, type=%T, value=%v", traderIDsRaw, traderIDsRaw)
	}
	if traderIDRaw, exists := payload["trader_id"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_id field, type=%T, value=%v", traderIDRaw, traderIDRaw)
	}

	// Check for trader_ids array first (new format)
	// Handle both []interface{} and []string types from JSON unmarshaling
	if traderIDsRaw, exists := payload["trader_ids"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_ids field, type=%T", traderIDsRaw)

		// Try []interface{} first (common case)
		if traderIDsArray, ok := traderIDsRaw.([]interface{}); ok && len(traderIDsArray) > 0 {
			log.Printf("🔍 extractTraderIDs: Successfully parsed trader_ids as []interface{}, count=%d", len(traderIDsArray))
			for _, tid := range traderIDsArray {
				if tidStr, ok := tid.(string); ok && tidStr != "" {
					// Verify trader belongs to this user
					_, _, _, err := s.database.GetTraderConfig(userID, tidStr)
					if err != nil {
						log.Printf("⚠️ Trader %s does not belong to user %s", tidStr, userID)
						continue // Skip invalid trader IDs but continue processing others
					}
					traderIDs = append(traderIDs, tidStr)
				}
			}
			if len(traderIDs) > 0 {
				log.Printf("🔍 extractTraderIDs: Returning %d trader IDs from trader_ids array: %v", len(traderIDs), traderIDs)
				return traderIDs, nil
			}
		} else if traderIDsStringArray, ok := traderIDsRaw.([]string); ok && len(traderIDsStringArray) > 0 {
			// Handle []string type (alternative JSON unmarshaling)
			log.Printf("🔍 extractTraderIDs: Successfully parsed trader_ids as []string, count=%d", len(traderIDsStringArray))
			for _, tidStr := range traderIDsStringArray {
				if tidStr != "" {
					// Verify trader belongs to this user
					_, _, _, err := s.database.GetTraderConfig(userID, tidStr)
					if err != nil {
						log.Printf("⚠️ Trader %s does not belong to user %s", tidStr, userID)
						continue // Skip invalid trader IDs but continue processing others
					}
					traderIDs = append(traderIDs, tidStr)
				}
			}
			if len(traderIDs) > 0 {
				log.Printf("🔍 extractTraderIDs: Returning %d trader IDs from trader_ids []string array: %v", len(traderIDs), traderIDs)
				return traderIDs, nil
			}
		} else {
			log.Printf("⚠️ extractTraderIDs: trader_ids found but not a valid array type (got %T)", traderIDsRaw)
		}
	} else {
		log.Printf("🔍 extractTraderIDs: trader_ids field not found in payload")
	}

	// Fallback to single trader_id (backward compatibility)
	if tid, ok := payload["trader_id"].(string); ok && tid != "" {
		log.Printf("🔍 extractTraderIDs: Using single trader_id: %s", tid)
		// Verify trader belongs to this user
		_, _, _, err := s.database.GetTraderConfig(userID, tid)
		if err != nil {
			log.Printf("⚠️ Trader %s does not belong to user %s", tid, userID)
			return nil, fmt.Errorf("invalid trader_id")
		}
		return []string{tid}, nil
	}

	// If no trader IDs provided, auto-assign first trader with TradingView enabled
	log.Printf("🔍 extractTraderIDs: No trader_id or trader_ids found, attempting auto-assign")
	traders, err := s.database.GetTradersWithTradingViewEnabled(userID)
	if err != nil || len(traders) == 0 {
		log.Printf("⚠️ User %s has no traders with TradingView enabled", userID)
		return nil, fmt.Errorf("no trader with TradingView enabled found, please specify trader_id or trader_ids in webhook or enable TradingView signal in trader config")
	}
	traderIDs = []string{traders[0].ID}
	log.Printf("📌 Auto-assigned trader: %s", traders[0].ID)
	return traderIDs, nil
}

// forwardTradingViewAlertToFollowers forwards TradingView alert immediately to all running followers
// This provides instant signal propagation before parent AI processing completes.
// IMPORTANT: We reuse the parsed TradingViewAlert from the database so the child
// receives the same structured information (symbol, side, entry, SL/TP, size)
// that the parent sees, instead of relying on raw JSON field names.
func (s *Server) forwardTradingViewAlertToFollowers(traderID, alertID string) {
	// Get all follower traders from database
	followerRecords, err := s.database.GetFollowerTraders(traderID)
	if err != nil {
		log.Printf("⚠️ [%s] Failed to get follower list for instant forwarding: %v", traderID, err)
		return
	}

	if len(followerRecords) == 0 {
		return // No followers
	}

	// Load the normalized TradingView alert from database so we can use the same
	// parsed values (entry, quantity, position_size, SL, TP, action) that the
	// parent trader uses.
	alert, err := s.database.GetTradingViewAlertByID(alertID)
	if err != nil {
		log.Printf("⚠️ [%s] Failed to load TradingView alert %s for instant forwarding: %v", traderID, alertID, err)
		return
	}

	// Prefer alert's own symbol/action for logging (they are already normalized)
	log.Printf("⚡ [%s] Instant forwarding TradingView alert to %d followers (alertID=%s, symbol=%s, action=%s)",
		traderID, len(followerRecords), alertID, alert.Symbol, alert.Action)

	// Get parent trader instance for context
	parentTrader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		log.Printf("⚠️ [%s] Parent trader not in memory for instant forwarding: %v", traderID, err)
		return
	}

	// Get parent trader account info (for context in signal)
	parentAccount, err := parentTrader.GetAccountInfo()
	parentEquity := 0.0
	parentInitialBalance := 0.0
	if err == nil {
		if equity, ok := parentAccount["total_equity"].(float64); ok {
			parentEquity = equity
		}
		// Get parent initial balance from database
		if allUserIDs, err := s.database.GetAllUsers(); err == nil {
			for _, uid := range allUserIDs {
				traders, err := s.database.GetTraders(uid)
				if err != nil {
					continue
				}
				for _, traderCfg := range traders {
					if traderCfg.ID == traderID {
						parentInitialBalance = traderCfg.InitialBalance
						break
					}
				}
				if parentInitialBalance > 0 {
					break
				}
			}
		}
	}

	// Map TradingView action ("buy"/"sell") to internal action format used by traders.
	internalAction := "open_long"
	switch strings.ToLower(alert.Action) {
	case "buy":
		internalAction = "open_long"
	case "sell":
		internalAction = "open_short"
	default:
		log.Printf("⚠️ [%s] Unknown TradingView alert action '%s' for alert %s, skipping instant forwarding",
			traderID, alert.Action, alertID)
		return
	}

	// Determine effective quantity, mirroring the parent's logic:
	// prefer position_size, fall back to quantity.
	quantity := alert.PositionSize
	if quantity == 0 {
		quantity = alert.Quantity
	}
	if quantity == 0 {
		log.Printf("⚠️ [%s] TradingView alert %s has zero quantity/position_size, skipping instant forwarding",
			traderID, alertID)
		return
	}

	// Calculate approximate notional size in USD from the alert's own entry price.
	positionSizeUSD := quantity * alert.Entry

	// Use SL/TP from the normalized alert.
	stopLoss := alert.SL
	takeProfit := alert.TP

	// For followers, we let their own AI decide leverage; we mainly want to pass
	// through the parent's exact alert context (direction, size, entry, SL/TP).
	leverage := 0

	// Build a rich reasoning string that encodes the original alert details so
	// follower AI can fully understand the parent's intent.
	reasoning := fmt.Sprintf(
		"TradingView alert from parent: action=%s symbol=%s entry=%.4f sl=%.4f tp=%.4f quantity=%.6f (position_size) exchange=%s pricetype=%s",
		alert.Action, alert.Symbol, alert.Entry, alert.SL, alert.TP, alert.PositionSize, alert.Exchange, alert.PriceType,
	)

	// Build decision from TradingView alert (this is what followers see as the
	// parent signal). Their own AI can accept/modify/reject it.
	tvDecision := &decision.Decision{
		Symbol:              alert.Symbol,
		Action:              internalAction,
		Leverage:            leverage,
		PositionSizeUSD:     positionSizeUSD,
		StopLoss:            stopLoss,
		TakeProfit:          takeProfit,
		Reasoning:           reasoning,
		Confidence:          85, // Default confidence for TradingView signals
		TradingViewSignalID: alertID,
	}

	// Send signal to each follower trader
	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, followerRecord := range followerRecords {
		followerTrader, err := s.traderManager.GetTrader(followerRecord.ID)
		if err != nil {
			log.Printf("⚠️ [%s] Follower %s (%s) not in memory for instant forwarding, skipping: %v",
				traderID, followerRecord.Name, followerRecord.ID, err)
			skipCount++
			continue
		}

		// Check if follower is running
		status := followerTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); !ok || !isRunning {
			log.Printf("⚠️ [%s] Follower %s (%s) not running for instant forwarding, skipping",
				traderID, followerRecord.Name, followerRecord.ID)
			skipCount++
			continue
		}

		// Create unique signal ID for each follower
		signalID := uuid.New().String()

		// Deep copy decision for each follower
		decisionCopy := &decision.Decision{
			Symbol:              tvDecision.Symbol,
			Action:              tvDecision.Action,
			Leverage:            tvDecision.Leverage,
			PositionSizeUSD:     tvDecision.PositionSizeUSD,
			StopLoss:            tvDecision.StopLoss,
			TakeProfit:          tvDecision.TakeProfit,
			Reasoning:           tvDecision.Reasoning,
			Confidence:          tvDecision.Confidence,
			TradingViewSignalID: tvDecision.TradingViewSignalID,
		}

		// Build ParentTradeSignal
		signal := &trader.ParentTradeSignal{
			ParentTraderID:       traderID,
			ParentTraderName:     parentTrader.GetName(),
			SignalID:             signalID,
			Timestamp:            time.Now(),
			Decision:             decisionCopy,
			ParentEquity:         parentEquity,
			ParentInitialBalance: parentInitialBalance,
		}

		// Send signal to follower (non-blocking)
		if err := followerTrader.TriggerParentTradeSignal(signal); err != nil {
			log.Printf("❌ [%s] Failed to send instant TradingView signal to follower %s (%s): %v",
				traderID, followerRecord.Name, followerRecord.ID, err)
			errorCount++
		} else {
			log.Printf("✓ [%s] Instant TradingView signal sent to follower %s (%s) (signal_id=%s)",
				traderID, followerRecord.Name, followerRecord.ID, signalID)
			successCount++
		}
	}

	log.Printf("⚡ [%s] Instant forwarding complete: success=%d, skipped=%d, failed=%d, total=%d",
		traderID, successCount, skipCount, errorCount, len(followerRecords))
}

// processTraderWebhook processes webhook for a single trader
func (s *Server) processTraderWebhook(userID, traderID string, payload map[string]interface{}, symbol, action string) gin.H {
	log.Printf("🔄 Processing webhook for trader: %s (symbol=%s, action=%s)", traderID, symbol, action)

	// Create alert record with retry logic for UNIQUE constraint errors
	var alertID string
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		alertID, err = s.database.CreateTradingViewAlert(userID, traderID, payload)
		if err == nil {
			break // Success
		}

		// Check if it's a UNIQUE constraint error (collision)
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint") || strings.Contains(errStr, "constraint failed") {
			if attempt < maxRetries {
				log.Printf("⚠️ Alert ID collision detected for trader %s (attempt %d/%d), retrying...", traderID, attempt, maxRetries)
				time.Sleep(time.Duration(attempt) * time.Millisecond) // Small delay before retry
				continue
			}
			log.Printf("❌ Failed to create TradingView alert for trader %s after %d attempts: %v", traderID, maxRetries, err)
		} else {
			// Other error, don't retry
			log.Printf("❌ Failed to create TradingView alert for trader %s: %v", traderID, err)
			break
		}
	}

	if err != nil {
		return gin.H{
			"trader_id": traderID,
			"success":   false,
			"error":     fmt.Sprintf("Failed to save alert: %v", err),
		}
	}

	log.Printf("✅ TradingView alert received: user=%s, trader=%s, symbol=%s, action=%s, alertID=%s", userID, traderID, symbol, action, alertID)
	log.Printf("INFO: Webhook received (trader=%s, symbol=%s, action=%s, alertID=%s)", traderID, symbol, action, alertID)

	// ⚡ INSTANT FORWARDING: Forward alert immediately to followers (before parent AI processing)
	// This allows followers to react instantly using their own AI prompts/models
	go s.forwardTradingViewAlertToFollowers(traderID, alertID)

	// Build response object
	response := gin.H{
		"success":   true,
		"message":   "Alert received",
		"trader_id": traderID,
		"symbol":    symbol,
		"action":    action,
		"alert_id":  alertID,
	}

	// If trader has TradingView enabled, trigger immediate decision cycle
	at, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		log.Printf("⚠️ Failed to get trader instance %s: %v", traderID, err)
		log.Printf("ERROR: Trader instance not found (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
		response["error"] = fmt.Sprintf("Failed to get trader instance: %v", err)
		response["success"] = false
		return response
	}

	// Confirm from database if TradingView is enabled
	traderConfig, _, _, cfgErr := s.database.GetTraderConfig(userID, traderID)
	if cfgErr != nil {
		log.Printf("⚠️ Failed to get trader config %s: %v", traderID, cfgErr)
		log.Printf("WARN: Skip trigger because trader config load failed (trader=%s, alertID=%s, err=%v)", traderID, alertID, cfgErr)
		response["warning"] = "Failed to get trader config, decision not triggered"
		return response
	}

	if !traderConfig.UseTradingView {
		log.Printf("⚠️ [%s] TradingView signal not enabled, skipping trigger", traderID)
		log.Printf("WARN: Skip trigger because UseTradingView=false at DB (trader=%s, alertID=%s)", traderID, alertID)
		response["warning"] = "Trader has not enabled TradingView signal, decision not triggered"
		return response
	}

	// Check if trader is running
	status := at.GetStatus()
	isRunning, _ := status["is_running"].(bool)
	memUseTV, _ := status["use_tradingview"].(bool)
	dbIsRunning := traderConfig.IsRunning // Running status in database
	response["trader_running"] = isRunning
	response["triggered"] = false
	log.Printf("ℹ️ [%s] TradingView trigger check: is_running(mem)=%v, is_running(DB)=%v, UseTradingView(DB)=%v, UseTradingView(mem)=%v", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, memUseTV)
	if memUseTV != traderConfig.UseTradingView {
		log.Printf("WARN: TradingView flag mismatch between DB and memory (trader=%s, db=%v, mem=%v)", traderID, traderConfig.UseTradingView, memUseTV)
	}
	log.Printf("INFO: Trigger check (trader=%s, is_running(mem)=%v, is_running(DB)=%v, UseTradingView=%v, alertID=%s)", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, alertID)

	if !isRunning {
		// If DB status shows trader is stopped (user explicitly stopped), should not auto-start
		if !dbIsRunning {
			log.Printf("⏹️ [%s] Trader is stopped (DB status), not auto-starting to handle webhook", traderID)
			log.Printf("INFO: Trader is stopped in DB, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
			response["warning"] = "Trader is stopped, please manually start trader to handle webhook"
			return response
		}

		// If DB status shows running but not in memory, trader may have crashed, can auto-start
		if traderConfig != nil && traderConfig.UseTradingView {
			// Reload config first to ensure latest settings
			if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, userID, traderID); reloadErr != nil {
				log.Printf("⚠️ Failed to reload trader config before auto-start: %v", reloadErr)
				response["warning"] = "Failed to reload trader config"
				return response
			}

			// Re-check database status (prevent state changes during reload)
			freshConfig, _, _, freshCfgErr := s.database.GetTraderConfig(userID, traderID)
			if freshCfgErr != nil {
				log.Printf("⚠️ Failed to get fresh trader config after reload: %v", freshCfgErr)
				response["warning"] = "Failed to get latest trader config"
				return response
			}

			// If reload shows DB status changed to stopped, do not auto-start
			if !freshConfig.IsRunning {
				log.Printf("⏹️ [%s] Trader is stopped (DB status) after reload, not auto-starting to handle webhook", traderID)
				log.Printf("INFO: Trader is stopped in DB after reload, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "Trader is stopped, please manually start trader to handle webhook"
				return response
			}

			// Get fresh instance after reload
			at, err = s.traderManager.GetTrader(traderID)
			if err != nil {
				log.Printf("❌ Failed to get trader after reload: %v", err)
				response["error"] = "Failed to get trader instance"
				return response
			}
			// Auto-start trader (TradingView mode, only when trader crashed)
			log.Printf("🔄 [%s] TradingView mode: detected trader crash, auto-restarting to handle webhook", traderConfig.Name)
			log.Printf("INFO: Auto-start trader for TradingView webhook (trader crashed, trader=%s, alertID=%s)", traderID, alertID)
			go func() {
				if err := at.Run(); err != nil {
					log.Printf("❌ [%s] Auto-start failed: %v", traderConfig.Name, err)
					// Update database status to stopped
					_ = s.database.UpdateTraderStatus(userID, traderID, false)
				}
			}()
			// Update database status
			_ = s.database.UpdateTraderStatus(userID, traderID, true)
			// Wait for trader to start
			time.Sleep(500 * time.Millisecond)
			// Check running status again
			status = at.GetStatus()
			isRunning, _ = status["is_running"].(bool)
			response["trader_running"] = isRunning

			if !isRunning {
				log.Printf("⚠️ [%s] Trader still not running after auto-start, skipping decision cycle trigger", traderConfig.Name)
				log.Printf("WARN: Auto-start failed, trader still not running (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "Trader auto-start failed, please check logs and manually start"
				response["trigger_error"] = "trader failed to start"
				return response
			}

			response["warning"] = "Trader not running, auto-started (TradingView mode)"
			log.Printf("✓ [%s] Trader auto-started, ready to handle webhook", traderConfig.Name)
		} else {
			log.Printf("⚠️ [%s] Trader not running and TradingView not enabled, cannot auto-start", traderID)
			log.Printf("WARN: Trader not running and UseTradingView=false, cannot auto-start (trader=%s, alertID=%s)", traderID, alertID)
			response["warning"] = "Trader not running, please manually start trader to handle webhook"
			return response
		}
	}

	// Trigger decision cycle
	if err := at.TriggerTradingViewDecisionCycle(alertID); err != nil {
		log.Printf("⚠️ [%s] Failed to trigger decision cycle: %v", traderID, err)
		log.Printf("ERROR: Failed to trigger TradingView decision (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
		response["trigger_error"] = err.Error()
	} else {
		log.Printf("✓ [%s] Successfully triggered TradingView decision cycle: alertID=%s", traderID, alertID)
		log.Printf("INFO: Triggered TradingView decision successfully (trader=%s, alertID=%s)", traderID, alertID)
		log.Printf("📝 Decision will be logged for trader %s after AI analysis completes", traderID)
		response["triggered"] = true
	}

	return response
}

// sanitizeNumericValue sanitizes numeric values, handling NaN, Infinity, and invalid cases
func sanitizeNumericValue(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}

	switch val := v.(type) {
	case float64:
		// Check for NaN or Infinity
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, false
		}
		return val, true
	case float32:
		f := float64(val)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case string:
		// Handle string representations of NaN, Infinity, null, etc.
		val = strings.TrimSpace(strings.ToLower(val))
		if val == "" || val == "null" || val == "nan" || val == "undefined" {
			return 0, false
		}
		// Try to parse as float
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return 0, false
			}
			return f, true
		}
		return 0, false
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// normalizeSymbol normalizes symbol format (handles perpetuals, removes suffixes, etc.)
func normalizeSymbol(symbol string) string {
	if symbol == "" {
		return symbol
	}

	// Convert to uppercase
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Handle TradingView perpetual format (e.g., "BTCUSDT.P" or "BTCUSDT:PERP")
	// Remove .P suffix (TradingView perpetual format)
	symbol = strings.TrimSuffix(symbol, ".P")

	// Remove :PERP suffix
	symbol = strings.TrimSuffix(symbol, ":PERP")

	// Remove :USDT:PERP format
	if strings.Contains(symbol, ":USDT:PERP") {
		symbol = strings.ReplaceAll(symbol, ":USDT:PERP", "USDT")
	}

	// Ensure it ends with USDT if it's a valid base symbol
	if !strings.HasSuffix(symbol, "USDT") && len(symbol) > 0 {
		// Check if it's a common crypto symbol (3-5 chars typically)
		if len(symbol) <= 5 && symbol != "USDT" {
			symbol = symbol + "USDT"
		}
	}

	return symbol
}

// sanitizeWebhookPayload sanitizes the webhook payload, removing NaN values and normalizing data
func sanitizeWebhookPayload(payload map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range payload {
		// Skip API key fields (they should remain as-is)
		if key == "apikey" || key == "api_key" || key == "apiKey" || key == "API_KEY" {
			sanitized[key] = value
			continue
		}

		// Handle symbol field - normalize format
		if key == "symbol" {
			if str, ok := value.(string); ok {
				sanitized[key] = normalizeSymbol(str)
			} else {
				sanitized[key] = value
			}
			continue
		}

		// Handle numeric fields - sanitize NaN values
		numericFields := []string{"entry", "sl", "tp", "stop_loss", "take_profit", "quantity", "position_size", "size"}
		isNumericField := false
		for _, field := range numericFields {
			if strings.EqualFold(key, field) {
				isNumericField = true
				break
			}
		}

		if isNumericField {
			if num, valid := sanitizeNumericValue(value); valid {
				sanitized[key] = num
			}
			// If invalid (NaN, etc.), skip the field (don't include it in sanitized payload)
		} else {
			// For non-numeric fields, keep as-is but handle nested structures
			switch v := value.(type) {
			case map[string]interface{}:
				// Recursively sanitize nested maps
				sanitized[key] = sanitizeWebhookPayload(v)
			case []interface{}:
				// Sanitize arrays
				sanitizedArray := make([]interface{}, 0, len(v))
				for _, item := range v {
					if itemMap, ok := item.(map[string]interface{}); ok {
						sanitizedArray = append(sanitizedArray, sanitizeWebhookPayload(itemMap))
					} else {
						sanitizedArray = append(sanitizedArray, item)
					}
				}
				sanitized[key] = sanitizedArray
			default:
				sanitized[key] = value
			}
		}
	}

	return sanitized
}

// handleTradingViewWebhook handles TradingView webhook requests
func (s *Server) handleTradingViewWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// Sanitize the payload (remove NaN values, normalize symbols, etc.)
	payload = sanitizeWebhookPayload(payload)
	log.Printf("🧹 Webhook payload sanitized (removed NaN values, normalized symbols)")

	// Extract apikey (support multiple field name variations)
	var apikey string
	var ok bool

	// Try different field name variations (case-insensitive)
	if apikey, ok = payload["apikey"].(string); !ok || apikey == "" {
		if apikey, ok = payload["api_key"].(string); !ok || apikey == "" {
			if apikey, ok = payload["apiKey"].(string); !ok || apikey == "" {
				if apikey, ok = payload["API_KEY"].(string); !ok || apikey == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "Missing or invalid apikey field",
						"hint":  "The payload must contain 'apikey', 'api_key', 'apiKey', or 'API_KEY' field",
					})
					return
				}
			}
		}
	}

	// Trim whitespace from API key
	apikey = strings.TrimSpace(apikey)
	if apikey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key cannot be empty"})
		return
	}

	// Log API key info (first 8 chars only for security)
	keyPreview := apikey
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8] + "..."
	}
	log.Printf("🔑 Webhook API key received (length: %d, preview: %s)", len(apikey), keyPreview)

	// Validate API key and get user
	user, err := s.database.GetUserByWebhookAPIKey(apikey)
	if err != nil {
		log.Printf("⚠️ Invalid webhook API key (preview: %s, error: %v)", keyPreview, err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid API key",
			"hint":  "Please check your API key in the webhook configuration page. Make sure you're using the correct key for your account.",
		})
		return
	}

	log.Printf("✅ Webhook API key validated for user: %s", user.ID)

	// Extract and normalize trader_ids (support both trader_id string and trader_ids array)
	traderIDs, err := s.extractTraderIDs(payload, user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📋 Webhook received: %d trader(s) to process: %v", len(traderIDs), traderIDs)

	// Validate required fields
	symbol, ok := payload["symbol"].(string)
	if !ok || symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid symbol field"})
		return
	}

	// Normalize symbol (in case it wasn't normalized during sanitization)
	symbol = normalizeSymbol(symbol)
	payload["symbol"] = symbol

	action, ok := payload["action"].(string)
	if !ok || (action != "buy" && action != "sell") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid action field (must be buy or sell)"})
		return
	}

	// Process traders in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := []gin.H{}

	for _, traderID := range traderIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			result := s.processTraderWebhook(user.ID, tid, payload, symbol, action)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(traderID)
	}

	wg.Wait()

	log.Printf("✅ Webhook processing complete: %d trader(s) processed, %d result(s) returned", len(traderIDs), len(results))

	// Build response - maintain backward compatibility for single trader
	if len(traderIDs) == 1 && len(results) == 1 {
		// Single trader format (backward compatible)
		c.JSON(http.StatusOK, results[0])
		return
	}

	// Multiple traders format
	successful := 0
	failed := 0
	for _, result := range results {
		if success, ok := result["success"].(bool); ok && success {
			successful++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Alerts received for multiple traders",
		"total_traders": len(traderIDs),
		"successful":    successful,
		"failed":        failed,
		"results":       results,
	})
}

// handleTradingViewWebhookGET handles GET requests to the webhook endpoint
// This provides endpoint information and supports testing with query parameters
func (s *Server) handleTradingViewWebhookGET(c *gin.Context) {
	// Check if this is a test request with query parameters
	apikey := strings.TrimSpace(c.Query("apikey"))
	if apikey != "" {
		// Try to process as a webhook with query parameters (for testing)
		payload := make(map[string]interface{})
		payload["apikey"] = apikey

		// Extract other query parameters
		if symbol := c.Query("symbol"); symbol != "" {
			payload["symbol"] = symbol
		}
		if action := c.Query("action"); action != "" {
			payload["action"] = action
		}
		if traderID := c.Query("trader_id"); traderID != "" {
			payload["trader_id"] = traderID
		}
		if traderIDs := c.Query("trader_ids"); traderIDs != "" {
			// Parse comma-separated trader IDs
			ids := strings.Split(traderIDs, ",")
			payload["trader_ids"] = ids
		}

		// Extract numeric fields from query parameters
		if entry := c.Query("entry"); entry != "" {
			payload["entry"] = entry
		}
		if sl := c.Query("sl"); sl != "" {
			payload["sl"] = sl
		}
		if tp := c.Query("tp"); tp != "" {
			payload["tp"] = tp
		}
		if quantity := c.Query("quantity"); quantity != "" {
			payload["quantity"] = quantity
		}
		if positionSize := c.Query("position_size"); positionSize != "" {
			payload["position_size"] = positionSize
		}

		// Sanitize the payload (remove NaN values, normalize symbols, etc.)
		payload = sanitizeWebhookPayload(payload)
		log.Printf("🧹 Webhook payload sanitized (GET, removed NaN values, normalized symbols)")

		// Validate we have minimum required fields
		if _, ok := payload["symbol"].(string); !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing required parameter: symbol",
				"message": "This endpoint accepts POST requests with JSON body. For GET requests, provide query parameters: apikey, symbol, action, and optionally trader_id or trader_ids",
			})
			return
		}
		if _, ok := payload["action"].(string); !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing required parameter: action",
				"message": "This endpoint accepts POST requests with JSON body. For GET requests, provide query parameters: apikey, symbol, action, and optionally trader_id or trader_ids",
			})
			return
		}

		// Process as webhook (reuse POST handler logic)
		// Validate API key and get user
		keyPreview := apikey
		if len(keyPreview) > 8 {
			keyPreview = keyPreview[:8] + "..."
		}
		log.Printf("🔑 Webhook API key received (GET, length: %d, preview: %s)", len(apikey), keyPreview)

		user, err := s.database.GetUserByWebhookAPIKey(apikey)
		if err != nil {
			log.Printf("⚠️ Invalid webhook API key (GET, preview: %s, error: %v)", keyPreview, err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
				"hint":  "Please check your API key in the webhook configuration page. Make sure you're using the correct key for your account.",
			})
			return
		}

		log.Printf("✅ Webhook API key validated (GET) for user: %s", user.ID)

		// Extract and normalize trader_ids
		traderIDs, err := s.extractTraderIDs(payload, user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		symbol, _ := payload["symbol"].(string)
		action, _ := payload["action"].(string)

		log.Printf("📋 Webhook received (GET): %d trader(s) to process: %v", len(traderIDs), traderIDs)

		// Process traders in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := []gin.H{}

		for _, traderID := range traderIDs {
			wg.Add(1)
			go func(tid string) {
				defer wg.Done()
				result := s.processTraderWebhook(user.ID, tid, payload, symbol, action)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(traderID)
		}

		wg.Wait()

		// Build response
		if len(traderIDs) == 1 && len(results) == 1 {
			c.JSON(http.StatusOK, results[0])
			return
		}

		successful := 0
		failed := 0
		for _, result := range results {
			if success, ok := result["success"].(bool); ok && success {
				successful++
			} else {
				failed++
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       "Alerts received for multiple traders",
			"total_traders": len(traderIDs),
			"successful":    successful,
			"failed":        failed,
			"results":       results,
		})
		return
	}

	// No query parameters - return endpoint information
	c.JSON(http.StatusOK, gin.H{
		"endpoint":        "/api/webhook/tradingview",
		"method":          "POST (recommended) or GET (for testing)",
		"description":     "TradingView webhook endpoint for receiving trading alerts",
		"required_fields": []string{"apikey", "symbol", "action"},
		"optional_fields": []string{"trader_id", "trader_ids", "entry", "sl", "tp", "quantity", "position_size", "exchange", "pricetype"},
		"example_post": gin.H{
			"apikey":    "your_api_key",
			"symbol":    "BTCUSDT",
			"action":    "buy",
			"trader_id": "optional_trader_id",
		},
		"example_get": "/api/webhook/tradingview?apikey=your_key&symbol=BTCUSDT&action=buy",
		"note":        "For production use, configure TradingView to send POST requests with JSON body",
	})
}

// handleGetWebhookInfo gets user's webhook information
func (s *Server) handleGetWebhookInfo(c *gin.Context) {
	userID := c.GetString("user_id")

	// Generate or get API key
	apiKey, err := s.database.GenerateWebhookAPIKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key", "details": err.Error()})
		return
	}

	// Build webhook URL (get host from request)
	host := c.Request.Host
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	webhookURL := fmt.Sprintf("%s://%s/api/webhook/tradingview", scheme, host)

	c.JSON(http.StatusOK, gin.H{
		"webhook_url": webhookURL,
		"api_key":     apiKey,
	})
}

// handleGetRecentAlerts gets recent alerts
func (s *Server) handleGetRecentAlerts(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// Use database method to get recent alerts
	alerts, err := s.database.GetRecentTradingViewAlerts(userID, traderID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}
