package api

import (
	"log"
	"net/http"
	"nofx/crypto"

	"github.com/gin-gonic/gin"
)

// CryptoHandler 加密 API 處理器
type CryptoHandler struct {
	cryptoService *crypto.CryptoService
}

// NewCryptoHandler 創建加密處理器
func NewCryptoHandler(cryptoService *crypto.CryptoService) *CryptoHandler {
	return &CryptoHandler{
		cryptoService: cryptoService,
	}
}

// ==================== 公鑰端點 ====================

// HandleGetPublicKey 獲取伺服器公鑰
func (h *CryptoHandler) HandleGetPublicKey(c *gin.Context) {
	publicKey := h.cryptoService.GetPublicKeyPEM()

	c.JSON(http.StatusOK, map[string]string{
		"public_key": publicKey,
		"algorithm":  "RSA-OAEP-2048",
	})
}

// ==================== 加密數據解密端點 ====================

// HandleDecryptSensitiveData 解密客戶端傳送的加密数据
func (h *CryptoHandler) HandleDecryptSensitiveData(c *gin.Context) {
	var payload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 解密
	decrypted, err := h.cryptoService.DecryptSensitiveData(&payload)
	if err != nil {
		log.Printf("❌ 解密失敗: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"plaintext": decrypted,
	})
}

// ==================== 審計日誌查詢端點 ====================

// 删除审计日志相关功能，在当前简化的实现中不需要
