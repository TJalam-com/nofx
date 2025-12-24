package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// AccountInfo LIGHTER account information
type AccountInfo struct {
	AccountIndex int64         `json:"account_index"`
	L1Address    string        `json:"l1_address"`
	SubAccounts  []AccountInfo `json:"sub_accounts,omitempty"` // Sub-accounts list (if available)
	// Other fields can be added based on actual API response
}

// LighterTraderV2 new implementation using official lighter-go SDK
type LighterTraderV2 struct {
	ctx        context.Context
	privateKey *ecdsa.PrivateKey // L1 wallet private key (for account identification, may be nil if wallet address is provided)
	walletAddr string            // Ethereum wallet address

	client  *http.Client
	baseURL string
	testnet bool // Always false (mainnet only)
	chainID uint32

	// SDK clients
	httpClient lighterClient.MinimalHTTPClient
	txClient   *lighterClient.TxClient

	// API Key management
	apiKeyPrivateKey string // 40-byte API Key private key (for signing transactions)
	apiKeyIndex      uint8  // API Key index (default 0)
	accountIndex     int64  // Account index

	// Sub-accounts support
	subAccounts []AccountInfo // List of sub-accounts (if available)

	// Authentication token
	authToken    string
	tokenExpiry  time.Time
	accountMutex sync.RWMutex

	// Market information cache
	symbolPrecision map[string]SymbolPrecision

	// Market index cache
	marketIndexMap map[string]uint8 // symbol -> market_id
	marketMutex    sync.RWMutex
}

// NewLighterTraderV2 creates a new LIGHTER trader (using official SDK)
// Parameters:
//   - walletAddr: Ethereum wallet address (L1 address)
//   - apiKeyPrivateKeyHex: API Key private key (40 bytes, for signing transactions)
//   - apiKeyIndex: API Key index (default 0)
//
// Note: Lighter only supports mainnet, testnet is disabled
func NewLighterTraderV2(walletAddr, apiKeyPrivateKeyHex string, apiKeyIndex int) (*LighterTraderV2, error) {
	// 1. Validate wallet address
	if walletAddr == "" {
		return nil, fmt.Errorf("wallet address is required")
	}

	// 2. Set API URL and Chain ID (mainnet only)
	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(42766) // Mainnet Chain ID

	// 3. Create HTTP client
	httpClient := lighterHTTP.NewClient(baseURL)

	trader := &LighterTraderV2{
		ctx:              context.Background(),
		privateKey:       nil, // Not needed when wallet address is provided
		walletAddr:       walletAddr,
		client:           &http.Client{Timeout: 30 * time.Second},
		baseURL:          baseURL,
		testnet:          false, // Always mainnet
		chainID:          chainID,
		httpClient:       httpClient,
		apiKeyPrivateKey: apiKeyPrivateKeyHex,
		apiKeyIndex:      uint8(apiKeyIndex),
		symbolPrecision:  make(map[string]SymbolPrecision),
		marketIndexMap:   make(map[string]uint8),
	}

	// 4. Initialize account (get account index)
	if err := trader.initializeAccount(); err != nil {
		return nil, fmt.Errorf("failed to initialize account: %w", err)
	}

	// 5. If no API Key provided, return error (API Key is required)
	if apiKeyPrivateKeyHex == "" {
		return nil, fmt.Errorf("API Key private key is required for Lighter V2")
	}

	// 6. Create TxClient (for signing transactions)
	txClient, err := lighterClient.NewTxClient(
		httpClient,
		apiKeyPrivateKeyHex,
		trader.accountIndex,
		trader.apiKeyIndex,
		trader.chainID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create TxClient: %w", err)
	}

	trader.txClient = txClient

	// 7. Verify API Key is correct
	if err := trader.checkClient(); err != nil {
		log.Printf("⚠️  API Key verification failed: %v", err)
		log.Printf("   You may need to regenerate API Key or check configuration")
		return trader, err
	}

	log.Printf("✓ LIGHTER trader initialized successfully (account=%d, apiKeyIndex=%d, mainnet only)",
		trader.accountIndex, trader.apiKeyIndex)

	return trader, nil
}

// initializeAccount initializes account information (gets account index)
func (t *LighterTraderV2) initializeAccount() error {
	// Get account information via L1 address
	accountInfo, err := t.getAccountByL1Address()
	if err != nil {
		return fmt.Errorf("failed to get account information: %w", err)
	}

	t.accountMutex.Lock()
	t.accountIndex = accountInfo.AccountIndex
	// Store sub-accounts if available
	if len(accountInfo.SubAccounts) > 0 {
		t.subAccounts = accountInfo.SubAccounts
		log.Printf("✓ Found %d sub-accounts", len(accountInfo.SubAccounts))
	}
	t.accountMutex.Unlock()

	log.Printf("✓ Account index: %d", t.accountIndex)
	return nil
}

