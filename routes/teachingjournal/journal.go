package teachingjournal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"gembongan_dev_backend/database"
	"time"
	"github.com/lib/pq"

	"github.com/gin-gonic/gin"
)

// GET /api/teacher/journals/schedule?date=YYYY-MM-DD
func GetTeacherSchedule(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDStr := normalizeUserID(userID)

	journalDate := c.Query("date")
	if journalDate == "" {
		journalDate = nowJakarta().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", journalDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid (YYYY-MM-DD)"})
		return
	}
	isoDow := int(t.Weekday())
	if isoDow == 0 {
		isoDow = 7
	}

	teacherID, schoolID, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT
			te.id,
			ct.id,
			COALESCE(te.subject_id::text,''),
			COALESCE(te.subject_name_snapshot,''),
			COALESCE(csg.name,''),
			ct.class_group_id,
			ct.class_sub_group_id,
			te.period_start,
			te.period_end,
			COALESCE(to_char(ts.start_time,'HH24:MI'),''),
			COALESCE(to_char(ts.end_time,'HH24:MI'),''),
			COALESCE(te.room,''),
			tj.id,
			COALESCE(tj.status,'')
		FROM timetable_entries te
		JOIN class_timetables ct ON ct.id = te.timetable_id
		LEFT JOIN class_sub_groups csg ON csg.id = ct.class_sub_group_id
		LEFT JOIN school_time_slots ts
		       ON ts.school_id = ct.school_id AND ts.period_number = te.period_start
		LEFT JOIN teaching_journals tj
		       ON tj.timetable_entry_id = te.id
		      AND tj.journal_date = $1::date
		      AND tj.is_active = TRUE
		WHERE te.teacher_id = $2
		  AND ct.status = 'active'
		  AND te.day_of_week = $3
		ORDER BY te.period_start ASC
	`, journalDate, teacherID, isoDow)

	if err != nil {
		writeInternalError(c, "Gagal memuat jadwal", err)
		return
	}
	defer rows.Close()

	// Tambahkan struct StudentItem untuk menampung data siswa
	type StudentItem struct {
		ID       string `json:"id"`
		FullName string `json:"full_name"`
		NISN     string `json:"nisn"`
	}

	type SlotItem struct {
		TimetableEntryID string        `json:"timetable_entry_id"`
		TimetableID      string        `json:"timetable_id"`
		SubjectID        string        `json:"subject_id"`
		SubjectName      string        `json:"subject_name"`
		ClassName        string        `json:"class_name"`
		ClassGroupID     string        `json:"class_group_id"`
		ClassSubGroupID  string        `json:"class_sub_group_id"`
		PeriodStart      int           `json:"period_start"`
		PeriodEnd        int           `json:"period_end"`
		StartTime        string        `json:"start_time"`
		EndTime          string        `json:"end_time"`
		Room             string        `json:"room"`
		JournalID        string        `json:"journal_id"`
		JournalStatus    string        `json:"journal_status"`
		Students         []StudentItem `json:"students"` 
	}

	items := []SlotItem{}
	for rows.Next() {
		var s SlotItem
		var tjID, tjStatus sql.NullString
		if err := rows.Scan(
			&s.TimetableEntryID, &s.TimetableID,
			&s.SubjectID, &s.SubjectName, &s.ClassName,
			&s.ClassGroupID, &s.ClassSubGroupID,
			&s.PeriodStart, &s.PeriodEnd,
			&s.StartTime, &s.EndTime, &s.Room,
			&tjID, &tjStatus,
		); err == nil {
			if tjID.Valid {
				s.JournalID = tjID.String
				s.JournalStatus = tjStatus.String
			}

			s.Students = []StudentItem{} 
			if s.ClassSubGroupID != "" {
				studentRows, err := database.DB.Query(`
					SELECT id, full_name, COALESCE(nisn, '') 
					FROM students 
					WHERE class_sub_group_id = $1 AND is_active = TRUE
					ORDER BY full_name ASC
				`, s.ClassSubGroupID)

				if err == nil {
					for studentRows.Next() {
						var st StudentItem
						if err := studentRows.Scan(&st.ID, &st.FullName, &st.NISN); err == nil {
							s.Students = append(s.Students, st)
						}
					}
					studentRows.Close()
				}
			}

			items = append(items, s)
		}
	}

	// Info hari libur
	var isHoliday bool
	var holidayName sql.NullString
	_ = database.DB.QueryRow(`
		SELECT TRUE, name FROM academic_calendar
		WHERE school_id = $1 AND calendar_date = $2::date
		  AND (day_type = 'holiday' OR is_attendance_required = FALSE)
		LIMIT 1
	`, schoolID, journalDate).Scan(&isHoliday, &holidayName)

	c.JSON(http.StatusOK, gin.H{
		"date":         journalDate,
		"day_of_week":  isoDow,
		"school_id":    schoolID,
		"teacher_id":   teacherID,
		"is_holiday":   isHoliday,
		"holiday_name": holidayName.String,
		"slots":        items,
		"total":        len(items),
	})
}

// POST /api/teacher/journals
func CreateTeachingJournal(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDStr := normalizeUserID(userID)

	var req CreateJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	ctx, err := validateTeacherOwnsSlot(userIDStr, req.TimetableEntryID, req.JournalDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cek duplikat
	var existingID string
	err = database.DB.QueryRow(`
		SELECT id FROM teaching_journals
		WHERE timetable_entry_id = $1 AND journal_date = $2::date AND is_active = TRUE
	`, req.TimetableEntryID, req.JournalDate).Scan(&existingID)
	if err == nil && existingID != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "Jurnal untuk slot ini pada tanggal tersebut sudah ada",
			"journal_id": existingID,
		})
		return
	}

	actualStart, err := parseHHMM(req.ActualStartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	actualEnd, err := parseHHMM(req.ActualEndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attachmentsJSON, _ := json.Marshal(req.Attachments)
	if attachmentsJSON == nil {
		attachmentsJSON = []byte("[]")
	}
	tagsArr := req.Tags
	if tagsArr == nil {
		tagsArr = []string{}
	}

	dailyScores := req.DailyScores
	if len(dailyScores) == 0 { dailyScores = []byte("[]") }
	studentDevs := req.StudentDevelopments
	if len(studentDevs) == 0 { studentDevs = []byte("[]") }

	var journalID string
	err = database.DB.QueryRow(`
		INSERT INTO teaching_journals (
			school_id, teacher_id, user_id, academic_year_id,
			timetable_entry_id, timetable_id,
			class_group_id, class_sub_group_id, subject_id,
			subject_name_snapshot, class_name_snapshot,
			journal_date, day_of_week, period_start, period_end,
			scheduled_start_time, scheduled_end_time,
			actual_start_time, actual_end_time,
			topic, learning_objectives, method, media_used, activities, reflection,
			student_attendance_count, total_students,
			assignment_given, next_plan,
			attachments, tags,
			status,
			daily_scores, student_developments, obstacles, solutions, bk_follow_up
		) VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid,
			$5, $6,
			NULLIF($7,'')::uuid, $8, NULLIF($9,'')::uuid,
			$10, NULLIF($11,''),
			$12::date, $13, $14, $15,
			NULLIF($16,'')::time, NULLIF($17,'')::time,
			NULLIF($18,'')::time, NULLIF($19,'')::time,
			$20, NULLIF($21,''), NULLIF($22,''), NULLIF($23,''), NULLIF($24,''), NULLIF($25,''),
			$26, $27,
			NULLIF($28,''), NULLIF($29,''),
			$30::jsonb, $31,
			'draft',
			$32::jsonb, $33::jsonb, NULLIF($34,''), NULLIF($35,''), $36
		) RETURNING id
	`,
		ctx.SchoolID, ctx.TeacherID, userIDStr, ctx.AcademicYearID,
		ctx.TimetableEntryID, ctx.TimetableID,
		ctx.ClassGroupID, ctx.ClassSubGroupID, ctx.SubjectID,
		ctx.SubjectName, ctx.ClassName,
		req.JournalDate, ctx.DayOfWeek, ctx.PeriodStart, ctx.PeriodEnd,
		ctx.ScheduledStart, ctx.ScheduledEnd,
		actualStart, actualEnd,
		req.Topic, req.LearningObjectives, req.Method, req.MediaUsed, req.Activities, req.Reflection,
		req.StudentAttendanceCount, req.TotalStudents,
		req.AssignmentGiven, req.NextPlan,
		string(attachmentsJSON), tagsArr,
		string(dailyScores), string(studentDevs), req.Obstacles, req.Solutions, req.BkFollowUp,
	).Scan(&journalID)

	if err != nil {
		writeInternalError(c, "Gagal menyimpan jurnal", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Jurnal berhasil dibuat",
		"journal_id": journalID,
		"status":     "draft",
	})
}
// GET /api/teacher/journals?from=&to=&subject_id=&status=&q=
func ListTeacherJournals(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDStr := normalizeUserID(userID)

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	query := `
		SELECT id, journal_date, subject_name_snapshot, class_name_snapshot,
		       period_start, period_end, topic, method, status,
		       student_attendance_count, total_students,
		       submitted_at, verified_at, created_at
		FROM teaching_journals
		WHERE teacher_id = $1 AND is_active = TRUE
	`
	args := []any{teacherID}
	idx := 2

	if v := c.Query("from"); v != "" {
		query += fmt.Sprintf(" AND journal_date >= $%d::date", idx)
		args = append(args, v)
		idx++
	}
	if v := c.Query("to"); v != "" {
		query += fmt.Sprintf(" AND journal_date <= $%d::date", idx)
		args = append(args, v)
		idx++
	}
	if v := c.Query("subject_id"); v != "" {
		query += fmt.Sprintf(" AND subject_id = $%d::uuid", idx)
		args = append(args, v)
		idx++
	}
	if v := c.Query("status"); v != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v := c.Query("q"); v != "" {
		query += fmt.Sprintf(" AND (topic ILIKE $%d OR COALESCE(reflection,'') ILIKE $%d)", idx, idx)
		args = append(args, "%"+v+"%")
		idx++
	}

	query += " ORDER BY journal_date DESC, period_start ASC LIMIT 500"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		writeInternalError(c, "Gagal memuat daftar jurnal", err)
		return
	}
	defer rows.Close()

	type Item struct {
		ID               string     `json:"id"`
		JournalDate      string     `json:"journal_date"`
		SubjectName      string     `json:"subject_name"`
		ClassName        string     `json:"class_name"`
		PeriodStart      int        `json:"period_start"`
		PeriodEnd        int        `json:"period_end"`
		Topic            string     `json:"topic"`
		Method           string     `json:"method"`
		Status           string     `json:"status"`
		AttendanceCount  *int       `json:"student_attendance_count"`
		TotalStudents    *int       `json:"total_students"`
		SubmittedAt      *time.Time `json:"submitted_at"`
		VerifiedAt       *time.Time `json:"verified_at"`
		CreatedAt        time.Time  `json:"created_at"`
	}

	list := []Item{}
	for rows.Next() {
		var it Item
		var d time.Time
		var att, tot sql.NullInt64
		var sub, ver sql.NullTime
		if err := rows.Scan(
			&it.ID, &d, &it.SubjectName, &it.ClassName,
			&it.PeriodStart, &it.PeriodEnd, &it.Topic, &it.Method, &it.Status,
			&att, &tot, &sub, &ver, &it.CreatedAt,
		); err == nil {
			it.JournalDate = d.Format("2006-01-02")
			if att.Valid { v := int(att.Int64); it.AttendanceCount = &v }
			if tot.Valid { v := int(tot.Int64); it.TotalStudents = &v }
			if sub.Valid { it.SubmittedAt = &sub.Time }
			if ver.Valid { it.VerifiedAt = &ver.Time }
			list = append(list, it)
		}
	}

	c.JSON(http.StatusOK, gin.H{"journals": list, "total": len(list)})
}

// GET /api/teacher/journals/:id
func GetTeachingJournal(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)
	journalID := c.Param("id")

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var (
		j              = map[string]any{}
		journalDate    time.Time
		subjectName    sql.NullString
		className      sql.NullString
		topic          sql.NullString
		objectives     sql.NullString
		method         sql.NullString
		mediaUsed      sql.NullString
		activities     sql.NullString
		reflection     sql.NullString
		assignment     sql.NullString
		nextPlan       sql.NullString
		actualStart    sql.NullString
		actualEnd      sql.NullString
		schedStart     sql.NullString
		schedEnd       sql.NullString
		attendanceC    sql.NullInt64
		totalStudents  sql.NullInt64
		submittedAt    sql.NullTime
		verifiedAt     sql.NullTime
		verificationBy sql.NullString
		verificationNt sql.NullString
		
		// Tipe data aman untuk NULL
		attachmentsRaw sql.NullString 
		tagsRaw        pq.StringArray 
		
		status           sql.NullString
		createdAt        sql.NullTime
		updatedAt        sql.NullTime
		timetableEntryID sql.NullString
		classSubGroupID  sql.NullString
		subjectID        sql.NullString
		
		dailyScoresRaw sql.NullString
		studentDevsRaw sql.NullString
		obstacles      sql.NullString
		solutions      sql.NullString
		bkFollowUp     sql.NullBool

		periodStart	   sql.NullString
		periodEnd	   sql.NullString
	
	)

	err = database.DB.QueryRow(`
		SELECT journal_date, subject_name_snapshot, class_name_snapshot,
		       topic, learning_objectives, method, media_used, activities, reflection,
		       assignment_given, next_plan,
		       to_char(actual_start_time,'HH24:MI'),
		       to_char(actual_end_time,'HH24:MI'),
		       to_char(scheduled_start_time,'HH24:MI'),
		       to_char(scheduled_end_time,'HH24:MI'),
		       student_attendance_count, total_students,
		       period_start, period_end,
		       submitted_at, verified_at, verified_by, verification_note,
		       attachments, tags, status, created_at, updated_at,
		       timetable_entry_id, class_sub_group_id, subject_id,
		       daily_scores, student_developments, obstacles, solutions, bk_follow_up
		FROM teaching_journals
		WHERE id = $1 AND teacher_id = $2 AND is_active = TRUE
	`, journalID, teacherID).Scan(
		&journalDate, &subjectName, &className,
		&topic, &objectives, &method, &mediaUsed, &activities, &reflection,
		&assignment, &nextPlan,
		&actualStart, &actualEnd, &schedStart, &schedEnd,
		&attendanceC, &totalStudents,
		&periodStart, &periodEnd,
		&submittedAt, &verifiedAt, &verificationBy, &verificationNt,
		&attachmentsRaw, &tagsRaw, &status, &createdAt, &updatedAt,
		&timetableEntryID, &classSubGroupID, &subjectID,
		&dailyScoresRaw, &studentDevsRaw, &obstacles, &solutions, &bkFollowUp,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Jurnal tidak ditemukan"})
			return
		}
		writeInternalError(c, "Gagal memuat jurnal", err)
		return
	}

	// PARSING DATA JSON
	var attachments any = []any{}
	if attachmentsRaw.Valid && attachmentsRaw.String != "" {
		_ = json.Unmarshal([]byte(attachmentsRaw.String), &attachments)
	}

	var dailyScores []map[string]any
	if dailyScoresRaw.Valid && dailyScoresRaw.String != "" {
		_ = json.Unmarshal([]byte(dailyScoresRaw.String), &dailyScores)
	}
	
	var studentDevs []map[string]any
	if studentDevsRaw.Valid && studentDevsRaw.String != "" {
		_ = json.Unmarshal([]byte(studentDevsRaw.String), &studentDevs)
	}

	// AMBIL NAMA SISWA & SISIPKAN KE JSON
	studentIDs := make(map[string]bool)
	for _, ds := range dailyScores {
		if id, ok := ds["student_id"].(string); ok {
			studentIDs[id] = true
		}
	}
	for _, sd := range studentDevs {
		if id, ok := sd["student_id"].(string); ok {
			studentIDs[id] = true
		}
	}

	studentNames := make(map[string]string)
	if len(studentIDs) > 0 {
		ids := []string{}
		for id := range studentIDs {
			ids = append(ids, id)
		}
		
		query := `SELECT id, full_name FROM students WHERE id = ANY($1)`
		rows, err := database.DB.Query(query, pq.Array(ids))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err == nil {
					studentNames[id] = name
				}
			}
		}
	}

	for i, ds := range dailyScores {
		if id, ok := ds["student_id"].(string); ok {
			if name, exists := studentNames[id]; exists {
				dailyScores[i]["student_name"] = name
			} else {
				dailyScores[i]["student_name"] = "Siswa Tidak Dikenal"
			}
		}
	}
	for i, sd := range studentDevs {
		if id, ok := sd["student_id"].(string); ok {
			if name, exists := studentNames[id]; exists {
				studentDevs[i]["student_name"] = name
			} else {
				studentDevs[i]["student_name"] = "Siswa Tidak Dikenal"
			}
		}
	}

	// MAPPING KE JSON RESPONSE
	j["id"] = journalID
	j["journal_date"] = journalDate.Format("2006-01-02")
	j["subject_name"] = subjectName.String
	j["class_name"] = className.String
	j["topic"] = topic.String
	j["learning_objectives"] = objectives.String
	j["method"] = method.String
	j["media_used"] = mediaUsed.String
	j["activities"] = activities.String
	j["reflection"] = reflection.String
	j["assignment_given"] = assignment.String
	j["next_plan"] = nextPlan.String
	j["actual_start_time"] = actualStart.String
	j["actual_end_time"] = actualEnd.String
	j["scheduled_start_time"] = schedStart.String
	j["scheduled_end_time"] = schedEnd.String
	j["student_attendance_count"] = intOrNil(attendanceC)
	j["total_students"] = intOrNil(totalStudents)
	j["submitted_at"] = timeOrNil(submittedAt)
	j["verified_at"] = timeOrNil(verifiedAt)
	j["verified_by"] = nilIfEmpty(verificationBy.String)
	j["verification_note"] = verificationNt.String
	j["attachments"] = attachments
	j["tags"] = []string(tagsRaw)
	j["daily_scores"] = dailyScores
	j["student_developments"] = studentDevs
	j["obstacles"] = obstacles.String
	j["solutions"] = solutions.String
	j["bk_follow_up"] = bkFollowUp.Bool

	c.JSON(http.StatusOK, gin.H{"journal": j})
}

// PUT /api/teacher/journals/:id  (hanya draft / rejected)
func UpdateTeachingJournal(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)
	journalID := c.Param("id")

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var currentStatus string
	err = database.DB.QueryRow(`
		SELECT status FROM teaching_journals
		WHERE id = $1 AND teacher_id = $2 AND is_active = TRUE
	`, journalID, teacherID).Scan(&currentStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jurnal tidak ditemukan"})
		return
	}
	if currentStatus == "submitted" || currentStatus == "verified" {
		c.JSON(http.StatusConflict, gin.H{"error": "Jurnal sudah dikumpulkan/diverifikasi, tidak bisa diubah"})
		return
	}

	var req UpdateJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	actualStart, err := parseHHMM(req.ActualStartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	actualEnd, err := parseHHMM(req.ActualEndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attachmentsJSON := []byte("[]")
	if req.Attachments != nil {
		attachmentsJSON, _ = json.Marshal(req.Attachments)
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	dailyScores := req.DailyScores
	if len(dailyScores) == 0 { dailyScores = []byte("[]") }
	studentDevs := req.StudentDevelopments
	if len(studentDevs) == 0 { studentDevs = []byte("[]") }

	_, err = database.DB.Exec(`
		UPDATE teaching_journals SET
			actual_start_time = NULLIF($1,'')::time,
			actual_end_time   = NULLIF($2,'')::time,
			topic             = COALESCE(NULLIF($3,''), topic),
			learning_objectives = NULLIF($4,''),
			method            = NULLIF($5,''),
			media_used        = NULLIF($6,''),
			activities        = NULLIF($7,''),
			reflection        = NULLIF($8,''),
			student_attendance_count = $9,
			total_students    = $10,
			assignment_given  = NULLIF($11,''),
			next_plan         = NULLIF($12,''),
			attachments       = $13::jsonb,
			tags              = $14,
			daily_scores       = $15::jsonb,
			student_developments = $16::jsonb,
			obstacles          = NULLIF($17,''),
			solutions          = NULLIF($18,''),
			bk_follow_up       = $19,
			status            = CASE WHEN status='rejected' THEN 'draft' ELSE status END,
			updated_at        = NOW()
		WHERE id = $20 AND teacher_id = $21 AND is_active = TRUE
	`, actualStart, actualEnd, req.Topic, req.LearningObjectives,
		req.Method, req.MediaUsed, req.Activities, req.Reflection,
		req.StudentAttendanceCount, req.TotalStudents,
		req.AssignmentGiven, req.NextPlan,
		string(attachmentsJSON), tags,
		string(dailyScores), string(studentDevs), req.Obstacles, req.Solutions, req.BkFollowUp,
		journalID, teacherID,
	)

	if err != nil {
		writeInternalError(c, "Gagal memperbarui jurnal", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jurnal berhasil diperbarui"})
}

// POST /api/teacher/journals/:id/submit
func SubmitTeachingJournal(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)
	journalID := c.Param("id")

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE teaching_journals SET
			status = 'submitted',
			submitted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND teacher_id = $2 AND is_active = TRUE
		  AND status IN ('draft','rejected')
	`, journalID, teacherID)

	if err != nil {
		writeInternalError(c, "Gagal submit jurnal", err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Jurnal tidak bisa dikumpulkan (mungkin sudah submitted/verified)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jurnal berhasil dikumpulkan"})
}
// DELETE /api/teacher/journals/:id
func DeleteTeachingJournal(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)
	journalID := c.Param("id")

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE teaching_journals SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND teacher_id = $2 AND is_active = TRUE
		  AND status = 'draft'
	`, journalID, teacherID)

	if err != nil {
		writeInternalError(c, "Gagal menghapus jurnal", err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Hanya jurnal berstatus draft yang bisa dihapus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jurnal berhasil dihapus"})
}
// GET /api/teacher/journals/stats?academic_year_id=&semester=
func GetTeacherJournalStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)

	teacherID, _, err := resolveTeacherID(userIDStr, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	type StatusCount struct {
		Status string `json:"status"`
		Total  int    `json:"total"`
	}
	statusRows, err := database.DB.Query(`
		SELECT status, COUNT(*) FROM teaching_journals
		WHERE teacher_id = $1 AND is_active = TRUE
		GROUP BY status
	`, teacherID)
	if err != nil {
		writeInternalError(c, "Gagal hitung statistik", err)
		return
	}
	statusList := []StatusCount{}
	for statusRows.Next() {
		var s StatusCount
		if err := statusRows.Scan(&s.Status, &s.Total); err == nil {
			statusList = append(statusList, s)
		}
	}
	statusRows.Close()

	type MonthCount struct {
		Month string `json:"month"`
		Total int    `json:"total"`
	}
	monthRows, err := database.DB.Query(`
		SELECT to_char(journal_date,'YYYY-MM'), COUNT(*)
		FROM teaching_journals
		WHERE teacher_id = $1 AND is_active = TRUE
		GROUP BY 1
		ORDER BY 1 DESC
		LIMIT 12
	`, teacherID)
	if err != nil {
		writeInternalError(c, "Gagal hitung per bulan", err)
		return
	}
	monthList := []MonthCount{}
	for monthRows.Next() {
		var m MonthCount
		if err := monthRows.Scan(&m.Month, &m.Total); err == nil {
			monthList = append(monthList, m)
		}
	}
	monthRows.Close()

	c.JSON(http.StatusOK, gin.H{
		"by_status": statusList,
		"by_month":  monthList,
	})
}

func intOrNil(v sql.NullInt64) any {
	if v.Valid { return int(v.Int64) }
	return nil
}
func timeOrNil(v sql.NullTime) any {
	if v.Valid { return v.Time }
	return nil
}
func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" { return nil }
	return s
}