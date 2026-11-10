package public

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)


type StartExamRequest struct {
	NISN        string `json:"nisn" binding:"required"`
	AccessCode  string `json:"access_code" binding:"required"`
	StudentName string `json:"student_name" binding:"required"`
}

type SubmitExamRequest struct {
	SessionID     string            `json:"session_id" binding:"required"`
	StudentName   string            `json:"student_name" binding:"required"`
	StudentNumber string            `json:"student_number" binding:"required"`
	NISN          string            `json:"nisn" binding:"required"`
	Answers       map[string]string `json:"answers" binding:"required"`
}

type SubmitLiveExamRequest struct {
	Answers          map[string]string `json:"answers" binding:"required"`
	TabSwitchCount   int               `json:"tab_switch_count"`
	TimeSpentSeconds int               `json:"time_spent_seconds"`
}

func nowJakarta() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return time.Now().In(loc)
}

func generateSessionToken() string {
	b := make([]byte, 24)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[time.Now().UnixNano()%62]
		time.Sleep(1 * time.Nanosecond)
	}
	return "SES-" + string(b)
}

func writeInternalError(c *gin.Context, publicMsg string, internalErr error) {
	if internalErr != nil {
		fmt.Printf("[INTERNAL ERROR] %s | detail: %v\n", publicMsg, internalErr)
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": publicMsg,
		},
	})
}


// GET /api/exam/session-questions
func GetExamSessionQuestions(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token sesi tidak disertakan"})
		return
	}

	var sessionID, examTitle string
	var questionBankID sql.NullString

	err := database.DB.QueryRow(`
		SELECT id, title, question_bank_id 
		FROM exam_sessions 
		WHERE (qr_code_token = $1 OR id::text = $1) AND deleted_at IS NULL
	`, token).Scan(&sessionID, &examTitle, &questionBankID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak ditemukan atau QR Code tidak valid"})
		return
	}

	if !questionBankID.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sesi ujian ini tidak memiliki bank soal yang terhubung"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, question_text, question_type, options, cognitive_level 
		FROM questions 
		WHERE question_bank_id = $1
	`, questionBankID.String)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat butir soal dari database: " + err.Error()})
		return
	}
	defer rows.Close()

	type QuestionItem struct {
		ID             string `json:"id"`
		QuestionText   string `json:"question_text"`
		QuestionType   string `json:"question_type"`
		Options        any    `json:"options"`
		CognitiveLevel string `json:"cognitive_level"`
	}

	var questions []QuestionItem
	for rows.Next() {
		var q QuestionItem
		var optionsJson []byte
		var cogLevel sql.NullString

		if err := rows.Scan(&q.ID, &q.QuestionText, &q.QuestionType, &optionsJson, &cogLevel); err != nil {
			continue
		}

		var parsedOptions any
		if len(optionsJson) > 0 {
			_ = json.Unmarshal(optionsJson, &parsedOptions)
		}
		q.Options = parsedOptions
		q.CognitiveLevel = cogLevel.String

		questions = append(questions, q)
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"title":      examTitle,
		"questions":  questions,
	})
}

// POST /api/exam/submit
func SubmitExam(c *gin.Context) {
	var req SubmitExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengumpulan ujian tidak lengkap: " + err.Error()})
		return
	}

	var existingID string
	err := database.DB.QueryRow(`
		SELECT id FROM exam_submissions WHERE exam_session_id = $1 AND nisn = $2
	`, req.SessionID, req.NISN).Scan(&existingID)

	if err == nil && existingID != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak! Anda (NISN " + req.NISN + ") sudah pernah mengumpulkan ujian ini."})
		return
	}

	var questionBankID string
	err = database.DB.QueryRow(`
		SELECT question_bank_id FROM exam_sessions WHERE id = $1
	`, req.SessionID).Scan(&questionBankID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak valid"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, correct_answer FROM questions WHERE question_bank_id = $1
	`, questionBankID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi kunci jawaban"})
		return
	}
	defer rows.Close()

	correctAnswersMap := make(map[string]string)
	totalQuestions := 0
	for rows.Next() {
		var qID string
		var correctAnsJSON []byte
		if err := rows.Scan(&qID, &correctAnsJSON); err == nil {
			var rawAns string
			if json.Unmarshal(correctAnsJSON, &rawAns) == nil {
				correctAnswersMap[qID] = rawAns
			} else {
				correctAnswersMap[qID] = string(correctAnsJSON)
			}
			totalQuestions++
		}
	}

	correctCount := 0
	for qID, studentAns := range req.Answers {
		if correctAns, exists := correctAnswersMap[qID]; exists {
			cleanedCorrect := strings.Trim(correctAns, `"`)
			if strings.EqualFold(strings.TrimSpace(studentAns), strings.TrimSpace(cleanedCorrect)) {
				correctCount++
			}
		}
	}

	var score float64 = 0
	if totalQuestions > 0 {
		score = (float64(correctCount) / float64(totalQuestions)) * 100
	}

	answersBytes, _ := json.Marshal(req.Answers)
	queryInsert := `
		INSERT INTO exam_submissions (exam_session_id, student_name, student_number, nisn, answers, score)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
	`

	_, err = database.DB.Exec(queryInsert, req.SessionID, req.StudentName, req.StudentNumber, req.NISN, string(answersBytes), score)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan jawaban: Siswa dengan NISN ini sudah tercatat mengumpulkan ujian."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ujian berhasil dikumpulkan",
		"score":   score,
	})
}