// getAccountByL1Address gets LIGHTER account information via L1 wallet address
// Supports both single account and sub_accounts array responses
func (t *LighterTraderV2) getAccountByL1Address() (*AccountInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/account?by=address&value=%s", t.baseURL, url.QueryEscape(strings.TrimSpace(t.walletAddr)))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("failed to get account (status %d): %s", resp.StatusCode, string(body))
	}

	// Try to parse response - could be single account or object with sub_accounts
	var accountInfo AccountInfo
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		// If single account parsing fails, try parsing as array (accountsByL1Address format)
		var accountsArray []AccountInfo
		if arrayErr := json.Unmarshal(body, &accountsArray); arrayErr == nil && len(accountsArray) > 0 {
			// First element is main account, rest are sub-accounts
			accountInfo = accountsArray[0]
			if len(accountsArray) > 1 {
				accountInfo.SubAccounts = accountsArray[1:]
			}
		} else {
			return nil, fmt.Errorf("failed to parse account response: %w", err)
		}
	}

	// If response contains sub_accounts array, it will be automatically parsed via JSON tags
	// Return the account info (sub_accounts will be populated if API returns them)
	return &accountInfo, nil
}

// checkClient verifies API Key is correct
func (t *LighterTraderV2) checkClient() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get API Key public key registered on server
	publicKey, err := t.httpClient.GetApiKey(t.accountIndex, t.apiKeyIndex)
	if err != nil {
		return fmt.Errorf("failed to get API Key: %w", err)
	}

	// Get local API Key public key
	pubKeyBytes := t.txClient.GetKeyManager().PubKeyBytes()
	localPubKey := hexutil.Encode(pubKeyBytes[:])
	localPubKey = strings.Replace(localPubKey, "0x", "", 1)

	// Compare public keys
	if publicKey != localPubKey {
		return fmt.Errorf("API Key mismatch: local=%s, server=%s", localPubKey, publicKey)
	}

	log.Printf("✓ API Key verification passed")
	return nil
}

// GenerateAndRegisterAPIKey 生成新的 API Key 並註冊到 LIGHTER
// 注意：這需要 L1 私鑰簽名，所以必須在有 L1 私鑰的情況下調用
func (t *LighterTraderV2) GenerateAndRegisterAPIKey(seed string) (privateKey, publicKey string, err error) {
	// 這個功能需要調用官方 SDK 的 GenerateAPIKey 函數
	// 但這是在 sharedlib 中的 CGO 函數，無法直接在純 Go 代碼中調用
	//
	// 解決方案：
	// 1. 讓用戶從 LIGHTER 官網生成 API Key
	// 2. 或者我們可以實現一個簡單的 API Key 生成包裝器

	return "", "", fmt.Errorf("GenerateAndRegisterAPIKey 功能待實現，請從 LIGHTER 官網生成 API Key")
}

// refreshAuthToken refreshes authentication token (using official SDK)
func (t *LighterTraderV2) refreshAuthToken() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	// Generate authentication token using official SDK (valid for 7 hours)
	deadline := time.Now().Add(7 * time.Hour)
	authToken, err := t.txClient.GetAuthToken(deadline)
	if err != nil {
		return fmt.Errorf("failed to generate authentication token: %w", err)
	}

	t.accountMutex.Lock()
	t.authToken = authToken
	t.tokenExpiry = deadline
	t.accountMutex.Unlock()

	log.Printf("✓ Authentication token generated (valid until: %s)", t.tokenExpiry.Format(time.RFC3339))
	return nil
}

// ensureAuthToken ensures authentication token is valid
func (t *LighterTraderV2) ensureAuthToken() error {
	t.accountMutex.RLock()
	expired := time.Now().After(t.tokenExpiry.Add(-30 * time.Minute)) // Refresh 30 minutes early
	t.accountMutex.RUnlock()

	if expired {
		log.Println("🔄 Authentication token expiring soon, refreshing...")
		return t.refreshAuthToken()
	}

	return nil
}

// GetExchangeType gets exchange type
func (t *LighterTraderV2) GetExchangeType() string {
	return "lighter"
}

// Cleanup cleans up resources
func (t *LighterTraderV2) Cleanup() error {
	log.Println("⏹  LIGHTER trader cleanup completed")
	return nil
}
