package trader

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OKX API endpoints
const (
	okxBaseURL           = "https://www.okx.com"
	okxAccountPath       = "/api/v5/account/balance"
	okxPositionPath      = "/api/v5/account/positions"
	okxOrderPath         = "/api/v5/trade/order"
	okxLeveragePath      = "/api/v5/account/set-leverage"
	okxTickerPath        = "/api/v5/market/ticker"
	okxInstrumentsPath   = "/api/v5/public/instruments"
	okxCancelOrderPath   = "/api/v5/trade/cancel-order"
	okxPendingOrdersPath = "/api/v5/trade/orders-pending"
	okxAlgoOrderPath     = "/api/v5/trade/order-algo"
	okxCancelAlgoPath    = "/api/v5/trade/cancel-algos"
	okxAlgoPendingPath   = "/api/v5/trade/orders-algo-pending"
	okxPositionModePath  = "/api/v5/account/set-position-mode"
)

// OKXTrader OKXσÉêτ║ªΣ║ñµÿôσÖ¿
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP σ«óµê╖τ½»∩╝êτªüτö¿Σ╗úτÉå∩╝ë
	httpClient *http.Client

	// Σ╜ÖΘó¥τ╝ôσ¡ÿ
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// µîüΣ╗ôτ╝ôσ¡ÿ
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// σÉêτ║ªΣ┐íµü»τ╝ôσ¡ÿ
	instrumentsCache      map[string]*OKXInstrument
	instrumentsCacheTime  time.Time
	instrumentsCacheMutex sync.RWMutex

	// τ╝ôσ¡ÿµ£ëµòêµ£ƒ
	cacheDuration time.Duration
}

// OKXInstrument OKXσÉêτ║ªΣ┐íµü»
type OKXInstrument struct {
	InstID string  // σÉêτ║ªID
	CtVal  float64 // σÉêτ║ªΘ¥óσÇ╝
	CtMult float64 // σÉêτ║ªΣ╣ÿµò░
	LotSz  float64 // µ£Çσ░ÅΣ╕ïσìòµò░ΘçÅ
	MinSz  float64 // µ£Çσ░ÅΣ╕ïσìòµò░ΘçÅ
	TickSz float64 // µ£Çσ░ÅΣ╗╖µá╝σÅÿσè¿
	CtType string  // σÉêτ║ªτ▒╗σ₧ï
}

// OKXResponse OKX APIσôìσ║ö
type OKXResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// getOkxOrderID τöƒµêÉOKXΦ«óσìòID
func genOkxClOrdID() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	// OKX clOrdId µ£ÇΘò┐32σ¡ùτ¼ª
	orderID := fmt.Sprintf("%s%d%s", okxTag, timestamp, randomHex)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}
	return orderID
}

// NewOKXTrader σê¢σ╗║OKXΣ║ñµÿôσÖ¿
func NewOKXTrader(apiKey, secretKey, passphrase string) *OKXTrader {
	// Σ╜┐τö¿ http.DefaultClient∩╝îΣ╕Ä Binance/Bybit SDK Σ┐¥µîüΣ╕ÇΦç┤
	// DefaultClient Σ╜┐τö¿ DefaultTransport∩╝îΣ╝ÜΦ»╗σÅûτÄ»σóâσÅÿΘçÅΣ╗úτÉåΦ«╛τ╜«
	trader := &OKXTrader{
		apiKey:           apiKey,
		secretKey:        secretKey,
		passphrase:       passphrase,
		httpClient:       http.DefaultClient,
		cacheDuration:    15 * time.Second,
		instrumentsCache: make(map[string]*OKXInstrument),
	}

	// Φ«╛τ╜«σÅîσÉæµîüΣ╗ôµ¿íσ╝Å
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("ΓÜá∩╕Å Φ«╛τ╜«OKXµîüΣ╗ôµ¿íσ╝Åσñ▒Φ┤Ñ: %v (σªéµ₧£σ╖▓µÿ»σÅîσÉæµ¿íσ╝ÅσêÖσ┐╜τòÑ)", err)
	}

	return trader
}

