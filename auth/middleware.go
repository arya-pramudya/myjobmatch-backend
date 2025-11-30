package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/myjobmatch/backend/models"
)

const (
	// AuthUserKey is the key used to store user info in gin context
	AuthUserKey = "auth_user"
	// AuthClaimsKey is the key used to store JWT claims in gin context
	AuthClaimsKey = "auth_claims"
)

// AuthMiddleware creates a middleware for JWT authentication
func AuthMiddleware(jwtService *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Authorization header required",
				Code:  http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Invalid authorization header format",
				Code:  http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Invalid or expired token",
				Code:    http.StatusUnauthorized,
				Details: err.Error(),
			})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set(AuthClaimsKey, claims)
		c.Next()
	}
}

// OptionalAuthMiddleware creates a middleware that optionally authenticates
// If token is present and valid, user info is added to context
// If token is missing or invalid, request continues without user info
func OptionalAuthMiddleware(jwtService *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		c.Set(AuthClaimsKey, claims)
		c.Next()
	}
}

// GetAuthClaims retrieves auth claims from gin context
func GetAuthClaims(c *gin.Context) *Claims {
	claims, exists := c.Get(AuthClaimsKey)
	if !exists {
		return nil
	}
	return claims.(*Claims)
}

// IsAuthenticated checks if user is authenticated
func IsAuthenticated(c *gin.Context) bool {
	return GetAuthClaims(c) != nil
}
