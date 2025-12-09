package config

import (
	"nofx/crypto"
	"os"
	"testing"
	"time"
)

// TestUpdateExchange_EmptyValuesShouldNotOverwrite test that empty values should not overwrite existing data
// This is the core of the Bug: current implementation will overwrite existing private keys with empty strings
func TestUpdateExchange_EmptyValuesShouldNotOverwrite(t *testing.T) {
	// Prepare test database
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-001"

	// Step 1: Create initial configuration (with private keys)
	initialAPIKey := "initial-api-key-12345"
	initialSecretKey := "initial-secret-key-67890"

	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true, // enabled
		initialAPIKey,
		initialSecretKey,
		false, // testnet
		"0xWalletAddress",
		"",
		"",
		"",
		"", // lighter_wallet_addr
		"", // lighter_private_key
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Step 2: Verify initial data is saved
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("Failed to get configuration: %v", err)
	}
	if len(exchanges) == 0 {
		t.Fatal("Configuration not found")
	}

	// After decryption, should see original values
	if exchanges[0].APIKey != initialAPIKey {
		t.Errorf("Initial APIKey incorrect, expected %s, got %s", initialAPIKey, exchanges[0].APIKey)
	}

	// Step 3: Update with empty values (simulate frontend sending empty values scenario)
	// 🐛 Bug reproduction: This should NOT overwrite existing private keys, but current implementation will overwrite
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false, // Only change enabled status
		"",    // Empty apiKey - should not overwrite
		"",    // Empty secretKey - should not overwrite
		true,  // Change testnet status
		"0xWalletAddress",
		"",
		"",
		"", // Empty aster_private_key - should not overwrite
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Step 4: Verify private keys were not overwritten by empty values
	exchanges, err = db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("Failed to get updated configuration: %v", err)
	}

	// 🎯 Key assertion: Private keys should remain unchanged
	if exchanges[0].APIKey != initialAPIKey {
		t.Errorf("❌ Bug confirmed: APIKey was overwritten by empty value! Expected %s, got %s", initialAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != initialSecretKey {
		t.Errorf("❌ Bug confirmed: SecretKey was overwritten by empty value! Expected %s, got %s", initialSecretKey, exchanges[0].SecretKey)
	}

	// Verify non-sensitive fields updated normally
	if exchanges[0].Enabled {
		t.Error("enabled should be updated to false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet should be updated to true")
	}
}

// TestUpdateExchange_AsterEmptyValuesShouldNotOverwrite test that Aster private key should not be overwritten by empty values
func TestUpdateExchange_AsterEmptyValuesShouldNotOverwrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-002"

	// Step 1: Create Aster configuration
	initialAsterKey := "aster-private-key-xyz123"

	err := db.UpdateExchange(
		userID,
		"aster",
		true,
		"",
		"",
		false,
		"",
		"0xAsterUser",
		"0xAsterSigner",
		initialAsterKey,
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Failed to initialize Aster: %v", err)
	}

	// Step 2: Update with empty values
	err = db.UpdateExchange(
		userID,
		"aster",
		false, // Only change enabled
		"",
		"",
		false,
		"",
		"0xAsterUser",
		"0xAsterSigner",
		"", // Empty aster_private_key
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Step 3: Verify aster_private_key was not overwritten
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("Failed to get configuration: %v", err)
	}

	if exchanges[0].AsterPrivateKey != initialAsterKey {
		t.Errorf("❌ Bug confirmed: AsterPrivateKey was overwritten by empty value! Expected %s, got %s", initialAsterKey, exchanges[0].AsterPrivateKey)
	}
}

