package school

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/middleware" 

	"github.com/gin-gonic/gin"
)

// ==========================================
// GET /api/school-admin/subjects
// ==========================================
func ListSubjects(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c) // Asumsi kita pindah helper ke middleware
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	isActiveFilter := c.Query("is_active")

	query := `
		SELECT 
			s.id, s.name, COALESCE(s.code, '') AS code,
			COALESCE(s.description, '') AS description,
			s.category,  
			s.is_active, s.created_at, s.updated_at,
			(
				SELECT COUNT(*) FROM teacher_subjects ts
				WHERE ts.subject_id = s.id
			) AS teacher_count
		FROM subjects s
		WHERE s.school_id = $1
	`
	args := []any{schoolID}

	switch isActiveFilter {
	case "true":
		query += " AND s.is_active = TRUE"
	case "false":
		query += " AND s.is_active = FALSE"
	}

	query += " ORDER BY s.name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat mapel: " + err.Error()})
		return
	}
	defer rows.Close()

	type SubjectItem struct {
		ID           string    `json:"id"`
		Name         string    `json:"name"`
		Code         string    `json:"code"`
		Description  string    `json:"description"`
		Category     string    `json:"category"`
		IsActive     bool      `json:"is_active"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		TeacherCount int       `json:"teacher_count"`
	}

	list := []SubjectItem{}
	for rows.Next() {
		var s SubjectItem
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Code, &s.Description,
			&s.Category,
			&s.IsActive, &s.CreatedAt, &s.UpdatedAt, &s.TeacherCount,
		); err == nil {
			list = append(list, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    len(list),
		"subjects": list,
	})
}

// ==========================================
// POST /api/school-admin/subjects
// ==========================================
func CreateSubject(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req CreateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var existingID string
	err = database.DB.QueryRow(`
		SELECT id FROM subjects 
		WHERE school_id = $1 AND LOWER(name) = LOWER($2)
	`, schoolID, strings.TrimSpace(req.Name)).Scan(&existingID)

	if err == nil && existingID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Mapel dengan nama tersebut sudah ada"})
		return
	}

	var subjectID string
	err = database.DB.QueryRow(`
		INSERT INTO subjects (school_id, name, code, description, category)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), COALESCE(NULLIF($5, ''), 'wajib'))
		RETURNING id
	`, schoolID, strings.TrimSpace(req.Name), req.Code, req.Description, req.Category).Scan(&subjectID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan mapel: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Mata pelajaran berhasil ditambahkan",
		"subject_id": subjectID,
	})
}

// ==========================================
// PUT /api/school-admin/subjects/:id
// ==========================================
func UpdateSubject(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	subjectID := c.Param("id")

	var req UpdateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	if req.Name != "" {
		var existingID string
		err = database.DB.QueryRow(`
			SELECT id FROM subjects 
			WHERE school_id = $1 AND LOWER(name) = LOWER($2) AND id <> $3
		`, schoolID, strings.TrimSpace(req.Name), subjectID).Scan(&existingID)

		if err == nil && existingID != "" {
			c.JSON(http.StatusConflict, gin.H{"error": "Mapel dengan nama tersebut sudah ada"})
			return
		}
	}

	result, err := database.DB.Exec(`
		UPDATE subjects SET
			name = COALESCE(NULLIF($1, ''), name),
			code = NULLIF($2, ''),
			description = NULLIF($3, ''),
			category = COALESCE(NULLIF($4, ''), category),
			is_active = COALESCE($5, is_active),
			updated_at = NOW()
		WHERE id = $6 AND school_id = $7
	`, strings.TrimSpace(req.Name), req.Code, req.Description, req.Category, req.IsActive, subjectID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update mapel: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil diperbarui"})
}

// ==========================================
// DELETE /api/school-admin/subjects/:id
// ==========================================
func DeleteSubject(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	subjectID := c.Param("id")

	var usedCount int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM teacher_subjects WHERE subject_id = $1
	`, subjectID).Scan(&usedCount)

	if usedCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf(
				"Mapel ini masih diampu oleh %d guru. Hapus relasi dulu atau nonaktifkan saja.",
				usedCount,
			),
		})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE subjects SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND school_id = $2
	`, subjectID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil dinonaktifkan"})
}

// ==========================================
// GET /api/school-admin/subjects/:id
// ==========================================
func GetSubjectDetail(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	subjectID := c.Param("id")

	var s struct {
		ID          string
		Name        string
		Code        sql.NullString
		Description sql.NullString
		Category    string
		IsActive    bool
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	err = database.DB.QueryRow(`
		SELECT id, name, code, description, category, is_active, created_at, updated_at
		FROM subjects WHERE id = $1 AND school_id = $2
	`, subjectID, schoolID).Scan(
		&s.ID, &s.Name, &s.Code, &s.Description, &s.Category,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	teacherRows, err := database.DB.Query(`
		SELECT 
			t.id AS teacher_id, t.nama_lengkap, 
			ts.is_primary
		FROM teacher_subjects ts
		JOIN teachers t ON ts.teacher_id = t.id
		WHERE ts.subject_id = $1 AND t.is_active = TRUE
		ORDER BY t.nama_lengkap ASC
	`, subjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer teacherRows.Close()

	type TeacherInfo struct {
		TeacherID   string `json:"teacher_id"`
		TeacherName string `json:"teacher_name"`
		IsPrimary   bool   `json:"is_primary"`
	}

	teachers := []TeacherInfo{}
	for teacherRows.Next() {
		var t TeacherInfo
		if err := teacherRows.Scan(&t.TeacherID, &t.TeacherName, &t.IsPrimary); err == nil {
			teachers = append(teachers, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"subject": gin.H{
			"id":          s.ID,
			"name":        s.Name,
			"code":        s.Code.String,
			"description": s.Description.String,
			"category":    s.Category,
			"is_active":   s.IsActive,
			"created_at":  s.CreatedAt,
			"updated_at":  s.UpdatedAt,
			"teachers":    teachers,
		},
	})
}

// ==========================================
// POST /api/school-admin/teachers/:id/subjects
// ==========================================
func AssignSubjectsToTeacher(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := middleware.GetAdminIDFromUser(c)

	teacherID := c.Param("id")

	var req AssignSubjectsToTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var exists bool
	err = database.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM teachers WHERE id = $1 AND school_id = $2
		)
	`, teacherID, schoolID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()

	_, _ = tx.Exec(`DELETE FROM teacher_subjects WHERE teacher_id = $1`, teacherID)

	insertedCount := 0
	for _, subjectID := range req.SubjectIDs {
		var validSchoolID string
		err := tx.QueryRow(`
			SELECT school_id FROM subjects WHERE id = $1 AND is_active = TRUE
		`, subjectID).Scan(&validSchoolID)
		if err != nil || validSchoolID != schoolID {
			log.Printf("[ASSIGN SUBJECTS] Skip subject %s: tidak valid", subjectID)
			continue
		}

		isPrimary := (subjectID == req.IsPrimary)

		_, err = tx.Exec(`
			INSERT INTO teacher_subjects 
				(teacher_id, subject_id, school_id, is_primary, assigned_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (teacher_id, subject_id) DO UPDATE SET
				is_primary = EXCLUDED.is_primary,
				assigned_at = NOW()
		`, teacherID, subjectID, schoolID, isPrimary, adminID)

		if err == nil {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("%d mapel berhasil di-assign ke guru", insertedCount),
		"assigned_count": insertedCount,
	})
}

