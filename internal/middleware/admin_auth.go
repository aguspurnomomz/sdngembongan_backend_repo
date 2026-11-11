package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gembongan_dev_backend/database"
)


type AdminClaims struct {
	AdminID      int    `json:"admin_id"`
	Role         string `json:"role"`
	Email        string `json:"email"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

func getJWTAdminSecret() ([]byte, error) {
	secret := os.Getenv("JWT_ADMIN_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_ADMIN_SECRET tidak dikonfigurasi di environment")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_ADMIN_SECRET terlalu pendek (minimal 32 karakter)")
	}
	return []byte(secret), nil
}


func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c, ErrCodeMissingToken, "Token tidak disertakan!")
			return
		}

		jwtSecret, err := getJWTAdminSecret()
		if err != nil {
			log.Printf("[AUTH] FATAL: %v", err)
			abortWithAuthError(c, http.StatusInternalServerError, "SERVER_MISCONFIGURED", "Konfigurasi server tidak valid")
			return
		}

		claims := &AdminClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			unauthorized(c, ErrCodeInvalidToken, "Token tidak valid atau kedaluwarsa")
			return
		}

		if claims.Role != "superadmin" {
			abortWithAuthError(c, http.StatusForbidden, ErrCodeForbidden, "Akses ditolak: Bukan level administrator")
			return
		}

		if claims.TokenVersion == 0 {
			unauthorized(c, ErrCodeTokenExpired, "Sesi Anda telah berakhir. Silakan login ulang.")
			return
		}

		if err := checkAdminTokenVersion(c.Request.Context(), claims.AdminID, claims.TokenVersion); err != nil {
			if errors.Is(err, errTokenRevoked) {
				unauthorized(c, "TOKEN_REVOKED", "Sesi Anda telah berakhir karena kata sandi telah diubah. Silakan login ulang.")
			} else {
				log.Printf("[AUTH] Gagal cek token_version admin_id=%d: %v", claims.AdminID, err)
				unauthorized(c, ErrCodeInvalidToken, "Token tidak valid")
			}
			return
		}

		c.Set("admin_email", claims.Email)
		c.Set("admin_id", claims.AdminID)
		c.Next()
	}
}

var errTokenRevoked = errors.New("token version mismatch")

func checkAdminTokenVersion(ctx context.Context, adminID int, jwtVersion int) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var dbVersion int
	err := database.DB.QueryRowContext(ctx,
		`SELECT COALESCE(token_version, 1) FROM super_admins WHERE id = $1`,
		adminID,
	).Scan(&dbVersion)

	if err != nil {
		return err
	}
	if jwtVersion != dbVersion {
		return errTokenRevoked
	}
	return nil
}