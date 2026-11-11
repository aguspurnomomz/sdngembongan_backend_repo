package attendance

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type CreateAttendanceSessionRequest struct {
	ShiftID string `json:"shift_id" binding:"required"`
	ClassGroupID    string `json:"class_group_id"`
	ClassSubGroupID string `json:"class_sub_group_id"`
	Title       string `json:"title"`
	SessionDate string `json:"session_date"` 
	Notes       string `json:"notes"`
}

// GET /school-admin/attendance/sessions/active-today
func GetActiveTodaySession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	now := nowJakarta()
	today := now.Format("2006-01-02")
	currentTime := now.Format("15:04:05")

	var sessionID, title, shiftName, shiftCode, status string
	var classGroupName, classSubGroupName string
	var totalStudents, totalCheckIn, totalAbsent int

	err = database.DB.QueryRow(`
		SELECT ases.id, ases.title, ases.status, sh.name, sh.code,
			COALESCE(cg.name, ''), COALESCE(csg.name, ''),
			ases.total_students, ases.total_check_in, ases.total_absent
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		LEFT JOIN class_sub_groups csg ON ases.class_sub_group_id = csg.id
		WHERE ases.school_id = $1 AND ases.session_date = $2 AND ases.status = 'open'
		AND sh.is_active = TRUE AND $3::time >= sh.check_in_start AND $3::time <= COALESCE(sh.check_out_end, sh.check_in_end)
		AND (ases.auto_close_at IS NULL OR ases.auto_close_at > NOW())
		ORDER BY sh.check_in_start ASC LIMIT 1
	`, schoolID, today, currentTime).Scan(
		&sessionID, &title, &status, &shiftName, &shiftCode,
		&classGroupName, &classSubGroupName, &totalStudents, &totalCheckIn, &totalAbsent,
	)

	if err == nil && sessionID != "" {
		c.JSON(http.StatusOK, gin.H{"available": true, "session": gin.H{
			"id": sessionID, "title": title, "status": status, "shift_name": shiftName, "shift_code": shiftCode,
			"class_group_name": classGroupName, "class_sub_group_name": classSubGroupName,
			"total_students": totalStudents, "total_check_in": totalCheckIn, "total_absent": totalAbsent,
		}})
		return
	}

	// 2. Auto Create Session
	var shiftID, shiftName2, shiftCode2 string
	var requireCheckOut bool
	var checkInEnd, checkOutEnd sql.NullTime
	var autoCloseAfterIn, autoCloseAfterOut sql.NullInt64

	err = database.DB.QueryRow(`
		SELECT id, name, code, require_check_out, check_in_end, check_out_end,
			auto_close_minutes_after_check_in, auto_close_minutes_after_check_out
		FROM attendance_shifts
		WHERE school_id = $1 AND is_active = TRUE AND $2::time >= check_in_start AND $2::time <= COALESCE(check_out_end, check_in_end)
		AND (applicable_days IS NULL OR EXTRACT(DOW FROM $3::date)::int = ANY(applicable_days))
		ORDER BY is_default DESC, check_in_start ASC LIMIT 1
	`, schoolID, currentTime, today).Scan(
		&shiftID, &shiftName2, &shiftCode2, &requireCheckOut,
		&checkInEnd, &checkOutEnd, &autoCloseAfterIn, &autoCloseAfterOut,
	)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"available": false, "message": "Tidak ada sesi absensi aktif saat ini", "server_time": now.Format(time.RFC3339)})
		return
	}

	var autoCloseBase time.Time
	autoCloseMinutes := 60
	if requireCheckOut && checkOutEnd.Valid {
		autoCloseBase = time.Date(now.Year(), now.Month(), now.Day(), checkOutEnd.Time.Hour(), checkOutEnd.Time.Minute(), 0, 0, now.Location())
		if autoCloseAfterOut.Valid { autoCloseMinutes = int(autoCloseAfterOut.Int64) }
	} else {
		autoCloseBase = time.Date(now.Year(), now.Month(), now.Day(), checkInEnd.Time.Hour(), checkInEnd.Time.Minute(), 0, 0, now.Location())
		if autoCloseAfterIn.Valid { autoCloseMinutes = int(autoCloseAfterIn.Int64) }
	}
	autoCloseTime := autoCloseBase.Add(time.Duration(autoCloseMinutes) * time.Minute)

	var newSessionID string
	autoTitle := fmt.Sprintf("%s - %s", shiftName2, today)

	tx, err := database.DB.Begin()
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"}); return }
	defer tx.Rollback()

	err = tx.QueryRow(`
		SELECT id FROM attendance_sessions
		WHERE school_id = $1 AND shift_id = $2 AND session_date = $3 AND status IN ('scheduled', 'open') LIMIT 1
	`, schoolID, shiftID, today).Scan(&newSessionID)

	if err != nil && err != sql.ErrNoRows { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal cek sesi existing"}); return }

	if newSessionID == "" {
		var count int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&count)
		err = tx.QueryRow(`
			INSERT INTO attendance_sessions (school_id, shift_id, title, session_date, status, opened_at, auto_close_at, total_students)
			VALUES ($1, $2, $3, $4, 'open', NOW(), $5, $6) RETURNING id
		`, schoolID, shiftID, autoTitle, today, autoCloseTime, count).Scan(&newSessionID)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi otomatis: " + err.Error()}); return }
	}

	if err := tx.Commit(); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit transaksi"}); return }

	var count int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&count)

	c.JSON(http.StatusOK, gin.H{"available": true, "auto_created": true, "session": gin.H{
		"id": newSessionID, "title": autoTitle, "status": "open", "shift_name": shiftName2, "shift_code": shiftCode2,
		"total_students": count, "total_check_in": 0, "total_absent": 0,
	}, "server_time": now.Format(time.RFC3339)})
}