// TestUpdateExchange_NonEmptyValuesShouldUpdate test that non-empty values should update normally
func TestUpdateExchange_NonEmptyValuesShouldUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-003"

	// Step 1: Create initial configuration
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"old-api-key",
		"old-secret-key",
		false,
		"0xOldWallet",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Step 2: Update with non-empty values
	newAPIKey := "new-api-key-456"
	newSecretKey := "new-secret-key-789"

	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		newAPIKey,
		newSecretKey,
		false,
		"0xNewWallet",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Step 3: Verify new values are updated
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("Failed to get configuration: %v", err)
	}

	if exchanges[0].APIKey != newAPIKey {
		t.Errorf("APIKey not updated, expected %s, got %s", newAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != newSecretKey {
		t.Errorf("SecretKey not updated, expected %s, got %s", newSecretKey, exchanges[0].SecretKey)
	}
	if exchanges[0].HyperliquidWalletAddr != "0xNewWallet" {
		t.Errorf("WalletAddr not updated")
	}
}

// TestUpdateExchange_PartialUpdateShouldWork test partial field update
func TestUpdateExchange_PartialUpdateShouldWork(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-005"

	// Create initial configuration
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"api-key-123",
		"secret-key-456",
		false,
		"0xWallet1",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Only update enabled and testnet, leave private keys empty
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false,
		"", // Leave empty
		"", // Leave empty
		true,
		"0xWallet2",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Partial update failed: %v", err)
	}

	// Verify
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("Failed to get configuration: %v", err)
	}

	// Private keys should remain unchanged
	if exchanges[0].APIKey != "api-key-123" {
		t.Errorf("APIKey should not change, expected api-key-123, got %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "secret-key-456" {
		t.Errorf("SecretKey should not change, expected secret-key-456, got %s", exchanges[0].SecretKey)
	}

	// Other fields should be updated
	if exchanges[0].Enabled {
		t.Error("enabled should be updated to false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet should be updated to true")
	}
	if exchanges[0].HyperliquidWalletAddr != "0xWallet2" {
		t.Error("wallet address should be updated")
	}
}

// TestUpdateExchange_MultipleExchangeTypes test different exchange types
func TestUpdateExchange_MultipleExchangeTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-006"

	testCases := []struct {
		exchangeID string
		name       string
		typ        string
	}{
		{"binance", "Binance Futures", "cex"},
		{"hyperliquid", "Hyperliquid", "dex"},
		{"aster", "Aster DEX", "dex"},
		{"unknown-exchange", "unknown-exchange Exchange", "cex"},
	}

	for _, tc := range testCases {
		t.Run(tc.exchangeID, func(t *testing.T) {
			err := db.UpdateExchange(
				userID,
				tc.exchangeID,
				true,
				"api-key-"+tc.exchangeID,
				"secret-key-"+tc.exchangeID,
				false,
				"",
				"",
				"",
				"",
				"",
				"",
				"", // lighter_api_key_private_key
				"", // okx_passphrase
			)
			if err != nil {
				t.Fatalf("Failed to create %s: %v", tc.exchangeID, err)
			}

			// Verify creation successful
			exchanges, err := db.GetExchanges(userID)
			if err != nil {
				t.Fatalf("Failed to get configuration: %v", err)
			}

			found := false
			for _, ex := range exchanges {
				if ex.ID == tc.exchangeID {
					found = true
					if ex.Name != tc.name {
						t.Errorf("Exchange name incorrect, expected %s, got %s", tc.name, ex.Name)
					}
					if ex.Type != tc.typ {
						t.Errorf("Exchange type incorrect, expected %s, got %s", tc.typ, ex.Type)
					}
					if ex.APIKey != "api-key-"+tc.exchangeID {
						t.Errorf("APIKey incorrect")
					}
					break
				}
			}

			if !found {
				t.Errorf("Exchange %s not found", tc.exchangeID)
			}
		})
	}
}

// TestUpdateExchange_MixedSensitiveFields test mixed update of sensitive and non-sensitive fields
func TestUpdateExchange_MixedSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-007"

	// Create initial configuration
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"old-api-key",
		"old-secret-key",
		false,
		"0xOldWallet",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Scenario 1: Only update apiKey, leave secretKey empty
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false,
		"new-api-key",
		"", // Leave empty
		true,
		"0xNewWallet",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update 1 failed: %v", err)
	}

	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey should be updated")
	}
	if exchanges[0].SecretKey != "old-secret-key" {
		t.Error("SecretKey should remain unchanged")
	}

	// Scenario 2: Only update secretKey, leave apiKey empty
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"", // Leave empty
		"new-secret-key",
		false,
		"0xFinalWallet",
		"",
		"",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update 2 failed: %v", err)
	}

	exchanges, _ = db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey should remain unchanged")
	}
	if exchanges[0].SecretKey != "new-secret-key" {
		t.Error("SecretKey should be updated")
	}
	if exchanges[0].Enabled != true {
		t.Error("Enabled should be updated to true")
	}
	if exchanges[0].HyperliquidWalletAddr != "0xFinalWallet" {
		t.Error("WalletAddr should be updated")
	}
}

