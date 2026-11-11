package teacher

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
	"gembongan_dev_backend/helpers"

	"github.com/gin-gonic/gin"
)

type InviteTeacherRequest struct {
	Email        string `json:"email" binding:"required,email"`
	FullName     string `json:"full_name" binding:"required"`
	NIP          string `json:"nip"`
	JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	RoleInSchool string `json:"role_in_school"`
	Notes        string `json:"notes"`
}

type CreateTeacherRequest struct {
	Email        string `json:"email" binding:"required,email"`
	FullName     string `json:"full_name" binding:"required"`
	Password     string `json:"password" binding:"required,min=6"`
	NIP          string `json:"nip"`
	JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	NoWhatsApp   string `json:"no_whatsapp"`
	RoleInSchool string `json:"role_in_school"`
	Notes        string `json:"notes"`
}

type RemoveTeacherRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type AssignTeacherRequest struct {
	UserID       string `json:"user_id" binding:"required"`
	RoleInSchool string `json:"role_in_school"`
	Notes        string `json:"notes"`
}

type UpdateTeacherRequest struct {
	NamaGuru     string `json:"nama_guru" binding:"required"`
	NIP          string `json:"nip"`
	JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	NoWhatsApp   string `json:"no_whatsapp"`
	RoleInSchool string `json:"role_in_school"`
}

type TeacherItem struct {
	TeacherID    string     `json:"teacher_id"`
	UserID       string     `json:"user_id"`
	NamaGuru     string     `json:"nama_guru"`
	NIPGuru      string     `json:"nip_guru"`
	JenisKelamin string     `json:"jenis_kelamin"`
	Email        string     `json:"email"`
	RoleInSchool string     `json:"role_in_school"`
	IsActive     bool       `json:"is_active"`
	TeacherType  string     `json:"teacher_type"`
	JoinedAt     time.Time  `json:"joined_at"`
	LeftAt       *time.Time `json:"left_at"`
}

func ListSchoolTeachers(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			t.id AS teacher_id,
			t.user_id,
			t.nama_lengkap AS nama_guru,
			COALESCE(t.nip, '') AS nip_guru,
			COALESCE(t.jenis_kelamin, '') AS jenis_kelamin,
			COALESCE(p.email_sekolah, '') AS email,
			COALESCE(sm.role_in_school, 'teacher') AS role_in_school,
			t.is_active,
			t.joined_at,
			sm.left_at,
			COALESCE(p.teacher_type, 'b2b') AS teacher_type
		FROM teachers t
		JOIN profiles p ON t.user_id = p.id
		LEFT JOIN school_members sm 
			ON sm.user_id = t.user_id AND sm.school_id = t.school_id
		WHERE t.school_id = $1
		ORDER BY t.is_active DESC, t.joined_at DESC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var list []TeacherItem
	for rows.Next() {
		var t TeacherItem
		var teacherType sql.NullString
		var leftAt sql.NullTime
		if err := rows.Scan(
			&t.TeacherID, &t.UserID, &t.NamaGuru, &t.NIPGuru, &t.JenisKelamin,
			&t.Email, &t.RoleInSchool, &t.IsActive, &t.JoinedAt, &leftAt, &teacherType,
		); err == nil {
			t.TeacherType = teacherType.String
			if leftAt.Valid {
				t.LeftAt = &leftAt.Time
			}
			list = append(list, t)
		}
	}

	if list == nil {
		list = []TeacherItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    len(list),
		"teachers": list,
	})
}

func InviteTeacher(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := helpers.GetAdminIDFromUser(c)

	var req InviteTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var existingUserID string
	err = database.DB.QueryRow(`
		SELECT id FROM profiles WHERE email_sekolah = $1
	`, req.Email).Scan(&existingUserID)

	if err != nil {
		supabaseURL := os.Getenv("SUPABASE_URL")
		serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

		supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)
		tempPassword := helpers.RandString(12)

		payloadAuth := map[string]any{
			"email":         req.Email,
			"password":      tempPassword,
			"email_confirm": true,
			"user_metadata": map[string]any{
				"full_name": req.FullName,
				"role":      "teacher",
			},
		}
		jsonAuthBody, _ := json.Marshal(payloadAuth)

		httpReq, _ := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("apikey", serviceRoleKey)
		httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke Supabase Auth"})
			return
		}
		defer resp.Body.Close()

		var authResp map[string]any
		json.NewDecoder(resp.Body).Decode(&authResp)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			errMsg := "Gagal membuat akun guru di Supabase Auth"
			if msg, ok := authResp["msg"].(string); ok {
				errMsg = msg
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		existingUserID, _ = authResp["id"].(string)
		if existingUserID == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase Auth"})
			return
		}

		_, _ = database.DB.Exec(`
			INSERT INTO profiles (
				id, nama_guru, nip_guru, jenis_kelamin, email_sekolah, 
				token_balance, is_active, teacher_type, registration_source, updated_at
			)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, 2, TRUE, 'b2b', 'invited_by_school', NOW())
			ON CONFLICT (id) DO UPDATE SET
				nama_guru = EXCLUDED.nama_guru,
				nip_guru = COALESCE(NULLIF(EXCLUDED.nip_guru, ''), profiles.nip_guru),
				jenis_kelamin = COALESCE(NULLIF(EXCLUDED.jenis_kelamin, ''), profiles.jenis_kelamin),
				updated_at = NOW()
		`, existingUserID, req.FullName, req.NIP, req.JenisKelamin, req.Email)
	}

	roleInSchool := req.RoleInSchool
	if roleInSchool == "" {
		roleInSchool = "teacher"
	}

	var membershipID string
	err = database.DB.QueryRow(`
		INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
		VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
		ON CONFLICT (user_id, school_id) DO UPDATE SET
			role_in_school = EXCLUDED.role_in_school,
			is_active = TRUE,
			left_at = NULL,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id
	`, existingUserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan membership: " + err.Error()})
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

	_, _ = database.DB.Exec(`
		UPDATE profiles 
		SET teacher_type = CASE 
			WHEN teacher_type = 'b2c' THEN 'hybrid'
			ELSE 'b2b'
		END,
		updated_at = NOW()
		WHERE id = $1
	`, existingUserID)

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Guru berhasil ditambahkan ke sekolah",
		"user_id":       existingUserID,
		"membership_id": membershipID,
		"email":         req.Email,
	})
}

