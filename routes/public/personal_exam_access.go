package public

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"teach_partner_dev/database"
	"github.com/gin-gonic/gin"
)


type StartPersonalExamRequest struct {
	AccessCode string `json:"access_code" binding:"required"`
}

type SubmitPersonalExamRequest struct {
	SessionToken     string            `json:"session_token" binding:"required"`
	Answers          map[string]string `json:"answers" binding:"required"`
	TabSwitchCount   int               `json:"tab_switch_count"`
	TimeSpentSeconds int               `json:"time_spent_seconds"`
}


func GetPersonalExamByCode(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access code tidak valid"})
		return
	}

	var (
		participantID      string
		examID             string
		examTitle          string
		examSubject        string
		examDescription    sql.NullString
		durationMinutes    int
		totalQuestions     int
		passingScore       sql.NullFloat64
		examStatus         string
		studentName        string
		studentNumber      sql.NullString
		expiresAt          sql.NullTime
		submissionID       sql.NullString
	)

	err := database.DB.QueryRow(`
		SELECT 
			ep.id, ep.exam_id,
			e.title, e.subject, e.description,
			e.duration_minutes, e.total_questions, e.passing_score,
			e.status,
			ps.full_name, ps.student_number,
			ep.expires_at,
			(SELECT sub.id FROM exam_submissions sub 
			 WHERE sub.student_id = ep.personal_student_id 
			   AND sub.exam_id = ep.exam_id LIMIT 1)
		FROM exam_participants ep
		JOIN school_exams e ON e.id = ep.exam_id
		JOIN personal_students ps ON ps.id = ep.personal_student_id
		WHERE ep.access_code = $1 
		  AND e.deleted_at IS NULL
	`, code).Scan(
		&participantID, &examID,
		&examTitle, &examSubject, &examDescription,
		&durationMinutes, &totalQuestions, &passingScore,
		&examStatus,
		&studentName, &studentNumber,
		&expiresAt,
		&submissionID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Access code tidak valid atau sudah tidak berlaku",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if examStatus != "published" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ujian belum dipublish atau sudah diarsipkan",
		})
		return
	}

	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		c.JSON(http.StatusGone, gin.H{
			"error": "Access code sudah kadaluarsa. Hubungi guru Anda.",
		})
		return
	}


	if submissionID.Valid {
		c.JSON(http.StatusConflict, gin.H{
			"error":          "Anda sudah mengumpulkan ujian ini",
			"has_submitted":  true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":             true,
		"participant_id":    participantID,
		"exam_id":           examID,
		"exam_title":        examTitle,
		"exam_subject":      examSubject,
		"exam_description":  examDescription.String,
		"duration_minutes":  durationMinutes,
		"total_questions":   totalQuestions,
		"passing_score":     passingScore.Float64,
		"student_name":      studentName,
		"student_number":    studentNumber.String,
		"has_submitted":     false,
	})
}