// TestUpdateExchange_OnlyNonSensitiveFields test updating only non-sensitive fields
func TestUpdateExchange_OnlyNonSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-008"

	// Create initial configuration (with all private keys)
	err := db.UpdateExchange(
		userID,
		"aster",
		true,
		"binance-api",
		"binance-secret",
		false,
		"",
		"0xUser1",
		"0xSigner1",
		"aster-private-key-1",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Only update non-sensitive fields (leave all private key fields empty)
	err = db.UpdateExchange(
		userID,
		"aster",
		false,
		"",
		"",
		true,
		"",
		"0xUser2",
		"0xSigner2",
		"",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify all private keys remain unchanged
	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "binance-api" {
		t.Errorf("APIKey should remain unchanged, got %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "binance-secret" {
		t.Errorf("SecretKey should remain unchanged, got %s", exchanges[0].SecretKey)
	}
	if exchanges[0].AsterPrivateKey != "aster-private-key-1" {
		t.Errorf("AsterPrivateKey should remain unchanged, got %s", exchanges[0].AsterPrivateKey)
	}

	// Verify non-sensitive fields are updated
	if exchanges[0].Enabled != false {
		t.Error("Enabled should be updated to false")
	}
	if exchanges[0].Testnet != true {
		t.Error("Testnet should be updated to true")
	}
	if exchanges[0].AsterUser != "0xUser2" {
		t.Error("AsterUser should be updated")
	}
	if exchanges[0].AsterSigner != "0xSigner2" {
		t.Error("AsterSigner should be updated")
	}
}

// TestUpdateExchange_AllSensitiveFieldsUpdate test updating all sensitive fields simultaneously
func TestUpdateExchange_AllSensitiveFieldsUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-009"

	// Create initial configuration
	err := db.UpdateExchange(
		userID,
		"binance",
		true,
		"old-api",
		"old-secret",
		false,
		"",
		"",
		"",
		"old-aster-key",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Update all sensitive fields simultaneously
	err = db.UpdateExchange(
		userID,
		"binance",
		false,
		"new-api",
		"new-secret",
		true,
		"0xWallet",
		"0xUser",
		"0xSigner",
		"new-aster-key",
		"",
		"",
		"", // lighter_api_key_private_key
		"", // okx_passphrase
	)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify all fields are updated
	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api" {
		t.Error("APIKey should be updated")
	}
	if exchanges[0].SecretKey != "new-secret" {
		t.Error("SecretKey should be updated")
	}
	if exchanges[0].AsterPrivateKey != "new-aster-key" {
		t.Error("AsterPrivateKey should be updated")
	}
	if !exchanges[0].Testnet {
		t.Error("Testnet should be updated to true")
	}
}

// setupTestDB create test database
func setupTestDB(t *testing.T) (*Database, func()) {
	// Create temporary database file
	tmpFile := t.TempDir() + "/test.db"

	db, err := NewDatabase(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create test users
	testUsers := []string{
		"test-user-001", "test-user-002", "test-user-003", "test-user-004", "test-user-005",
		"test-user-006", "test-user-007", "test-user-008", "test-user-009",
		"test-user-persistence", "user1", "user2",
	}
	for _, userID := range testUsers {
		user := &User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  false,
		}
		_ = db.CreateUser(user)
	}

	// Set encryption service (for testing encryption functionality)
	// Create temporary RSA key
	rsaKeyPath := t.TempDir() + "/test_rsa_key"
	cryptoService, err := crypto.NewCryptoService(rsaKeyPath)
	if err != nil {
		// If creation fails, continue testing without encryption
		t.Logf("Warning: unable to create encryption service, will test without encryption: %v", err)
	} else {
		db.SetCryptoService(cryptoService)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpFile)
		os.RemoveAll(rsaKeyPath)
	}

	return db, cleanup
}

// TestWALModeEnabled test if WAL mode is enabled
// TDD: This test should fail because current code does not enable WAL mode
func TestWALModeEnabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Query current journal_mode
	var journalMode string
	err := db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}

	// Expected to be WAL mode
	if journalMode != "wal" {
		t.Errorf("Expected journal_mode=wal, got %s", journalMode)
	}
}

