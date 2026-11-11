package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type SupabaseClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"` 
	jwt.RegisteredClaims
}

type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func abortWithAuthError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": AuthError{Code: code, Message: message},
	})
}

const (
	ErrCodeMissingToken = "MISSING_TOKEN"
	ErrCodeInvalidToken = "INVALID_TOKEN"
	ErrCodeTokenExpired = "TOKEN_EXPIRED"
	ErrCodeForbidden    = "FORBIDDEN"
)

func unauthorized(c *gin.Context, code, msg string) {
	abortWithAuthError(c, http.StatusUnauthorized, code, msg)
}