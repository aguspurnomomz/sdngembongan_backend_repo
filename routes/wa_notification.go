package routes

import (
	"database/sql"
	"strings"

	"gembongan_dev_backend/database"
)


func enqueueWANotification(
	schoolID, studentID, sessionID, recordID, eventType, attendanceStatus string,
) {
	
	validEvents := map[string]bool{
		"check_in":    true,
		"check_out":   true,
	}
	
	eventKey := eventType
	if eventType == "check_in" && attendanceStatus == "late" {
		eventKey = "late"
	}
	if eventType == "check_out" && attendanceStatus == "early_leave" {
		eventKey = "early_leave"
	}
	if !validEvents[eventType] && eventKey != "late" && eventKey != "early_leave" {
		return
	}

	var parentPhone, parentName, studentName, className string
	var optIn bool
	err := database.DB.QueryRow(`
		SELECT 
			COALESCE(s.parent_phone, ''), COALESCE(s.parent_name, ''),
			s.full_name, COALESCE(cg.name, ''),
			COALESCE(s.parent_whatsapp_opt_in, false)
		FROM students s
		LEFT JOIN class_groups cg ON s.class_group_id = cg.id
		WHERE s.id = $1
	`, studentID).Scan(&parentPhone, &parentName, &studentName, &className, &optIn)
	if err != nil || parentPhone == "" || !optIn {
		return
	}

	var templateBody string
	err = database.DB.QueryRow(`
		SELECT template_body FROM wa_notification_templates
		WHERE school_id = $1 AND event_type = $2 AND is_active = TRUE
	`, schoolID, eventKey).Scan(&templateBody)
	if err != nil {
		return
	}

	now := nowJakarta()
	msg := strings.NewReplacer(
		"{{nama}}", studentName,
		"{{kelas}}", className,
		"{{jam}}", now.Format("15:04"),
		"{{tanggal}}", now.Format("02 January 2006"),
		"{{status}}", attendanceStatus,
		"{{nama_orangtua}}", parentName,
	).Replace(templateBody)

	var recID interface{} = nil
	if recordID != "" {
		recID = recordID
	}
	_, _ = database.DB.Exec(`
		INSERT INTO wa_notification_queue 
			(school_id, student_id, session_id, attendance_record_id,
			 event_type, recipient_phone, message_body)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, schoolID, studentID, sessionID, recID, eventKey, parentPhone, msg)
}

var _ = sql.ErrNoRows