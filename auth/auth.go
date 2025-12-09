package auth

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret JWT secret, will be dynamically set from configuration
var JWTSecret []byte

// tokenBlacklist Token blacklist for logout (memory only, cleaned by expiration time)
var tokenBlacklist = struct {
	sync.RWMutex
	items map[string]time.Time
}{items: make(map[string]time.Time)}

// maxBlacklistEntries Maximum blacklist capacity threshold
const maxBlacklistEntries = 100_000

// OTPIssuer OTP issuer name
const OTPIssuer = "nofxAI"

// SetJWTSecret Set JWT secret
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// BlacklistToken Add token to blacklist until expiration
func BlacklistToken(token string, exp time.Time) {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	tokenBlacklist.items[token] = exp

	// If exceeds capacity threshold, perform expiration cleanup; if still exceeds limit, log warning
	if len(tokenBlacklist.items) > maxBlacklistEntries {
		now := time.Now()
		for t, e := range tokenBlacklist.items {
			if now.After(e) {
				delete(tokenBlacklist.items, t)
			}
		}
		if len(tokenBlacklist.items) > maxBlacklistEntries {
			log.Printf("auth: token blacklist size (%d) exceeds limit (%d) after sweep; consider reducing JWT TTL or using a shared persistent store",
				len(tokenBlacklist.items), maxBlacklistEntries)
		}
	}
}

// IsTokenBlacklisted Check if token is in blacklist (auto-cleanup on expiration)
func IsTokenBlacklisted(token string) bool {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	if exp, ok := tokenBlacklist.items[token]; ok {
		if time.Now().After(exp) {
			delete(tokenBlacklist.items, token)
			return false
		}
		return true
	}
	return false
}

// Claims JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// HashPassword Hash password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword Verify password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateOTPSecret Generate OTP secret
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      OTPIssuer,
		AccountName: uuid.New().String(),
	})
	if err != nil {
		return "", err
	}

	return key.Secret(), nil
}

// VerifyOTP Verify OTP code
func VerifyOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateJWT Generate JWT token
func GenerateJWT(userID, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24 hour expiration
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateJWT Validate JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("Invalid token")
}

// GetOTPQRCodeURL Get OTP QR code URL
func GetOTPQRCodeURL(secret, email string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", OTPIssuer, email, secret, OTPIssuer)
}