// TestSynchronousMode test synchronous mode settings
// TDD: Verify data durability settings
func TestSynchronousMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Query synchronous settings
	var synchronous int
	err := db.db.QueryRow("PRAGMA synchronous").Scan(&synchronous)
	if err != nil {
		t.Fatalf("Failed to query synchronous: %v", err)
	}

	// Expected to be FULL (2) to ensure data durability
	if synchronous != 2 {
		t.Errorf("Expected synchronous=2 (FULL), got %d", synchronous)
	}
}

// TestDataPersistenceAcrossReopen test if data persists after closing and reopening database
// TDD: Simulate Docker restart scenario
func TestDataPersistenceAcrossReopen(t *testing.T) {
	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test_persistence_*.db")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// Set encryption service
	rsaKeyPath := "test_rsa_key.pem"
	cryptoService, err := crypto.NewCryptoService(rsaKeyPath)
	if err != nil {
		t.Fatalf("Failed to initialize encryption service: %v", err)
	}
	defer os.RemoveAll(rsaKeyPath)

	userID := "test-user-persistence"
	testAPIKey := "test-api-key-should-persist"
	testSecretKey := "test-secret-key-should-persist"

	// First time: open database and write data
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database first time: %v", err)
		}
		db.SetCryptoService(cryptoService)

		// Create persistent test user to avoid foreign key constraint failure
		_ = db.CreateUser(&User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  true,
		})

		// Write exchange configuration
		err = db.UpdateExchange(
			userID,
			"binance",
			true,
			testAPIKey,
			testSecretKey,
			false,
			"",
			"",
			"",
			"",
			"",
			"",
			"", // lighter_api_key_private_key
			"", // okx_passphrase
		)
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}

		// Simulate normal shutdown
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}

	// Second time: open database and verify data is still there
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database second time: %v", err)
		}
		db.SetCryptoService(cryptoService)
		defer db.Close()

		// Read data
		exchanges, err := db.GetExchanges(userID)
		if err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}

		if len(exchanges) == 0 {
			t.Fatal("Data lost: no exchange configurations found")
		}

		// Verify data integrity
		found := false
		for _, ex := range exchanges {
			if ex.ID == "binance" {
				found = true
				if ex.APIKey != testAPIKey {
					t.Errorf("API Key lost or corrupted, expected %s, got %s", testAPIKey, ex.APIKey)
				}
				if ex.SecretKey != testSecretKey {
					t.Errorf("Secret Key lost or corrupted, expected %s, got %s", testSecretKey, ex.SecretKey)
				}
			}
		}

		if !found {
			t.Error("Data lost: binance configuration not found")
		}
	}
}

// TestConcurrentWritesWithWAL test concurrent writes with WAL mode
// TDD: WAL mode should support better concurrent performance
func TestConcurrentWritesWithWAL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// This test verifies multiple concurrent writes can succeed
	// WAL mode has better concurrent performance, but SQLite may still have brief locks
	done := make(chan bool, 2)
	errors := make(chan error, 10)

	// Concurrent write 1
	go func() {
		for i := 0; i < 3; i++ {
			err := db.UpdateExchange(
				"user1",
				"binance",
				true,
				"key1",
				"secret1",
				false,
				"",
				"",
				"",
				"",
				"",
				"",
				"", // lighter_api_key_private_key
				"", // okx_passphrase
			)
			if err != nil {
				errors <- err
			}
			// Small delay to reduce lock conflicts
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Concurrent write 2
	go func() {
		for i := 0; i < 3; i++ {
			err := db.UpdateExchange(
				"user2",
				"hyperliquid",
				true,
				"key2",
				"secret2",
				false,
				"0xWallet",
				"",
				"",
				"",
				"",
				"",
				"", // lighter_api_key_private_key
				"", // okx_passphrase
			)
			if err != nil {
				errors <- err
			}
			// Small delay to reduce lock conflicts
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent write error: %v", err)
		errorCount++
	}

	// WAL mode should handle concurrency, but may have a few lock errors
	// We allow up to 2 errors
	if errorCount > 2 {
		t.Errorf("Concurrent write failures too many: %d", errorCount)
	}
}