func StartPersonalExam(c *gin.Context) {
	var req StartPersonalExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.AccessCode))

	var (
		participantID      string
		examID             string
		studentID          string
		studentName        string
		durationMinutes    int
		totalQuestions     int
		passingScore       sql.NullFloat64
		examStatus         string
		questionsJSON      []byte
		scoringConfig      []byte
		expiresAt          sql.NullTime
		submissionID       sql.NullString
	)

	err := database.DB.QueryRow(`
		SELECT 
			ep.id, ep.exam_id, ep.personal_student_id,
			ps.full_name,
			e.duration_minutes, e.total_questions, e.passing_score,
			e.status, e.questions, e.scoring_config,
			ep.expires_at,
			(SELECT sub.id FROM exam_submissions sub 
			 WHERE sub.student_id = ep.personal_student_id 
			   AND sub.exam_id = ep.exam_id LIMIT 1)
		FROM exam_participants ep
		JOIN school_exams e ON e.id = ep.exam_id
		JOIN personal_students ps ON ps.id = ep.personal_student_id
		WHERE ep.access_code = $1
		  AND e.deleted_at IS NULL
	`, code).Scan(
		&participantID, &examID, &studentID,
		&studentName,
		&durationMinutes, &totalQuestions, &passingScore,
		&examStatus, &questionsJSON, &scoringConfig,
		&expiresAt,
		&submissionID,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Access code tidak valid"})
		return
	}

	if examStatus != "published" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ujian tidak aktif"})
		return
	}

	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		c.JSON(http.StatusGone, gin.H{"error": "Access code sudah kadaluarsa"})
		return
	}

	if submissionID.Valid {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "Anda sudah mengumpulkan ujian ini",
			"has_submitted": true,
		})
		return
	}

	var existingSessionToken string
	var existingExpires time.Time

	err = database.DB.QueryRow(`
		SELECT session_token, expires_at
		FROM exam_live_sessions
		WHERE schedule_id IS NULL
		  AND exam_id = $1
		  AND student_id = $2
		  AND submitted_at IS NULL
		ORDER BY started_at DESC LIMIT 1
	`, examID, studentID).Scan(&existingSessionToken, &existingExpires)

	if err == nil && existingSessionToken != "" {

		var snapshot []byte
		err = database.DB.QueryRow(`
			SELECT questions_snapshot, exam_config_snapshot
			FROM exam_live_sessions WHERE session_token = $1
		`, existingSessionToken).Scan(&snapshot, &[]byte{})

		var questions []any
		_ = json.Unmarshal(snapshot, &questions)

		c.JSON(http.StatusOK, gin.H{
			"resumed":          true,
			"session_token":    existingSessionToken,
			"exam_id":          examID,
			"exam_title":       "Ujian Personal",
			"student_name":     studentName,
			"duration_minutes": durationMinutes,
			"passing_score":    passingScore.Float64,
			"expires_at":       existingExpires.Format(time.RFC3339),
			"questions":        questions,
		})
		return
	}

	var questions []map[string]any
	if err := json.Unmarshal(questionsJSON, &questions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Format soal error"})
		return
	}

	for _, q := range questions {
		delete(q, "correct_answer")
		delete(q, "answer_key")
		delete(q, "explanation")
		delete(q, "rubric")
	}
	sanitizedJSON, _ := json.Marshal(questions)

	sessionToken := generatePublicSessionToken()

	expiresTime := time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	examConfig := map[string]any{
		"duration_minutes": durationMinutes,
		"passing_score":    passingScore.Float64,
		"total_score":      totalQuestions,
	}
	configJSON, _ := json.Marshal(examConfig)

	_, err = database.DB.Exec(`
		INSERT INTO exam_live_sessions (
			schedule_id, exam_id, student_id, school_id, owner_user_id,
			session_token,
			student_name_snapshot, student_nisn_snapshot,
			questions_snapshot, exam_config_snapshot,
			started_at, expires_at,
			ip_address, user_agent
		) VALUES (
			NULL, $1, $2, NULL, 
			(SELECT owner_user_id FROM exam_participants WHERE id = $3),
			$4,
			$5, NULL,
			$6::jsonb, $7::jsonb,
			NOW(), $8,
			$9, $10
		)
	`, examID, studentID, participantID,
		sessionToken,
		studentName,
		sanitizedJSON, configJSON,
		expiresTime, c.ClientIP(), c.Request.UserAgent(),
	)

	if err != nil {

		fmt.Printf("[START PUBLIC EXAM] Insert error (fallback needed): %v\n", err)

		_, err = database.DB.Exec(`
			INSERT INTO exam_live_sessions (
				schedule_id, student_id, school_id, owner_user_id,
				session_token,
				student_name_snapshot, student_nisn_snapshot,
				questions_snapshot, exam_config_snapshot,
				started_at, expires_at,
				ip_address, user_agent
			) VALUES (
				NULL, $1, NULL,
				(SELECT owner_user_id FROM exam_participants WHERE id = $2),
				$3,
				$4, NULL,
				$5::jsonb, $6::jsonb,
				NOW(), $7,
				$8, $9
			)
		`, studentID, participantID,
			sessionToken,
			studentName,
			sanitizedJSON, configJSON,
			expiresTime, c.ClientIP(), c.Request.UserAgent())

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi ujian: " + err.Error()})
			return
		}
	}

	var examTitle string
	_ = database.DB.QueryRow(`SELECT title FROM school_exams WHERE id = $1`, examID).Scan(&examTitle)

	c.JSON(http.StatusOK, gin.H{
		"resumed":          false,
		"session_token":    sessionToken,
		"exam_id":          examID,
		"exam_title":       examTitle,
		"student_name":     studentName,
		"duration_minutes": durationMinutes,
		"passing_score":    passingScore.Float64,
		"started_at":       time.Now().Format(time.RFC3339),
		"expires_at":       expiresTime.Format(time.RFC3339),
		"questions":        questions,
	})
}


