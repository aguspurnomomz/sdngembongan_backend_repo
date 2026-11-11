package attendance

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)


type RegisterRFIDRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	RFIDUID   string `json:"rfid_uid" binding:"required"`
	CardLabel string `json:"card_label"`
	Notes     string `json:"notes"`
}

type UpdateRFIDRequest struct {
	CardLabel string `json:"card_label"`
	IsActive  *bool  `json:"is_active"`
	Notes     string `json:"notes"`
}

type ScanRFIDRequest struct {
	SessionID  string `json:"session_id"`
	DeviceCode string `json:"device_code"`
	RFIDUID    string `json:"rfid_uid" binding:"required"`
	ForceMode  string `json:"force_mode"`
}

type BulkRegisterRFIDRequest struct {
	ClassSubGroupID string `json:"class_sub_group_id" binding:"required"`
	RFIDUID         string `json:"rfid_uid" binding:"required"`
}

type CreateDeviceRequest struct {
	DeviceName         string `json:"device_name" binding:"required"`
	LocationNote       string `json:"location_note"`
	DeviceType         string `json:"device_type"`
	DefaultSessionMode string `json:"default_session_mode"`
}


func normalizeRFIDUID(uid string) string {
	uid = strings.ToUpper(strings.TrimSpace(uid))
	uid = strings.ReplaceAll(uid, " ", "")
	uid = strings.ReplaceAll(uid, "-", "")
	uid = strings.ReplaceAll(uid, ":", "")
	return uid
}