// GET /school-admin/attendance/sessions
func GetAttendanceSessions(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	dateFrom := c.Query("from")
	dateTo := c.Query("to")
	status := c.Query("status")
	limit := c.DefaultQuery("limit", "100")

	query := `
		SELECT ases.id, ases.title, ases.session_date, ases.status, ases.shift_id, 
			COALESCE(sh.name, '') AS shift_name, COALESCE(sh.code, '') AS shift_code,
			ases.class_group_id, COALESCE(cg.name, '') AS class_group_name,
			ases.class_sub_group_id, COALESCE(csg.name, '') AS class_sub_group_name,
			ases.total_students, ases.total_check_in, ases.total_check_out, ases.total_absent,
			ases.opened_at, ases.closed_at, ases.created_at
		FROM attendance_sessions ases
		LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		LEFT JOIN class_sub_groups csg ON ases.class_sub_group_id = csg.id
		WHERE ases.school_id = $1
	`
	args := []any{schoolID}
	argIdx := 2

	if dateFrom != "" { query += fmt.Sprintf(" AND ases.session_date >= $%d", argIdx); args = append(args, dateFrom); argIdx++ }
	if dateTo != "" { query += fmt.Sprintf(" AND ases.session_date <= $%d", argIdx); args = append(args, dateTo); argIdx++ }
	if status != "" { query += fmt.Sprintf(" AND ases.status = $%d", argIdx); args = append(args, status); argIdx++ }

	query += fmt.Sprintf(" ORDER BY ases.session_date DESC, ases.created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := database.DB.Query(query, args...)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat sesi: " + err.Error()}); return }
	defer rows.Close()

	type SessionItem struct {
		ID                 string     `json:"id"`
		Title              string     `json:"title"`
		SessionDate        string     `json:"session_date"`
		Status             string     `json:"status"`
		ShiftID            string     `json:"shift_id"`
		ShiftName          string     `json:"shift_name"`
		ShiftCode          string     `json:"shift_code"`
		ClassGroupID       *string    `json:"class_group_id"`
		ClassGroupName     string     `json:"class_group_name"`
		ClassSubGroupID    *string    `json:"class_sub_group_id"`
		ClassSubGroupName  string     `json:"class_sub_group_name"`
		TotalStudents      int        `json:"total_students"`
		TotalCheckIn       int        `json:"total_check_in"`
		TotalCheckOut      int        `json:"total_check_out"`
		TotalAbsent        int        `json:"total_absent"`
		OpenedAt           *time.Time `json:"opened_at"`
		ClosedAt           *time.Time `json:"closed_at"`
		CreatedAt          time.Time  `json:"created_at"`
	}

	var sessions []SessionItem
	for rows.Next() {
		var s SessionItem
		var dateVal time.Time
		var cgID, csgID sql.NullString
		var openedAt, closedAt sql.NullTime

		if err := rows.Scan(
			&s.ID, &s.Title, &dateVal, &s.Status, &s.ShiftID, &s.ShiftName, &s.ShiftCode,
			&cgID, &s.ClassGroupName, &csgID, &s.ClassSubGroupName,
			&s.TotalStudents, &s.TotalCheckIn, &s.TotalCheckOut, &s.TotalAbsent,
			&openedAt, &closedAt, &s.CreatedAt,
		); err == nil {
			s.SessionDate = dateVal.Format("2006-01-02")
			if cgID.Valid { s.ClassGroupID = &cgID.String }
			if csgID.Valid { s.ClassSubGroupID = &csgID.String }
			if openedAt.Valid { s.OpenedAt = &openedAt.Time }
			if closedAt.Valid { s.ClosedAt = &closedAt.Time }
			sessions = append(sessions, s)
		}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// GET /school-admin/attendance/sessions/today
func GetTodaySessions(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	rows, err := database.DB.Query(`
		SELECT ases.id, ases.title, ases.session_date, ases.status, ases.shift_id, COALESCE(sh.name, '') AS shift_name,
			ases.class_group_id, COALESCE(cg.name, '') AS class_group_name, ases.total_students, ases.total_check_in, ases.total_absent
		FROM attendance_sessions ases
		LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		WHERE ases.school_id = $1 AND ases.session_date = CURRENT_DATE AND ases.status IN ('scheduled', 'open')
		ORDER BY ases.created_at ASC
	`, schoolID)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	defer rows.Close()

	type TodayItem struct {
		ID              string  `json:"id"`
		Title           string  `json:"title"`
		SessionDate     string  `json:"session_date"`
		Status          string  `json:"status"`
		ShiftID         string  `json:"shift_id"`
		ShiftName       string  `json:"shift_name"`
		ClassGroupID    *string `json:"class_group_id"`
		ClassGroupName  string  `json:"class_group_name"`
		TotalStudents   int     `json:"total_students"`
		TotalCheckIn    int     `json:"total_check_in"`
		TotalAbsent     int     `json:"total_absent"`
	}

	var sessions []TodayItem
	for rows.Next() {
		var s TodayItem
		var dateVal time.Time
		var cgID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.Title, &dateVal, &s.Status, &s.ShiftID, &s.ShiftName,
			&cgID, &s.ClassGroupName, &s.TotalStudents, &s.TotalCheckIn, &s.TotalAbsent,
		); err == nil {
			s.SessionDate = dateVal.Format("2006-01-02")
			if cgID.Valid { s.ClassGroupID = &cgID.String }
			sessions = append(sessions, s)
		}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// GET /school-admin/attendance/sessions/:id
func GetSessionDetail(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	sessionID := c.Param("id")
	var s struct {
		ID, Title, Status, ShiftID, ShiftName, ShiftCode, ClassGroupName, ClassSubGroupName string
		SessionDate time.Time
		ClassGroupID, ClassSubGroupID sql.NullString
		TotalStudents, TotalCheckIn, TotalCheckOut, TotalAbsent int
		OpenedAt, ClosedAt sql.NullTime
	}

	err = database.DB.QueryRow(`
		SELECT ases.id, ases.title, ases.session_date, ases.status, ases.shift_id, COALESCE(sh.name, ''), COALESCE(sh.code, ''),
			ases.class_group_id, COALESCE(cg.name, ''), ases.class_sub_group_id, COALESCE(csg.name, ''),
			ases.total_students, ases.total_check_in, ases.total_check_out, ases.total_absent, ases.opened_at, ases.closed_at
		FROM attendance_sessions ases
		LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		LEFT JOIN class_sub_groups csg ON ases.class_sub_group_id = csg.id
		WHERE ases.id = $1 AND ases.school_id = $2
	`, sessionID, schoolID).Scan(
		&s.ID, &s.Title, &s.SessionDate, &s.Status, &s.ShiftID, &s.ShiftName, &s.ShiftCode,
		&s.ClassGroupID, &s.ClassGroupName, &s.ClassSubGroupID, &s.ClassSubGroupName,
		&s.TotalStudents, &s.TotalCheckIn, &s.TotalCheckOut, &s.TotalAbsent, &s.OpenedAt, &s.ClosedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows { c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"}); return }
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return
	}

	result := gin.H{
		"id": s.ID, "title": s.Title, "session_date": s.SessionDate.Format("2006-01-02"), "status": s.Status,
		"shift_id": s.ShiftID, "shift_name": s.ShiftName, "shift_code": s.ShiftCode,
		"class_group_name": s.ClassGroupName, "class_sub_group_name": s.ClassSubGroupName,
		"total_students": s.TotalStudents, "total_check_in": s.TotalCheckIn, "total_check_out": s.TotalCheckOut, "total_absent": s.TotalAbsent,
	}
	if s.ClassGroupID.Valid { result["class_group_id"] = s.ClassGroupID.String }
	if s.ClassSubGroupID.Valid { result["class_sub_group_id"] = s.ClassSubGroupID.String }
	if s.OpenedAt.Valid { result["opened_at"] = s.OpenedAt.Time }
	if s.ClosedAt.Valid { result["closed_at"] = s.ClosedAt.Time }

	c.JSON(http.StatusOK, gin.H{"session": result})
}

// POST /school-admin/attendance/sessions
func CreateAttendanceSession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)

	var req CreateAttendanceSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()}); return }

	var shiftName, shiftCode string
	var checkInStart, checkInEnd time.Time
	var requireCheckOut bool
	var shiftConfigJSON []byte
	var checkOutEnd sql.NullTime
	var autoCloseAfterCheckOut sql.NullInt64

	err = database.DB.QueryRow(`
		SELECT name, code, check_in_start, check_in_end, require_check_out, check_out_end, auto_close_minutes_after_check_out, row_to_json(attendance_shifts)
		FROM attendance_shifts WHERE id = $1 AND school_id = $2
	`, req.ShiftID, schoolID).Scan(
		&shiftName, &shiftCode, &checkInStart, &checkInEnd, &requireCheckOut, &checkOutEnd, &autoCloseAfterCheckOut, &shiftConfigJSON,
	)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Shift tidak ditemukan"}); return }

	sessionDate := req.SessionDate
	if sessionDate == "" { sessionDate = nowJakarta().Format("2006-01-02") }

	title := req.Title
	if title == "" {
		className := "Semua Kelas"
		if req.ClassSubGroupID != "" {
			var subName string
			_ = database.DB.QueryRow(`SELECT name FROM class_sub_groups WHERE id = $1`, req.ClassSubGroupID).Scan(&subName)
			if subName != "" { className = subName }
		} else if req.ClassGroupID != "" {
			var cgName string
			_ = database.DB.QueryRow(`SELECT name FROM class_groups WHERE id = $1`, req.ClassGroupID).Scan(&cgName)
			if cgName != "" { className = cgName }
		}
		title = fmt.Sprintf("%s - %s (%s)", shiftName, className, sessionDate)
	}

	countQuery := `SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`
	countArgs := []any{schoolID}
	countIdx := 2

	var classGroupID, classSubGroupID interface{} = nil, nil
	if req.ClassGroupID != "" {
		classGroupID = req.ClassGroupID
		countQuery += fmt.Sprintf(" AND class_group_id = $%d", countIdx)
		countArgs = append(countArgs, req.ClassGroupID)
		countIdx++
	}
	if req.ClassSubGroupID != "" {
		classSubGroupID = req.ClassSubGroupID
		countQuery += fmt.Sprintf(" AND class_sub_group_id = $%d", countIdx)
		countArgs = append(countArgs, req.ClassSubGroupID)
		countIdx++
	}

	var totalStudents int
	_ = database.DB.QueryRow(countQuery, countArgs...).Scan(&totalStudents)

	now := nowJakarta()
	var autoCloseBase time.Time
	autoCloseMinutes := 60

	if requireCheckOut && checkOutEnd.Valid {
		autoCloseBase = time.Date(now.Year(), now.Month(), now.Day(), checkOutEnd.Time.Hour(), checkOutEnd.Time.Minute(), 0, 0, now.Location())
		if autoCloseAfterCheckOut.Valid { autoCloseMinutes = int(autoCloseAfterCheckOut.Int64) }
	} else {
		autoCloseBase = time.Date(now.Year(), now.Month(), now.Day(), checkInEnd.Hour(), checkInEnd.Minute(), 0, 0, now.Location())
		var acmIn sql.NullInt64
		_ = database.DB.QueryRow(`SELECT auto_close_minutes_after_check_in FROM attendance_shifts WHERE id = $1`, req.ShiftID).Scan(&acmIn)
		if acmIn.Valid { autoCloseMinutes = int(acmIn.Int64) }
	}
	autoCloseTime := autoCloseBase.Add(time.Duration(autoCloseMinutes) * time.Minute)

	var existingSessionID, existingStatus string
	err = database.DB.QueryRow(`
		SELECT id, status FROM attendance_sessions
		WHERE school_id = $1 AND shift_id = $2 AND session_date = $3 AND status IN ('scheduled', 'open') LIMIT 1
	`, schoolID, req.ShiftID, sessionDate).Scan(&existingSessionID, &existingStatus)

	if err == nil && existingSessionID != "" {
		c.JSON(http.StatusOK, gin.H{"message": "Sesi untuk shift dan tanggal ini sudah ada", "id": existingSessionID, "session_id": existingSessionID, "status": existingStatus, "already_exists": true})
		return
	}

	if err != nil && err != sql.ErrNoRows { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal cek sesi existing: " + err.Error()}); return }

	var sessionID string
	err = database.DB.QueryRow(`
		INSERT INTO attendance_sessions (
			school_id, shift_id, class_group_id, class_sub_group_id, title, session_date, shift_config_snapshot,
			status, opened_by, auto_close_at, total_students, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'scheduled', $8, $9, $10, NULLIF($11, '')) RETURNING id
	`, schoolID, req.ShiftID, classGroupID, classSubGroupID, title, sessionDate, shiftConfigJSON, adminID, autoCloseTime, totalStudents, req.Notes).Scan(&sessionID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi: " + err.Error()}); return }
	c.JSON(http.StatusCreated, gin.H{"message": "Sesi berhasil dibuat", "id": sessionID, "session_id": sessionID, "already_exists": false})
}

// POST /school-admin/attendance/sessions/:id/open
func OpenSession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	sessionID := c.Param("id")
	adminID, _ := getAdminIDFromUser(c)

	result, err := database.DB.Exec(`
		UPDATE attendance_sessions SET status = 'open', opened_at = COALESCE(opened_at, NOW()), opened_by = COALESCE(opened_by, $1), updated_at = NOW()
		WHERE id = $2 AND school_id = $3 AND status IN ('scheduled', 'open')
	`, adminID, sessionID, schoolID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	affected, _ := result.RowsAffected()
	if affected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan atau sudah ditutup"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "Sesi berhasil dibuka"})
}

// POST /school-admin/attendance/sessions/:id/close
func CloseSession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	sessionID := c.Param("id")
	adminID, _ := getAdminIDFromUser(c)

	var sStatus string
	err = database.DB.QueryRow(`SELECT status FROM attendance_sessions WHERE id = $1 AND school_id = $2`, sessionID, schoolID).Scan(&sStatus)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"}); return }

	if sStatus == "closed" || sStatus == "auto_closed" { c.JSON(http.StatusConflict, gin.H{"error": "Sesi sudah ditutup"}); return }

	var absentCount int
	err = database.DB.QueryRow(`SELECT auto_mark_absent_for_session($1)`, sessionID).Scan(&absentCount)
	if err != nil { absentCount = 0 }

	_, _ = database.DB.Exec(`
		UPDATE attendance_sessions SET status = 'closed', closed_by = $1, closed_at = NOW(), updated_at = NOW() WHERE id = $2
	`, adminID, sessionID)

	_, _ = database.DB.Exec(`SELECT recalculate_session_stats($1)`, sessionID)

	c.JSON(http.StatusOK, gin.H{"message": "Sesi berhasil ditutup", "absent_count": absentCount})
}

