package exam

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// BLOCK Student — admin sekolah blokir siswa dari ujian
// POST /api/school-admin/exam-live-sessions/:sessionId/block
func StudentExamBlock(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			var req BlockStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Ambil session info + validasi milik sekolah ini
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah dalam status diblokir"})
				return
			}

			// 2. Update DB
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = true,
					blocked_at = NOW(),
					blocked_by = $1,
					blocked_reason = $2,
					unblocked_at = NULL,
					unblocked_by = NULL,
					last_activity_at = NOW()
				WHERE id = $3
			`, adminID, req.Reason, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal blokir: " + err.Error()})
				return
			}

			// 3. Broadcast via Supabase Realtime (async — jangan block response)
			go func() {
				event := map[string]any{
					"type":          "block",
					"student_id":    studentID,
					"session_token": sessionToken,
					"reason":        req.Reason,
					"blocked_by":    adminID,
					"blocked_at":    time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[BLOCK] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Siswa %s berhasil diblokir", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
				"reason":       req.Reason,
			})
}

// POST /api/school-admin/exam-live-sessions/:sessionId/unblock
func StudentExamOpenBlock(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			// 1. Validasi
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if !isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa tidak sedang diblokir"})
				return
			}

			// 2. Update DB
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = false,
					unblocked_at = NOW(),
					unblocked_by = $1,
					last_activity_at = NOW()
				WHERE id = $2
			`, adminID, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal unblock: " + err.Error()})
				return
			}

			// 3. Broadcast unblock
			go func() {
				event := map[string]any{
					"type":          "unblock",
					"student_id":    studentID,
					"session_token": sessionToken,
					"unblocked_by":  adminID,
					"unblocked_at":  time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[UNBLOCK] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Blokir untuk %s berhasil dicabut", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
			})
}

// WARN Student — kirim peringatan realtime
// POST /api/school-admin/exam-live-sessions/:sessionId/warn
func StudentExamWarnRealtime(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			var req WarnStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Validasi
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sedang diblokir, tidak bisa dikirim peringatan"})
				return
			}

			// 2. Update DB — track last warning
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					last_warning_at = NOW(),
					last_warning_message = $1,
					last_activity_at = NOW()
				WHERE id = $2
			`, req.Message, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan peringatan: " + err.Error()})
				return
			}

			// 3. Broadcast warning
			go func() {
				event := map[string]any{
					"type":          "warning",
					"student_id":    studentID,
					"session_token": sessionToken,
					"message":       req.Message,
					"sent_by":       adminID,
					"sent_at":       time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[WARN] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Peringatan terkirim ke %s", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
			})

}
