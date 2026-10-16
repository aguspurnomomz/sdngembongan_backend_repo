package teacher

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"os"

	"teach_partner_dev/database"
	"teach_partner_dev/routes/helpers"

	"github.com/gin-gonic/gin"
)



type AssignParticipantsRequest struct {
	PersonalStudentIDs []string `json:"personal_student_ids" binding:"required,min=1"`
	ExpiresAt          string   `json:"expires_at"` // opsional, ISO8601
}

type GenerateAccessCodesRequest struct {
	ParticipantIDs []string `json:"participant_ids"`
}

type ParticipantItem struct {
	ID                  string     `json:"id"`                   
	PersonalStudentID   string     `json:"personal_student_id"`
	FullName            string     `json:"full_name"`
	StudentNumber       string     `json:"student_number"`
	NISN                string     `json:"nisn"`
	Email               string     `json:"email"`
	AccessCode          *string    `json:"access_code"`
	AssignedAt          time.Time  `json:"assigned_at"`
	ExpiresAt           *time.Time `json:"expires_at"`
	ExamURL             *string    `json:"exam_url,omitempty"`      
	SubmissionStatus    *string    `json:"submission_status"`       
	SubmissionID        *string    `json:"submission_id"`
	Percentage          *float64   `json:"percentage"`
	IsPassed            *bool      `json:"is_passed"`
}


func generateAccessCode() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 8

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i := range bytes {
		bytes[i] = charset[int(bytes[i])%len(charset)]
	}
	return string(bytes), nil
}


func ListPersonalExamParticipants(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	var examExists string
	err = database.DB.QueryRow(`
		SELECT id FROM school_exams 
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&examExists)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			ep.id, ep.personal_student_id,
			ps.full_name,
			COALESCE(ps.student_number, ''),
			COALESCE(ps.nisn, ''),
			COALESCE(ps.email, ''),
			ep.access_code, ep.assigned_at, ep.expires_at,
			sub.id AS submission_id,
			sub.status AS submission_status,
			sub.percentage, sub.is_passed
		FROM exam_participants ep
		JOIN personal_students ps ON ps.id = ep.personal_student_id
		LEFT JOIN exam_submissions sub 
			ON sub.student_id = ep.personal_student_id 
			AND sub.exam_id = ep.exam_id
		WHERE ep.exam_id = $1
		ORDER BY ps.full_name ASC
	`, examID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat peserta: " + err.Error()})
		return
	}
	defer rows.Close()

	baseURL := getFrontendBaseURL(c)

	participants := []ParticipantItem{}
	for rows.Next() {
		var p ParticipantItem
		var accessCode, submissionID, submissionStatus sql.NullString
		var expiresAt sql.NullTime
		var percentage sql.NullFloat64
		var isPassed sql.NullBool

		err := rows.Scan(
			&p.ID, &p.PersonalStudentID,
			&p.FullName, &p.StudentNumber, &p.NISN, &p.Email,
			&accessCode, &p.AssignedAt, &expiresAt,
			&submissionID, &submissionStatus,
			&percentage, &isPassed,
		)
		if err != nil {
			continue
		}

		if accessCode.Valid {
			p.AccessCode = &accessCode.String
			url := fmt.Sprintf("%s/personal-exam?code=%s", baseURL, accessCode.String)
			p.ExamURL = &url
		}
		if expiresAt.Valid { p.ExpiresAt = &expiresAt.Time }
		if submissionID.Valid { p.SubmissionID = &submissionID.String }
		if submissionStatus.Valid {
			p.SubmissionStatus = &submissionStatus.String
		} else {
			s := "not_started"
			p.SubmissionStatus = &s
		}
		if percentage.Valid { p.Percentage = &percentage.Float64 }
		if isPassed.Valid { p.IsPassed = &isPassed.Bool }

		participants = append(participants, p)
	}

	// Hitung stats
	var withCode, withoutCode, submittedCount int
	for _, p := range participants {
		if p.AccessCode != nil {
			withCode++
		} else {
			withoutCode++
		}
		if p.SubmissionStatus != nil && *p.SubmissionStatus != "not_started" {
			submittedCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":         len(participants),
		"with_code":     withCode,
		"without_code":  withoutCode,
		"submitted":     submittedCount,
		"participants":  participants,
	})
}


