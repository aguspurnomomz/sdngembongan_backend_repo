package teacher

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/routes/helpers"
	tokensvc "gembongan_dev_backend/routes/token"

	"github.com/gin-gonic/gin"
)

type PersonalExamQuestionInput struct {
	ID             string          `json:"id"`
	Type           string          `json:"type" binding:"required,oneof=multiple_choice essay"`
	Order          int             `json:"order"`
	QuestionText   string          `json:"question_text" binding:"required"`
	Options        json.RawMessage `json:"options"`
	CorrectAnswer  string          `json:"correct_answer"`
	AnswerKey      string          `json:"answer_key"`
	Rubric         json.RawMessage `json:"rubric"`
	Explanation    string          `json:"explanation"`
	CognitiveLevel string          `json:"cognitive_level"`
	Score          float64         `json:"score"`
	MinWords       int             `json:"min_words"`
	ImageURL       string          `json:"image_url"`
}

type CreatePersonalExamRequest struct {
	Title           string                      `json:"title" binding:"required,min=3,max=200"`
	Description     string                      `json:"description"`
	Subject         string                      `json:"subject" binding:"required"`
	ExamType        string                      `json:"exam_type"` 
	GradeLevel      string                      `json:"grade_level"`
	Phase           string                      `json:"phase"`
	DurationMinutes int                         `json:"duration_minutes" binding:"required,min=1"`
	PassingScore    float64                     `json:"passing_score"`
	ScoringConfig   json.RawMessage             `json:"scoring_config"`
	Questions       []PersonalExamQuestionInput `json:"questions" binding:"required,min=1"`
}


func ListPersonalExams(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	status := c.Query("status")           
	subject := c.Query("subject")
	search := strings.TrimSpace(c.Query("search"))
	examType := c.Query("exam_type")

	query := `
		SELECT 
			e.id, e.title, COALESCE(e.description, ''), e.subject, e.exam_type,
			COALESCE(e.grade_level, ''), COALESCE(e.phase, ''),
			e.duration_minutes, e.total_questions, e.total_score,
			COALESCE(e.passing_score, 0),
			e.status, e.is_active,
			e.created_at, e.updated_at, e.published_at,
			(SELECT COUNT(*) FROM exam_participants ep WHERE ep.exam_id = e.id) AS participant_count
		FROM school_exams e
		WHERE e.owner_user_id = $1 AND e.deleted_at IS NULL
	`
	args := []any{scope.UserID}
	argIdx := 2

	if status != "" {
		query += fmt.Sprintf(" AND e.status = $%d", argIdx)
		args = append(args, status); argIdx++
	}
	if subject != "" {
		query += fmt.Sprintf(" AND e.subject = $%d", argIdx)
		args = append(args, subject); argIdx++
	}
	if examType != "" {
		query += fmt.Sprintf(" AND e.exam_type = $%d", argIdx)
		args = append(args, examType); argIdx++
	}
	if search != "" {
		query += fmt.Sprintf(" AND (e.title ILIKE $%d OR e.description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+search+"%"); argIdx++
	}

	query += " ORDER BY e.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar ujian: " + err.Error()})
		return
	}
	defer rows.Close()

	type ExamItem struct {
		ID               string     `json:"id"`
		Title            string     `json:"title"`
		Description      string     `json:"description"`
		Subject          string     `json:"subject"`
		ExamType         string     `json:"exam_type"`
		GradeLevel       string     `json:"grade_level"`
		Phase            string     `json:"phase"`
		DurationMinutes  int        `json:"duration_minutes"`
		TotalQuestions   int        `json:"total_questions"`
		TotalScore       float64    `json:"total_score"`
		PassingScore     float64    `json:"passing_score"`
		Status           string     `json:"status"`
		IsActive         bool       `json:"is_active"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
		PublishedAt      *time.Time `json:"published_at"`
		ParticipantCount int        `json:"participant_count"`
	}

	exams := []ExamItem{}
	for rows.Next() {
		var e ExamItem
		var desc, gradeLevel, phase sql.NullString
		var passingScore sql.NullFloat64
		var publishedAt sql.NullTime

		err := rows.Scan(
			&e.ID, &e.Title, &desc, &e.Subject, &e.ExamType,
			&gradeLevel, &phase,
			&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &passingScore,
			&e.Status, &e.IsActive,
			&e.CreatedAt, &e.UpdatedAt, &publishedAt,
			&e.ParticipantCount,
		)
		if err != nil {
			continue
		}

		e.Description = desc.String
		e.GradeLevel = gradeLevel.String
		e.Phase = phase.String
		if passingScore.Valid { e.PassingScore = passingScore.Float64 }
		if publishedAt.Valid { e.PublishedAt = &publishedAt.Time }

		exams = append(exams, e)
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(exams),
		"exams": exams,
	})
}


func GetPersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	var e struct {
		ID               string
		Title            string
		Description      sql.NullString
		Subject          string
		ExamType         string
		GradeLevel       sql.NullString
		Phase            sql.NullString
		DurationMinutes  int
		TotalQuestions   int
		TotalScore       float64
		PassingScore     sql.NullFloat64
		Status           string
		IsActive         bool
		CreatedAt        time.Time
		UpdatedAt        time.Time
		PublishedAt      sql.NullTime
		Questions        []byte
		ScoringConfig    []byte
	}

	err = database.DB.QueryRow(`
		SELECT 
			id, title, description, subject, exam_type,
			grade_level, phase,
			duration_minutes, total_questions, total_score, passing_score,
			status, is_active,
			created_at, updated_at, published_at,
			questions, scoring_config
		FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(
		&e.ID, &e.Title, &e.Description, &e.Subject, &e.ExamType,
		&e.GradeLevel, &e.Phase,
		&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &e.PassingScore,
		&e.Status, &e.IsActive,
		&e.CreatedAt, &e.UpdatedAt, &e.PublishedAt,
		&e.Questions, &e.ScoringConfig,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan atau bukan milik Anda"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exam": gin.H{
			"id":               e.ID,
			"title":            e.Title,
			"description":      e.Description.String,
			"subject":          e.Subject,
			"exam_type":        e.ExamType,
			"grade_level":      e.GradeLevel.String,
			"phase":            e.Phase.String,
			"duration_minutes": e.DurationMinutes,
			"total_questions":  e.TotalQuestions,
			"total_score":      e.TotalScore,
			"passing_score":    e.PassingScore.Float64,
			"status":           e.Status,
			"is_active":        e.IsActive,
			"created_at":       e.CreatedAt,
			"updated_at":       e.UpdatedAt,
			"published_at":     e.PublishedAt.Time,
			"questions":        json.RawMessage(e.Questions),
			"scoring_config":   json.RawMessage(e.ScoringConfig),
		},
	})
}


func CreatePersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req CreatePersonalExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Hitung total score
	var totalScore float64
	for _, q := range req.Questions {
		totalScore += q.Score
	}
	if totalScore == 0 {
		totalScore = 100
	}

	examType := req.ExamType
	if examType == "" { examType = "regular" }

	scoringConfig := req.ScoringConfig
	if len(scoringConfig) == 0 {
		scoringConfig = json.RawMessage(`{"multiple_choice":1,"essay":5,"essay_rubric":"manual"}`)
	}

	questionsJSON, _ := json.Marshal(req.Questions)

	var examID string
	err = database.DB.QueryRow(`
		INSERT INTO school_exams (
			school_id, owner_user_id, academic_year_id, created_by,
			title, description, subject, exam_type, grade_level, phase,
			duration_minutes, total_questions, total_score, passing_score,
			scoring_config, questions, status
		) VALUES (
			NULL, $1, NULL, NULL,
			$2, NULLIF($3,''), $4, $5, NULLIF($6,''), NULLIF($7,''),
			$8, $9, $10, $11,
			$12::jsonb, $13::jsonb, 'draft'
		) RETURNING id
	`, scope.UserID,
		req.Title, req.Description, req.Subject, examType, req.GradeLevel, req.Phase,
		req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
		scoringConfig, questionsJSON,
	).Scan(&examID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan ujian: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ujian berhasil dibuat (draft). Draft gratis — token akan di-charge saat publish.",
		"id":      examID,
	})
}


func UpdatePersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	// Cek status
	var currentStatus string
	err = database.DB.QueryRow(`
		SELECT status FROM school_exams 
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&currentStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	if currentStatus != "draft" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Hanya ujian berstatus draft yang bisa diubah",
		})
		return
	}

	var req CreatePersonalExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var totalScore float64
	for _, q := range req.Questions {
		totalScore += q.Score
	}
	if totalScore == 0 { totalScore = 100 }

	examType := req.ExamType
	if examType == "" { examType = "regular" }

	scoringConfig := req.ScoringConfig
	if len(scoringConfig) == 0 {
		scoringConfig = json.RawMessage(`{"multiple_choice":1,"essay":5,"essay_rubric":"manual"}`)
	}

	questionsJSON, _ := json.Marshal(req.Questions)

	_, err = database.DB.Exec(`
		UPDATE school_exams SET
			title = $1,
			description = NULLIF($2,''),
			subject = $3,
			exam_type = $4,
			grade_level = NULLIF($5,''),
			phase = NULLIF($6,''),
			duration_minutes = $7,
			total_questions = $8,
			total_score = $9,
			passing_score = $10,
			scoring_config = $11::jsonb,
			questions = $12::jsonb,
			updated_at = NOW()
		WHERE id = $13 AND owner_user_id = $14
	`, req.Title, req.Description, req.Subject, examType,
		req.GradeLevel, req.Phase,
		req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
		scoringConfig, questionsJSON,
		examID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update ujian: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil diperbarui"})
}


func CalculatePersonalExamCost(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	var totalQuestions int
	var currentStatus string
	err = database.DB.QueryRow(`
		SELECT total_questions, status FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&totalQuestions, &currentStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	// Hitung biaya
	cost, err := tokensvc.CalculatePublishCost(scope.TeacherType, totalQuestions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hitung biaya: " + err.Error()})
		return
	}

	// Ambil saldo current
	var currentBalance int
	_ = database.DB.QueryRow(`SELECT token_balance FROM profiles WHERE id = $1`, scope.UserID).Scan(&currentBalance)

	c.JSON(http.StatusOK, gin.H{
		"exam_id":            examID,
		"status":             currentStatus,
		"total_questions":    totalQuestions,
		"teacher_type":       scope.TeacherType,
		"base_cost":          cost.BaseCost,
		"manual_extra_cost":  cost.ManualExtraCost,
		"total_cost":         cost.TotalCost,
		"current_balance":    currentBalance,
		"sufficient":         currentBalance >= cost.TotalCost,
		"breakdown":          cost.Breakdown,
	})
}


func PublishPersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	// 1. Cek exam existing & status
	var totalQuestions int
	var currentStatus string
	err = database.DB.QueryRow(`
		SELECT total_questions, status FROM school_exams
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID).Scan(&totalQuestions, &currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if currentStatus == "published" {
		c.JSON(http.StatusConflict, gin.H{"error": "Ujian sudah dipublish sebelumnya"})
		return
	}
	if currentStatus != "draft" {
		c.JSON(http.StatusConflict, gin.H{"error": "Hanya ujian draft yang bisa dipublish"})
		return
	}
	if totalQuestions == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ujian belum punya soal"})
		return
	}

	// 2. Charge token (composite: base + manual extra)
	cost, chargeErr := tokensvc.ChargePublishExam(
		scope.UserID,
		scope.TeacherType,
		examID,
		totalQuestions,
	)

	if chargeErr != nil {
		if chargeErr == tokensvc.ErrInsufficientToken {
			// Ambil saldo untuk info
			var currentBalance int
			_ = database.DB.QueryRow(`SELECT token_balance FROM profiles WHERE id = $1`, scope.UserID).Scan(&currentBalance)

			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": gin.H{
					"code":    "INSUFFICIENT_TOKEN",
					"message": "Token tidak cukup untuk mempublish ujian ini",
				},
				"required":        cost.TotalCost,
				"current_balance": currentBalance,
				"breakdown":       cost.Breakdown,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal charge token: " + chargeErr.Error()})
		return
	}

	// 3. Update status = published
	_, err = database.DB.Exec(`
		UPDATE school_exams 
		SET status = 'published', published_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND status = 'draft'
	`, examID, scope.UserID)

	if err != nil {
		// Rollback token charge!
		if cost.TotalCost > 0 {
			_, _, refundErr := tokensvc.RefundTokens(
				scope.UserID,
				cost.TotalCost,
				"refund_publish_failed",
				"exam",
				examID,
				"Rollback: update status publish gagal",
			)
			if refundErr != nil {
				fmt.Printf("[CRITICAL] Rollback publish charge FAILED for exam=%s: %v\n", examID, refundErr)
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal publish: " + err.Error()})
		return
	}

	// 4. Ambil saldo terbaru untuk response
	var newBalance int
	_ = database.DB.QueryRow(`SELECT token_balance FROM profiles WHERE id = $1`, scope.UserID).Scan(&newBalance)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Ujian berhasil dipublikasikan",
		"exam_id":         examID,
		"charged":         cost.TotalCost,
		"new_balance":     newBalance,
		"breakdown":       cost.Breakdown,
	})
}


func ArchivePersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	result, err := database.DB.Exec(`
		UPDATE school_exams 
		SET status = 'archived', updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND status IN ('draft', 'published')
	`, examID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan atau tidak bisa diarsipkan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil diarsipkan"})
}


func DeletePersonalExam(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	examID := c.Param("id")

	// Cek apakah ada peserta yang sudah submit
	var submittedCount int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM exam_submissions WHERE exam_id = $1
	`, examID).Scan(&submittedCount)

	if submittedCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Tidak bisa dihapus: sudah ada %d submission. Arsipkan saja.", submittedCount),
		})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE school_exams 
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
	`, examID, scope.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil dihapus"})
}