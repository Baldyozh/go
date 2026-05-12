package auth

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey is used for storing user context
type ContextKey string

const (
	UserIDKey   ContextKey = "user_id"
	UsernameKey ContextKey = "username"
)

// AuthMiddleware is a middleware that validates JWT tokens
func (s *JWTService) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		claims, err := s.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UsernameKey, claims.Username)

		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission is a middleware that checks if user has required permission
func RequirePermission(authRepo AuthPermissionChecker, permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(int)
			if !ok {
				http.Error(w, "User not authenticated", http.StatusUnauthorized)
				return
			}

			hasPermission, err := authRepo.HasPermission(r.Context(), userID, permission)
			if err != nil {
				http.Error(w, "Failed to check permission", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				http.Error(w, "Permission denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AuthPermissionChecker is an interface for checking permissions
type AuthPermissionChecker interface {
	HasPermission(ctx context.Context, userID int, permissionName string) (bool, error)
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(r *http.Request) (int, bool) {
	userID, ok := r.Context().Value(UserIDKey).(int)
	return userID, ok
}

// GetUsernameFromContext extracts username from context
func GetUsernameFromContext(r *http.Request) (string, bool) {
	username, ok := r.Context().Value(UsernameKey).(string)
	return username, ok
}
