package school

import (
	"database/sql"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type CreateStudentRequest struct {
	ClassSubGroupID string `json:"class_sub_group_id" binding:"required"`
	FullName        string `json:"full_name" binding:"required"`
	StudentNumber   string `json:"student_number"`
	NISN            string `json:"nisn"`
	JenisKelamin    string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
}

type UpdateStudentRequest struct {
	ClassSubGroupID string `json:"class_sub_group_id"`
	FullName        string `json:"full_name"`
	StudentNumber   string `json:"student_number"`
	NISN            string `json:"nisn"`
	JenisKelamin    string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	IsActive        *bool  `json:"is_active"`
}

// GET /school-admin/students
func GetStudents(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	subClassID := c.Query("sub_class_id")

	query := `
		SELECT 
			s.id, s.full_name, COALESCE(s.student_number, ''), 
			COALESCE(s.nisn, ''), COALESCE(s.jenis_kelamin, ''),
			s.is_active, s.created_at, s.class_sub_group_id,
			COALESCE(csg.name, '') AS sub_class_name, csg.class_group_id,
			COALESCE(cg.name, '') AS class_group_name, COALESCE(cg.class_type, 'Umum') AS class_group_type
		FROM students s
		LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
		LEFT JOIN class_groups cg ON csg.class_group_id = cg.id
		WHERE s.school_id = $1
	`
	args := []any{schoolID}

	if subClassID != "" {
		query += ` AND s.class_sub_group_id = $2`
		args = append(args, subClassID)
	}

	query += ` ORDER BY cg.level ASC, csg.name ASC, NULLIF(s.student_number, '')::int ASC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data murid: " + err.Error()})
		return
	}
	defer rows.Close()

	type StudentItem struct {
		ID              string    `json:"id"`
		FullName        string    `json:"full_name"`
		StudentNumber   string    `json:"student_number"`
		NISN            string    `json:"nisn"`
		JenisKelamin    string    `json:"jenis_kelamin"`
		IsActive        bool      `json:"is_active"`
		CreatedAt       time.Time `json:"created_at"`
		ClassSubGroupID *string   `json:"class_sub_group_id"`
		SubClassName    string    `json:"sub_class_name"`
		ClassGroupID    *string   `json:"class_group_id"`
		ClassGroupName  string    `json:"class_group_name"`
		ClassGroupType  string    `json:"class_group_type"`
	}

	var students []StudentItem
	for rows.Next() {
		var s StudentItem
		var subGroupID, classGroupID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.FullName, &s.StudentNumber, &s.NISN, &s.JenisKelamin,
			&s.IsActive, &s.CreatedAt, &subGroupID, &s.SubClassName, &classGroupID, &s.ClassGroupName, &s.ClassGroupType,
		); err == nil {
			if subGroupID.Valid { s.ClassSubGroupID = &subGroupID.String }
			if classGroupID.Valid { s.ClassGroupID = &classGroupID.String }
			students = append(students, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"total": len(students), "students": students})
}

// POST /school-admin/students
func CreateStudent(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	var req CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var classGroupID string
	err = database.DB.QueryRow(`SELECT class_group_id FROM class_sub_groups WHERE id = $1 AND school_id = $2`, req.ClassSubGroupID, schoolID).Scan(&classGroupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak ditemukan atau bukan milik sekolah Anda"})
		return
	}

	if req.NISN != "" {
		var existingID string
		err = database.DB.QueryRow(`SELECT id FROM students WHERE school_id = $1 AND nisn = $2`, schoolID, req.NISN).Scan(&existingID)
		if err == nil && existingID != "" {
			c.JSON(http.StatusConflict, gin.H{"error": "NISN sudah terdaftar di sekolah ini"})
			return
		}
	}

	var studentID string
	err = database.DB.QueryRow(`
		INSERT INTO students (school_id, class_group_id, class_sub_group_id, full_name, student_number, nisn, jenis_kelamin, is_active)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), TRUE) RETURNING id
	`, schoolID, classGroupID, req.ClassSubGroupID, req.FullName, req.StudentNumber, req.NISN, req.JenisKelamin).Scan(&studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data murid: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Murid berhasil ditambahkan", "id": studentID})
}

// PUT /school-admin/students/:id
func UpdateStudent(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	studentID := c.Param("id")

	var req UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var newClassGroupID interface{} = nil
	if req.ClassSubGroupID != "" {
		var cgID string
		err = database.DB.QueryRow(`SELECT class_group_id FROM class_sub_groups WHERE id = $1 AND school_id = $2`, req.ClassSubGroupID, schoolID).Scan(&cgID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tujuan tidak valid"})
			return
		}
		newClassGroupID = cgID
	}

	var isActive interface{} = nil
	if req.IsActive != nil { isActive = *req.IsActive }

	var subGroupID interface{} = nil
	if req.ClassSubGroupID != "" { subGroupID = req.ClassSubGroupID }

	result, err := database.DB.Exec(`
		UPDATE students SET
			full_name = COALESCE(NULLIF($1, ''), full_name),
			student_number = NULLIF($2, ''),
			nisn = NULLIF($3, ''),
			jenis_kelamin = COALESCE(NULLIF($4, ''), jenis_kelamin), 
			class_sub_group_id = $5,
			class_group_id = COALESCE($6, class_group_id),
			is_active = COALESCE($7, is_active),
			updated_at = NOW()
		WHERE id = $8 AND school_id = $9
	`, req.FullName, req.StudentNumber, req.NISN, req.JenisKelamin,
		subGroupID, newClassGroupID, isActive, studentID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data murid: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Murid tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data murid berhasil diperbarui"})
}

// DELETE /school-admin/students/:id
func DeleteStudent(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	studentID := c.Param("id")
	result, err := database.DB.Exec(`DELETE FROM students WHERE id = $1 AND school_id = $2`, studentID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus murid: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Murid tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Murid berhasil dihapus"})
}

// GET /school-admin/students/stats
func GetStudentStats(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT csg.id, csg.name, COALESCE(csg.capacity, 0), cg.name AS class_group_name,
			COUNT(s.id) AS student_count
		FROM class_sub_groups csg
		JOIN class_groups cg ON csg.class_group_id = cg.id
		LEFT JOIN students s ON s.class_sub_group_id = csg.id AND s.is_active = TRUE
		WHERE csg.school_id = $1
		GROUP BY csg.id, csg.name, csg.capacity, cg.name
		ORDER BY cg.name ASC, csg.name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil statistik: " + err.Error()})
		return
	}
	defer rows.Close()

	type StatItem struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Capacity       int    `json:"capacity"`
		ClassGroupName string `json:"class_group_name"`
		StudentCount   int    `json:"student_count"`
	}

	var stats []StatItem
	for rows.Next() {
		var s StatItem
		if err := rows.Scan(&s.ID, &s.Name, &s.Capacity, &s.ClassGroupName, &s.StudentCount); err == nil {
			stats = append(stats, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// GET /school-admin/students/gender-stats
func GetStudentGenderStats(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var totalL, totalP, totalNull int
	_ = database.DB.QueryRow(`
		SELECT 
			COUNT(*) FILTER (WHERE jenis_kelamin = 'L') AS total_l,
			COUNT(*) FILTER (WHERE jenis_kelamin = 'P') AS total_p,
			COUNT(*) FILTER (WHERE jenis_kelamin IS NULL OR jenis_kelamin = '') AS total_null
		FROM students 
		WHERE school_id = $1 AND is_active = TRUE
	`, schoolID).Scan(&totalL, &totalP, &totalNull)

	c.JSON(http.StatusOK, gin.H{
		"laki_laki": totalL, "perempuan": totalP, "belum_diisi": totalNull, "total": totalL + totalP + totalNull,
	})
}