func CreateTeacher(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := helpers.GetAdminIDFromUser(c)

	var req CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	emailLower := strings.ToLower(strings.TrimSpace(req.Email))

	var existingProfileID string
	err = database.DB.QueryRow(`
		SELECT id FROM profiles WHERE LOWER(email_sekolah) = $1
	`, emailLower).Scan(&existingProfileID)

	if err == nil && existingProfileID != "" {
		var alreadyMember bool
		_ = database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM school_members 
				WHERE user_id = $1 AND school_id = $2 
				AND is_active = TRUE AND left_at IS NULL
			)
		`, existingProfileID, schoolID).Scan(&alreadyMember)

		if alreadyMember {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Guru dengan email ini sudah terdaftar di sekolah Anda",
			})
			return
		}

		roleInSchool := req.RoleInSchool
		if roleInSchool == "" {
			roleInSchool = "teacher"
		}

		var membershipID string
		err = database.DB.QueryRow(`
			INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
			VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
			ON CONFLICT (user_id, school_id) DO UPDATE SET
				is_active = TRUE,
				left_at = NULL,
				role_in_school = EXCLUDED.role_in_school,
				notes = EXCLUDED.notes,
				updated_at = NOW()
			RETURNING id
		`, existingProfileID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan guru: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Guru sudah punya akun, berhasil ditambahkan ke sekolah.",
			"user_id":       existingProfileID,
			"membership_id": membershipID,
			"email":         emailLower,
			"is_new_user":   false,
		})
		return
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || serviceRoleKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Konfigurasi Supabase belum lengkap di server",
		})
		return
	}

	supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)

	payloadAuth := map[string]any{
		"email":         emailLower,
		"password":      req.Password,
		"email_confirm": true,
		"user_metadata": map[string]any{
			"full_name":     req.FullName,
			"role":          "teacher",
			"jenis_kelamin": req.JenisKelamin,
		},
	}
	jsonAuthBody, _ := json.Marshal(payloadAuth)

	httpReq, err := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan request ke Supabase"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", serviceRoleKey)
	httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke Supabase Auth"})
		return
	}
	defer resp.Body.Close()

	var authResp map[string]any
	json.NewDecoder(resp.Body).Decode(&authResp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errMsg := "Gagal membuat akun guru di Supabase Auth"
		if msg, ok := authResp["msg"].(string); ok {
			errMsg = msg
		} else if message, ok := authResp["message"].(string); ok {
			errMsg = message
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	newUserID, ok := authResp["id"].(string)
	if !ok || newUserID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase"})
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO profiles (
			id, email_sekolah, token_balance, is_active, teacher_type, 
			registration_source, updated_at
		)
		VALUES ($1, $2, 2, TRUE, 'b2b', 'created_by_school_admin', NOW())
		ON CONFLICT (id) DO UPDATE SET
			email_sekolah = EXCLUDED.email_sekolah,
			is_active = TRUE,
			updated_at = NOW()
	`, newUserID, emailLower)

	if err != nil {
		log.Printf("[CREATE TEACHER] Gagal insert profiles untuk user=%s: %v", newUserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Akun dibuat tapi gagal menyimpan profil: " + err.Error(),
		})
		return
	}

	roleInSchool := req.RoleInSchool
	if roleInSchool == "" {
		roleInSchool = "teacher"
	}

	var teacherID string
	err = database.DB.QueryRow(`
		INSERT INTO teachers (
			user_id, school_id, nama_lengkap, nip, jenis_kelamin, no_whatsapp, is_active
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), TRUE)
		ON CONFLICT (user_id) DO UPDATE SET
			school_id = EXCLUDED.school_id,
			nama_lengkap = EXCLUDED.nama_lengkap,
			nip = COALESCE(NULLIF(EXCLUDED.nip, ''), teachers.nip),
			jenis_kelamin = COALESCE(NULLIF(EXCLUDED.jenis_kelamin, ''), teachers.jenis_kelamin),
			no_whatsapp = COALESCE(NULLIF(EXCLUDED.no_whatsapp, ''), teachers.no_whatsapp),
			is_active = TRUE,
			updated_at = NOW()
		RETURNING id
	`, newUserID, schoolID, req.FullName, req.NIP, req.JenisKelamin, req.NoWhatsApp).Scan(&teacherID)

	if err != nil {
		log.Printf("[CREATE TEACHER] Gagal insert teachers untuk user=%s: %v", newUserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Akun dibuat tapi gagal menyimpan data guru: " + err.Error(),
		})
		return
	}

	var membershipID string
	err = database.DB.QueryRow(`
		INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
		VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
		ON CONFLICT (user_id, school_id) DO UPDATE SET
			is_active = TRUE,
			left_at = NULL,
			role_in_school = EXCLUDED.role_in_school,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id
	`, newUserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

	if err != nil {
		log.Printf("[CREATE TEACHER] Gagal insert school_members untuk user=%s: %v", newUserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Akun & profil dibuat, tapi gagal mendaftarkan ke sekolah: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Guru berhasil ditambahkan. Kirim email & password ke guru secara manual.",
		"user_id":        newUserID,
		"teacher_id":     teacherID,
		"membership_id":  membershipID,
		"email":          emailLower,
		"password":       req.Password,
		"full_name":      req.FullName,
		"nip":            req.NIP,
		"jenis_kelamin":  req.JenisKelamin,
		"no_whatsapp":    req.NoWhatsApp,
		"role_in_school": roleInSchool,
		"is_new_user":    true,
	})
}

