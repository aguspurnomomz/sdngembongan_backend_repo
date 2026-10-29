// routes/superadmin/auth.go
package superadmin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gembongan_dev_backend/database" 

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)


// getJWTAdminSecret 
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

// dummyBcryptHash dipakai untuk constant time comparison saat email tidak ditemukan
var dummyBcryptHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// writeInternalError — helper untuk log detail internal, tapi response ke user generic
func writeInternalError(c *gin.Context, publicMsg string, internalErr error) {
	if internalErr != nil {
		log.Printf("[INTERNAL ERROR] %s | detail: %v", publicMsg, internalErr)
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": publicMsg,
		},
	})
}

// nowJakarta  return current time in WIB timezone
func nowJakarta() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return time.Now().In(loc)
}

// logSuperAdminActivity catat aktivitas admin ke DB
func logSuperAdminActivity(adminID int, action, ipAddress, userAgent, location, details string) {
	query := `INSERT INTO super_admin_logs (admin_id, action, ip_address, user_agent, location, details) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := database.DB.Exec(query, adminID, action, ipAddress, userAgent, location, details)
	if err != nil {
		fmt.Printf("Gagal mencatat log superadmin: %v\n", err)
	}
}

// sendWhatsAppNotification kirim notifikasi via WA Gateway (async)
func sendWhatsAppNotification(targetPhone string, message string) {
	apiToken := os.Getenv("WA_GATEWAY_TOKEN")
	apiURL := os.Getenv("WA_GATEWAY_URL")

	if apiToken == "" || apiURL == "" || targetPhone == "" {
		return
	}

	payload := map[string]string{
		"target":  targetPhone,
		"message": message,
	}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	_, _ = client.Do(req)
}

// Login — POST /api/superadmin/superadmin-login
func Login(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"message": "Format input tidak valid",
				"details": err.Error(),
			},
		})
		return
	}

	const genericCredentialError = "Email atau kata sandi salah"

	var adminID int
	var hashedPassword string
	var adminName string
	var failedAttempts int
	var lockedUntil sql.NullTime
	var waNumber sql.NullString
	var tokenVersion int

	query := `SELECT id, password_hash, nama, failed_login_attempts, locked_until, no_whatsapp, token_version FROM super_admins WHERE email = $1`
	err := database.DB.QueryRow(query, req.Email).Scan(
		&adminID, &hashedPassword, &adminName, &failedAttempts, &lockedUntil, &waNumber, &tokenVersion,
	)

	// Siapkan lokasi (dipakai di log & WA)
	lokasiStr := "Tidak diizinkan / Tidak tersedia"
	mapsLink := "Tidak tersedia"
	if req.Location != nil {
		lokasiStr = fmt.Sprintf("%.6f, %.6f", req.Location.Latitude, req.Location.Longitude)
		mapsLink = fmt.Sprintf("%.6f, %.6f (https://maps.google.com/?q=%.6f,%.6f)",
			req.Location.Latitude, req.Location.Longitude,
			req.Location.Latitude, req.Location.Longitude)
	}

	// Email tidak ditemukan
	if err == sql.ErrNoRows {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "INVALID_CREDENTIALS",
				"message": genericCredentialError,
			},
		})
		return
	}

	if err != nil {
		writeInternalError(c, "Terjadi kesalahan pada server", err)
		return
	}

	// Cek lock dengan reset jika sudah expired
	if lockedUntil.Valid {
		var isLocked bool
		var sisaDetik int

		err = database.DB.QueryRow(`
			SELECT 
				COALESCE(locked_until > NOW(), false) AS is_locked,
				COALESCE(GREATEST(0, EXTRACT(EPOCH FROM (locked_until - NOW()))::int), 0) AS sisa_detik
			FROM super_admins 
			WHERE id = $1
		`, adminID).Scan(&isLocked, &sisaDetik)

		if err != nil {
			writeInternalError(c, "Terjadi kesalahan pada server", err)
			return
		}

		if isLocked {
			c.Header("Retry-After", strconv.Itoa(sisaDetik))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":                "ACCOUNT_LOCKED",
					"message":             fmt.Sprintf("Akun dikunci sementara. Coba lagi dalam %d detik.", sisaDetik),
					"retry_after_seconds": sisaDetik,
				},
			})
			return
		}

		// Reset counter biar user punya 3 kesempatan baru
		if _, resetErr := database.DB.Exec(
			`UPDATE super_admins SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`,
			adminID,
		); resetErr != nil {
			log.Printf("[LOGIN] Gagal reset lock counter untuk admin_id=%d: %v", adminID, resetErr)
		}
		failedAttempts = 0
	}

	// Verifikasi password
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		failedAttempts++

		if failedAttempts >= 3 {
			_, dbErr := database.DB.Exec(
				`UPDATE super_admins SET failed_login_attempts = $1, locked_until = NOW() + INTERVAL '1 minute' WHERE id = $2`,
				failedAttempts, adminID,
			)
			if dbErr != nil {
				log.Printf("[LOGIN] Gagal update lock untuk admin_id=%d: %v", adminID, dbErr)
			}

			// Kirim notifikasi WA (async, non-blocking)
			if waNumber.Valid && waNumber.String != "" {
				ipClient := c.ClientIP()
				userAgent := c.Request.UserAgent()
				pesanAlert := fmt.Sprintf(
					"PERINGATAN KEAMANAN\n\nTerdeteksi 3 kali percobaan login yang gagal pada akun Superadmin Anda.\nAkun telah dikunci sementara selama 1 menit.\n\nDetail Akses:\n- Lokasi: %s\n- Alamat IP: %s\n- Perangkat: %s",
					mapsLink, ipClient, userAgent,
				)
				go sendWhatsAppNotification(waNumber.String, pesanAlert)
			}

			logSuperAdminActivity(adminID, "LOGIN_LOCKED", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Akun dikunci 1 menit karena 3x salah password")

			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":                "ACCOUNT_LOCKED",
					"message":             "Terlalu banyak percobaan gagal. Akun dikunci selama 60 detik.",
					"retry_after_seconds": 60,
				},
			})
			return
		}

		_, dbErr := database.DB.Exec(
			`UPDATE super_admins SET failed_login_attempts = $1 WHERE id = $2`,
			failedAttempts, adminID,
		)
		if dbErr != nil {
			log.Printf("[LOGIN] Gagal update failed_attempts untuk admin_id=%d: %v", adminID, dbErr)
		}

		logSuperAdminActivity(adminID, "LOGIN_FAILED", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Password salah")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "INVALID_CREDENTIALS",
				"message": genericCredentialError,
			},
		})
		return
	}

	// Reset counter setelah login berhasil
	_, _ = database.DB.Exec(
		`UPDATE super_admins SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`,
		adminID,
	)

	logSuperAdminActivity(adminID, "LOGIN_SUCCESS", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Login berhasil")

	// Notif WA login berhasil
	if waNumber.Valid && waNumber.String != "" {
		ipClient := c.ClientIP()
		userAgent := c.Request.UserAgent()
		pesanWA := fmt.Sprintf(
			"INFORMASI LOGIN SUPERADMIN\n\nAkun Superadmin Anda baru saja masuk pada %s.\n\nDetail Akses:\n- Lokasi (Koordinat): %s\n- Alamat IP: %s\n- Perangkat: %s\n\nJika ini bukan Anda, segera amankan akun Anda! \n\nCopyright © 2025 - 2026 SkoolaGo. All Rights Reserved.",
			time.Now().Format("02-01-2006 15:04:05"),
			mapsLink, ipClient, userAgent,
		)
		go sendWhatsAppNotification(waNumber.String, pesanWA)
	}

	// Generate JWT
	jwtSecret, err := getJWTAdminSecret()
	if err != nil {
		log.Printf("[LOGIN] FATAL: %v", err)
		writeInternalError(c, "Terjadi kesalahan pada server", err)
		return
	}

	const tokenTTL = 24 * time.Hour
	claims := jwt.MapClaims{
		"admin_id":      adminID,
		"email":         req.Email,
		"role":          "superadmin",
		"token_version": tokenVersion,
		"exp":           time.Now().Add(tokenTTL).Unix(),
		"iat":           time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		writeInternalError(c, "Gagal menghasilkan token admin", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token":      tokenString,
			"token_type": "Bearer",
			"expires_in": int(tokenTTL.Seconds()),
			"admin": gin.H{
				"id":    adminID,
				"name":  adminName,
				"email": req.Email,
				"role":  "superadmin",
			},
		},
	})
}

// POST /api/superadmin/change-password
func ChangePassword(c *gin.Context) {
	var adminID int

	if adminIDRaw, exists := c.Get("admin_id"); exists {
		switch v := adminIDRaw.(type) {
		case int:
			adminID = v
		case int64:
			adminID = int(v)
		case float64:
			adminID = int(v)
		default:
			writeInternalError(c, "Format admin_id tidak dikenali",
				fmt.Errorf("unexpected admin_id type: %T", adminIDRaw))
			return
		}
	} else {
		emailRaw, ok := c.Get("admin_email")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_SESSION",
					"message": "Sesi admin tidak dikenali",
				},
			})
			return
		}
		email, ok := emailRaw.(string)
		if !ok || email == "" {
			writeInternalError(c, "Format admin_email tidak dikenali", nil)
			return
		}
		if err := database.DB.QueryRow(
			`SELECT id FROM super_admins WHERE email = $1`, email,
		).Scan(&adminID); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_SESSION",
					"message": "Sesi admin tidak dikenali",
				},
			})
			return
		}
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"message": "Format input tidak valid (minimal 12 karakter untuk superadmin)",
				"details": err.Error(),
			},
		})
		return
	}

	if len(req.NewPassword) < 12 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "PASSWORD_TOO_SHORT",
				"message": "Kata sandi baru minimal 12 karakter untuk superadmin",
			},
		})
		return
	}

	if req.NewPassword == req.OldPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "PASSWORD_UNCHANGED",
				"message": "Kata sandi baru harus berbeda dari kata sandi lama",
			},
		})
		return
	}

	var currentHash string
	var waNumber sql.NullString

	if err := database.DB.QueryRow(
		`SELECT password_hash, no_whatsapp FROM super_admins WHERE id = $1`,
		adminID,
	).Scan(&currentHash, &waNumber); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "ADMIN_NOT_FOUND",
					"message": "Data admin tidak ditemukan",
				},
			})
			return
		}
		writeInternalError(c, "Gagal mengambil data admin", err)
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword))
	if err != nil {
		logSuperAdminActivity(adminID, "CHANGE_PASSWORD_FAILED",
			c.ClientIP(), c.Request.UserAgent(), "", "Kata sandi lama salah")

		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_OLD_PASSWORD",
				"message": "Kata sandi lama salah",
			},
		})
		return
	}

	newHashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeInternalError(c, "Gagal memproses kata sandi baru", err)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		writeInternalError(c, "Gagal mulai transaksi", err)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE super_admins SET
			password_hash = $1,
			failed_login_attempts = 0,    
			locked_until = NULL, 
			token_version = token_version + 1,        
			updated_at = NOW()
		WHERE id = $2
	`, string(newHashBytes), adminID)
	if err != nil {
		writeInternalError(c, "Gagal menyimpan kata sandi baru", err)
		return
	}

	if err := tx.Commit(); err != nil {
		writeInternalError(c, "Gagal commit perubahan", err)
		return
	}

	lokasiStr := "Tidak diizinkan / Tidak tersedia"
	mapsLink := "Tidak tersedia"
	if req.Location != nil {
		lokasiStr = fmt.Sprintf("%.6f, %.6f", req.Location.Latitude, req.Location.Longitude)
		mapsLink = fmt.Sprintf("%.6f, %.6f (https://maps.google.com/?q=%.6f,%.6f)",
			req.Location.Latitude, req.Location.Longitude,
			req.Location.Latitude, req.Location.Longitude)
	}

	logSuperAdminActivity(adminID, "CHANGE_PASSWORD",
		c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Berhasil mengubah kata sandi")

	if waNumber.Valid && waNumber.String != "" {
		pesanWA := fmt.Sprintf(
			"PERINGATAN KEAMANAN\n\nKata sandi akun Superadmin Anda baru saja diubah pada %s WIB.\n\nDetail Akses:\n- Lokasi: %s\n- Alamat IP: %s\n- Perangkat: %s\n\nJika Anda tidak merasa melakukan ini, segera amankan akun Anda!",
			nowJakarta().Format("02-01-2006 15:04:05"),
			mapsLink,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		go sendWhatsAppNotification(waNumber.String, pesanWA)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Kata sandi berhasil diperbarui. Silakan login ulang.",
		"session_ended": true,
	})
}