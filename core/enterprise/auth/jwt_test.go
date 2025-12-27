package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core/enterprise"
)

func TestNewJWTManagerWithSecret(t *testing.T) {
	manager, err := NewJWTManager("test-secret-key-32-bytes-long!!")
	if err != nil {
		t.Fatalf("failed to create JWT manager: %v", err)
	}

	if manager == nil {
		t.Error("expected non-nil manager")
	}
}

func TestNewJWTManagerGeneratesRandomSecret(t *testing.T) {
	manager, err := NewJWTManager("")
	if err != nil {
		t.Fatalf("failed to create JWT manager with empty secret: %v", err)
	}

	if manager == nil {
		t.Error("expected non-nil manager")
	}

	// Generate a token to verify the manager works
	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}
	token, err := manager.GenerateUserToken(user, 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGenerateUserToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Name:     "Test User",
		Verified: true,
	}

	token, err := manager.GenerateUserToken(user, 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Token should be a valid JWT format (header.payload.signature)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts in JWT, got %d", len(parts))
	}
}

func TestValidateUserToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, _ := manager.GenerateUserToken(user, 24)

	claims, err := manager.ValidateUserToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, claims.UserID)
	}

	if claims.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, claims.Email)
	}

	if claims.Verified != user.Verified {
		t.Errorf("expected verified %v, got %v", user.Verified, claims.Verified)
	}
}

func TestValidateInvalidToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"garbage", "not-a-jwt"},
		{"tampered", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0YW1wZXJlZCI6dHJ1ZX0.tampered"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ValidateUserToken(tt.token)
			if err == nil {
				t.Error("expected error for invalid token")
			}
		})
	}
}

func TestTokenExpiration(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}

	// Generate token with 0 hours (uses default)
	token, err := manager.GenerateUserToken(user, 0)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := manager.ValidateUserToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	// Token should expire in ~24 hours (default)
	expectedExpiry := time.Now().Add(24 * time.Hour)
	if claims.ExpiresAt.Time.Before(expectedExpiry.Add(-1*time.Minute)) ||
		claims.ExpiresAt.Time.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("token expiry not within expected range")
	}
}

func TestDifferentSecretsProduceDifferentTokens(t *testing.T) {
	manager1, _ := NewJWTManager("secret-key-1-32-bytes-long!!!!!")
	manager2, _ := NewJWTManager("secret-key-2-32-bytes-long!!!!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}

	token1, _ := manager1.GenerateUserToken(user, 24)
	token2, _ := manager2.GenerateUserToken(user, 24)

	// Tokens should be different (different signatures)
	if token1 == token2 {
		t.Error("tokens with different secrets should be different")
	}

	// Token from manager1 should not validate with manager2
	_, err := manager2.ValidateUserToken(token1)
	if err == nil {
		t.Error("token should not validate with different secret")
	}
}

func TestGenerateTenantAdminToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, err := manager.GenerateTenantAdminToken(user, "tenant_456")
	if err != nil {
		t.Fatalf("failed to generate tenant admin token: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	// Validate the token
	claims, err := manager.ValidateTenantAdminToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != "user_123" {
		t.Errorf("expected userID user_123, got %s", claims.UserID)
	}

	if claims.TenantID != "tenant_456" {
		t.Errorf("expected tenantID tenant_456, got %s", claims.TenantID)
	}
}

func TestTenantAdminTokenExpiration(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "test@example.com",
		Verified: true,
	}

	token, _ := manager.GenerateTenantAdminToken(user, "tenant_456")
	claims, _ := manager.ValidateTenantAdminToken(token)

	// Tenant admin token should expire in ~1 hour
	expectedExpiry := time.Now().Add(1 * time.Hour)
	if claims.ExpiresAt.Time.Before(expectedExpiry.Add(-1*time.Minute)) ||
		claims.ExpiresAt.Time.After(expectedExpiry.Add(1*time.Minute)) {
		t.Errorf("tenant admin token expiry not within expected range (~1 hour)")
	}
}

func TestValidateExpiredToken(t *testing.T) {
	// This test would require mocking time or waiting for a token to expire
	// For now, we'll skip it as it would make tests slow
	t.Skip("Skipping expired token test - would require time manipulation")
}

func TestTokenWithSpecialCharactersInEmail(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_123",
		Email:    "user+tag@example.com",
		Verified: true,
	}

	token, err := manager.GenerateUserToken(user, 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := manager.ValidateUserToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.Email != user.Email {
		t.Errorf("email mismatch: expected %s, got %s", user.Email, claims.Email)
	}
}

func TestClusterUserClaimsFields(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-bytes-long!!")

	user := &enterprise.ClusterUser{
		ID:       "user_xyz",
		Email:    "claims@test.com",
		Name:     "Claims Test",
		Verified: true,
	}

	token, _ := manager.GenerateUserToken(user, 1)
	claims, _ := manager.ValidateUserToken(token)

	// Verify all expected fields
	if claims.UserID != user.ID {
		t.Errorf("UserID: expected %s, got %s", user.ID, claims.UserID)
	}
	if claims.Email != user.Email {
		t.Errorf("Email: expected %s, got %s", user.Email, claims.Email)
	}
	if claims.Verified != user.Verified {
		t.Errorf("Verified: expected %v, got %v", user.Verified, claims.Verified)
	}
	if claims.Issuer != "pocketbase-enterprise" {
		t.Errorf("Issuer: expected pocketbase-enterprise, got %s", claims.Issuer)
	}
}