func UpdateTeacher(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	teacherID := c.Param("id")

	var req UpdateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var userID string
	err = database.DB.QueryRow(`
		SELECT user_id FROM teachers 
		WHERE id = $1 AND school_id = $2
	`, teacherID, schoolID).Scan(&userID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan di sekolah ini"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE teachers 
		SET nama_lengkap = $1, nip = NULLIF($2, ''), jenis_kelamin = NULLIF($3, ''), no_whatsapp = NULLIF($4, ''), updated_at = NOW()
		WHERE id = $5 AND school_id = $6
	`, req.NamaGuru, req.NIP, req.JenisKelamin, req.NoWhatsApp, teacherID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update data guru: " + err.Error()})
		return
	}

	_, err = tx.Exec(`
		UPDATE profiles 
		SET nama_guru = $1, nip_guru = NULLIF($2, ''), jenis_kelamin = NULLIF($3, ''), updated_at = NOW()
		WHERE id = $4
	`, req.NamaGuru, req.NIP, req.JenisKelamin, userID)
	if err != nil {
		fmt.Printf("[UPDATE TEACHER] Gagal update profil untuk user=%s: %v\n", userID, err)
	}

	if req.RoleInSchool != "" {
		_, err = tx.Exec(`
			UPDATE school_members 
			SET role_in_school = $1, updated_at = NOW()
			WHERE user_id = $2 AND school_id = $3
		`, req.RoleInSchool, userID, schoolID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update peran guru: " + err.Error()})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan perubahan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data guru berhasil diperbarui"})
}

func AssignTeacher(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := helpers.GetAdminIDFromUser(c)

	var req AssignTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var exists string
	err = database.DB.QueryRow(`SELECT id FROM profiles WHERE id = $1`, req.UserID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		return
	}

	roleInSchool := req.RoleInSchool
	if roleInSchool == "" {
		roleInSchool = "teacher"
	}

	var membershipID string
	err = database.DB.QueryRow(`
		INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
		VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
		ON CONFLICT (user_id, school_id) DO UPDATE SET
			role_in_school = EXCLUDED.role_in_school,
			is_active = TRUE,
			left_at = NULL,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id
	`, req.UserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	_, _ = database.DB.Exec(`
		UPDATE profiles 
		SET teacher_type = CASE 
			WHEN teacher_type = 'b2c' THEN 'hybrid'
			ELSE 'b2b'
		END,
		updated_at = NOW()
		WHERE id = $1
	`, req.UserID)

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Guru berhasil di-assign ke sekolah",
		"membership_id": membershipID,
	})
}

func RemoveTeacher(c *gin.Context) {
	schoolID, err := helpers.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	teacherID := c.Param("id")

	var req RemoveTeacherRequest
	_ = c.ShouldBindJSON(&req)

	var userID string
	err = database.DB.QueryRow(`
		SELECT user_id FROM teachers 
		WHERE id = $1 AND school_id = $2 AND is_active = TRUE
	`, teacherID, schoolID).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan di sekolah ini"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE teachers 
		SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND school_id = $2
	`, teacherID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = tx.Exec(`
		UPDATE school_members 
		SET is_active = FALSE,
			left_at = NOW(),
			notes = COALESCE(notes, '') || ' | Removed: ' || COALESCE(NULLIF($1, ''), 'no reason'),
			updated_at = NOW()
		WHERE user_id = $2 AND school_id = $3 AND is_active = TRUE
	`, req.Reason, userID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Guru berhasil di-remove dari sekolah"})
}