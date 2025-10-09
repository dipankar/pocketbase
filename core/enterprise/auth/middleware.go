package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserClaimsKey  contextKey = "user_claims"
	AdminTokenKey  contextKey = "admin_token"
)

// RequireUserAuth is middleware that requires cluster user authentication
func RequireUserAuth(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Expecting "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := jwtManager.ValidateUserToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Check if user is verified
			if !claims.Verified {
				http.Error(w, "Email not verified", http.StatusForbidden)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdminAuth is middleware that requires cluster admin authentication
func RequireAdminAuth(validateAdminToken func(string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header or X-Admin-Token header
			token := r.Header.Get("X-Admin-Token")
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && parts[0] == "Bearer" {
						token = parts[1]
					}
				}
			}

			if token == "" {
				http.Error(w, "Admin token required", http.StatusUnauthorized)
				return
			}

			// Validate admin token
			if !validateAdminToken(token) {
				http.Error(w, "Invalid admin token", http.StatusUnauthorized)
				return
			}

			// Add token to context
			ctx := context.WithValue(r.Context(), AdminTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserClaims retrieves user claims from context
func GetUserClaims(ctx context.Context) (*ClusterUserClaims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*ClusterUserClaims)
	return claims, ok
}

// GetAdminToken retrieves admin token from context
func GetAdminToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(AdminTokenKey).(string)
	return token, ok
}