type scanResult struct {
	Success      bool
	Mode         string
	Status       string
	StudentID    string
	StudentName  string
	StudentNISN  string
	SubClassName string
	PhotoURL     string
	RecordID     string
	ErrorCode    string
	ErrorMessage string
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func processAttendanceScan(
	schoolID, sessionID, scanSource, scanValue string,
	deviceID, deviceLabel string,
	adminID string,
	forceMode string,
) (*scanResult, error) {

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("gagal mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	var sessionStatus, shiftID string
	var sessionClassGroupID, sessionClassSubGroupID sql.NullString
	var requireCheckOut bool
	var ciStart, ciEnd, coStart, coEnd sql.NullString
	var autoCloseAt sql.NullTime

	err = tx.QueryRow(`
		SELECT 
			ases.status, ases.shift_id,
			ases.class_group_id, ases.class_sub_group_id,
			sh.check_in_start::text, sh.check_in_end::text,
			sh.require_check_out,
			sh.check_out_start::text, sh.check_out_end::text,
			ases.auto_close_at
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		WHERE ases.id = $1 AND ases.school_id = $2
		FOR UPDATE OF ases
	`, sessionID, schoolID).Scan(
		&sessionStatus, &shiftID,
		&sessionClassGroupID, &sessionClassSubGroupID,
		&ciStart, &ciEnd,
		&requireCheckOut, &coStart, &coEnd,
		&autoCloseAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return &scanResult{
				Success:      false,
				ErrorCode:    "session_not_found",
				ErrorMessage: "Sesi absensi tidak ditemukan",
			}, nil
		}
		return nil, fmt.Errorf("gagal ambil session: %w", err)
	}

	if sessionStatus != "open" && sessionStatus != "scheduled" {
		return &scanResult{
			Success:      false,
			ErrorCode:    "session_closed",
			ErrorMessage: "Sesi sudah ditutup",
		}, nil
	}

	if sessionStatus == "scheduled" && adminID != "" {
		_, err = tx.Exec(`
			UPDATE attendance_sessions 
			SET status = 'open', 
			    opened_at = COALESCE(opened_at, NOW()), 
			    opened_by = COALESCE(opened_by, $1)
			WHERE id = $2 AND status = 'scheduled'
		`, adminID, sessionID)
		if err != nil {
			return nil, fmt.Errorf("gagal auto-open session: %w", err)
		}
	}

	var studentID, studentName, studentNISN, photoURL string
	var studentClassGroupID, studentClassSubGroupID sql.NullString

	switch scanSource {
	case "qr":
		err = tx.QueryRow(`
			SELECT s.id, s.full_name, COALESCE(s.nisn,''),
			       s.class_group_id, s.class_sub_group_id
			FROM student_qr_codes sqc
			JOIN students s ON sqc.student_id = s.id
			WHERE sqc.qr_token = $1 AND sqc.school_id = $2 AND sqc.is_active = TRUE
		`, scanValue, schoolID).Scan(
			&studentID, &studentName, &studentNISN,
			&studentClassGroupID, &studentClassSubGroupID,
		)
		photoURL = ""
	case "rfid":
		err = tx.QueryRow(`
			SELECT s.id, s.full_name, COALESCE(s.nisn,''),
			       s.class_group_id, s.class_sub_group_id
			FROM student_rfid_cards src
			JOIN students s ON src.student_id = s.id
			WHERE src.rfid_uid = $1 AND src.school_id = $2 AND src.is_active = TRUE
		`, scanValue, schoolID).Scan(
			&studentID, &studentName, &studentNISN,
			&studentClassGroupID, &studentClassSubGroupID,
		)
		photoURL = ""
	default:
		return &scanResult{
			Success:      false,
			ErrorCode:    "invalid_source",
			ErrorMessage: "Sumber scan tidak valid",
		}, nil
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return &scanResult{
				Success:      false,
				ErrorCode:    "not_found",
				ErrorMessage: "Kartu tidak dikenal",
			}, nil
		}
		return nil, fmt.Errorf("gagal lookup student: %w", err)
	}

	if sessionClassGroupID.Valid && studentClassGroupID.Valid {
		if sessionClassGroupID.String != studentClassGroupID.String {
			return &scanResult{
				Success:      false,
				ErrorCode:    "wrong_class",
				ErrorMessage: "Siswa bukan bagian dari kelas sesi ini",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
	}
	if sessionClassSubGroupID.Valid && studentClassSubGroupID.Valid {
		if sessionClassSubGroupID.String != studentClassSubGroupID.String {
			return &scanResult{
				Success:      false,
				ErrorCode:    "wrong_class",
				ErrorMessage: "Siswa bukan bagian dari sub kelas sesi ini",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
	}

	var hasCheckIn, hasCheckOut bool
	_ = tx.QueryRow(`
		SELECT 
			EXISTS(SELECT 1 FROM attendance_records 
				WHERE session_id = $1 AND student_id = $2 AND record_type = 'check_in'),
			EXISTS(SELECT 1 FROM attendance_records 
				WHERE session_id = $1 AND student_id = $2 AND record_type = 'check_out')
	`, sessionID, studentID).Scan(&hasCheckIn, &hasCheckOut)

	// ==========================================
	// 1. TENTUKAN MODE (DARI RECORD, BUKAN JAM)
	// ==========================================
	var mode string
	if forceMode == "check_in" || forceMode == "check_out" {
		// Validasi force mode terhadap state
		if forceMode == "check_in" && hasCheckIn {
			return &scanResult{
				Success:      false,
				ErrorCode:    "duplicate",
				ErrorMessage: "Siswa sudah tercatat masuk. Tidak bisa check-in dua kali.",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		if forceMode == "check_out" && !hasCheckIn {
			return &scanResult{
				Success:      false,
				ErrorCode:    "invalid_force_mode",
				ErrorMessage: "Siswa belum tercatat masuk. Tidak bisa check-out.",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		if forceMode == "check_out" && hasCheckOut {
			return &scanResult{
				Success:      false,
				ErrorCode:    "duplicate",
				ErrorMessage: "Siswa sudah tercatat pulang di sesi ini.",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		mode = forceMode
	} else {
		// Logika otomatis berdasarkan record
		if !hasCheckIn {
			mode = "check_in"
		} else if requireCheckOut && !hasCheckOut {
			mode = "check_out"
		} else {
			errMsg := "Siswa sudah tercatat sebelumnya"
			if requireCheckOut && hasCheckOut {
				errMsg = "Siswa sudah tercatat masuk dan pulang di sesi ini"
			} else if !requireCheckOut && hasCheckIn {
				errMsg = "Siswa sudah tercatat masuk. Shift ini tidak memerlukan scan pulang."
			}
			return &scanResult{
				Success:      false,
				ErrorCode:    "duplicate",
				ErrorMessage: errMsg,
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
	}

	// ==========================================
	// 2. VALIDASI WINDOW (TOLAK DI LUAR JAM)
	// ==========================================
	now := nowJakarta()

	// Cek auto_close_at
	if autoCloseAt.Valid && now.After(autoCloseAt.Time) {
		return &scanResult{
			Success:      false,
			ErrorCode:    "session_expired",
			ErrorMessage: "Sesi absensi sudah melewati batas waktu. Hubungi guru piket.",
			StudentName:  studentName,
			StudentNISN:  studentNISN,
		}, nil
	}

	currentTimeStr := now.Format("15:04:05")
	currentMin := timeStrToMinutes(currentTimeStr)

	switch mode {
	case "check_in":
		if !ciStart.Valid || !ciEnd.Valid {
			return &scanResult{
				Success:      false,
				ErrorCode:    "invalid_shift_config",
				ErrorMessage: "Konfigurasi shift tidak lengkap",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		startMin := timeStrToMinutes(ciStart.String)
		endMin := timeStrToMinutes(ciEnd.String)

		if currentMin < startMin {
			return &scanResult{
				Success:      false,
				ErrorCode:    "too_early",
				ErrorMessage: fmt.Sprintf("Belum waktunya absen masuk. Absen dibuka pukul %s.", ciStart.String[:5]),
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		if currentMin > endMin {
			return &scanResult{
				Success:      false,
				ErrorCode:    "check_in_closed",
				ErrorMessage: fmt.Sprintf("Waktu absen masuk sudah berakhir pukul %s. Hubungi guru piket untuk absensi manual.", ciEnd.String[:5]),
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}

	case "check_out":
		if !coStart.Valid || !coEnd.Valid {
			return &scanResult{
				Success:      false,
				ErrorCode:    "no_check_out",
				ErrorMessage: "Shift ini tidak support check-out",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		startMin := timeStrToMinutes(coStart.String)
		endMin := timeStrToMinutes(coEnd.String)

		if currentMin < startMin {
			return &scanResult{
				Success:      false,
				ErrorCode:    "too_early_checkout",
				ErrorMessage: fmt.Sprintf("Belum waktunya absen pulang. Absen pulang dibuka pukul %s.", coStart.String[:5]),
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		if currentMin > endMin {
			return &scanResult{
				Success:      false,
				ErrorCode:    "check_out_closed",
				ErrorMessage: fmt.Sprintf("Waktu absen pulang sudah berakhir pukul %s. Hubungi guru piket untuk absensi manual.", coEnd.String[:5]),
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
	}

	// ==========================================
	// 3. HITUNG STATUS & SIMPAN
	// ==========================================
	currentTime := fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())

	var status string
	var deviation int

	if mode == "check_in" {
		status = computeCheckInStatusFromStrings(ciStart.String, ciEnd.String, currentTime)
		deviation = computeCheckInDeviation(ciStart.String, ciEnd.String, currentTime)
	} else {
		status = computeCheckOutStatusFromStrings(coStart.String, coEnd.String, currentTime)
	}

	var rfidUIDVal interface{} = nil
	if scanSource == "rfid" {
		rfidUIDVal = scanValue
	}
	var deviceIDVal, deviceLabelVal interface{} = nil, nil
	if deviceID != "" {
		deviceIDVal = deviceID
	}
	if deviceLabel != "" {
		deviceLabelVal = deviceLabel
	}

	var recordID string
	err = tx.QueryRow(`
		INSERT INTO attendance_records (
			session_id, student_id, school_id,
			record_type, status, scanned_at, scanned_by,
			scan_method, deviation_minutes,
			rfid_uid, device_id, device_label
		) VALUES ($1, $2, $3, $4, $5, NOW(), $6, $7, $8, $9, $10, $11)
		ON CONFLICT (session_id, student_id, record_type) DO NOTHING
		RETURNING id
	`, sessionID, studentID, schoolID,
		mode, status, nullIfEmptyStr(adminID),
		scanSource, deviation,
		rfidUIDVal, deviceIDVal, deviceLabelVal,
	).Scan(&recordID)

	if err != nil {
		if err == sql.ErrNoRows {
			return &scanResult{
				Success:      false,
				ErrorCode:    "duplicate",
				ErrorMessage: "Siswa sudah tercatat di sesi ini (concurrent scan)",
				StudentName:  studentName,
				StudentNISN:  studentNISN,
			}, nil
		}
		return nil, fmt.Errorf("gagal insert attendance record: %w", err)
	}

	if mode == "check_in" {
		_, err = tx.Exec(`
			UPDATE attendance_sessions 
			SET total_check_in = total_check_in + 1,
				total_on_time = CASE WHEN $2 IN ('on_time','early') 
					THEN total_on_time + 1 ELSE total_on_time END,
				total_late = CASE WHEN $2 IN ('late','very_late') 
					THEN total_late + 1 ELSE total_late END,
				updated_at = NOW()
			WHERE id = $1
		`, sessionID, status)
	} else {
		_, err = tx.Exec(`
			UPDATE attendance_sessions 
			SET total_check_out = total_check_out + 1,
				updated_at = NOW()
			WHERE id = $1
		`, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal update counter session: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO student_attendance_summary 
			(student_id, school_id, session_id, summary_date,
			 check_in_status, check_in_time,
			 check_out_status, check_out_time,
			 is_complete, is_absent)
		VALUES ($1, $2, $3, CURRENT_DATE,
			CASE WHEN $4 = 'check_in' THEN $5::text ELSE NULL END,
			CASE WHEN $4 = 'check_in' THEN $6::time ELSE NULL END,
			CASE WHEN $4 = 'check_out' THEN $5::text ELSE NULL END,
			CASE WHEN $4 = 'check_out' THEN $6::time ELSE NULL END,
			false, false)
		ON CONFLICT (student_id, session_id) DO UPDATE SET
			check_in_status = CASE WHEN $4 = 'check_in' 
				THEN $5 ELSE student_attendance_summary.check_in_status END,
			check_in_time = CASE WHEN $4 = 'check_in' 
				THEN $6::time ELSE student_attendance_summary.check_in_time END,
			check_out_status = CASE WHEN $4 = 'check_out' 
				THEN $5 ELSE student_attendance_summary.check_out_status END,
			check_out_time = CASE WHEN $4 = 'check_out' 
				THEN $6::time ELSE student_attendance_summary.check_out_time END,
			is_complete = CASE WHEN $4 = 'check_out' 
				THEN true ELSE student_attendance_summary.is_complete END,
			updated_at = NOW()
	`, studentID, schoolID, sessionID,
		mode, status, currentTime)
	if err != nil {
		return nil, fmt.Errorf("gagal upsert summary: %w", err)
	}

	if scanSource == "rfid" {
		_, err = tx.Exec(`
			UPDATE student_rfid_cards SET last_used_at = NOW()
			WHERE rfid_uid = $1 AND school_id = $2
		`, scanValue, schoolID)
		if err != nil {
			return nil, fmt.Errorf("gagal update last_used_at: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("gagal commit: %w", err)
	}

	go func(sid, stuid, sessid, recid, evType, st string) {
		defer func() { recover() }()
		enqueueWANotification(sid, stuid, sessid, recid, evType, st)
	}(schoolID, studentID, sessionID, recordID, mode, status)

	return &scanResult{
		Success:      true,
		Mode:         mode,
		Status:       status,
		StudentID:    studentID,
		StudentName:  studentName,
		StudentNISN:  studentNISN,
		SubClassName: studentClassSubGroupID.String,
		PhotoURL:     photoURL,
		RecordID:     recordID,
	}, nil
}


func nullIfEmptyStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func computeCheckInStatusFromStrings(startStr, endStr, currentStr string) string {
	start := timeStrToMinutes(startStr)
	end := timeStrToMinutes(endStr)
	cur := timeStrToMinutes(currentStr)

	if cur < start {
		return "early"
	}
	if cur > end {
		return "late"
	}
	rangeMin := end - start
	if rangeMin > 0 {
		firstThird := start + rangeMin/3
		if cur < firstThird {
			return "early"
		}
	}
	return "on_time"
}

func computeCheckInDeviation(startStr, endStr, currentStr string) int {
	start := timeStrToMinutes(startStr)
	end := timeStrToMinutes(endStr)
	cur := timeStrToMinutes(currentStr)
	if cur < start {
		return cur - start
	}
	if cur > end {
		return cur - end
	}
	return 0
}

func computeCheckOutStatusFromStrings(startStr, endStr, currentStr string) string {
	start := timeStrToMinutes(startStr)
	end := timeStrToMinutes(endStr)
	cur := timeStrToMinutes(currentStr)

	if cur < start {
		return "early_leave"
	}
	if cur > end {
		return "late_leave"
	}
	rangeMin := end - start
	if rangeMin > 0 {
		firstThird := start + rangeMin/3
		if cur < firstThird {
			return "early_leave"
		}
	}
	return "on_time_leave"
}

func timeStrToMinutes(t string) int {
	t = strings.TrimSpace(t)
	parts := strings.Split(t, ":")
	if len(parts) < 2 {
		return 0
	}
	h := 0
	m := 0
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	return h*60 + m
}

func HandleListRFIDCards(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	classSubGroupID := c.Query("sub_class_id")
	classGroupID := c.Query("class_group_id")
	search := c.Query("search")
	hasCard := c.Query("has_card") // "yes" | "no" | ""

	query := `
		SELECT 
			s.id AS student_id, s.full_name, COALESCE(s.nisn,''),
			COALESCE(s.student_number,''),
			COALESCE(csg.name,'') AS sub_class_name,
			COALESCE(cg.name,'') AS class_name,
			COALESCE(cg.level,'') AS class_level,
			src.id AS card_id,
			src.rfid_uid,
			COALESCE(src.card_label,'') AS card_label,
			src.is_active,
			src.registered_at,
			src.last_used_at
		FROM students s
		LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
		LEFT JOIN class_groups cg ON s.class_group_id = cg.id
		LEFT JOIN student_rfid_cards src 
			ON src.student_id = s.id AND src.is_active = TRUE
		WHERE s.school_id = $1 AND s.is_active = TRUE
	`
	args := []any{schoolID}
	idx := 2

	if classSubGroupID != "" {
		query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", idx)
		args = append(args, classSubGroupID)
		idx++
	}
	if classGroupID != "" {
		query += fmt.Sprintf(" AND s.class_group_id = $%d", idx)
		args = append(args, classGroupID)
		idx++
	}
	if search != "" {
		query += fmt.Sprintf(" AND (s.full_name ILIKE $%d OR s.nisn ILIKE $%d OR src.rfid_uid ILIKE $%d)",
			idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}
	// if hasCard == "yes" {
	// 	query += " AND src.id IS NOT NULL"
	// } else if hasCard == "no" {
	// 	query += " AND src.id IS NULL"
	// }
	switch hasCard {
	case "yes":
		query += " AND src.id IS NOT NULL"
	case "no":
		query += " AND src.id IS NULL"
	}

	query += " ORDER BY cg.level ASC, csg.name ASC, s.full_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Item struct {
		StudentID     string  `json:"student_id"`
		FullName      string  `json:"full_name"`
		NISN          string  `json:"nisn"`
		StudentNumber string  `json:"student_number"`
		SubClassName  string  `json:"sub_class_name"`
		ClassName     string  `json:"class_name"`
		ClassLevel    string  `json:"class_level"`
		CardID        *string `json:"card_id"`
		RFIDUID       *string `json:"rfid_uid"`
		CardLabel     string  `json:"card_label"`
		IsActive      *bool   `json:"is_active"`
		RegisteredAt  *string `json:"registered_at"`
		LastUsedAt    *string `json:"last_used_at"`
	}

	var list []Item
	var totalStudents, withCard, withoutCard, activeCards int

	for rows.Next() {
		var it Item
		var cardID, rfidUID sql.NullString
		var isActive sql.NullBool
		var registeredAt, lastUsedAt sql.NullTime

		err := rows.Scan(
			&it.StudentID, &it.FullName, &it.NISN, &it.StudentNumber,
			&it.SubClassName, &it.ClassName, &it.ClassLevel,
			&cardID, &rfidUID, &it.CardLabel,
			&isActive, &registeredAt, &lastUsedAt,
		)
		if err != nil {
			continue
		}

		totalStudents++
		if cardID.Valid {
			it.CardID = &cardID.String
			withCard++
			if isActive.Valid && isActive.Bool {
				activeCards++
			}
		} else {
			withoutCard++
		}
		if rfidUID.Valid {
			it.RFIDUID = &rfidUID.String
		}
		if isActive.Valid {
			it.IsActive = &isActive.Bool
		}
		if registeredAt.Valid {
			s := registeredAt.Time.Format("2006-01-02T15:04:05Z07:00")
			it.RegisteredAt = &s
		}
		if lastUsedAt.Valid {
			s := lastUsedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			it.LastUsedAt = &s
		}

		list = append(list, it)
	}

	if list == nil {
		list = []Item{}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_students": totalStudents,
		"with_card":      withCard,
		"without_card":   withoutCard,
		"active_cards":   activeCards,
		"students":       list,
	})
}

func HandleRegisterRFID(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req RegisterRFIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	uid := normalizeRFIDUID(req.RFIDUID)
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UID kartu tidak boleh kosong"})
		return
	}

	var existingStudentID string
	err = database.DB.QueryRow(`
		SELECT id FROM students WHERE id = $1 AND school_id = $2
	`, req.StudentID, schoolID).Scan(&existingStudentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan"})
		return
	}

	var existingCardID, existingStudentName string
	err = database.DB.QueryRow(`
		SELECT src.id, s.full_name
		FROM student_rfid_cards src
		JOIN students s ON src.student_id = s.id
		WHERE src.school_id = $1 AND src.rfid_uid = $2
	`, schoolID, uid).Scan(&existingCardID, &existingStudentName)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("UID kartu sudah terdaftar untuk %s", existingStudentName),
			"code":  "uid_taken",
		})
		return
	}

	var existingActiveID string
	_ = database.DB.QueryRow(`
		SELECT id FROM student_rfid_cards 
		WHERE student_id = $1 AND is_active = TRUE
	`, req.StudentID).Scan(&existingActiveID)
	if existingActiveID != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Siswa ini sudah punya kartu aktif. Nonaktifkan dulu sebelum daftar baru.",
			"code":  "student_has_active_card",
		})
		return
	}

	var cardID string
	err = database.DB.QueryRow(`
		INSERT INTO student_rfid_cards 
			(student_id, school_id, rfid_uid, card_label, notes, registered_by)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6)
		RETURNING id
	`, req.StudentID, schoolID, uid, req.CardLabel, req.Notes, adminID).Scan(&cardID)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			c.JSON(http.StatusConflict, gin.H{"error": "Kartu sudah terdaftar", "code": "uid_taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Kartu RFID berhasil didaftarkan",
		"card_id":  cardID,
		"rfid_uid": uid,
	})
}

func HandleBulkRegisterRFID(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req BulkRegisterRFIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	uid := normalizeRFIDUID(req.RFIDUID)
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UID tidak boleh kosong"})
		return
	}

	var existingID string
	_ = database.DB.QueryRow(`
		SELECT id FROM student_rfid_cards 
		WHERE school_id = $1 AND rfid_uid = $2
	`, schoolID, uid).Scan(&existingID)
	if existingID != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "UID kartu sudah dipakai",
			"code":         "uid_taken",
			"student_name": "-",
		})
		return
	}

	var studentID, studentName string
	err = database.DB.QueryRow(`
		SELECT s.id, s.full_name
		FROM students s
		LEFT JOIN student_rfid_cards src 
			ON src.student_id = s.id AND src.is_active = TRUE
		WHERE s.school_id = $1 
		  AND s.class_sub_group_id = $2
		  AND s.is_active = TRUE
		  AND src.id IS NULL
		ORDER BY s.full_name ASC
		LIMIT 1
	`, schoolID, req.ClassSubGroupID).Scan(&studentID, &studentName)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Semua siswa di kelas ini sudah punya kartu",
			"code":  "all_registered",
		})
		return
	}

	var cardID string
	err = database.DB.QueryRow(`
		INSERT INTO student_rfid_cards 
			(student_id, school_id, rfid_uid, registered_by)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, studentID, schoolID, uid, adminID).Scan(&cardID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":      true,
		"student_id":   studentID,
		"student_name": studentName,
		"card_id":      cardID,
		"rfid_uid":     uid,
	})
}

func HandleDeleteRFID(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE student_rfid_cards 
		SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND school_id = $2
	`, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kartu tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kartu berhasil dinonaktifkan"})
}

func HandleRevokeRFID(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		DELETE FROM student_rfid_cards WHERE id = $1 AND school_id = $2
	`, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kartu tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kartu dicabut, UID bisa didaftarkan ulang"})
}

func HandleScanRFID(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req ScanRFIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	uid := normalizeRFIDUID(req.RFIDUID)
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UID tidak boleh kosong"})
		return
	}

	sessionID := req.SessionID
	deviceLabel := ""

	if sessionID == "" {
		if req.DeviceCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id atau device_code wajib diisi"})
			return
		}
		var resErr error
		sessionID, deviceLabel, resErr = resolveActiveSessionForDevice(schoolID, req.DeviceCode)
		if resErr != nil {
			c.JSON(http.StatusConflict, gin.H{"error": resErr.Error(), "code": "no_active_session"})
			return
		}
	}

	result, procErr := processAttendanceScan(
		schoolID, sessionID, "rfid", uid,
		req.DeviceCode, deviceLabel, adminID, req.ForceMode,
	)
	if procErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": procErr.Error()})
		return
	}

	if !result.Success {
		status := http.StatusBadRequest
		switch result.ErrorCode {
		case "not_found":
			status = http.StatusNotFound
		case "duplicate":
			status = http.StatusConflict
		case "session_not_found":
			status = http.StatusNotFound
		case "session_closed":
			status = http.StatusConflict
		case "wrong_class", "no_check_out":
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{
			"success": false,
			"error":   result.ErrorMessage,
			"code":    result.ErrorCode,
			"student": gin.H{
				"full_name": result.StudentName,
				"nisn":      result.StudentNISN,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    result.Mode,
		"status":  result.Status,
		"student": gin.H{
			"id":             result.StudentID,
			"full_name":      result.StudentName,
			"nisn":           result.StudentNISN,
			"sub_class_name": result.SubClassName,
			"photo_url":      result.PhotoURL,
		},
		"record_id":  result.RecordID,
		"scanned_at": nowJakarta(),
	})
}

func resolveActiveSessionForDevice(schoolID, deviceCode string) (string, string, error) {
	var deviceID, deviceName, locationNote, sessionMode string
	err := database.DB.QueryRow(`
		SELECT id, device_name, COALESCE(location_note,''), default_session_mode
		FROM attendance_devices
		WHERE school_id = $1 AND device_code = $2 AND is_active = TRUE
	`, schoolID, deviceCode).Scan(&deviceID, &deviceName, &locationNote, &sessionMode)
	if err != nil {
		return "", "", fmt.Errorf("device tidak dikenal atau tidak aktif")
	}

	_, _ = database.DB.Exec(`
		UPDATE attendance_devices SET last_ping_at = NOW() WHERE id = $1
	`, deviceID)

	if sessionMode != "auto_shift" {
		return "", deviceName, fmt.Errorf("device ini dalam mode manual, session_id wajib diisi")
	}

	now := nowJakarta()
	nowTime := now.Format("15:04:05")


	var sessionID string
	err = database.DB.QueryRow(`
		SELECT ases.id
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		WHERE ases.school_id = $1
		  AND ases.session_date = CURRENT_DATE
		  AND ases.status IN ('scheduled', 'open')
		  AND $2::time BETWEEN sh.check_in_start AND sh.check_in_end
		ORDER BY
		  CASE WHEN ases.status = 'open' THEN 0 ELSE 1 END,
		  ases.created_at DESC
		LIMIT 1
	`, schoolID, nowTime).Scan(&sessionID)

	if err == nil && sessionID != "" {
		return sessionID, deviceName, nil
	}

	
	err = database.DB.QueryRow(`
		SELECT id FROM attendance_sessions
		WHERE school_id = $1
		  AND session_date = CURRENT_DATE
		  AND status = 'open'
		ORDER BY COALESCE(opened_at, created_at) DESC
		LIMIT 1
	`, schoolID).Scan(&sessionID)

	if err == nil && sessionID != "" {
		return sessionID, deviceName, nil
	}

	return "", deviceName, fmt.Errorf("tidak ada sesi absensi aktif saat ini")
}

func HandleListDevices(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, device_name, device_code, device_type,
		       COALESCE(location_note,''), default_session_mode,
		       is_active, last_ping_at, created_at
		FROM attendance_devices
		WHERE school_id = $1
		ORDER BY device_name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type DeviceItem struct {
		ID                 string     `json:"id"`
		DeviceName         string     `json:"device_name"`
		DeviceCode         string     `json:"device_code"`
		DeviceType         string     `json:"device_type"`
		LocationNote       string     `json:"location_note"`
		DefaultSessionMode string     `json:"default_session_mode"`
		IsActive           bool       `json:"is_active"`
		LastPingAt         *time.Time `json:"last_ping_at"`
		CreatedAt          time.Time  `json:"created_at"`
	}

	var list []DeviceItem
	for rows.Next() {
		var d DeviceItem
		var lastPing sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.DeviceName, &d.DeviceCode, &d.DeviceType,
			&d.LocationNote, &d.DefaultSessionMode,
			&d.IsActive, &lastPing, &d.CreatedAt,
		); err == nil {
			if lastPing.Valid {
				d.LastPingAt = &lastPing.Time
			}
			list = append(list, d)
		}
	}

	if list == nil {
		list = []DeviceItem{}
	}

	c.JSON(http.StatusOK, gin.H{"total": len(list), "devices": list})
}

func HandleCreateDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	deviceType := req.DeviceType
	if deviceType == "" {
		deviceType = "kiosk"
	}
	sessionMode := req.DefaultSessionMode
	if sessionMode == "" {
		sessionMode = "auto_shift"
	}

	base := strings.ToUpper(strings.TrimSpace(req.DeviceName))
	base = strings.ReplaceAll(base, " ", "-")
	var cleaned strings.Builder
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	deviceCode := cleaned.String()
	if deviceCode == "" {
		deviceCode = "DEVICE"
	}

	var existing string
	_ = database.DB.QueryRow(`
		SELECT device_code FROM attendance_devices WHERE device_code = $1
	`, deviceCode).Scan(&existing)
	if existing != "" {
		deviceCode = deviceCode + "-" + randString(4)
	}

	var deviceID string
	err = database.DB.QueryRow(`
		INSERT INTO attendance_devices
			(school_id, device_name, device_code, device_type,
			 location_note, default_session_mode, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7)
		RETURNING id
	`, schoolID, req.DeviceName, deviceCode, deviceType,
		req.LocationNote, sessionMode, adminID).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Device berhasil dibuat",
		"device_id":   deviceID,
		"device_code": deviceCode,
		"kiosk_url":   fmt.Sprintf("/kiosk/%s", deviceCode),
	})
}

func HandleUpdateDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		DeviceName         string `json:"device_name"`
		LocationNote       string `json:"location_note"`
		DefaultSessionMode string `json:"default_session_mode"`
		IsActive           *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
		return
	}

	_, err = database.DB.Exec(`
		UPDATE attendance_devices SET
			device_name = COALESCE(NULLIF($1,''), device_name),
			location_note = COALESCE(NULLIF($2,''), location_note),
			default_session_mode = COALESCE(NULLIF($3,''), default_session_mode),
			is_active = COALESCE($4, is_active),
			updated_at = NOW()
		WHERE id = $5 AND school_id = $6
	`, req.DeviceName, req.LocationNote, req.DefaultSessionMode,
		req.IsActive, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device berhasil diperbarui"})
}

func HandleDeleteDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		DELETE FROM attendance_devices WHERE id = $1 AND school_id = $2
	`, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device berhasil dihapus"})
}