func AssignPersonalExamParticipants(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	var examStatus string
	err = database.DB.QueryRow(`
		SELECT status FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&examStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	if examStatus == "archived" {
		c.JSON(http.StatusConflict, gin.H{"error": "Ujian sudah diarsipkan"})
		return
	}

	var req AssignParticipantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Parse expires_at (opsional)
	var expiresAt interface{} = nil
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format expires_at tidak valid (harus RFC3339)"})
			return
		}
		expiresAt = t
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()

	assigned := 0
	skipped := 0

	for _, studentID := range req.PersonalStudentIDs {
		var validStudent string
		err := tx.QueryRow(`
			SELECT id FROM personal_students
			WHERE id = $1 AND owner_user_id = $2 AND is_active = TRUE
		`, studentID, scope.UserID).Scan(&validStudent)
		if err != nil {
			skipped++
			continue
		}

		result, err := tx.Exec(`
			INSERT INTO exam_participants (
				exam_id, personal_student_id, owner_user_id, expires_at
			) VALUES ($1, $2, $3, $4)
			ON CONFLICT (exam_id, personal_student_id) DO NOTHING
		`, examID, studentID, scope.UserID, expiresAt)

		if err != nil {
			skipped++
			continue
		}

		rows, _ := result.RowsAffected()
		if rows > 0 {
			assigned++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  fmt.Sprintf("%d siswa berhasil di-assign", assigned),
		"assigned": assigned,
		"skipped":  skipped,
	})
}


func RemovePersonalExamParticipant(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")
	participantID := c.Param("participantId")

	var submissionID sql.NullString
	_ = database.DB.QueryRow(`
		SELECT sub.id FROM exam_submissions sub
		JOIN exam_participants ep ON ep.personal_student_id = sub.student_id AND ep.exam_id = sub.exam_id
		WHERE ep.id = $1
	`, participantID).Scan(&submissionID)

	if submissionID.Valid {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Tidak bisa remove: siswa sudah submit ujian ini",
		})
		return
	}

	result, err := database.DB.Exec(`
		DELETE FROM exam_participants
		WHERE id = $1 AND exam_id = $2 AND owner_user_id = $3
	`, participantID, examID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Peserta tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Peserta berhasil dihapus dari ujian"})
}


func GenerateAccessCodes(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	var req GenerateAccessCodesRequest
	_ = c.ShouldBindJSON(&req)

	// Validasi exam milik guru & sudah published
	var examStatus string
	err = database.DB.QueryRow(`
		SELECT status FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&examStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	if examStatus != "published" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ujian harus dipublish dulu sebelum bisa generate access code",
		})
		return
	}


	query := `
		SELECT ep.id FROM exam_participants ep
		WHERE ep.exam_id = $1 AND ep.access_code IS NULL
	`
	args := []any{examID}
	argIdx := 2

	if len(req.ParticipantIDs) > 0 {
		// Filter spesifik participant IDs
		placeholders := make([]string, len(req.ParticipantIDs))
		for i, pid := range req.ParticipantIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, pid)
			argIdx++
		}
		query += " AND ep.id IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query peserta: " + err.Error()})
		return
	}
	defer rows.Close()

	var participantIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err == nil {
			participantIDs = append(participantIDs, pid)
		}
	}

	if len(participantIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":   "Semua peserta sudah punya access code",
			"generated": 0,
		})
		return
	}

	// Generate kode untuk tiap participant (retry kalau collision)
	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()

	generated := 0
	for _, pid := range participantIDs {
		var code string
		var codeSet bool

		// Retry max 5x kalau collision (sangat jarang)
		for attempt := 0; attempt < 5; attempt++ {
			c, err := generateAccessCode()
			if err != nil {
				continue
			}

			// Coba update (unique constraint akan reject kalau collision)
			result, err := tx.Exec(`
				UPDATE exam_participants 
				SET access_code = $1
				WHERE id = $2 AND access_code IS NULL
			`, c, pid)

			if err != nil {
				// Kemungkinan unique violation, retry
				continue
			}

			rows, _ := result.RowsAffected()
			if rows > 0 {
				code = c
				codeSet = true
				break
			}
		}

		if codeSet {
			generated++
		} else {
			fmt.Printf("[WARN] Gagal generate code untuk participant %s\n", pid)
		}
		_ = code
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d access code berhasil dibuat", generated),
		"generated": generated,
		"total":     len(participantIDs),
	})
}


