package api

import (
	"log"
	"net/http"
	"nofx/crypto"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CryptoHandler Encryption API handler
type CryptoHandler struct {
	cryptoService *crypto.CryptoService
}

// NewCryptoHandler Creates encryption handler
func NewCryptoHandler(cryptoService *crypto.CryptoService) *CryptoHandler {
	return &CryptoHandler{
		cryptoService: cryptoService,
	}
}

// ==================== Public Key Endpoint ====================

// HandleGetPublicKey Get server public key
func (h *CryptoHandler) HandleGetPublicKey(c *gin.Context) {
	publicKey := h.cryptoService.GetPublicKeyPEM()

	c.JSON(http.StatusOK, map[string]string{
		"public_key": publicKey,
		"algorithm":  "RSA-OAEP-2048",
	})
}

// ==================== Encrypted Data Decryption Endpoint ====================

// HandleDecryptSensitiveData Decrypt encrypted data sent from client
func (h *CryptoHandler) HandleDecryptSensitiveData(c *gin.Context) {
	var payload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Decrypt
	decrypted, err := h.cryptoService.DecryptSensitiveData(&payload)
	if err != nil {
		log.Printf("❌ Decryption failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"plaintext": decrypted,
	})
}

// ==================== Crypto Configuration Endpoint ====================

// HandleGetCryptoConfig Get encryption configuration status
func (h *CryptoHandler) HandleGetCryptoConfig(c *gin.Context) {
	transportEncryptionEnabled := strings.ToLower(os.Getenv("TRANSPORT_ENCRYPTION")) == "true"
	
	// Check if public key is available
	publicKeyAvailable := h.cryptoService.GetPublicKeyPEM() != ""
	
	c.JSON(http.StatusOK, map[string]interface{}{
		"transport_encryption_enabled": transportEncryptionEnabled,
		"public_key_available":         publicKeyAvailable,
	})
}

// ==================== Audit Log Query Endpoint ====================

// Audit log functionality removed, not needed in current simplified implementation
