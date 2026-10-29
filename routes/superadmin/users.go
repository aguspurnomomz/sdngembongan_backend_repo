package superadmin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// GET /api/superadmin/registered-users
func ListRegisteredUsers(c *gin.Context) {
	filterType := c.DefaultQuery("type", "all")

	query := `
		SELECT 
			p.id, 
			COALESCE(p.nama_guru, ''), 
			COALESCE(p.nip_guru, ''), 
			COALESCE(p.nama_sekolah, ''), 
			COALESCE(p.mata_pelajaran, ''), 
			p.token_balance, 
			p.is_active, 
			p.last_login, 
			p.updated_at,
			COALESCE(p.teacher_type, 'b2c') AS teacher_type,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'school_id', sm.school_id,
					'school_name', s.school_name,
					'school_active', s.is_active,
					'role_in_school', sm.role_in_school
				))
				FROM school_members sm
				JOIN schools s ON sm.school_id = s.id
				WHERE sm.user_id = p.id 
				AND sm.is_active = TRUE 
				AND sm.left_at IS NULL), 
				'[]'::json
			) AS memberships
		FROM profiles p
		WHERE COALESCE(p.role, 'independent_teacher') <> 'school_admin'
	`

	args := []any{}
	if filterType != "all" {
		query += ` AND COALESCE(p.teacher_type, 'b2c') = $1`
		args = append(args, filterType)
	}

	query += ` ORDER BY p.updated_at DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pengguna: " + err.Error()})
		return
	}
	defer rows.Close()

	type UserListItem struct {
		ID            string          `json:"id"`
		NamaGuru      string          `json:"nama_guru"`
		NipGuru       string          `json:"nip_guru"`
		NamaSekolah   string          `json:"nama_sekolah"`
		MataPelajaran string          `json:"mata_pelajaran"`
		TokenBalance  int             `json:"token_balance"`
		IsActive      bool            `json:"is_active"`
		LastLogin     *time.Time      `json:"last_login"`
		UpdatedAt     time.Time       `json:"updated_at"`
		TeacherType   string          `json:"teacher_type"`
		Memberships   json.RawMessage `json:"memberships"`
	}

	users := []UserListItem{}
	for rows.Next() {
		var u UserListItem
		var namaGuru, nipGuru, namaSekolah, mataPelajaran sql.NullString
		var lastLogin sql.NullTime
		var membershipsJSON []byte

		err := rows.Scan(
			&u.ID, &namaGuru, &nipGuru, &namaSekolah, &mataPelajaran,
			&u.TokenBalance, &u.IsActive, &lastLogin, &u.UpdatedAt,
			&u.TeacherType, &membershipsJSON,
		)
		if err != nil {
			continue
		}

		u.NamaGuru = namaGuru.String
		u.NipGuru = nipGuru.String
		u.NamaSekolah = namaSekolah.String
		u.MataPelajaran = mataPelajaran.String
		if lastLogin.Valid {
			u.LastLogin = &lastLogin.Time
		}
		u.Memberships = json.RawMessage(membershipsJSON)

		users = append(users, u)
	}

	// Hitung statistik
	var totalB2C, totalB2B, totalHybrid int
	for _, u := range users {
		switch u.TeacherType {
		case "b2c":
			totalB2C++
		case "b2b":
			totalB2B++
		case "hybrid":
			totalHybrid++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_users":  len(users),
		"total_b2c":    totalB2C,
		"total_b2b":    totalB2B,
		"total_hybrid": totalHybrid,
		"users":        users,
	})
}

// PATCH /api/superadmin/users/:id/status
func UpdateUserStatus(c *gin.Context) {
	userID := c.Param("id")

	var req UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	result, err := database.DB.Exec(
		`UPDATE profiles SET is_active = $1, updated_at = NOW() WHERE id = $2`,
		req.IsActive, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status pengguna: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pengguna tidak ditemukan"})
		return
	}

	statusText := "diaktifkan"
	if !req.IsActive {
		statusText = "diblokir"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Pengguna berhasil %s", statusText),
		"user_id":   userID,
		"is_active": req.IsActive,
	})
}