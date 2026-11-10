package school

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type AcademicYearRequest struct {
	Name      string `json:"name" binding:"required"`
	Semester  string `json:"semester" binding:"required"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type UpdateSchoolProfileRequest struct {
	SchoolName     string  `json:"school_name" binding:"required"`
	Npsn           string  `json:"npsn" binding:"required"`
	Address        string  `json:"address"`
	City           string  `json:"city"`
	Email          string  `json:"email"`
	Accreditation  string  `json:"accreditation"`
	SchoolStatus   string  `json:"school_status"`
	Phone          string  `json:"phone"`
	Website        string  `json:"website"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	FoundationName string  `json:"foundation_name"`
}

type CreateCalendarEventRequest struct {
	CalendarDate         string   `json:"calendar_date" binding:"required"`
	DayType              string   `json:"day_type" binding:"required"`
	Name                 string   `json:"name" binding:"required"`
	Description          string   `json:"description"`
	IsAttendanceRequired *bool    `json:"is_attendance_required"`
	IncludeInReport      *bool    `json:"include_in_report"`
	TargetClassGroupIDs  []string `json:"target_class_group_ids"`
}

type UpdateCalendarEventRequest struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	IsAttendanceRequired *bool    `json:"is_attendance_required"`
	IncludeInReport      *bool    `json:"include_in_report"`
	TargetClassGroupIDs  []string `json:"target_class_group_ids"`
}

// GET /school-admin/academic-years
func GetAcademicYears(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")
	
	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Data admin sekolah tidak valid atau tidak aktif"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, name, semester, start_date, end_date, is_active, created_at 
		FROM school_academic_years 
		WHERE school_id = $1 
		ORDER BY created_at DESC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tahun akademik"})
		return
	}
	defer rows.Close()

	type AcademicYear struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Semester  string    `json:"semester"`
		StartDate *string   `json:"start_date"`
		EndDate   *string   `json:"end_date"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
	}

	var years []AcademicYear
	for rows.Next() {
		var y AcademicYear
		var start, end sql.NullTime
		if err := rows.Scan(&y.ID, &y.Name, &y.Semester, &start, &end, &y.IsActive, &y.CreatedAt); err == nil {
			if start.Valid {
				s := start.Time.Format("2006-01-02")
				y.StartDate = &s
			}
			if end.Valid {
				s := end.Time.Format("2006-01-02")
				y.EndDate = &s
			}
			years = append(years, y)
		}
	}

	c.JSON(http.StatusOK, gin.H{"academic_years": years})
}

// POST /school-admin/academic-years
func CreateAcademicYear(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	var req AcademicYearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	var yearID string
	err = database.DB.QueryRow(`
		INSERT INTO school_academic_years (school_id, name, semester, start_date, end_date, is_active)
		VALUES ($1, $2, $3, NULLIF($4, '')::date, NULLIF($5, '')::date, FALSE)
		RETURNING id
	`, schoolID, req.Name, req.Semester, req.StartDate, req.EndDate).Scan(&yearID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan tahun akademik: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tahun akademik berhasil ditambahkan", "id": yearID})
}

// PATCH /school-admin/academic-years/:id/activate
func ActivateAcademicYear(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	yearID := c.Param("id")

	_, _ = database.DB.Exec(`UPDATE school_academic_years SET is_active = FALSE WHERE school_id = $1`, schoolID)
	_, err = database.DB.Exec(`UPDATE school_academic_years SET is_active = TRUE WHERE id = $1 AND school_id = $2`, yearID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan tahun akademik"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tahun akademik aktif berhasil diperbarui"})
}

// GET /school-admin/profile
func GetSchoolProfile(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")
	
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE id = $1 AND is_active = TRUE LIMIT 1
	`, userID).Scan(&schoolID)

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	var id, schoolName, npsn, address, jenjang, city, email, accreditation, schoolStatus, phone, website, foundationName sql.NullString
	var latitude, longitude sql.NullFloat64
	var isActive bool
	var createdAt time.Time

	err = database.DB.QueryRow(`
		SELECT 
			id, school_name, npsn, COALESCE(address, ''), COALESCE(city, ''),     
			COALESCE(email, ''), COALESCE(accreditation, ''), COALESCE(school_status, 'Negeri'),
			COALESCE(phone, ''), COALESCE(website, ''), latitude, longitude,
			COALESCE(foundation_name, ''), COALESCE(jenjang, 'SMP'), is_active, created_at 
		FROM schools WHERE id = $1
	`, schoolID).Scan(
		&id, &schoolName, &npsn, &address, &city, &email, 
		&accreditation, &schoolStatus, &phone, &website, &latitude, &longitude, &foundationName,
		&jenjang, &isActive, &createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Data sekolah tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"school": gin.H{
			"id": id.String, "school_name": schoolName.String, "npsn": npsn.String,
			"address": address.String, "city": city.String, "email": email.String,
			"accreditation": accreditation.String, "school_status": schoolStatus.String,
			"phone": phone.String, "website": website.String, "latitude": latitude.Float64,
			"longitude": longitude.Float64, "foundation_name": foundationName.String,
			"jenjang": jenjang.String, "is_active": isActive, "created_at": createdAt,
		},
	})
}

// PUT /school-admin/profile
func UpdateSchoolProfile(c *gin.Context) {
	var schoolID string
	userID, _ := c.Get("user_id")

	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
		return
	}

	var req UpdateSchoolProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	_, err = database.DB.Exec(`
		UPDATE schools SET 
			school_name = $1, npsn = $2, address = $3, city = $4, email = $5, 
			accreditation = $6, school_status = $7, phone = $8, website = $9, 
			latitude = $10, longitude = $11, foundation_name = $12, updated_at = NOW() 
		WHERE id = $13
	`, req.SchoolName, req.Npsn, req.Address, req.City, req.Email, 
		req.Accreditation, req.SchoolStatus, req.Phone, req.Website, 
		req.Latitude, req.Longitude, req.FoundationName, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data sekolah: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profil sekolah berhasil diperbarui"})
}

