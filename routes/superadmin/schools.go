package superadmin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// POST /api/superadmin/schools
func CreateSchool(c *gin.Context) {
	var req CreateSchoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data sekolah tidak valid: " + err.Error()})
		return
	}

	var existingID string
	err := database.DB.QueryRow(`SELECT id FROM schools WHERE npsn = $1`, req.Npsn).Scan(&existingID)
	if err == nil && existingID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Sekolah dengan NPSN tersebut sudah terdaftar di sistem"})
		return
	}

	var schoolID string
	err = database.DB.QueryRow(
		`INSERT INTO schools (school_name, npsn, address, jenjang, is_active) VALUES ($1, $2, $3, $4, TRUE) RETURNING id`,
		req.SchoolName, req.Npsn, req.Address, req.Jenjang,
	).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data sekolah ke database: " + err.Error()})
		return
	}

	// Log aktivitas
	if adminID, ok := GetAdminIDFromContext(c); ok {
		LogSuperAdminActivity(adminID, "CREATE_SCHOOL", c.ClientIP(),
			c.Request.UserAgent(), "",
			fmt.Sprintf("Mendaftarkan sekolah baru: %s (NPSN: %s)", req.SchoolName, req.Npsn))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Sekolah berhasil didaftarkan",
		"school_id":   schoolID,
		"school_name": req.SchoolName,
		"npsn":        req.Npsn,
	})
}

// GET /api/superadmin/schools
func ListSchools(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, school_name, npsn, address, is_active, created_at 
		FROM schools 
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar sekolah: " + err.Error()})
		return
	}
	defer rows.Close()

	type SchoolItem struct {
		ID         string    `json:"id"`
		SchoolName string    `json:"school_name"`
		Npsn       string    `json:"npsn"`
		Address    string    `json:"address"`
		IsActive   bool      `json:"is_active"`
		CreatedAt  time.Time `json:"created_at"`
	}

	schools := []SchoolItem{}
	for rows.Next() {
		var s SchoolItem
		var address sql.NullString
		if err := rows.Scan(&s.ID, &s.SchoolName, &s.Npsn, &address, &s.IsActive, &s.CreatedAt); err == nil {
			s.Address = address.String
			schools = append(schools, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_schools": len(schools),
		"schools":       schools,
	})
}

