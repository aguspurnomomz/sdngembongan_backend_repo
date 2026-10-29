package school

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/middleware"

	"github.com/gin-gonic/gin"
)

// GET /api/school-admin/timetables
func ListTimetables(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	classSubGroupID := c.Query("class_sub_group_id")
	academicYearID := c.Query("academic_year_id")

	query := `
		SELECT 
			ct.id, ct.class_group_id, ct.class_sub_group_id, ct.academic_year_id,
			ct.semester, ct.status, ct.effective_from, ct.effective_to, ct.notes,
			ct.created_at, ct.updated_at,
			COALESCE(cg.name, '') AS class_group_name,
			COALESCE(csg.name, '') AS class_sub_group_name,
			COALESCE(say.name, '') AS academic_year_name
		FROM class_timetables ct
		LEFT JOIN class_groups cg ON ct.class_group_id = cg.id
		LEFT JOIN class_sub_groups csg ON ct.class_sub_group_id = csg.id
		LEFT JOIN school_academic_years say ON ct.academic_year_id = say.id
		WHERE ct.school_id = $1
	`
	args := []any{schoolID}
	argIdx := 2

	if classSubGroupID != "" {
		query += fmt.Sprintf(" AND ct.class_sub_group_id = $%d", argIdx)
		args = append(args, classSubGroupID)
		argIdx++
	}
	if academicYearID != "" {
		query += fmt.Sprintf(" AND ct.academic_year_id = $%d", argIdx)
		args = append(args, academicYearID)
		argIdx++
	}

	query += " ORDER BY ct.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat jadwal: " + err.Error()})
		return
	}
	defer rows.Close()

	type TimetableItem struct {
		ID                string    `json:"id"`
		ClassGroupID      *string   `json:"class_group_id"`
		ClassSubGroupID   string    `json:"class_sub_group_id"`
		AcademicYearID    string    `json:"academic_year_id"`
		Semester          string    `json:"semester"`
		Status            string    `json:"status"`
		EffectiveFrom     *string   `json:"effective_from"`
		EffectiveTo       *string   `json:"effective_to"`
		Notes             string    `json:"notes"`
		CreatedAt         time.Time `json:"created_at"`
		UpdatedAt         time.Time `json:"updated_at"`
		ClassGroupName    string    `json:"class_group_name"`
		ClassSubGroupName string    `json:"class_sub_group_name"`
		AcademicYearName  string    `json:"academic_year_name"`
	}

	list := []TimetableItem{}
	for rows.Next() {
		var t TimetableItem
		var cgID sql.NullString
		var effFrom, effTo sql.NullTime
		var notes sql.NullString

		err := rows.Scan(
			&t.ID, &cgID, &t.ClassSubGroupID, &t.AcademicYearID,
			&t.Semester, &t.Status, &effFrom, &effTo, &notes,
			&t.CreatedAt, &t.UpdatedAt,
			&t.ClassGroupName, &t.ClassSubGroupName, &t.AcademicYearName,
		)
		if err != nil {
			continue
		}
		if cgID.Valid {
			t.ClassGroupID = &cgID.String
		}
		if effFrom.Valid {
			s := effFrom.Time.Format("2006-01-02")
			t.EffectiveFrom = &s
		}
		if effTo.Valid {
			s := effTo.Time.Format("2006-01-02")
			t.EffectiveTo = &s
		}
		t.Notes = notes.String

		list = append(list, t)
	}

	c.JSON(http.StatusOK, gin.H{"total": len(list), "timetables": list})
}