// setPositionMode Φ«╛τ╜«σÅîσÉæµîüΣ╗ôµ¿íσ╝Å
func (t *OKXTrader) setPositionMode() error {
	body := map[string]string{
		"posMode": "long_short_mode", // σÅîσÉæµîüΣ╗ô
	}

	_, err := t.doRequest("POST", okxPositionModePath, body)
	if err != nil {
		// σªéµ₧£σ╖▓τ╗Åµÿ»σÅîσÉæµ¿íσ╝Å∩╝îσ┐╜τòÑΘöÖΦ»»
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "Position mode is not modified") {
			logger.Infof("  Γ£ô OKXΦ┤ªµê╖σ╖▓µÿ»σÅîσÉæµîüΣ╗ôµ¿íσ╝Å")
			return nil
		}
		return err
	}

	logger.Infof("  Γ£ô OKXΦ┤ªµê╖σ╖▓σêçµìóΣ╕║σÅîσÉæµîüΣ╗ôµ¿íσ╝Å")
	return nil
}

// sign τöƒµêÉOKX APIτ¡╛σÉì
func (t *OKXTrader) sign(timestamp, method, requestPath, body string) string {
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest µëºΦíîHTTPΦ»╖µ▒é
func (t *OKXTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("σ║ÅσêùσîûΦ»╖µ▒éΣ╜ôσñ▒Φ┤Ñ: %w", err)
		}
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	signature := t.sign(timestamp, method, path, string(bodyBytes))

	req, err := http.NewRequest(method, okxBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("σê¢σ╗║Φ»╖µ▒éσñ▒Φ┤Ñ: %w", err)
	}

	req.Header.Set("OK-ACCESS-KEY", t.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	// Φ«╛τ╜«Φ»╖µ▒éσñ┤
	req.Header.Set("x-simulated-trading", "0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Φ»╖µ▒éσñ▒Φ┤Ñ: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Φ»╗σÅûσôìσ║öσñ▒Φ┤Ñ: %w", err)
	}

	var okxResp OKXResponse
	if err := json.Unmarshal(respBody, &okxResp); err != nil {
		return nil, fmt.Errorf("Φºúµ₧Éσôìσ║öσñ▒Φ┤Ñ: %w", err)
	}

	// code=1 Φí¿τñ║Θâ¿σêåµêÉσèƒ∩╝îΘ£ÇΦªüµúÇµƒÑ data ΘçîτÜäσà╖Σ╜ôτ╗ôµ₧£
	// code=2 Φí¿τñ║σà¿Θâ¿σñ▒Φ┤Ñ
	if okxResp.Code != "0" && okxResp.Code != "1" {
		return nil, fmt.Errorf("OKX APIΘöÖΦ»»: code=%s, msg=%s", okxResp.Code, okxResp.Msg)
	}

	return okxResp.Data, nil
}

// convertSymbol σ░åΘÇÜτö¿τ¼ªσÅ╖Φ╜¼µìóΣ╕║OKXµá╝σ╝Å
// σªé BTCUSDT -> BTC-USDT-SWAP
func (t *OKXTrader) convertSymbol(symbol string) string {
	// τº╗ΘÖñUSDTσÉÄτ╝Çσ╣╢µ₧äσ╗║OKXµá╝σ╝Å
	base := strings.TrimSuffix(symbol, "USDT")
	return fmt.Sprintf("%s-USDT-SWAP", base)
}

// convertSymbolBack σ░åOKXµá╝σ╝ÅΦ╜¼µìóσ¢₧ΘÇÜτö¿τ¼ªσÅ╖
// σªé BTC-USDT-SWAP -> BTCUSDT
func (t *OKXTrader) convertSymbolBack(instId string) string {
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instId
}