// GET /school-admin/dashboard-stats
func GetDashboardStats(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var totalStudents, maleStudents, femaleStudents int
	var totalTeachers, maleTeachers, femaleTeachers int

	_ = database.DB.QueryRow(`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE jenis_kelamin = 'L'), COUNT(*) FILTER (WHERE jenis_kelamin = 'P')
		FROM students WHERE school_id = $1 AND is_active = TRUE
	`, schoolID).Scan(&totalStudents, &maleStudents, &femaleStudents)

	_ = database.DB.QueryRow(`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE jenis_kelamin = 'L'), COUNT(*) FILTER (WHERE jenis_kelamin = 'P')
		FROM teachers WHERE school_id = $1 AND is_active = TRUE
	`, schoolID).Scan(&totalTeachers, &maleTeachers, &femaleTeachers)

	c.JSON(http.StatusOK, gin.H{
		"students": gin.H{"total": totalStudents, "male": maleStudents, "female": femaleStudents},
		"teachers": gin.H{"total": totalTeachers, "male": maleTeachers, "female": femaleTeachers},
	})
}

// GET /school-admin/calendar
func GetCalendarEvents(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	fromDate := c.Query("from")
	toDate := c.Query("to")
	dayType := c.Query("type")

	query := `
		SELECT id, calendar_date, day_type, name, COALESCE(description, ''),
			is_attendance_required, include_in_report,
			COALESCE(target_class_group_ids, ARRAY[]::uuid[]), created_at
		FROM academic_calendar WHERE school_id = $1
	`
	args := []any{schoolID}
	argIdx := 2

	if fromDate != "" {
		query += fmt.Sprintf(" AND calendar_date >= $%d", argIdx)
		args = append(args, fromDate)
		argIdx++
	}
	if toDate != "" {
		query += fmt.Sprintf(" AND calendar_date <= $%d", argIdx)
		args = append(args, toDate)
		argIdx++
	}
	if dayType != "" {
		query += fmt.Sprintf(" AND day_type = $%d", argIdx)
		args = append(args, dayType)
		argIdx++
	}

	query += " ORDER BY calendar_date ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil kalender: " + err.Error()})
		return
	}
	defer rows.Close()

	type CalendarItem struct {
		ID                   string    `json:"id"`
		CalendarDate         string    `json:"calendar_date"`
		DayType              string    `json:"day_type"`
		Name                 string    `json:"name"`
		Description          string    `json:"description"`
		IsAttendanceRequired bool      `json:"is_attendance_required"`
		IncludeInReport      bool      `json:"include_in_report"`
		TargetClassGroupIDs  []string  `json:"target_class_group_ids"`
		CreatedAt            time.Time `json:"created_at"`
	}

	var list []CalendarItem
	for rows.Next() {
		var item CalendarItem
		var dateVal time.Time
		var targetIDs []string

		if err := rows.Scan(
			&item.ID, &dateVal, &item.DayType, &item.Name, &item.Description,
			&item.IsAttendanceRequired, &item.IncludeInReport, &targetIDs, &item.CreatedAt,
		); err == nil {
			item.CalendarDate = dateVal.Format("2006-01-02")
			item.TargetClassGroupIDs = targetIDs
			list = append(list, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"calendar": list, "total": len(list)})
}

// POST /school-admin/calendar
func CreateCalendarEvent(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	isAttendanceRequired := true
	if req.IsAttendanceRequired != nil { isAttendanceRequired = *req.IsAttendanceRequired }
	includeInReport := true
	if req.IncludeInReport != nil { includeInReport = *req.IncludeInReport }

	var targetIDs interface{} = nil
	if len(req.TargetClassGroupIDs) > 0 { targetIDs = req.TargetClassGroupIDs }

	var newID string
	err = database.DB.QueryRow(`
		INSERT INTO academic_calendar (
			school_id, calendar_date, day_type, name, description,
			is_attendance_required, include_in_report, target_class_group_ids, created_by
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9) RETURNING id
	`, schoolID, req.CalendarDate, req.DayType, req.Name, req.Description,
		isAttendanceRequired, includeInReport, targetIDs, adminID).Scan(&newID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Event kalender berhasil dibuat", "id": newID})
}

// PUT /school-admin/calendar/:id
func UpdateCalendarEvent(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	eventID := c.Param("id")
	var req UpdateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
		return
	}

	var targetIDs interface{} = nil
	if len(req.TargetClassGroupIDs) > 0 { targetIDs = req.TargetClassGroupIDs }

	result, err := database.DB.Exec(`
		UPDATE academic_calendar SET
			name = COALESCE(NULLIF($1, ''), name),
			description = COALESCE(NULLIF($2, ''), description),
			is_attendance_required = COALESCE($3, is_attendance_required),
			include_in_report = COALESCE($4, include_in_report),
			target_class_group_ids = COALESCE($5, target_class_group_ids),
			updated_at = NOW()
		WHERE id = $6 AND school_id = $7
	`, req.Name, req.Description, req.IsAttendanceRequired, req.IncludeInReport, targetIDs, eventID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event berhasil diperbarui"})
}

// DELETE /school-admin/calendar/:id
func DeleteCalendarEvent(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(
		`DELETE FROM academic_calendar WHERE id = $1 AND school_id = $2`,
		c.Param("id"), schoolID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event berhasil dihapus"})
}