func RegenerateParticipantCode(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")
	participantID := c.Param("participantId")

	// Cek participant milik guru ini & belum submit
	var currentSubmission sql.NullString
	var currentCode sql.NullString

	err = database.DB.QueryRow(`
		SELECT 
			ep.access_code,
			(SELECT sub.id FROM exam_submissions sub 
			 WHERE sub.student_id = ep.personal_student_id AND sub.exam_id = ep.exam_id LIMIT 1)
		FROM exam_participants ep
		WHERE ep.id = $1 AND ep.exam_id = $2 AND ep.owner_user_id = $3
	`, participantID, examID, scope.UserID).Scan(&currentCode, &currentSubmission)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Peserta tidak ditemukan"})
		return
	}

	if currentSubmission.Valid {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Tidak bisa regenerate: siswa sudah submit ujian",
		})
		return
	}

	// Generate new code dengan retry
	var newCode string
	for attempt := 0; attempt < 5; attempt++ {
		c, err := generateAccessCode()
		if err != nil { continue }

		_, err = database.DB.Exec(`
			UPDATE exam_participants SET access_code = $1
			WHERE id = $2 AND owner_user_id = $3
		`, c, participantID, scope.UserID)

		if err == nil {
			newCode = c
			break
		}
	}

	if newCode == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate kode baru, coba lagi"})
		return
	}

	baseURL := getFrontendBaseURL(c)
	examURL := fmt.Sprintf("%s/personal-exam?code=%s", baseURL, newCode)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Access code berhasil di-regenerate",
		"access_code": newCode,
		"exam_url":    examURL,
	})
}


func GetPersonalExamQRSheet(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	// Ambil exam info
	var examTitle, examSubject string
	var totalQuestions int
	var durationMinutes int
	var examDate time.Time
	err = database.DB.QueryRow(`
		SELECT title, subject, total_questions, duration_minutes, created_at
		FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&examTitle, &examSubject, &totalQuestions, &durationMinutes, &examDate)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			ps.full_name,
			COALESCE(ps.student_number, ''),
			COALESCE(ps.nisn, ''),
			ep.access_code
		FROM exam_participants ep
		JOIN personal_students ps ON ps.id = ep.personal_student_id
		WHERE ep.exam_id = $1 AND ep.access_code IS NOT NULL
		ORDER BY ps.full_name ASC
	`, examID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type QREntry struct {
		FullName      string `json:"full_name"`
		StudentNumber string `json:"student_number"`
		NISN          string `json:"nisn"`
		AccessCode    string `json:"access_code"`
		ExamURL       string `json:"exam_url"`
	}

	baseURL := getFrontendBaseURL(c)

	entries := []QREntry{}
	for rows.Next() {
		var e QREntry
		if err := rows.Scan(&e.FullName, &e.StudentNumber, &e.NISN, &e.AccessCode); err == nil {
			e.ExamURL = fmt.Sprintf("%s/personal-exam?code=%s", baseURL, e.AccessCode)
			entries = append(entries, e)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"exam": gin.H{
			"id":               examID,
			"title":            examTitle,
			"subject":          examSubject,
			"total_questions":  totalQuestions,
			"duration_minutes": durationMinutes,
			"exam_date":        examDate.Format("2006-01-02"),
		},
		"total":   len(entries),
		"entries": entries,
	})
}

func getFrontendBaseURL(c *gin.Context) string {
	if base := strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL")); base != "" {
		return strings.TrimRight(base, "/")
	}
	if origin := c.GetHeader("Origin"); origin != "" {
		return strings.TrimRight(origin, "/")
	}
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