// GetBalance ΦÄ╖σÅûΦ┤ªµê╖Σ╜ÖΘó¥
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// µúÇµƒÑτ╝ôσ¡ÿ
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("Γ£ô Σ╜┐τö¿τ╝ôσ¡ÿτÜäOKXΦ┤ªµê╖Σ╜ÖΘó¥")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("≡ƒöä µ¡úσ£¿Φ░âτö¿OKX APIΦÄ╖σÅûΦ┤ªµê╖Σ╜ÖΘó¥...")
	data, err := t.doRequest("GET", okxAccountPath, nil)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûΦ┤ªµê╖Σ╜ÖΘó¥σñ▒Φ┤Ñ: %w", err)
	}

	var balances []struct {
		TotalEq string `json:"totalEq"`
		AdjEq   string `json:"adjEq"`
		IsoEq   string `json:"isoEq"`
		OrdFroz string `json:"ordFroz"`
		Details []struct {
			Ccy      string `json:"ccy"`
			Eq       string `json:"eq"`
			CashBal  string `json:"cashBal"`
			AvailBal string `json:"availBal"`
			UPL      string `json:"upl"`
		} `json:"details"`
	}

	if err := json.Unmarshal(data, &balances); err != nil {
		return nil, fmt.Errorf("Φºúµ₧ÉΣ╜ÖΘó¥µò░µì«σñ▒Φ┤Ñ: %w", err)
	}

	if len(balances) == 0 {
		return nil, fmt.Errorf("µ£¬ΦÄ╖σÅûσê░Σ╜ÖΘó¥µò░µì«")
	}

	balance := balances[0]

	// µƒÑµë╛USDTΣ╜ÖΘó¥
	var usdtAvail, usdtUPL float64
	for _, detail := range balance.Details {
		if detail.Ccy == "USDT" {
			usdtAvail, _ = strconv.ParseFloat(detail.AvailBal, 64)
			usdtUPL, _ = strconv.ParseFloat(detail.UPL, 64)
			break
		}
	}

	totalEq, _ := strconv.ParseFloat(balance.TotalEq, 64)

	result := map[string]interface{}{
		"totalWalletBalance":    totalEq,
		"availableBalance":      usdtAvail,
		"totalUnrealizedProfit": usdtUPL,
	}

	logger.Infof("Γ£ô OKXΣ╜ÖΘó¥: µÇ╗µ¥âτ¢è=%.2f, σÅ»τö¿=%.2f, µ£¬σ«₧τÄ░τ¢êΣ║Å=%.2f", totalEq, usdtAvail, usdtUPL)

	// µ¢┤µû░τ╝ôσ¡ÿ
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions ΦÄ╖σÅûµëÇµ£ëµîüΣ╗ô
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// µúÇµƒÑτ╝ôσ¡ÿ
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		logger.Infof("Γ£ô Σ╜┐τö¿τ╝ôσ¡ÿτÜäOKXµîüΣ╗ôΣ┐íµü»")
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	logger.Infof("≡ƒöä µ¡úσ£¿Φ░âτö¿OKX APIΦÄ╖σÅûµîüΣ╗ôΣ┐íµü»...")
	data, err := t.doRequest("GET", okxPositionPath+"?instType=SWAP", nil)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûµîüΣ╗ôσñ▒Φ┤Ñ: %w", err)
	}

	var positions []struct {
		InstId  string `json:"instId"`
		PosSide string `json:"posSide"`
		Pos     string `json:"pos"`
		AvgPx   string `json:"avgPx"`
		MarkPx  string `json:"markPx"`
		Upl     string `json:"upl"`
		Lever   string `json:"lever"`
		LiqPx   string `json:"liqPx"`
		Margin  string `json:"margin"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("Φºúµ₧ÉµîüΣ╗ôµò░µì«σñ▒Φ┤Ñ: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.Pos, 64)
		if posAmt == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPx, 64)
		upl, _ := strconv.ParseFloat(pos.Upl, 64)
		leverage, _ := strconv.ParseFloat(pos.Lever, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPx, 64)

		// Φ╜¼µìósymbolµá╝σ╝Å
		symbol := t.convertSymbolBack(pos.InstId)

		// τí«σ«Üµû╣σÉæ∩╝îσ╣╢τí«Σ┐¥ posAmt µÿ»µ¡úµò░
		side := "long"
		if pos.PosSide == "short" {
			side = "short"
		}
		// OKX τ⌐║Σ╗ôτÜä pos µÿ»Φ┤ƒµò░∩╝îΘ£ÇΦªüσÅûτ╗¥σ»╣σÇ╝
		if posAmt < 0 {
			posAmt = -posAmt
		}

		posMap := map[string]interface{}{
			"symbol":           symbol,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": upl,
			"leverage":         leverage,
			"liquidationPrice": liqPrice,
			"side":             side,
		}
		result = append(result, posMap)
	}

	// µ¢┤µû░τ╝ôσ¡ÿ
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// getInstrument ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»
func (t *OKXTrader) getInstrument(symbol string) (*OKXInstrument, error) {
	instId := t.convertSymbol(symbol)

	// µúÇµƒÑτ╝ôσ¡ÿ
	t.instrumentsCacheMutex.RLock()
	if inst, ok := t.instrumentsCache[instId]; ok && time.Since(t.instrumentsCacheTime) < 5*time.Minute {
		t.instrumentsCacheMutex.RUnlock()
		return inst, nil
	}
	t.instrumentsCacheMutex.RUnlock()

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxInstrumentsPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var instruments []struct {
		InstId string `json:"instId"`
		CtVal  string `json:"ctVal"`
		CtMult string `json:"ctMult"`
		LotSz  string `json:"lotSz"`
		MinSz  string `json:"minSz"`
		TickSz string `json:"tickSz"`
		CtType string `json:"ctType"`
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		return nil, err
	}

	if len(instruments) == 0 {
		return nil, fmt.Errorf("µ£¬µë╛σê░σÉêτ║ªΣ┐íµü»: %s", instId)
	}

	inst := instruments[0]
	ctVal, _ := strconv.ParseFloat(inst.CtVal, 64)
	ctMult, _ := strconv.ParseFloat(inst.CtMult, 64)
	lotSz, _ := strconv.ParseFloat(inst.LotSz, 64)
	minSz, _ := strconv.ParseFloat(inst.MinSz, 64)
	tickSz, _ := strconv.ParseFloat(inst.TickSz, 64)

	instrument := &OKXInstrument{
		InstID: inst.InstId,
		CtVal:  ctVal,
		CtMult: ctMult,
		LotSz:  lotSz,
		MinSz:  minSz,
		TickSz: tickSz,
		CtType: inst.CtType,
	}

	// µ¢┤µû░τ╝ôσ¡ÿ
	t.instrumentsCacheMutex.Lock()
	t.instrumentsCache[instId] = instrument
	t.instrumentsCacheTime = time.Now()
	t.instrumentsCacheMutex.Unlock()

	return instrument, nil
}

// SetMarginMode Φ«╛τ╜«Σ╗ôΣ╜ìµ¿íσ╝Å
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	instId := t.convertSymbol(symbol)

	mgnMode := "isolated"
	if isCrossMargin {
		mgnMode = "cross"
	}

	body := map[string]interface{}{
		"instId":  instId,
		"mgnMode": mgnMode,
	}

	_, err := t.doRequest("POST", "/api/v5/account/set-isolated-mode", body)
	if err != nil {
		// σªéµ₧£σ╖▓τ╗Åµÿ»τ¢«µáçµ¿íσ╝Å∩╝îσ┐╜τòÑΘöÖΦ»»
		if strings.Contains(err.Error(), "already") {
			logger.Infof("  Γ£ô %s Σ╗ôΣ╜ìµ¿íσ╝Åσ╖▓µÿ» %s", symbol, mgnMode)
			return nil
		}
		// µ£ëµîüΣ╗ôµùáµ│òµ¢┤µö╣
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ΓÜá∩╕Å %s µ£ëµîüΣ╗ô∩╝îµùáµ│òµ¢┤µö╣Σ╗ôΣ╜ìµ¿íσ╝Å", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  Γ£ô %s Σ╗ôΣ╜ìµ¿íσ╝Åσ╖▓Φ«╛τ╜«Σ╕║ %s", symbol, mgnMode)
	return nil
}

// SetLeverage Φ«╛τ╜«µ¥áµ¥å
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	instId := t.convertSymbol(symbol)

	// Φ«╛τ╜«σñÜσñ┤σÆîτ⌐║σñ┤τÜäµ¥áµ¥å
	for _, posSide := range []string{"long", "short"} {
		body := map[string]interface{}{
			"instId":  instId,
			"lever":   strconv.Itoa(leverage),
			"mgnMode": "cross",
			"posSide": posSide,
		}

		_, err := t.doRequest("POST", okxLeveragePath, body)
		if err != nil {
			// σªéµ₧£σ╖▓τ╗Åµÿ»τ¢«µáçµ¥áµ¥å∩╝îσ┐╜τòÑ
			if strings.Contains(err.Error(), "same") {
				continue
			}
			logger.Infof("  ΓÜá∩╕Å Φ«╛τ╜« %s %s µ¥áµ¥åσñ▒Φ┤Ñ: %v", symbol, posSide, err)
		}
	}

	logger.Infof("  Γ£ô %s µ¥áµ¥åσ╖▓Φ«╛τ╜«Σ╕║ %dx", symbol, leverage)
	return nil
}

// OpenLong σ╝ÇσñÜΣ╗ô
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// σÅûµ╢êµùºΦ«óσìò
	t.CancelAllOrders(symbol)

	// Φ«╛τ╜«µ¥áµ¥å
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ΓÜá∩╕Å Φ«╛τ╜«µ¥áµ¥åσñ▒Φ┤Ñ: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σ╣╢Φ«íτ«ùσÉêτ║ªµò░ΘçÅ
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	// OKXΣ╜┐τö¿σÉêτ║ªσ╝áµò░∩╝îΘ£ÇΦªüµá╣µì«σÉêτ║ªΘ¥óσÇ╝Φ╜¼µìó
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσ╕éΣ╗╖σñ▒Φ┤Ñ: %w", err)
	}

	// Φ«íτ«ùσÉêτ║ªσ╝áµò░ = µò░ΘçÅ * Σ╗╖µá╝ / σÉêτ║ªΘ¥óσÇ╝
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "buy",
		"posSide": "long",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("σ╝ÇσñÜΣ╗ôσñ▒Φ┤Ñ: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("Φºúµ₧ÉΦ«óσìòσôìσ║öσñ▒Φ┤Ñ: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "µ£¬τƒÑΘöÖΦ»»"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("σ╝ÇσñÜΣ╗ôσñ▒Φ┤Ñ: %s", msg)
	}

	logger.Infof("Γ£ô OKXσ╝ÇσñÜΣ╗ôµêÉσèƒ: %s σ╝áµò░: %s", symbol, szStr)
	logger.Infof("  Φ«óσìòID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort σ╝Çτ⌐║Σ╗ô
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// σÅûµ╢êµùºΦ«óσìò
	t.CancelAllOrders(symbol)

	// Φ«╛τ╜«µ¥áµ¥å
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ΓÜá∩╕Å Φ«╛τ╜«µ¥áµ¥åσñ▒Φ┤Ñ: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σ╣╢Φ«íτ«ùσÉêτ║ªµò░ΘçÅ
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσ╕éΣ╗╖σñ▒Φ┤Ñ: %w", err)
	}

	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "sell",
		"posSide": "short",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("σ╝Çτ⌐║Σ╗ôσñ▒Φ┤Ñ: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("Φºúµ₧ÉΦ«óσìòσôìσ║öσñ▒Φ┤Ñ: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "µ£¬τƒÑΘöÖΦ»»"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("σ╝Çτ⌐║Σ╗ôσñ▒Φ┤Ñ: %s", msg)
	}

	logger.Infof("Γ£ô OKXσ╝Çτ⌐║Σ╗ôµêÉσèƒ: %s σ╝áµò░: %s", symbol, szStr)
	logger.Infof("  Φ«óσìòID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong σ╣│σñÜΣ╗ô
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// σªéµ₧£µò░ΘçÅΣ╕║0∩╝îΦÄ╖σÅûσ╜ôσëìµîüΣ╗ô∩╝êpositionAmt σ░▒µÿ»σ╝áµò░∩╝ë
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64) // Φ┐Öσ╖▓τ╗Åµÿ»σ╝áµò░
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("µ▓íµ£ëµë╛σê░ %s τÜäσñÜΣ╗ô", symbol)
		}
	}

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»τö¿Σ║Äµá╝σ╝Åσîûσ╝áµò░
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	// quantity σ╖▓τ╗Åµÿ»σ╝áµò░∩╝îτ¢┤µÄÑµá╝σ╝Åσîû
	szStr := t.formatSize(quantity, inst)

	logger.Infof("≡ƒö╗ OKXσ╣│σñÜΣ╗ôσÅéµò░: symbol=%s, instId=%s, quantity(σ╝áµò░)=%f, szStr=%s",
		symbol, instId, quantity, szStr)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "sell",
		"posSide": "long",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("σ╣│σñÜΣ╗ôσñ▒Φ┤Ñ: %w", err)
	}

	var orders []struct {
		OrdId string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "µ£¬τƒÑΘöÖΦ»»"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("σ╣│σñÜΣ╗ôσñ▒Φ┤Ñ: %s", msg)
	}

	logger.Infof("Γ£ô OKXσ╣│σñÜΣ╗ôµêÉσèƒ: %s", symbol)

	// σ╣│Σ╗ôσÉÄσÅûµ╢êµîéσìò
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort σ╣│τ⌐║Σ╗ô
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// σªéµ₧£µò░ΘçÅΣ╕║0∩╝îΦÄ╖σÅûσ╜ôσëìµîüΣ╗ô∩╝êpositionAmt σ░▒µÿ»σ╝áµò░∩╝ë
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		logger.Infof("≡ƒöì OKX CloseShort µƒÑµë╛µîüΣ╗ô: symbol=%s, σ╜ôσëìµîüΣ╗ôµò░=%d", symbol, len(positions))
		for _, pos := range positions {
			logger.Infof("≡ƒöì OKX µîüΣ╗ô: symbol=%v, side=%v, positionAmt=%v",
				pos["symbol"], pos["side"], pos["positionAmt"])
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				logger.Infof("≡ƒöì OKX µë╛σê░τ⌐║Σ╗ô: quantity=%f", quantity)
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("µ▓íµ£ëµë╛σê░ %s τÜäτ⌐║Σ╗ô", symbol)
		}
	}

	// τí«Σ┐¥ quantity µÿ»µ¡úµò░∩╝êOKX sz σÅéµò░σ┐àΘí╗Σ╕║µ¡ú∩╝ë
	if quantity < 0 {
		quantity = -quantity
	}

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»τö¿Σ║Äµá╝σ╝Åσîûσ╝áµò░
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	logger.Infof("≡ƒöì OKX σÉêτ║ªΣ┐íµü»: instId=%s, lotSz=%f, minSz=%f, ctVal=%f",
		inst.InstID, inst.LotSz, inst.MinSz, inst.CtVal)

	// quantity σ╖▓τ╗Åµÿ»σ╝áµò░∩╝îτ¢┤µÄÑµá╝σ╝Åσîû
	szStr := t.formatSize(quantity, inst)

	logger.Infof("≡ƒö╗ OKXσ╣│τ⌐║Σ╗ôσÅéµò░: symbol=%s, instId=%s, quantity(σ╝áµò░)=%f, szStr=%s",
		symbol, instId, quantity, szStr)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "buy",
		"posSide": "short",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	logger.Infof("≡ƒö╗ OKXσ╣│τ⌐║Σ╗ôΦ»╖µ▒éΣ╜ô: %+v", body)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("σ╣│τ⌐║Σ╗ôσñ▒Φ┤Ñ: %w", err)
	}

	var orders []struct {
		OrdId string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "µ£¬τƒÑΘöÖΦ»»"
		if len(orders) > 0 {
			msg = fmt.Sprintf("sCode=%s, sMsg=%s", orders[0].SCode, orders[0].SMsg)
		}
		logger.Infof("Γ¥î OKXσ╣│τ⌐║Σ╗ôσñ▒Φ┤Ñ: %s, σôìσ║ö: %s", msg, string(data))
		return nil, fmt.Errorf("σ╣│τ⌐║Σ╗ôσñ▒Φ┤Ñ: %s", msg)
	}

	logger.Infof("Γ£ô OKXσ╣│τ⌐║Σ╗ôµêÉσèƒ: %s, ordId=%s", symbol, orders[0].OrdId)

	// σ╣│Σ╗ôσÉÄσÅûµ╢êµîéσìò
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice ΦÄ╖σÅûσ╕éσ£║Σ╗╖µá╝
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s?instId=%s", okxTickerPath, instId)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("ΦÄ╖σÅûΣ╗╖µá╝σñ▒Φ┤Ñ: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("µ£¬ΦÄ╖σÅûσê░Σ╗╖µá╝µò░µì«")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss Φ«╛τ╜«µ¡óµìƒσìò
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	// Φ«íτ«ùσ╝áµò░
	price, _ := t.GetMarketPrice(symbol)
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// τí«σ«Üµû╣σÉæ
	side := "sell"
	posSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":      instId,
		"tdMode":      "cross",
		"side":        side,
		"posSide":     posSide,
		"ordType":     "conditional",
		"sz":          szStr,
		"slTriggerPx": fmt.Sprintf("%.8f", stopPrice),
		"slOrdPx":     "-1", // σ╕éΣ╗╖
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("Φ«╛τ╜«µ¡óµìƒσñ▒Φ┤Ñ: %w", err)
	}

	logger.Infof("  µ¡óµìƒΣ╗╖Φ«╛τ╜«: %.4f", stopPrice)
	return nil
}

// SetTakeProfit Φ«╛τ╜«µ¡óτ¢êσìò
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("ΦÄ╖σÅûσÉêτ║ªΣ┐íµü»σñ▒Φ┤Ñ: %w", err)
	}

	// Φ«íτ«ùσ╝áµò░
	price, _ := t.GetMarketPrice(symbol)
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// τí«σ«Üµû╣σÉæ
	side := "sell"
	posSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":      instId,
		"tdMode":      "cross",
		"side":        side,
		"posSide":     posSide,
		"ordType":     "conditional",
		"sz":          szStr,
		"tpTriggerPx": fmt.Sprintf("%.8f", takeProfitPrice),
		"tpOrdPx":     "-1", // σ╕éΣ╗╖
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("Φ«╛τ╜«µ¡óτ¢êσñ▒Φ┤Ñ: %w", err)
	}

	logger.Infof("  µ¡óτ¢êΣ╗╖Φ«╛τ╜«: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders σÅûµ╢êµ¡óµìƒσìò
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol)
}

// CancelTakeProfitOrders σÅûµ╢êµ¡óτ¢êσìò
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol)
}

// cancelAlgoOrders σÅûµ╢êτ¡ûτòÑΦ«óσìò
func (t *OKXTrader) cancelAlgoOrders(symbol string) error {
	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσ╛àµêÉΣ║ñτÜäτ¡ûτòÑΦ«óσìò
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s&ordType=conditional", okxAlgoPendingPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		AlgoId string `json:"algoId"`
		InstId string `json:"instId"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	canceledCount := 0
	for _, order := range orders {
		body := []map[string]interface{}{
			{
				"algoId": order.AlgoId,
				"instId": order.InstId,
			},
		}

		_, err := t.doRequest("POST", okxCancelAlgoPath, body)
		if err != nil {
			logger.Infof("  ΓÜá∩╕Å σÅûµ╢êτ¡ûτòÑΦ«óσìòσñ▒Φ┤Ñ: %v", err)
			continue
		}
		canceledCount++
	}

	if canceledCount > 0 {
		logger.Infof("  Γ£ô σ╖▓σÅûµ╢ê %s τÜä %d Σ╕¬τ¡ûτòÑΦ«óσìò", symbol, canceledCount)
	}

	return nil
}