// DELETE /school-admin/attendance/sessions/:id
func DeleteSession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	result, err := database.DB.Exec(`DELETE FROM attendance_sessions WHERE id = $1 AND school_id = $2`, c.Param("id"), schoolID)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }

	affected, _ := result.RowsAffected()
	if affected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "Sesi berhasil dihapus"})
}

// GET /school-admin/attendance/sessions/:id/records
func GetSessionRecords(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	sessionID := c.Param("id")
	var classGroupID, classSubGroupID sql.NullString
	err = database.DB.QueryRow(`SELECT class_group_id, class_sub_group_id FROM attendance_sessions WHERE id = $1 AND school_id = $2`, sessionID, schoolID).Scan(&classGroupID, &classSubGroupID)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"}); return }

	query := `
		SELECT s.id AS student_id, s.full_name AS student_name, COALESCE(s.nisn, '') AS nisn, COALESCE(csg.name, '') AS sub_class_name,
			ci.status AS check_in_status, ci.scanned_at AS check_in_at, co.status AS check_out_status, co.scanned_at AS check_out_at,
			COALESCE(ci.status = 'absent', false) AS is_absent
		FROM students s
		LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
		LEFT JOIN attendance_records ci ON ci.student_id = s.id AND ci.session_id = $1 AND ci.record_type = 'check_in'
		LEFT JOIN attendance_records co ON co.student_id = s.id AND co.session_id = $1 AND co.record_type = 'check_out'
		WHERE s.school_id = $2 AND s.is_active = TRUE
	`
	args := []any{sessionID, schoolID}
	argIdx := 3

	if classGroupID.Valid { query += fmt.Sprintf(" AND s.class_group_id = $%d", argIdx); args = append(args, classGroupID.String); argIdx++ }
	if classSubGroupID.Valid { query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx); args = append(args, classSubGroupID.String); argIdx++ }
	query += " ORDER BY s.full_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query records: " + err.Error()}); return }
	defer rows.Close()

	type RecordItem struct {
		StudentID        string  `json:"student_id"`
		StudentName      string  `json:"student_name"`
		NISN             string  `json:"nisn"`
		SubClassName     string  `json:"sub_class_name"`
		CheckInStatus    *string `json:"check_in_status"`
		CheckInTime      *string `json:"check_in_time"`
		CheckOutStatus   *string `json:"check_out_status"`
		CheckOutTime     *string `json:"check_out_time"`
		IsAbsent         bool    `json:"is_absent"`
	}

	var records []RecordItem
	for rows.Next() {
		var r RecordItem
		var ciStatus, coStatus sql.NullString
		var ciAt, coAt sql.NullTime

		if err := rows.Scan(
			&r.StudentID, &r.StudentName, &r.NISN, &r.SubClassName, &ciStatus, &ciAt, &coStatus, &coAt, &r.IsAbsent,
		); err == nil {
			if ciStatus.Valid { r.CheckInStatus = &ciStatus.String }
			if coStatus.Valid { r.CheckOutStatus = &coStatus.String }
			if ciAt.Valid { t := ciAt.Time.Format("15:04"); r.CheckInTime = &t }
			if coAt.Valid { t := coAt.Time.Format("15:04"); r.CheckOutTime = &t }
			records = append(records, r)
		}
	}
	c.JSON(http.StatusOK, gin.H{"records": records, "total": len(records)})
}