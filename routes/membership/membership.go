package membership

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"database/sql"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type JoinSchoolRequest struct {
	SchoolID     string `json:"school_id" binding:"required"`
	RoleInSchool string `json:"role_in_school"`
	Notes        string `json:"notes"`
}

type LeaveSchoolRequest struct {
	SchoolID string `json:"school_id" binding:"required"`
	Reason   string `json:"reason"`
}

type MembershipItem struct {
	ID           string     `json:"id"`
	SchoolID     string     `json:"school_id"`
	SchoolName   string     `json:"school_name"`
	NPSN         string     `json:"npsn"`
	Jenjang      string     `json:"jenjang"`
	RoleInSchool string     `json:"role_in_school"`
	IsActive     bool       `json:"is_active"`
	SchoolActive bool       `json:"school_active"`
	JoinedAt     time.Time  `json:"joined_at"`
	LeftAt       *time.Time `json:"left_at"`
}

type AvailableSchoolItem struct {
	ID         string `json:"id"`
	SchoolName string `json:"school_name"`
	NPSN       string `json:"npsn"`
	Jenjang    string `json:"jenjang"`
	Address    string `json:"address"`
}

func JoinSchool(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	var req JoinSchoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Validasi sekolah ada & aktif
	var schoolExists bool
	var schoolName string
	err := database.DB.QueryRow(`
		SELECT COALESCE(is_active, false), school_name
		FROM schools WHERE id = $1
	`, req.SchoolID).Scan(&schoolExists, &schoolName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sekolah tidak ditemukan"})
		return
	}

	if !schoolExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sekolah sedang dinonaktifkan, tidak bisa join"})
		return
	}

	roleInSchool := req.RoleInSchool
	if roleInSchool == "" {
		roleInSchool = "teacher"
	}

	// Insert atau update membership
	var membershipID string
	err = database.DB.QueryRow(`
		INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes)
		VALUES ($1, $2, $3, TRUE, NULLIF($4, ''))
		ON CONFLICT (user_id, school_id) DO UPDATE SET
			role_in_school = EXCLUDED.role_in_school,
			is_active = TRUE,
			left_at = NULL,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id
	`, userIDStr, req.SchoolID, roleInSchool, req.Notes).Scan(&membershipID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal bergabung dengan sekolah: " + err.Error()})
		return
	}

	_, errRole := database.DB.Exec(`
		INSERT INTO school_member_roles (member_id, role_id, is_active, started_at)
		SELECT $1, sr.id, TRUE, NOW()
		FROM school_roles sr
		WHERE sr.code = $2
		ON CONFLICT (member_id, role_id) DO UPDATE SET
			is_active = TRUE,
			ended_at = NULL
	`, membershipID, roleInSchool)

	if errRole != nil {
		log.Printf("[JOIN SCHOOL] Gagal assign role ke school_member_roles: %v", errRole)
	}

	// Update teacher_type menjadi hybrid/b2b
	_, _ = database.DB.Exec(`
		UPDATE profiles 
		SET teacher_type = CASE 
			WHEN teacher_type = 'b2c' THEN 'hybrid'
			WHEN teacher_type = 'b2b' THEN 'b2b'
			ELSE 'b2b'
		END,
		updated_at = NOW()
		WHERE id = $1
	`, userIDStr)

	c.JSON(http.StatusCreated, gin.H{
		"message":        fmt.Sprintf("Berhasil bergabung dengan %s", schoolName),
		"membership_id":  membershipID,
		"school_id":      req.SchoolID,
		"school_name":    schoolName,
		"role_in_school": roleInSchool,
	})
}

func LeaveSchool(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	var req LeaveSchoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Soft leave
	result, err := database.DB.Exec(`
		UPDATE school_members 
		SET is_active = FALSE, 
			left_at = NOW(), 
			notes = COALESCE(notes, '') || ' | Left: ' || COALESCE(NULLIF($1, ''), 'no reason'),
			updated_at = NOW()
		WHERE user_id = $2 AND school_id = $3 AND is_active = TRUE
	`, req.Reason, userIDStr, req.SchoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal keluar dari sekolah: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Membership tidak ditemukan atau sudah tidak aktif"})
		return
	}

	// Cek apakah masih punya membership aktif lain
	var activeMemberships int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM school_members 
		WHERE user_id = $1 AND is_active = TRUE AND left_at IS NULL
	`, userIDStr).Scan(&activeMemberships)

	// Update teacher_type: kalau sudah tidak ada membership → b2c
	if activeMemberships == 0 {
		_, _ = database.DB.Exec(`
			UPDATE profiles SET teacher_type = 'b2c', updated_at = NOW()
			WHERE id = $1
		`, userIDStr)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Berhasil keluar dari sekolah",
		"remaining_membership": activeMemberships,
	})
}

func ListMySchools(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	rows, err := database.DB.Query(`
		SELECT 
			sm.id, sm.school_id, s.school_name, s.npsn, s.jenjang,
			sm.role_in_school, sm.is_active, sm.joined_at, sm.left_at,
			COALESCE(s.is_active, false) AS school_active
		FROM school_members sm
		JOIN schools s ON sm.school_id = s.id
		WHERE sm.user_id = $1
		ORDER BY sm.joined_at DESC
	`, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var list []MembershipItem
	for rows.Next() {
		var m MembershipItem
		var npsn, jenjang sql.NullString
		var leftAt sql.NullTime
		if err := rows.Scan(
			&m.ID, &m.SchoolID, &m.SchoolName, &npsn, &jenjang,
			&m.RoleInSchool, &m.IsActive, &m.JoinedAt, &leftAt, &m.SchoolActive,
		); err == nil {
			m.NPSN = npsn.String
			m.Jenjang = jenjang.String
			if leftAt.Valid {
				m.LeftAt = &leftAt.Time
			}
			list = append(list, m)
		}
	}

	if list == nil {
		list = []MembershipItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":       len(list),
		"memberships": list,
	})
}

func ListAvailableSchools(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	// Tampilkan sekolah aktif yang belum di-join user ini
	rows, err := database.DB.Query(`
		SELECT s.id, s.school_name, s.npsn, COALESCE(s.jenjang, 'SMP'), s.address
		FROM schools s
		WHERE s.is_active = TRUE
		  AND NOT EXISTS (
			SELECT 1 FROM school_members sm 
			WHERE sm.user_id = $1 
			  AND sm.school_id = s.id 
			  AND sm.is_active = TRUE
		  )
		ORDER BY s.school_name ASC
	`, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var list []AvailableSchoolItem
	for rows.Next() {
		var s AvailableSchoolItem
		var npsn, address sql.NullString
		if err := rows.Scan(&s.ID, &s.SchoolName, &npsn, &s.Jenjang, &address); err == nil {
			s.NPSN = npsn.String
			s.Address = address.String
			list = append(list, s)
		}
	}

	if list == nil {
		list = []AvailableSchoolItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(list),
		"schools": list,
	})
}

var _ = time.Time{}