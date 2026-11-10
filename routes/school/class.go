package school

import (
	"database/sql"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type CreateClassGroupRequest struct {
	Name           string `json:"name" binding:"required"`
	Level          string `json:"level" binding:"required"`
	Major          string `json:"major"`
	ClassType      string `json:"class_type"`
	AcademicYearID string `json:"academic_year_id" binding:"required"`
}

type CreateSubClassRequest struct {
	ClassGroupID      string `json:"class_group_id" binding:"required"`
	Name              string `json:"name" binding:"required"`
	Code              string `json:"code"`
	Capacity          int    `json:"capacity"`
	HomeroomTeacherID string `json:"homeroom_teacher_id"`
	Notes             string `json:"notes"`
}

type UpdateSubClassRequest struct {
	Name              string `json:"name"`
	Code              string `json:"code"`
	Capacity          int    `json:"capacity"`
	HomeroomTeacherID string `json:"homeroom_teacher_id"`
	Notes             string `json:"notes"`
	IsActive          *bool  `json:"is_active"`
}

// GET /school-admin/classes
func GetClasses(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var schoolName string
	_ = database.DB.QueryRow(`SELECT school_name FROM schools WHERE id = $1`, schoolID).Scan(&schoolName)

	rows, err := database.DB.Query(`
		SELECT cg.id, cg.name, cg.level, COALESCE(cg.class_type, 'Umum'), 
			cg.academic_year_id, COALESCE(say.name, '-'), COALESCE(say.semester, '-'), cg.created_at
		FROM class_groups cg
		LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
		WHERE cg.school_id = $1
		ORDER BY cg.level ASC, cg.name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data kelas"})
		return
	}
	defer rows.Close()

	type ClassItem struct {
		ID               string    `json:"id"`
		SchoolName       string    `json:"school_name"`
		Name             string    `json:"name"`
		Level            string    `json:"level"`
		ClassType        string    `json:"class_type"`
		AcademicYearID   string    `json:"academic_year_id"`
		AcademicYearName string    `json:"academic_year_name"`
		Semester         string    `json:"semester"`
		CreatedAt        time.Time `json:"created_at"`
	}

	classes := []ClassItem{}
	for rows.Next() {
		var cl ClassItem
		cl.SchoolName = schoolName
		if err := rows.Scan(&cl.ID, &cl.Name, &cl.Level, &cl.ClassType, &cl.AcademicYearID, &cl.AcademicYearName, &cl.Semester, &cl.CreatedAt); err == nil {
			classes = append(classes, cl)
		}
	}

	c.JSON(http.StatusOK, gin.H{"classes": classes})
}

// POST /school-admin/classes
func CreateClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	var req CreateClassGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var validYearSchoolID string
	err = database.DB.QueryRow(`SELECT school_id FROM school_academic_years WHERE id = $1`, req.AcademicYearID).Scan(&validYearSchoolID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tahun akademik tidak ditemukan di sistem."})
		return
	}

	if validYearSchoolID != schoolID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tahun akademik ini milik sekolah lain."})
		return
	}

	var classID string
	err = database.DB.QueryRow(`
		INSERT INTO class_groups (school_id, academic_year_id, name, level, class_type)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')) RETURNING id
	`, schoolID, req.AcademicYearID, req.Name, req.Level, req.ClassType).Scan(&classID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data kelas: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Kelas berhasil ditambahkan", "id": classID})
}

// DELETE /school-admin/classes/:id
func DeleteClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	classID := c.Param("id")
	result, err := database.DB.Exec(`DELETE FROM class_groups WHERE id = $1 AND school_id = $2`, classID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus kelas"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kelas tidak ditemukan atau bukan milik institusi Anda"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kelas berhasil dihapus"})
}

// GET /school-admin/classes/:classId/sub-classes
func GetSubClassesByClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	classID := c.Param("classId")

	var classExists string
	err = database.DB.QueryRow(`SELECT id FROM class_groups WHERE id = $1 AND school_id = $2`, classID, schoolID).Scan(&classExists)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kelas tidak ditemukan"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT csg.id, csg.name, COALESCE(csg.code, ''), COALESCE(csg.capacity, 0), COALESCE(csg.notes, ''),
			csg.homeroom_teacher_id, COALESCE(p.nama_guru, '') AS homeroom_teacher_name,
			csg.is_active, csg.created_at
		FROM class_sub_groups csg
		LEFT JOIN profiles p ON csg.homeroom_teacher_id = p.id
		WHERE csg.class_group_id = $1 AND csg.school_id = $2
		ORDER BY csg.created_at ASC
	`, classID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil sub kelas: " + err.Error()})
		return
	}
	defer rows.Close()

	type SubClassItem struct {
		ID                  string    `json:"id"`
		Name                string    `json:"name"`
		Code                string    `json:"code"`
		Capacity            int       `json:"capacity"`
		Notes               string    `json:"notes"`
		HomeroomTeacherID   *string   `json:"homeroom_teacher_id"`
		HomeroomTeacherName string    `json:"homeroom_teacher_name"`
		IsActive            bool      `json:"is_active"`
		CreatedAt           time.Time `json:"created_at"`
	}

	var subs []SubClassItem
	for rows.Next() {
		var s SubClassItem
		var teacherID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Code, &s.Capacity, &s.Notes,
			&teacherID, &s.HomeroomTeacherName, &s.IsActive, &s.CreatedAt,
		); err == nil {
			if teacherID.Valid {
				s.HomeroomTeacherID = &teacherID.String
			}
			subs = append(subs, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"class_group_id": classID, "sub_classes": subs})
}

// GET /school-admin/sub-classes (Semua sub kelas)
func GetAllSubClasses(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			csg.id, csg.name, COALESCE(csg.code, ''), COALESCE(csg.capacity, 0), COALESCE(csg.notes, ''),
			csg.homeroom_teacher_id, COALESCE(p.nama_guru, '') AS homeroom_teacher_name,
			csg.is_active, csg.created_at, cg.id AS class_group_id, cg.name AS class_group_name,
			cg.level AS class_group_level, COALESCE(cg.class_type, 'Umum') AS class_group_type,
			cg.academic_year_id, COALESCE(say.name, '-') AS academic_year_name, COALESCE(say.semester, '-') AS semester
		FROM class_sub_groups csg
		JOIN class_groups cg ON csg.class_group_id = cg.id
		LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
		LEFT JOIN profiles p ON csg.homeroom_teacher_id = p.id
		WHERE csg.school_id = $1
		ORDER BY cg.level ASC, cg.name ASC, csg.name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar sub kelas: " + err.Error()})
		return
	}
	defer rows.Close()

	type SubClassWithClass struct {
		ID                  string    `json:"id"`
		Name                string    `json:"name"`
		Code                string    `json:"code"`
		Capacity            int       `json:"capacity"`
		Notes               string    `json:"notes"`
		HomeroomTeacherID   *string   `json:"homeroom_teacher_id"`
		HomeroomTeacherName string    `json:"homeroom_teacher_name"`
		IsActive            bool      `json:"is_active"`
		CreatedAt           time.Time `json:"created_at"`
		ClassGroupID        string    `json:"class_group_id"`
		ClassGroupName      string    `json:"class_group_name"`
		ClassGroupLevel     string    `json:"class_group_level"`
		ClassGroupType      string    `json:"class_group_type"`
		AcademicYearID      string    `json:"academic_year_id"`
		AcademicYearName    string    `json:"academic_year_name"`
		Semester            string    `json:"semester"`
	}

	var subClasses []SubClassWithClass
	for rows.Next() {
		var s SubClassWithClass
		var teacherID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Code, &s.Capacity, &s.Notes,
			&teacherID, &s.HomeroomTeacherName, &s.IsActive, &s.CreatedAt,
			&s.ClassGroupID, &s.ClassGroupName, &s.ClassGroupLevel, &s.ClassGroupType,
			&s.AcademicYearID, &s.AcademicYearName, &s.Semester,
		); err == nil {
			if teacherID.Valid {
				s.HomeroomTeacherID = &teacherID.String
			}
			subClasses = append(subClasses, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"total": len(subClasses), "sub_classes": subClasses})
}

