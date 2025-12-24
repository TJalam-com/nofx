# LIGHTER DEX Configuration Investigation

## Current Implementation Analysis

### Backend Requirements (trader/lighter_trader_v2.go)

**Function:** `NewLighterTraderV2(walletAddr, apiKeyPrivateKeyHex string, apiKeyIndex int)`

**Required Parameters:**
1. `walletAddr` - Ethereum wallet address (L1 address) - **REQUIRED**
2. `apiKeyPrivateKeyHex` - API Key private key (40 bytes, for signing transactions) - **REQUIRED**
3. `apiKeyIndex` - API Key index (default 0) - **OPTIONAL** (defaults to 0)

**Note:** The L1 private key (`lighterPrivateKey`) is marked as **deprecated** in the code comments and is NOT needed when wallet address is provided.

### Database Schema (config/database.go)

The database stores these fields:
- `lighter_wallet_addr` - Wallet address (L1)
- `lighter_private_key` - L1 private key (deprecated, not used)
- `lighter_api_key_private_key` - API Key private key (40 bytes) - **REQUIRED**
- `lighter_api_key_index` - API Key index (default 0)

### Frontend Validation Issue (web/src/components/traders/ExchangeConfigModal.tsx)

**Line 277:** Current validation checks:
```typescript
if (!lighterWalletAddr.trim() || !lighterPrivateKey.trim()) {
  setIsLoading(false)
  return
}
```

**Problem:** The frontend validates `lighterPrivateKey` (L1 private key), but:
1. The backend doesn't actually use `lighterPrivateKey` - it's deprecated
2. The backend REQUIRES `lighterAPIKeyPrivateKey` (API Key private key)
3. The frontend shows `lighterAPIKeyPrivateKey` as optional (no `required` attribute, line 997)

### Frontend Form Fields

1. **L1 Wallet Address** (`lighterWalletAddr`) - ✅ Required (line 937)
2. **L1 Private Key** (`lighterPrivateKey`) - ✅ Required (line 971) - **WRONG! This is deprecated**
3. **API Key Private Key** (`lighterApiKeyPrivateKey`) - ❌ Optional (line 997) - **WRONG! This should be required**
4. **API Key Index** (`lighterApiKeyIndex`) - ✅ Optional (defaults to 0)

## Critical Bug Found

**Issue:** Frontend validation and backend requirements are mismatched:

- Frontend validates: `lighterWalletAddr` + `lighterPrivateKey` (deprecated field)
- Backend requires: `lighterWalletAddr` + `lighterAPIKeyPrivateKey` (API Key)

**Result:** Users can submit the form with:
- ✅ Wallet address
- ✅ L1 private key (not used by backend)
- ❌ Missing API Key private key (required by backend)

This causes a 500 error when starting the trader because `NewLighterTraderV2` fails with "API Key private key is required for Lighter V2" (line 105-106).

## Required Fields Summary

### For LIGHTER DEX Configuration:

**Required:**
1. **L1 Wallet Address** (`lighterWalletAddr`) - Ethereum wallet address
2. **API Key Private Key** (`lighterAPIKeyPrivateKey`) - 40-byte API Key private key for signing transactions

**Optional:**
3. **API Key Index** (`lighterApiKeyIndex`) - Defaults to 0 if not provided

**Deprecated (Not Used):**
4. **L1 Private Key** (`lighterPrivateKey`) - No longer needed when wallet address is provided

## Backend Code Flow

1. **trader/auto_trader.go (line 324-340):**
   - Checks if `LighterWalletAddr` and `LighterAPIKeyPrivateKey` are set
   - Calls `NewLighterTraderV2(walletAddr, apiKeyPrivateKey, apiKeyIndex)`

2. **trader/lighter_trader_v2.go (line 71-134):**
   - Validates wallet address (line 73-75)
   - Validates API Key private key (line 105-107)
   - Initializes account (gets account index from L1 address)
   - Creates TxClient with API Key
   - Verifies API Key matches server

3. **manager/trader_manager.go (line 1417-1422):**
   - Maps exchange config to trader config:
     - `LighterWalletAddr` → `traderConfig.LighterWalletAddr`
     - `LighterAPIKeyPrivateKey` → `traderConfig.LighterAPIKeyPrivateKey`
     - `LighterAPIKeyIndex` → `traderConfig.LighterAPIKeyIndex`

## Fix Required

### Frontend Changes Needed:

1. **Remove validation for `lighterPrivateKey`** (line 277)
2. **Add validation for `lighterAPIKeyPrivateKey`** (make it required)
3. **Update form submission** to not send `lighterPrivateKey` (or mark it as deprecated)
4. **Update button disable logic** (line 1074-1075) to check `lighterAPIKeyPrivateKey` instead of `lighterPrivateKey`

### Suggested Fix:

```typescript
// Line 277: Change validation
if (!lighterWalletAddr.trim() || !lighterApiKeyPrivateKey.trim()) {
  setIsLoading(false)
  return
}

// Line 1074-1075: Update button disable condition
(selectedExchange.id === 'lighter' &&
  (!lighterWalletAddr.trim() || !lighterApiKeyPrivateKey.trim())) ||
```

## API Key Generation

According to the code comments (line 229-240 in lighter_trader_v2.go):
- API Keys should be generated from LIGHTER official website
- The `GenerateAndRegisterAPIKey` function is not implemented
- Users must generate API Keys through LIGHTER's interface

## References

- Backend Implementation: `trader/lighter_trader_v2.go`
- Frontend Form: `web/src/components/traders/ExchangeConfigModal.tsx`
- Database Schema: `config/database.go`
- Trader Manager: `manager/trader_manager.go`
- Auto Trader Config: `trader/auto_trader.go`

