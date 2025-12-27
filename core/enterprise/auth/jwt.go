package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey string
}

// NewJWTManager creates a new JWT manager with a secret key
// If secretKey is empty, a cryptographically secure random key is generated
// WARNING: Random keys are not persistent across restarts - set POCKETBASE_JWT_SECRET for production
func NewJWTManager(secretKey string) (*JWTManager, error) {
	if secretKey == "" {
		// Generate a secure random key (32 bytes = 256 bits)
		randomKey := make([]byte, 32)
		_, err := rand.Read(randomKey)
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		secretKey = base64.StdEncoding.EncodeToString(randomKey)
		log.Printf("[JWT] WARNING: No JWT secret provided, generated random key")
		log.Printf("[JWT] For production, set POCKETBASE_JWT_SECRET environment variable or jwtSecret in config")
		log.Printf("[JWT] Random keys are NOT persistent across restarts - user sessions will be invalidated!")
	}

	return &JWTManager{
		secretKey: secretKey,
	}, nil
}

// ClusterUserClaims represents JWT claims for cluster users
type ClusterUserClaims struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
	jwt.RegisteredClaims
}

// TenantAdminClaims represents JWT claims for SSO to tenant admin
type TenantAdminClaims struct {
	UserID   string `json:"userId"`
	TenantID string `json:"tenantId"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateUserToken generates a JWT token for a cluster user
func (j *JWTManager) GenerateUserToken(user *enterprise.ClusterUser, expirationHours int) (string, error) {
	if expirationHours <= 0 {
		expirationHours = 24 // Default 24 hours
	}

	claims := ClusterUserClaims{
		UserID:   user.ID,
		Email:    user.Email,
		Name:     user.Name,
		Verified: user.Verified,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pocketbase-enterprise",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

// ValidateUserToken validates a cluster user JWT token
func (j *JWTManager) ValidateUserToken(tokenString string) (*ClusterUserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ClusterUserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*ClusterUserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GenerateTenantAdminToken generates a SSO token for accessing tenant admin
func (j *JWTManager) GenerateTenantAdminToken(user *enterprise.ClusterUser, tenantID string) (string, error) {
	claims := TenantAdminClaims{
		UserID:   user.ID,
		TenantID: tenantID,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)), // 1 hour SSO token
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pocketbase-enterprise-sso",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

// ValidateTenantAdminToken validates a tenant admin SSO token
func (j *JWTManager) ValidateTenantAdminToken(tokenString string) (*TenantAdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TenantAdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TenantAdminClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GenerateAdminToken generates a long-lived admin token
func (j *JWTManager) GenerateAdminToken(name string) string {
	// Admin tokens are simple random strings, not JWTs
	// They're stored in the control plane and validated there
	return enterprise.GenerateAdminToken()
}