// GET /api/exam/check-student
func CheckStudentExamStatus(c *gin.Context) {
	sessionID := c.Query("session_id")
	nisn := c.Query("nisn")

	if sessionID == "" || nisn == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter tidak lengkap"})
		return
	}

	var existingID string
	err := database.DB.QueryRow(`
		SELECT id FROM exam_submissions WHERE exam_session_id = $1 AND nisn = $2
	`, sessionID, nisn).Scan(&existingID)

	if err == nil && existingID != "" {
		c.JSON(http.StatusOK, gin.H{"has_submitted": true, "message": "NISN ini sudah pernah mengumpulkan ujian."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"has_submitted": false})
}

// GET /api/exam/:sessionToken/status
func GetExamSessionStatus(c *gin.Context) {
	sessionToken := c.Param("sessionToken")

	var isBlocked bool
	var blockedReason, lastWarningMessage sql.NullString
	var blockedAt, lastWarningAt, submittedAt sql.NullTime
	var expiresAt time.Time

	err := database.DB.QueryRow(`
		SELECT 
			COALESCE(is_blocked, false),
			blocked_reason,
			blocked_at,
			last_warning_at,
			last_warning_message,
			submitted_at,
			expires_at
		FROM exam_live_sessions 
		WHERE session_token = $1
	`, sessionToken).Scan(
		&isBlocked,
		&blockedReason,
		&blockedAt,
		&lastWarningAt,
		&lastWarningMessage,
		&submittedAt,
		&expiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak ditemukan"})
			return
		}
		writeInternalError(c, "Gagal memuat status sesi", err)
		return
	}

	remainingSeconds := int(time.Until(expiresAt).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	var blockedAtStr, lastWarningAtStr interface{} = nil, nil
	if blockedAt.Valid {
		blockedAtStr = blockedAt.Time.Format(time.RFC3339)
	}
	if lastWarningAt.Valid {
		lastWarningAtStr = lastWarningAt.Time.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"is_blocked":           isBlocked,
		"blocked_reason":       blockedReason.String,
		"blocked_at":           blockedAtStr,
		"last_warning_at":      lastWarningAtStr,
		"last_warning_message": lastWarningMessage.String,
		"is_submitted":         submittedAt.Valid,
		"remaining_seconds":    remainingSeconds,
		"expires_at":           expiresAt.Format(time.RFC3339),
		"server_time":          nowJakarta().Format(time.RFC3339),
	})
}

