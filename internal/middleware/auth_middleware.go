package middleware

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	k := mustJWKS()
	issuer := os.Getenv("SUPABASE_JWT_ISSUER")

	return func(c *gin.Context) {
		tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c, ErrCodeMissingToken, "Token tidak ditemukan atau format salah")
			return
		}

		claims := &SupabaseClaims{}
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			k.Keyfunc,
			jwt.WithIssuer(issuer),
			jwt.WithAudience("authenticated"),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			handleJWTError(c, err)
			return
		}

		if claims.Sub == "" {
			unauthorized(c, ErrCodeInvalidToken, "User ID tidak ditemukan dalam token")
			return
		}

		c.Set("user_id", claims.Sub)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

// extractBearerToken mem-parse header "Authorization: Bearer <token>".
// Case-insensitive pada prefix "Bearer".
func extractBearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// handleJWTError memetakan error JWT ke response yang sesuai.
func handleJWTError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		unauthorized(c, ErrCodeTokenExpired, "Token telah kedaluwarsa")
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		unauthorized(c, ErrCodeInvalidToken, "Token belum berlaku")
	case errors.Is(err, jwt.ErrTokenMalformed),
		errors.Is(err, jwt.ErrTokenSignatureInvalid),
		errors.Is(err, jwt.ErrTokenUnverifiable):
		unauthorized(c, ErrCodeInvalidToken, "Token tidak valid")
	default:
		// Log detail, tapi jangan bocor ke client
		if err != nil {
			log.Printf("[AUTH] JWT parse error: %v", err)
		}
		unauthorized(c, ErrCodeInvalidToken, "Token tidak valid atau kedaluwarsa")
	}
}

// pastikan net/http terpakai (untuk kompilasi); kalau tidak, hapus import.
var _ = http.StatusUnauthorized