//POST /api/school-admin/timetables
func CreateTimetable(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := middleware.GetAdminIDFromUser(c)

	var req CreateTimetableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Validasi sub-kelas milik sekolah ini
	var exists bool
	err = database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM class_sub_groups WHERE id = $1 AND school_id = $2)
	`, req.ClassSubGroupID, schoolID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak valid"})
		return
	}

	var timetableID string
	err = database.DB.QueryRow(`
		INSERT INTO class_timetables (
			school_id, class_group_id, class_sub_group_id, academic_year_id,
			semester, curriculum_structure_id, status, effective_from, effective_to, notes, created_by
		) VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,'')::uuid, 'draft', 
				NULLIF($7,'')::date, NULLIF($8,'')::date, NULLIF($9,''), $10)
		RETURNING id
	`, schoolID, req.ClassGroupID, req.ClassSubGroupID, req.AcademicYearID,
		req.Semester, req.CurriculumStructureID, req.EffectiveFrom, req.EffectiveTo, req.Notes, adminID).Scan(&timetableID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jadwal: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Jadwal berhasil dibuat", "id": timetableID})
}

// GET /api/school-admin/timetables/:id
func GetTimetableDetail(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")

	// 1. Ambil header
	var t struct {
		ID              string
		ClassSubGroupID string
		AcademicYearID  string
		Semester        string
		Status          string
		Notes           sql.NullString
	}
	err = database.DB.QueryRow(`
		SELECT id, class_sub_group_id, academic_year_id, semester, status, notes
		FROM class_timetables WHERE id = $1 AND school_id = $2
	`, timetableID, schoolID).Scan(
		&t.ID, &t.ClassSubGroupID, &t.AcademicYearID,
		&t.Semester, &t.Status, &t.Notes,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
		return
	}

	// 2. Ambil entries
	rows, err := database.DB.Query(`
		SELECT 
			te.id, te.day_of_week, te.period_start, te.period_end,
			te.subject_id, te.subject_name_snapshot, te.teacher_id,
			COALESCE(t.nama_lengkap, '') AS teacher_name,
			COALESCE(te.room, ''), COALESCE(te.notes, '')
		FROM timetable_entries te
		LEFT JOIN teachers t ON te.teacher_id = t.id
		WHERE te.timetable_id = $1
		ORDER BY te.day_of_week ASC, te.period_start ASC
	`, timetableID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type EntryItem struct {
		ID                  string  `json:"id"`
		DayOfWeek           int     `json:"day_of_week"`
		PeriodStart         int     `json:"period_start"`
		PeriodEnd           int     `json:"period_end"`
		SubjectID           *string `json:"subject_id"`
		SubjectNameSnapshot string  `json:"subject_name_snapshot"`
		TeacherID           *string `json:"teacher_id"`
		TeacherName         string  `json:"teacher_name"`
		Room                string  `json:"room"`
		Notes               string  `json:"notes"`
	}

	entries := []EntryItem{}
	for rows.Next() {
		var e EntryItem
		var subID, teacherID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.DayOfWeek, &e.PeriodStart, &e.PeriodEnd,
			&subID, &e.SubjectNameSnapshot, &teacherID, &e.TeacherName,
			&e.Room, &e.Notes,
		); err == nil {
			if subID.Valid {
				e.SubjectID = &subID.String
			}
			if teacherID.Valid {
				e.TeacherID = &teacherID.String
			}
			entries = append(entries, e)
		}
	}

	// 3. Ambil time_slots DEFAULT
	type TimeSlotItem struct {
		PeriodNumber int    `json:"period_number"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		IsBreak      bool   `json:"is_break"`
		IsOverride   bool   `json:"is_override"`
	}

	defaultMap := map[int]TimeSlotItem{}

	defaultSlotRows, err := database.DB.Query(`
		SELECT period_number, start_time, end_time, is_break
		FROM school_time_slots
		WHERE school_id = $1
		ORDER BY period_number ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer defaultSlotRows.Close()

	for defaultSlotRows.Next() {
		var s TimeSlotItem
		var start, end time.Time
		if err := defaultSlotRows.Scan(&s.PeriodNumber, &start, &end, &s.IsBreak); err == nil {
			s.StartTime = start.Format("15:04")
			s.EndTime = end.Format("15:04")
			s.IsOverride = false
			defaultMap[s.PeriodNumber] = s
		}
	}

	// 4. Ambil OVERRIDE per hari
	type overrideKey struct {
		Day    int
		Period int
	}
	overrideMap := map[overrideKey]TimeSlotItem{}

	overrideRows, err := database.DB.Query(`
		SELECT day_of_week, period_number, start_time, end_time, is_break
		FROM school_time_slot_overrides
		WHERE school_id = $1
	`, schoolID)
	if err == nil {
		defer overrideRows.Close()
		for overrideRows.Next() {
			var day, period int
			var start, end time.Time
			var isBreak bool
			if err := overrideRows.Scan(&day, &period, &start, &end, &isBreak); err == nil {
				overrideMap[overrideKey{Day: day, Period: period}] = TimeSlotItem{
					PeriodNumber: period,
					StartTime:    start.Format("15:04"),
					EndTime:      end.Format("15:04"),
					IsBreak:      isBreak,
					IsOverride:   true,
				}
			}
		}
	}

	// 5. Bangun list default (flat, backward-compat)
	defaultList := []TimeSlotItem{}
	for period := 1; ; period++ {
		if s, ok := defaultMap[period]; ok {
			defaultList = append(defaultList, s)
		} else {
			if _, hasNext := defaultMap[period+1]; !hasNext {
				break
			}
		}
	}

	// 6. Bangun time_slots_by_day (merge override > default)
	timeSlotsByDay := map[string][]TimeSlotItem{}

	for day := 1; day <= 7; day++ {
		daySlots := []TimeSlotItem{}
		for period := 1; ; period++ {
			defSlot, hasDefault := defaultMap[period]
			ov, hasOverride := overrideMap[overrideKey{Day: day, Period: period}]

			if hasOverride {
				daySlots = append(daySlots, ov)
				continue
			}
			if hasDefault {
				daySlots = append(daySlots, defSlot)
				continue
			}

			_, hasNextDefault := defaultMap[period+1]
			_, hasNextOverride := overrideMap[overrideKey{Day: day, Period: period + 1}]
			if !hasNextDefault && !hasNextOverride {
				break
			}
		}
		timeSlotsByDay[strconv.Itoa(day)] = daySlots
	}

	c.JSON(http.StatusOK, gin.H{
		"timetable": gin.H{
			"id":                 t.ID,
			"class_sub_group_id": t.ClassSubGroupID,
			"academic_year_id":   t.AcademicYearID,
			"semester":           t.Semester,
			"status":             t.Status,
			"notes":              t.Notes.String,
		},
		"entries":           entries,
		"time_slots":        defaultList,
		"time_slots_by_day": timeSlotsByDay,
	})
}

// DELETE /api/school-admin/timetables/:id
func DeleteTimetable(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")

	result, err := database.DB.Exec(`
		DELETE FROM class_timetables 
		WHERE id = $1 AND school_id = $2
	`, timetableID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus jadwal: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan atau bukan milik sekolah Anda"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jadwal beserta seluruh baris pelajaran berhasil dihapus"})
}

// PATCH /api/school-admin/timetables/:id/activate
func ActivateTimetable(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")

	var currentStatus string
	err = database.DB.QueryRow(`
		SELECT status FROM class_timetables WHERE id = $1 AND school_id = $2
	`, timetableID, schoolID).Scan(&currentStatus)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
		return
	}

	if currentStatus == "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah aktif"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
		return
	}
	defer tx.Rollback()

	var subGroupID, academicYearID, semester string
	_ = tx.QueryRow(`
		SELECT class_sub_group_id, academic_year_id, semester 
		FROM class_timetables WHERE id = $1
	`, timetableID).Scan(&subGroupID, &academicYearID, &semester)

	// Arsipkan jadwal lain di sub-kelas dan tahun akademik yang sama
	_, err = tx.Exec(`
		UPDATE class_timetables 
		SET status = 'archived', updated_at = NOW()
		WHERE class_sub_group_id = $1 AND academic_year_id = $2 AND semester = $3 AND id <> $4
	`, subGroupID, academicYearID, semester, timetableID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengarsipkan jadwal lama: " + err.Error()})
		return
	}

	// Aktifkan jadwal ini
	_, err = tx.Exec(`
		UPDATE class_timetables 
		SET status = 'active', updated_at = NOW()
		WHERE id = $1
	`, timetableID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan jadwal: " + err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jadwal berhasil diaktifkan"})
}

// POST /api/school-admin/timetables/:id/entries
func CreateTimetableEntry(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")

	// Validasi timetable milik sekolah
	var exists bool
	_ = database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM class_timetables WHERE id = $1 AND school_id = $2)
	`, timetableID, schoolID).Scan(&exists)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
		return
	}

	var req CreateTimetableEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Ambil subject name untuk snapshot
	var subjectName string
	if req.SubjectID != "" {
		_ = database.DB.QueryRow(`SELECT name FROM subjects WHERE id = $1`, req.SubjectID).Scan(&subjectName)
	}

	var entryID string
	err = database.DB.QueryRow(`
		INSERT INTO timetable_entries (
			timetable_id, day_of_week, period_start, period_end,
			subject_id, subject_name_snapshot, teacher_id, room, notes
		) VALUES ($1, $2, $3, $4, NULLIF($5,'')::uuid, $6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,''))
		RETURNING id
	`, timetableID, req.DayOfWeek, req.PeriodStart, req.PeriodEnd,
		req.SubjectID, subjectName, req.TeacherID, req.Room, req.Notes).Scan(&entryID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan baris jadwal: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Baris jadwal berhasil ditambahkan", "id": entryID})
}