// POST /api/exam/start
func StartExam(c *gin.Context) {
	var req StartExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak lengkap: " + err.Error()})
		return
	}

	// 1. Cari schedule dari access code
	var scheduleID, examID, schoolID, status string
	var scheduleDate time.Time
	var startTime, endTime time.Time
	var classSubGroupID, classGroupID sql.NullString
	var requireLogin bool
	var examTitle, subject string
	var passingScore float64
	var durationMinutes int
	var durationMode string

	err := database.DB.QueryRow(`
		SELECT 
			es.id, es.exam_id, es.school_id, es.status,
			es.schedule_date, es.start_time, es.end_time,
			es.class_group_id, es.class_sub_group_id,
			es.require_login, es.duration_minutes,
			es.duration_mode, 
			e.title, e.subject, COALESCE(e.passing_score, 0)
		FROM exam_schedules es
		JOIN school_exams e ON es.exam_id = e.id
		WHERE es.access_code = $1 
		AND es.deleted_at IS NULL
		AND es.status IN ('scheduled', 'ongoing')
	`, strings.ToUpper(req.AccessCode)).Scan(
		&scheduleID, &examID, &schoolID, &status,
		&scheduleDate, &startTime, &endTime,
		&classGroupID, &classSubGroupID,
		&requireLogin, &durationMinutes,
		&durationMode,
		&examTitle, &subject, &passingScore,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kode ujian tidak valid atau ujian sudah berakhir"})
		return
	}

	// 2. Cek waktu
	now := nowJakarta()
	scheduleStart := time.Date(
		scheduleDate.Year(), scheduleDate.Month(), scheduleDate.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, now.Location(),
	)
	scheduleEnd := time.Date(
		scheduleDate.Year(), scheduleDate.Month(), scheduleDate.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, now.Location(),
	)

	var expiresAt time.Time

	if durationMode == "flexible" {
		if now.Before(scheduleStart) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Ujian belum dibuka. Dibuka pukul %s", startTime.Format("15:04")),
			})
			return
		}
		if now.After(scheduleEnd) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Window ujian sudah ditutup"})
			return
		}

		durationEnd := now.Add(time.Duration(durationMinutes) * time.Minute)
		if durationEnd.Before(scheduleEnd) {
			expiresAt = durationEnd
		} else {
			expiresAt = scheduleEnd
		}

	} else {
		if now.Before(scheduleStart.Add(-10 * time.Minute)) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Ujian belum dimulai. Mulai pukul %s", startTime.Format("15:04")),
			})
			return
		}
		if now.After(scheduleEnd) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Waktu ujian sudah berakhir"})
			return
		}
		expiresAt = scheduleEnd
	}

	// 3. Cari student
	var studentID, actualName, actualNISN string
	var studentClassSubGroupID sql.NullString

	err = database.DB.QueryRow(`
		SELECT id, full_name, COALESCE(nisn, ''), class_sub_group_id
		FROM students 
		WHERE school_id = $1 
		AND nisn = $2
		AND LOWER(full_name) = LOWER($3)
		AND is_active = TRUE
	`, schoolID, req.NISN, req.StudentName).Scan(
		&studentID, &actualName, &actualNISN, &studentClassSubGroupID,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Data siswa tidak cocok. Periksa NISN dan nama Anda.",
		})
		return
	}

	if classSubGroupID.Valid && studentClassSubGroupID.Valid {
		if classSubGroupID.String != studentClassSubGroupID.String {
			c.JSON(http.StatusForbidden, gin.H{"error": "Anda bukan peserta ujian ini"})
			return
		}
	}

	// 4. Cek apakah sudah submit
	var existingSubID string
	err = database.DB.QueryRow(`
		SELECT id FROM exam_submissions 
		WHERE schedule_id = $1 AND student_id = $2
	`, scheduleID, studentID).Scan(&existingSubID)
	if err == nil && existingSubID != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda sudah mengumpulkan ujian ini"})
		return
	}

	// 5. Cek existing live session
	var existingToken string
	var existingExpires time.Time
	err = database.DB.QueryRow(`
		SELECT session_token, expires_at
		FROM exam_live_sessions
		WHERE schedule_id = $1 AND student_id = $2 AND submitted_at IS NULL
	`, scheduleID, studentID).Scan(&existingToken, &existingExpires)

	var sessionToken string

	if err == nil && existingToken != "" {
		sessionToken = existingToken
		expiresAt = existingExpires
	} else {
		sessionToken = generateSessionToken()
		durationEnd := now.Add(time.Duration(durationMinutes) * time.Minute)
		if durationEnd.Before(scheduleEnd) {
			expiresAt = durationEnd
		} else {
			expiresAt = scheduleEnd
		}

		var questionsJSON []byte
		var scoringConfig []byte
		err = database.DB.QueryRow(`
			SELECT questions, scoring_config FROM school_exams WHERE id = $1
		`, examID).Scan(&questionsJSON, &scoringConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat soal"})
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

		examConfig := map[string]any{
			"duration_minutes": durationMinutes,
			"passing_score":    passingScore,
			"total_score":      len(questions),
		}
		configJSON, _ := json.Marshal(examConfig)

		_, err = database.DB.Exec(`
			INSERT INTO exam_live_sessions (
				schedule_id, student_id, school_id,
				session_token,
				student_name_snapshot, student_nisn_snapshot,
				questions_snapshot, exam_config_snapshot,
				started_at, expires_at,
				ip_address, user_agent
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb,
				NOW(), $9, $10, $11
			)
		`, scheduleID, studentID, schoolID,
			sessionToken,
			actualName, actualNISN,
			sanitizedJSON, configJSON,
			expiresAt, c.ClientIP(), c.Request.UserAgent(),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi ujian: " + err.Error()})
			return
		}

		if status == "scheduled" {
			_, _ = database.DB.Exec(`
				UPDATE exam_schedules SET status = 'ongoing', updated_at = NOW()
				WHERE id = $1
			`, scheduleID)
		}
	}

	// 6. Ambil questions_snapshot
	var questionsSnapshot []byte
	var examConfigSnapshot []byte
	err = database.DB.QueryRow(`
		SELECT questions_snapshot, exam_config_snapshot
		FROM exam_live_sessions WHERE session_token = $1
	`, sessionToken).Scan(&questionsSnapshot, &examConfigSnapshot)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat soal"})
		return
	}

	var questions any
	var examConfig struct {
		DurationMinutes int     `json:"duration_minutes"`
		PassingScore    float64 `json:"passing_score"`
		TotalScore      int     `json:"total_score"`
	}
	_ = json.Unmarshal(questionsSnapshot, &questions)
	_ = json.Unmarshal(examConfigSnapshot, &examConfig)

	remainingSeconds := int(time.Until(expiresAt).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"session_token":    sessionToken,
		"schedule_id":      scheduleID,
		"exam_id":          examID,
		"student_id":       studentID,
		"student_name":     actualName,
		"student_nisn":     actualNISN,
		"exam_title":       examTitle,
		"exam_subject":     subject,
		"duration_minutes": examConfig.DurationMinutes,
		"passing_score":    examConfig.PassingScore,
		"total_score":      examConfig.TotalScore,
		"started_at":       now.Format(time.RFC3339),
		"expires_at":       expiresAt.Format(time.RFC3339),
		"server_time":      now.Format(time.RFC3339),
		"remaining_seconds": remainingSeconds,
		"questions":        questions,
	})
}