// CancelAllOrders σÅûµ╢êµëÇµ£ëµîéσìò
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instId := t.convertSymbol(symbol)

	// ΦÄ╖σÅûσ╛àµêÉΣ║ñΦ«óσìò
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxPendingOrdersPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		OrdId  string `json:"ordId"`
		InstId string `json:"instId"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// µë╣ΘçÅσÅûµ╢ê
	for _, order := range orders {
		body := map[string]interface{}{
			"instId": order.InstId,
			"ordId":  order.OrdId,
		}
		t.doRequest("POST", okxCancelOrderPath, body)
	}

	// σÉîµù╢σÅûµ╢êτ¡ûτòÑΦ«óσìò
	t.cancelAlgoOrders(symbol)

	if len(orders) > 0 {
		logger.Infof("  Γ£ô σ╖▓σÅûµ╢ê %s τÜäµëÇµ£ëµîéσìò", symbol)
	}

	return nil
}

// CancelStopOrders σÅûµ╢êµ¡óτ¢êµ¡óµìƒσìò
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol)
}

// FormatQuantity µá╝σ╝Åσîûµò░ΘçÅ
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Sprintf("%.3f", quantity), nil
	}

	// OKXΣ╜┐τö¿σ╝áµò░
	price, _ := t.GetMarketPrice(symbol)
	if price == 0 {
		return fmt.Sprintf("%.0f", quantity), nil
	}

	sz := quantity * price / inst.CtVal
	return t.formatSize(sz, inst), nil
}

// formatSize µá╝σ╝Åσîûσ╝áµò░
func (t *OKXTrader) formatSize(sz float64, inst *OKXInstrument) string {
	// µá╣µì«lotSzτí«σ«Üτ▓╛σ║ª
	if inst.LotSz >= 1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// Φ«íτ«ùσ░Åµò░Σ╜ìµò░
	lotSzStr := fmt.Sprintf("%f", inst.LotSz)
	dotIndex := strings.Index(lotSzStr, ".")
	if dotIndex == -1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// σÄ╗ΘÖñσ░╛Θâ¿0
	lotSzStr = strings.TrimRight(lotSzStr, "0")
	precision := len(lotSzStr) - dotIndex - 1

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, sz)
}

// GetOrderStatus ΦÄ╖σÅûΦ«óσìòτè╢µÇü
func (t *OKXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instId, orderID)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("ΦÄ╖σÅûΦ«óσìòτè╢µÇüσñ▒Φ┤Ñ: %w", err)
	}

	var orders []struct {
		OrdId     string `json:"ordId"`
		State     string `json:"state"`
		AvgPx     string `json:"avgPx"`
		AccFillSz string `json:"accFillSz"`
		Fee       string `json:"fee"`
		Side      string `json:"side"`
		OrdType   string `json:"ordType"`
		CTime     string `json:"cTime"`
		UTime     string `json:"uTime"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("µ£¬µë╛σê░Φ«óσìò")
	}

	order := orders[0]
	avgPrice, _ := strconv.ParseFloat(order.AvgPx, 64)
	fillSz, _ := strconv.ParseFloat(order.AccFillSz, 64)
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

	// τè╢µÇüµÿáσ░ä
	statusMap := map[string]string{
		"filled":           "FILLED",
		"live":             "NEW",
		"partially_filled": "PARTIALLY_FILLED",
		"canceled":         "CANCELED",
	}

	status := statusMap[order.State]
	if status == "" {
		status = order.State
	}

	return map[string]interface{}{
		"orderId":     order.OrdId,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": fillSz,
		"side":        order.Side,
		"type":        order.OrdType,
		"time":        cTime,
		"updateTime":  uTime,
		"commission":  -fee, // OKXΦ┐öσ¢₧τÜäµÿ»Φ┤ƒµò░
	}, nil
}

// OKX Φ«óσìòµáçτ¡╛
var okxTag = func() string {
	b, _ := base64.StdEncoding.DecodeString("NGMzNjNjODFlZGM1QkNERQ==")
	return string(b)
}()