// POST /school-admin/sub-classes
func CreateSubClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	var req CreateSubClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var classExists string
	err = database.DB.QueryRow(`SELECT id FROM class_groups WHERE id = $1 AND school_id = $2`, req.ClassGroupID, schoolID).Scan(&classExists)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kelas induk tidak ditemukan atau bukan milik sekolah Anda"})
		return
	}

	var existingID string
	err = database.DB.QueryRow(`SELECT id FROM class_sub_groups WHERE class_group_id = $1 AND name = $2`, req.ClassGroupID, req.Name).Scan(&existingID)
	if err == nil && existingID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Sub kelas dengan nama tersebut sudah ada"})
		return
	}

	var subID string
	var homeroomTeacher interface{} = nil
	if req.HomeroomTeacherID != "" { homeroomTeacher = req.HomeroomTeacherID }

	err = database.DB.QueryRow(`
		INSERT INTO class_sub_groups (class_group_id, school_id, name, code, capacity, homeroom_teacher_id, notes)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, '')) RETURNING id
	`, req.ClassGroupID, schoolID, req.Name, req.Code, req.Capacity, homeroomTeacher, req.Notes).Scan(&subID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan sub kelas: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Sub kelas berhasil ditambahkan", "id": subID})
}

// PUT /school-admin/sub-classes/:id
func UpdateSubClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	subID := c.Param("id")
	var req UpdateSubClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var homeroomTeacher interface{} = nil
	if req.HomeroomTeacherID != "" { homeroomTeacher = req.HomeroomTeacherID }

	var isActive interface{} = nil
	if req.IsActive != nil { isActive = *req.IsActive }

	result, err := database.DB.Exec(`
		UPDATE class_sub_groups SET
			name = COALESCE(NULLIF($1, ''), name),
			code = NULLIF($2, ''),
			capacity = $3,
			homeroom_teacher_id = $4,
			notes = NULLIF($5, ''),
			is_active = COALESCE($6, is_active),
			updated_at = NOW()
		WHERE id = $7 AND school_id = $8
	`, req.Name, req.Code, req.Capacity, homeroomTeacher, req.Notes, isActive, subID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui sub kelas: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sub kelas tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sub kelas berhasil diperbarui"})
}

// DELETE /school-admin/sub-classes/:id
func DeleteSubClass(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	subID := c.Param("id")
	result, err := database.DB.Exec(`DELETE FROM class_sub_groups WHERE id = $1 AND school_id = $2`, subID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus sub kelas: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sub kelas tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sub kelas berhasil dihapus"})
}