// POST /api/superadmin/school-admins
func CreateSchoolAdmin(c *gin.Context) {
	var req CreateSchoolAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid: " + err.Error()})
		return
	}

	var schoolExists string
	err := database.DB.QueryRow(`SELECT id FROM schools WHERE id = $1`, req.SchoolID).Scan(&schoolExists)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Institusi sekolah tidak ditemukan"})
		return
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseURL == "" || serviceRoleKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Konfigurasi Supabase URL atau Service Role Key belum lengkap di server"})
		return
	}

	supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)
	payloadAuth := map[string]any{
		"email":         req.Email,
		"password":      req.Password,
		"email_confirm": true,
		"user_metadata": map[string]any{
			"full_name": req.FullName,
			"role":      "school_admin",
		},
	}
	jsonAuthBody, _ := json.Marshal(payloadAuth)

	httpReq, err := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan request ke Supabase Auth"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", serviceRoleKey)
	httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke layanan Supabase Auth"})
		return
	}
	defer resp.Body.Close()

	var authResp map[string]any
	json.NewDecoder(resp.Body).Decode(&authResp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errMsg := "Gagal membuat akun di Supabase Auth"
		if msg, ok := authResp["msg"].(string); ok {
			errMsg = msg
		} else if message, ok := authResp["message"].(string); ok {
			errMsg = message
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	userID, ok := authResp["id"].(string)
	if !ok || userID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase Auth"})
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO school_admins (id, school_id, full_name, email, nip, user_type, is_active, updated_at) 
		VALUES ($1, $2, $3, $4, $5, 'school_admin', TRUE, NOW())
	`, userID, req.SchoolID, req.FullName, req.Email, req.Nip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data admin sekolah ke database: " + err.Error()})
		return
	}

	_, profErr := database.DB.Exec(`
		INSERT INTO profiles (
			id, email_sekolah, token_balance, is_active,
			role, teacher_type, school_id, registration_source, updated_at
		)
		VALUES ($1, $2, 0, TRUE, 'school_admin', 'b2b', $3, 'created_by_superadmin', NOW())
		ON CONFLICT (id) DO UPDATE SET
			email_sekolah = EXCLUDED.email_sekolah,
			role = 'school_admin',
			teacher_type = 'b2b',
			school_id = EXCLUDED.school_id,
			registration_source = 'created_by_superadmin',
			updated_at = NOW()
	`, userID, req.Email, req.SchoolID)

	if profErr != nil {
		log.Printf("[CREATE SCHOOL ADMIN] Gagal upsert profile untuk user=%s: %v", userID, profErr)
	} else {
		log.Printf("[CREATE SCHOOL ADMIN] Profile upserted untuk user=%s (school_id=%s)", userID, req.SchoolID)
	}

	if adminID, ok := GetAdminIDFromContext(c); ok {
		LogSuperAdminActivity(adminID, "CREATE_SCHOOL_ADMIN", c.ClientIP(),
			c.Request.UserAgent(), "",
			fmt.Sprintf("Membuat akun admin sekolah untuk: %s (%s)", req.FullName, req.Email))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Akun Admin Sekolah berhasil dibuat dan tersimpan.",
		"admin_id":  userID,
		"email":     req.Email,
		"school_id": req.SchoolID,
	})
}

// ET /api/superadmin/school-admins
func ListSchoolAdmins(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT sa.id, sa.full_name, sa.email, sa.nip, sa.is_active, sa.created_at, s.school_name, s.npsn
		FROM school_admins sa
		JOIN schools s ON sa.school_id = s.id
		ORDER BY sa.created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar admin sekolah: " + err.Error()})
		return
	}
	defer rows.Close()

	type SchoolAdminItem struct {
		ID         string    `json:"id"`
		FullName   string    `json:"full_name"`
		Email      string    `json:"email"`
		Nip        string    `json:"nip"`
		IsActive   bool      `json:"is_active"`
		CreatedAt  time.Time `json:"created_at"`
		SchoolName string    `json:"school_name"`
		Npsn       string    `json:"npsn"`
	}

	admins := []SchoolAdminItem{}
	for rows.Next() {
		var a SchoolAdminItem
		var nip sql.NullString
		if err := rows.Scan(&a.ID, &a.FullName, &a.Email, &nip, &a.IsActive, &a.CreatedAt, &a.SchoolName, &a.Npsn); err == nil {
			a.Nip = nip.String
			admins = append(admins, a)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_admins": len(admins),
		"admins":       admins,
	})
}

// PATCH /api/superadmin/schools/:id/status
func UpdateSchoolStatus(c *gin.Context) {
	adminID, ok := GetAdminIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "MISSING_SESSION",
				"message": "Sesi superadmin tidak valid",
			},
		})
		return
	}

	schoolID := c.Param("id")

	var req struct {
		IsActive bool   `json:"is_active"`
		Reason   string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	if len(strings.TrimSpace(req.Reason)) < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alasan wajib diisi (minimal 5 karakter)"})
		return
	}

	// Cek sekolah ada
	var currentActive bool
	var schoolName string
	err := database.DB.QueryRow(`
		SELECT COALESCE(is_active, false), school_name 
		FROM schools WHERE id = $1
	`, schoolID).Scan(&currentActive, &schoolName)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sekolah tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal cek sekolah: " + err.Error()})
		return
	}

	if currentActive == req.IsActive {
		statusText := "aktif"
		if !req.IsActive {
			statusText = "nonaktif"
		}
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Sekolah sudah berstatus %s", statusText),
		})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()


	_, err = tx.Exec(`UPDATE schools SET is_active = $1, updated_at = NOW() WHERE id = $2`,
		req.IsActive, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status sekolah: " + err.Error()})
		return
	}

	var adminCount, studentCount int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM school_admins WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&adminCount)
	_ = tx.QueryRow(`SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&studentCount)

	autoClosedSessions := 0

	if !req.IsActive {
		// Cari semua sesi open/scheduled
		rows, err := tx.Query(`
			SELECT id FROM attendance_sessions
			WHERE school_id = $1 AND status IN ('open', 'scheduled')
		`, schoolID)
		if err == nil {
			var sessionIDs []string
			for rows.Next() {
				var sid string
				if err := rows.Scan(&sid); err == nil {
					sessionIDs = append(sessionIDs, sid)
				}
			}
			rows.Close()

			for _, sid := range sessionIDs {
				_, _ = tx.Exec(`SELECT auto_mark_absent_for_session($1)`, sid)
				_, _ = tx.Exec(`
					UPDATE attendance_sessions SET
						status = 'auto_closed',
						closed_at = NOW(),
						updated_at = NOW()
					WHERE id = $1
				`, sid)
				_, _ = tx.Exec(`SELECT recalculate_session_stats($1)`, sid)
			}
			autoClosedSessions = len(sessionIDs)
		}

		// Nonaktifkan school_members
		_, _ = tx.Exec(`
			UPDATE school_members 
			SET is_active = FALSE, updated_at = NOW()
			WHERE school_id = $1 AND is_active = TRUE
		`, schoolID)
	} else {
		// Aktifkan kembali school_members yang belum pernah left
		_, _ = tx.Exec(`
			UPDATE school_members 
			SET is_active = TRUE, updated_at = NOW()
			WHERE school_id = $1 AND left_at IS NULL
		`, schoolID)
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit: " + err.Error()})
		return
	}

	// Log aktivitas
	action := "SCHOOL_DEACTIVATED"
	if req.IsActive {
		action = "SCHOOL_ACTIVATED"
	}
	details := fmt.Sprintf(
		"[%s] %s | Reason: %s | Admins: %d | Students: %d | Auto-closed sessions: %d",
		action, schoolName, req.Reason, adminCount, studentCount, autoClosedSessions,
	)
	LogSuperAdminActivity(adminID, action, c.ClientIP(), c.Request.UserAgent(), "", details)

	statusText := "dinonaktifkan"
	if req.IsActive {
		statusText = "diaktifkan"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              fmt.Sprintf("Sekolah berhasil %s", statusText),
		"school_id":            schoolID,
		"school_name":          schoolName,
		"is_active":            req.IsActive,
		"affected_admins":      adminCount,
		"affected_students":    studentCount,
		"auto_closed_sessions": autoClosedSessions,
	})
}