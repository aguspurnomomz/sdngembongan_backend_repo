package teacher

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"teach_partner_dev/database"
	"teach_partner_dev/routes/helpers"

	"github.com/gin-gonic/gin"
)

type CreatePersonalStudentRequest struct {
	FullName      string `json:"full_name" binding:"required,min=2,max=200"`
	StudentNumber string `json:"student_number"` 
	NISN          string `json:"nisn"`           
	Email         string `json:"email"`          
	Notes         string `json:"notes"`          
}

type UpdatePersonalStudentRequest struct {
	FullName      string `json:"full_name"`
	StudentNumber string `json:"student_number"`
	NISN          string `json:"nisn"`
	Email         string `json:"email"`
	Notes         string `json:"notes"`
	IsActive      *bool  `json:"is_active"`
}

type PersonalStudentResponse struct {
	ID              string    `json:"id"`
	OwnerUserID     string    `json:"owner_user_id"`
	FullName        string    `json:"full_name"`
	StudentNumber   string    `json:"student_number"`
	NISN            string    `json:"nisn"`
	Email           string    `json:"email"`
	Notes           string    `json:"notes"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	TotalExamsTaken int `json:"total_exams_taken,omitempty"`
}


func ListPersonalStudents(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Filter opsional
	search := strings.TrimSpace(c.Query("search"))    
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	query := `
		SELECT 
			id, owner_user_id, full_name,
			COALESCE(student_number, ''),
			COALESCE(nisn, ''),
			COALESCE(email, ''),
			COALESCE(notes, ''),
			is_active, created_at, updated_at
		FROM personal_students
		WHERE owner_user_id = $1
	`
	args := []any{scope.UserID}
	argIdx := 2

	if activeOnly {
		query += " AND is_active = TRUE"
	}

	if search != "" {
		query += fmt.Sprintf(
			" AND (full_name ILIKE $%d OR nisn ILIKE $%d OR student_number ILIKE $%d)",
			argIdx, argIdx, argIdx,
		)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	query += " ORDER BY full_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat data siswa: " + err.Error()})
		return
	}
	defer rows.Close()

	students := []PersonalStudentResponse{}
	for rows.Next() {
		var s PersonalStudentResponse
		if err := rows.Scan(
			&s.ID, &s.OwnerUserID, &s.FullName,
			&s.StudentNumber, &s.NISN, &s.Email, &s.Notes,
			&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
		); err == nil {
			students = append(students, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    len(students),
		"students": students,
	})
}


func GetPersonalStudent(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	studentID := c.Param("id")

	var s PersonalStudentResponse
	err = database.DB.QueryRow(`
		SELECT 
			id, owner_user_id, full_name,
			COALESCE(student_number, ''),
			COALESCE(nisn, ''),
			COALESCE(email, ''),
			COALESCE(notes, ''),
			is_active, created_at, updated_at
		FROM personal_students
		WHERE id = $1 AND owner_user_id = $2
	`, studentID, scope.UserID).Scan(
		&s.ID, &s.OwnerUserID, &s.FullName,
		&s.StudentNumber, &s.NISN, &s.Email, &s.Notes,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan atau bukan milik Anda"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Hitung berapa ujian yang sudah diikuti (opsional)
	_ = database.DB.QueryRow(`
		SELECT COUNT(DISTINCT exam_id)
		FROM exam_participants
		WHERE personal_student_id = $1
	`, studentID).Scan(&s.TotalExamsTaken)

	c.JSON(http.StatusOK, gin.H{"student": s})
}

func CreatePersonalStudent(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req CreatePersonalStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid: " + err.Error()})
		return
	}

	// Trim whitespace
	req.FullName = strings.TrimSpace(req.FullName)
	req.StudentNumber = strings.TrimSpace(req.StudentNumber)
	req.NISN = strings.TrimSpace(req.NISN)
	req.Email = strings.TrimSpace(req.Email)
	req.Notes = strings.TrimSpace(req.Notes)

	// Cek duplikat NISN (kalau diisi) — unique per owner
	if req.NISN != "" {
		var existingID string
		err := database.DB.QueryRow(`
			SELECT id FROM personal_students 
			WHERE owner_user_id = $1 AND nisn = $2 AND is_active = TRUE
		`, scope.UserID, req.NISN).Scan(&existingID)

		if err == nil && existingID != "" {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("NISN %s sudah terdaftar di daftar siswa Anda", req.NISN),
			})
			return
		}
	}

	var studentID string
	err = database.DB.QueryRow(`
		INSERT INTO personal_students (
			owner_user_id, full_name, student_number, nisn, email, notes, is_active
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), TRUE
		) RETURNING id
	`, scope.UserID, req.FullName, req.StudentNumber, req.NISN, req.Email, req.Notes).Scan(&studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data siswa: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Siswa berhasil ditambahkan",
		"id":      studentID,
	})
}


func UpdatePersonalStudent(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	studentID := c.Param("id")

	var req UpdatePersonalStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid: " + err.Error()})
		return
	}

	// Cek ownership dulu
	var existingID string
	err = database.DB.QueryRow(`
		SELECT id FROM personal_students 
		WHERE id = $1 AND owner_user_id = $2
	`, studentID, scope.UserID).Scan(&existingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan atau bukan milik Anda"})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	req.StudentNumber = strings.TrimSpace(req.StudentNumber)
	req.NISN = strings.TrimSpace(req.NISN)
	req.Email = strings.TrimSpace(req.Email)
	req.Notes = strings.TrimSpace(req.Notes)

	if req.NISN != "" {
		var dupID string
		err := database.DB.QueryRow(`
			SELECT id FROM personal_students 
			WHERE owner_user_id = $1 AND nisn = $2 AND is_active = TRUE AND id != $3
		`, scope.UserID, req.NISN, studentID).Scan(&dupID)

		if err == nil && dupID != "" {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("NISN %s sudah terdaftar di siswa lain", req.NISN),
			})
			return
		}
	}

	var isActive interface{} = nil
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	result, err := database.DB.Exec(`
		UPDATE personal_students SET
			full_name = COALESCE(NULLIF($1, ''), full_name),
			student_number = NULLIF($2, ''),
			nisn = NULLIF($3, ''),
			email = NULLIF($4, ''),
			notes = NULLIF($5, ''),
			is_active = COALESCE($6, is_active),
			updated_at = NOW()
		WHERE id = $7 AND owner_user_id = $8
	`, req.FullName, req.StudentNumber, req.NISN, req.Email, req.Notes,
		isActive, studentID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update siswa: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data siswa berhasil diperbarui"})
}


func DeletePersonalStudent(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	studentID := c.Param("id")


	result, err := database.DB.Exec(`
		UPDATE personal_students SET
			is_active = FALSE,
			updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND is_active = TRUE
	`, studentID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus siswa: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan atau sudah tidak aktif"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Siswa berhasil dihapus"})
}