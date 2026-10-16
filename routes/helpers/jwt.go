package helpers

import (
	"fmt"
	"os"
	"strings"
	"log"
	"net/http"

	"teach_partner_dev/database"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

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
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_TOKEN",
					"message": "Token tidak disertakan!",
				},
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		jwtSecret, err := getJWTAdminSecret()
		if err != nil {
			log.Printf("[AUTH] FATAL: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "SERVER_MISCONFIGURED",
					"message": "Konfigurasi server tidak valid",
				},
			})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Token tidak valid atau kedaluwarsa",
				},
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "superadmin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Akses ditolak: Bukan level administrator",
				},
			})
			c.Abort()
			return
		}

		adminIDFloat, ok := claims["admin_id"].(float64)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "INVALID_TOKEN",
						"message": "Token tidak valid",
					},
				})
				c.Abort()
				return
			}
			adminID := int(adminIDFloat)

			jwtVersionFloat, ok := claims["token_version"].(float64)
			if !ok {
				// Token lama (sebelum implementasi token_version) → paksa re-login
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Sesi Anda telah berakhir. Silakan login ulang.",
					},
				})
				c.Abort()
				return
			}
			jwtVersion := int(jwtVersionFloat)

			var dbVersion int
			err = database.DB.QueryRow(
				`SELECT COALESCE(token_version, 1) FROM super_admins WHERE id = $1`,
				adminID,
			).Scan(&dbVersion)
			if err != nil {
				// Fail-safe: kalau tidak bisa cek, reject
				log.Printf("[AUTH] Gagal cek token_version untuk admin_id=%d: %v", adminID, err)
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "INVALID_TOKEN",
						"message": "Token tidak valid",
					},
				})
				c.Abort()
				return
			}

			if jwtVersion != dbVersion {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "TOKEN_REVOKED",
						"message": "Sesi Anda telah berakhir karena kata sandi telah diubah. Silakan login ulang.",
					},
				})
				c.Abort()
				return
			}

			c.Set("admin_email", claims["email"])
			c.Set("admin_id", adminID)

			c.Next()
	}
}