// POST /api/exam/:sessionToken/submit
func SubmitLiveExam(c *gin.Context) {
	sessionToken := c.Param("sessionToken")

	var req SubmitLiveExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var sessionID, scheduleID, examID, schoolID, studentID string
	var studentName, studentNISN string
	var questionsSnapshot []byte
	var startedAt time.Time
	var submittedAt sql.NullTime

	err := database.DB.QueryRow(`
		SELECT 
			els.id, els.schedule_id,
			es.exam_id,
			els.school_id, els.student_id,
			els.student_name_snapshot, els.student_nisn_snapshot,
			els.questions_snapshot,
			els.started_at,
			els.submitted_at
		FROM exam_live_sessions els
		JOIN exam_schedules es ON es.id = els.schedule_id
		WHERE els.session_token = $1
	`, sessionToken).Scan(
		&sessionID, &scheduleID,
		&examID,
		&schoolID, &studentID,
		&studentName, &studentNISN,
		&questionsSnapshot,
		&startedAt,
		&submittedAt,
	)
	if err != nil {
		fmt.Printf("[SUBMIT] Gagal ambil session: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak valid"})
		return
	}

	if submittedAt.Valid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Ujian sudah dikumpulkan sebelumnya"})
		return
	}

	var passingScore float64
	var examQuestionsJSON []byte
	err = database.DB.QueryRow(`
		SELECT COALESCE(passing_score, 0), questions 
		FROM school_exams 
		WHERE id = $1
	`, examID).Scan(&passingScore, &examQuestionsJSON)
	if err != nil {
		fmt.Printf("[SUBMIT] Gagal ambil exam info: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil data ujian"})
		return
	}

	var examQuestions []struct {
		ID            string  `json:"id"`
		Type          string  `json:"type"`
		CorrectAnswer string  `json:"correct_answer"`
		AnswerKey     string  `json:"answer_key"`
		Score         float64 `json:"score"`
	}
	if err := json.Unmarshal(examQuestionsJSON, &examQuestions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Format soal error"})
		return
	}

	var totalScore float64
	var maxScore float64
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
			schedule_id, exam_id, school_id, student_id,
			student_name, student_nisn,
			started_at, submitted_at,
			answers, total_score, max_score, percentage, is_passed,
			status, tab_switch_count, time_spent_seconds,
			ip_address, user_agent
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, NOW(),
			$8::jsonb, $9, $10, $11, $12,
			$13, $14, $15, $16, $17
		) RETURNING id
	`, scheduleID, examID, schoolID, studentID,
		studentName, studentNISN,
		startedAt,
		answersJSON, totalScore, maxScore, percentage, percentage >= passingScore,
		status, req.TabSwitchCount, req.TimeSpentSeconds,
		c.ClientIP(), c.Request.UserAgent(),
	).Scan(&submissionID)
	if err != nil {
		fmt.Printf("[SUBMIT] Gagal insert submission: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan jawaban: " + err.Error()})
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE exam_live_sessions 
		SET submitted_at = NOW() 
		WHERE id = $1
	`, sessionID)

	_, _ = database.DB.Exec(`
		UPDATE exam_schedules SET
			total_submitted = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1),
			total_graded = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1 AND status = 'graded'),
			average_score = (SELECT AVG(percentage) FROM exam_submissions WHERE schedule_id = $1),
			highest_score = (SELECT MAX(percentage) FROM exam_submissions WHERE schedule_id = $1),
			lowest_score = (SELECT MIN(percentage) FROM exam_submissions WHERE schedule_id = $1),
			updated_at = NOW()
		WHERE id = $1
	`, scheduleID)

	c.JSON(http.StatusOK, gin.H{
		"submission_id": submissionID,
		"total_score":   totalScore,
		"max_score":     maxScore,
		"percentage":    percentage,
		"is_passed":     percentage >= passingScore,
		"message":       "Jawaban berhasil dikumpulkan. Soal essay akan dikoreksi manual oleh guru.",
	})
}