// ==========================================
// GET /api/school-admin/teachers/:id/subjects
// ==========================================
func GetTeacherSubjects(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	teacherID := c.Param("id")

	var teacherName string
	err = database.DB.QueryRow(`
		SELECT nama_lengkap FROM teachers 
		WHERE id = $1 AND school_id = $2
	`, teacherID, schoolID).Scan(&teacherName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			s.id, s.name, COALESCE(s.code, '') AS code,
			ts.is_primary, ts.assigned_at
		FROM teacher_subjects ts
		JOIN subjects s ON ts.subject_id = s.id
		WHERE ts.teacher_id = $1
		ORDER BY ts.is_primary DESC, s.name ASC
	`, teacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type SubjectAssignment struct {
		SubjectID   string    `json:"subject_id"`
		SubjectName string    `json:"subject_name"`
		SubjectCode string    `json:"subject_code"`
		IsPrimary   bool      `json:"is_primary"`
		AssignedAt  time.Time `json:"assigned_at"`
	}

	list := []SubjectAssignment{}
	for rows.Next() {
		var s SubjectAssignment
		if err := rows.Scan(
			&s.SubjectID, &s.SubjectName, &s.SubjectCode,
			&s.IsPrimary, &s.AssignedAt,
		); err == nil {
			list = append(list, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id":   teacherID,
		"teacher_name": teacherName,
		"total":        len(list),
		"subjects":     list,
	})
}

// ==========================================
// DELETE /api/school-admin/teachers/:id/subjects/:subjectId
// ==========================================
func RemoveTeacherSubject(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	teacherID := c.Param("id")
	subjectID := c.Param("subjectId")

	result, err := database.DB.Exec(`
		DELETE FROM teacher_subjects 
		WHERE teacher_id = $1 AND subject_id = $2 AND school_id = $3
	`, teacherID, subjectID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Relasi tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil dihapus dari guru"})
}