func SubmitPersonalExam(c *gin.Context) {
	var req SubmitPersonalExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var (
		sessionID    string
		examID       string
		studentID    string
		ownerUserID  string
		studentName  string
		startedAt    time.Time
		submittedAt  sql.NullTime
	)

	err := database.DB.QueryRow(`
		SELECT 
			els.id, els.exam_id, els.student_id,
			COALESCE(els.owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid),
			els.student_name_snapshot,
			els.started_at, els.submitted_at
		FROM exam_live_sessions els
		WHERE els.session_token = $1
	`, req.SessionToken).Scan(
		&sessionID, &examID, &studentID,
		&ownerUserID,
		&studentName,
		&startedAt, &submittedAt,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak valid"})
		return
	}

	if submittedAt.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "Ujian sudah dikumpulkan"})
		return
	}

	var examQuestionsJSON []byte
	var passingScore sql.NullFloat64

	err = database.DB.QueryRow(`
		SELECT questions, passing_score 
		FROM school_exams WHERE id = $1
	`, examID).Scan(&examQuestionsJSON, &passingScore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil data exam"})
		return
	}

	var examQuestions []struct {
		ID            string  `json:"id"`
		Type          string  `json:"type"`
		CorrectAnswer string  `json:"correct_answer"`
		Score         float64 `json:"score"`
	}

	if err := json.Unmarshal(examQuestionsJSON, &examQuestions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Format soal error"})
		return
	}

	var totalScore, maxScore float64
	var hasEssay bool
	answersDetail := []map[string]any{}

	for _, q := range examQuestions {
		maxScore += q.Score
		studentAns := strings.TrimSpace(req.Answers[q.ID])

		detail := map[string]any{
			"question_id": q.ID,
			"type":        q.Type,
			"answer":      studentAns,
			"max_score":   q.Score,
		}

		if q.Type == "multiple_choice" {
			isCorrect := strings.EqualFold(studentAns, strings.TrimSpace(q.CorrectAnswer))
			detail["is_correct"] = isCorrect
			if isCorrect {
				detail["score"] = q.Score
				totalScore += q.Score
			} else {
				detail["score"] = 0.0
			}
		} else {
			detail["score"] = nil
			detail["is_correct"] = nil
			hasEssay = true
		}

		answersDetail = append(answersDetail, detail)
	}

	percentage := 0.0
	if maxScore > 0 {
		percentage = (totalScore / maxScore) * 100
	}

	status := "graded"
	if hasEssay {
		status = "graded_with_pending"
	}

	answersJSON, _ := json.Marshal(answersDetail)

	var submissionID string
	err = database.DB.QueryRow(`
		INSERT INTO exam_submissions (
			schedule_id, exam_id, school_id, owner_user_id,
			student_id, student_name, student_nisn,
			started_at, submitted_at,
			answers, total_score, max_score, percentage, is_passed,
			status, tab_switch_count, time_spent_seconds,
			ip_address, user_agent
		) VALUES (
			NULL, $1, NULL, $2,
			$3, $4, NULL,
			$5, NOW(),
			$6::jsonb, $7, $8, $9, $10,
			$11, $12, $13,
			$14, $15
		) RETURNING id
	`, examID, ownerUserID,
		studentID, studentName,
		startedAt,
		answersJSON, totalScore, maxScore, percentage, percentage >= passingScore.Float64,
		status, req.TabSwitchCount, req.TimeSpentSeconds,
		c.ClientIP(), c.Request.UserAgent(),
	).Scan(&submissionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan jawaban: " + err.Error()})
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE exam_live_sessions SET submitted_at = NOW() WHERE id = $1
	`, sessionID)

	c.JSON(http.StatusOK, gin.H{
		"submission_id": submissionID,
		"total_score":   totalScore,
		"max_score":     maxScore,
		"percentage":    percentage,
		"is_passed":     percentage >= passingScore.Float64,
		"has_pending_essay": hasEssay,
		"message":       "Jawaban berhasil dikumpulkan. Essay akan dinilai oleh guru.",
	})
}


func GetPersonalExamResult(c *gin.Context) {
	sessionToken := c.Param("sessionToken")

	var (
		submissionID string
		totalScore   sql.NullFloat64
		maxScore     sql.NullFloat64
		percentage   sql.NullFloat64
		isPassed     sql.NullBool
		status       string
		submittedAt  sql.NullTime
	)

	err := database.DB.QueryRow(`
		SELECT 
			sub.id, sub.total_score, sub.max_score, sub.percentage, sub.is_passed,
			sub.status, sub.submitted_at
		FROM exam_submissions sub
		JOIN exam_live_sessions els ON els.student_id = sub.student_id
		WHERE els.session_token = $1
		ORDER BY sub.submitted_at DESC LIMIT 1
	`, sessionToken).Scan(
		&submissionID, &totalScore, &maxScore, &percentage, &isPassed,
		&status, &submittedAt,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hasil tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"submission_id": submissionID,
		"total_score":   totalScore.Float64,
		"max_score":     maxScore.Float64,
		"percentage":    percentage.Float64,
		"is_passed":     isPassed.Bool,
		"status":        status,
		"submitted_at":  submittedAt.Time,
	})
}


func generatePublicSessionToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 24)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1 * time.Nanosecond)
	}
	return "PUBSES-" + string(b)
}

// ==========================================
// Helper: dummy reference biar import helpers tidak unused
// (kita tidak pakai helpers di file ini karena public endpoint)
// ==========================================
// var _ = helpers.MustGetPersonalScope // force import (untuk future use)