// DELETE /api/school-admin/timetables/:id/entries/:entryId
func DeleteTimetableEntry(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")
	entryID := c.Param("entryId")

	result, err := database.DB.Exec(`
		DELETE FROM timetable_entries 
		WHERE id = $1 AND timetable_id = $2 
		AND timetable_id IN (SELECT id FROM class_timetables WHERE school_id = $3)
	`, entryID, timetableID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Baris jadwal tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Baris jadwal berhasil dihapus"})
}

// VALIDATE TIMETABLE OST /api/school-admin/timetables/:id/validate
func ValidateTimetable(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	timetableID := c.Param("id")

	rows, err := database.DB.Query(`
		SELECT te.day_of_week, te.period_start, te.period_end, te.teacher_id, t.nama_lengkap
		FROM timetable_entries te
		JOIN teachers t ON te.teacher_id = t.id            
		WHERE te.timetable_id = $1 AND te.teacher_id IS NOT NULL
	`, timetableID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Conflict struct {
		Day     int    `json:"day"`
		Period  string `json:"period"`
		Teacher string `json:"teacher"`
		Message string `json:"message"`
	}
	conflicts := []Conflict{}

	for rows.Next() {
		var day, start, end int
		var teacherID, teacherName string
		if err := rows.Scan(&day, &start, &end, &teacherID, &teacherName); err != nil {
			continue
		}

		// Cari bentrok di timetable LAIN (sekolah sama)
		var conflictCount int
		_ = database.DB.QueryRow(`
			SELECT COUNT(*) FROM timetable_entries te2
			JOIN class_timetables ct2 ON te2.timetable_id = ct2.id
			WHERE ct2.school_id = $1
			AND te2.timetable_id <> $2
			AND te2.teacher_id = $3
			AND te2.day_of_week = $4
			AND te2.period_start <= $5
			AND te2.period_end >= $6
		`, schoolID, timetableID, teacherID, day, end, start).Scan(&conflictCount)

		if conflictCount > 0 {
			conflicts = append(conflicts, Conflict{
				Day:     day,
				Period:  fmt.Sprintf("%d-%d", start, end),
				Teacher: teacherName,
				Message: fmt.Sprintf("Guru %s bentrok dengan jadwal lain di jam ini", teacherName),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":     len(conflicts) == 0,
		"conflicts": conflicts,
	})
}