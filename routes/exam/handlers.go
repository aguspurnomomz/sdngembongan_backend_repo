package exam

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"strings"
	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)


// GET /api/school-admin/exams?status=draft&subject=Matematika&search=uts
func ListExamsDraft(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			status := c.Query("status")
			subject := c.Query("subject")
			search := c.Query("search")
			examType := c.Query("exam_type")

			query := `
				SELECT 
					e.id, e.title, COALESCE(e.description, ''), e.subject, e.exam_type,
					COALESCE(e.grade_level, ''), COALESCE(e.phase, ''),
					e.duration_minutes, e.total_questions, e.total_score, 
					COALESCE(e.passing_score, 0),
					e.status, e.is_active,
					e.created_at, e.updated_at, e.published_at,
					COALESCE(say.name, '') AS academic_year_name,
					COALESCE(say.semester, '') AS semester,
					COALESCE(
						(SELECT json_agg(json_build_object(
							'id', et.id,
							'class_group_id', et.class_group_id,
							'class_sub_group_id', et.class_sub_group_id,
							'class_group_name', cg.name,
							'class_sub_group_name', csg.name
						))
						FROM exam_targets et
						LEFT JOIN class_groups cg ON et.class_group_id = cg.id
						LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
						WHERE et.exam_id = e.id),
						'[]'::json
					) AS targets
				FROM school_exams e
				LEFT JOIN school_academic_years say ON e.academic_year_id = say.id
				WHERE e.school_id = $1 AND e.deleted_at IS NULL
			`
			args := []any{schoolID}
			argIdx := 2

			if status != "" {
				query += fmt.Sprintf(" AND e.status = $%d", argIdx)
				args = append(args, status)
				argIdx++
			}
			if subject != "" {
				query += fmt.Sprintf(" AND e.subject = $%d", argIdx)
				args = append(args, subject)
				argIdx++
			}
			if examType != "" {
				query += fmt.Sprintf(" AND e.exam_type = $%d", argIdx)
				args = append(args, examType)
				argIdx++
			}
			if search != "" {
				query += fmt.Sprintf(" AND (e.title ILIKE $%d OR e.description ILIKE $%d)", argIdx, argIdx)
				args = append(args, "%"+search+"%")
				argIdx++
			}

			query += " ORDER BY e.created_at DESC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar ujian: " + err.Error()})
				return
			}
			defer rows.Close()

			type ExamListItem struct {
				ID                string          `json:"id"`
				Title             string          `json:"title"`
				Description       string          `json:"description"`
				Subject           string          `json:"subject"`
				ExamType          string          `json:"exam_type"`
				GradeLevel        string          `json:"grade_level"`
				Phase             string          `json:"phase"`
				DurationMinutes   int             `json:"duration_minutes"`
				TotalQuestions    int             `json:"total_questions"`
				TotalScore        float64         `json:"total_score"`
				PassingScore      float64         `json:"passing_score"`
				Status            string          `json:"status"`
				IsActive          bool            `json:"is_active"`
				CreatedAt         time.Time       `json:"created_at"`
				UpdatedAt         time.Time       `json:"updated_at"`
				PublishedAt       *time.Time      `json:"published_at"`
				AcademicYearName  string          `json:"academic_year_name"`
				Semester          string          `json:"semester"`
				Targets           json.RawMessage `json:"targets"`
			}

			var exams []ExamListItem
			for rows.Next() {
				var e ExamListItem
				var desc, gradeLevel, phase, ayName, semester sql.NullString
				var passingScore sql.NullFloat64
				var publishedAt sql.NullTime
				var targetsJSON []byte

				err := rows.Scan(
					&e.ID, &e.Title, &desc, &e.Subject, &e.ExamType,
					&gradeLevel, &phase,
					&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &passingScore,
					&e.Status, &e.IsActive,
					&e.CreatedAt, &e.UpdatedAt, &publishedAt,
					&ayName, &semester, &targetsJSON,
				)
				if err != nil {
					continue
				}

				e.Description = desc.String
				e.GradeLevel = gradeLevel.String
				e.Phase = phase.String
				e.AcademicYearName = ayName.String
				e.Semester = semester.String
				if passingScore.Valid {
					e.PassingScore = passingScore.Float64
				}
				if publishedAt.Valid {
					e.PublishedAt = &publishedAt.Time
				}
				e.Targets = json.RawMessage(targetsJSON)

				exams = append(exams, e)
			}

			c.JSON(http.StatusOK, gin.H{
				"total": len(exams),
				"exams": exams,
			})
}

// POST /api/school-admin/exams
func ListExams(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
		}

	examID := c.Param("id")

	var e struct {
		ID                string
		Title             string
		Description       sql.NullString
		Subject           string
		ExamType          string
		GradeLevel        sql.NullString
		Phase             sql.NullString
		AcademicYearID    sql.NullString
		DurationMinutes   int
		TotalQuestions    int
		TotalScore        float64
		PassingScore      sql.NullFloat64
		AutoGradeMC       bool
		ManualGradeEssay  bool
		Status            string
		IsActive          bool
		CreatedAt         time.Time
		UpdatedAt         time.Time
		PublishedAt       sql.NullTime
		Questions         []byte
		ScoringConfig     []byte
		AcademicYearName  sql.NullString
		Semester          sql.NullString		

	}

	err = database.DB.QueryRow(`
		SELECT 
			e.id, e.title, e.description, e.subject, e.exam_type,
			e.grade_level, e.phase, e.academic_year_id,
			e.duration_minutes, e.total_questions, e.total_score, e.passing_score,
			e.auto_grade_mc, e.manual_grade_essay,
			e.status, e.is_active,
			e.created_at, e.updated_at, e.published_at,
			e.questions, e.scoring_config,
			say.name, say.semester
		FROM school_exams e
			LEFT JOIN school_academic_years say ON e.academic_year_id = say.id
		WHERE e.id = $1 AND e.school_id = $2 AND e.deleted_at IS NULL
			`, examID, schoolID).Scan(
				&e.ID, &e.Title, &e.Description, &e.Subject, &e.ExamType,
				&e.GradeLevel, &e.Phase, &e.AcademicYearID,
				&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &e.PassingScore,
				&e.AutoGradeMC, &e.ManualGradeEssay,
				&e.Status, &e.IsActive,
				&e.CreatedAt, &e.UpdatedAt, &e.PublishedAt,
				&e.Questions, &e.ScoringConfig,
				&e.AcademicYearName, &e.Semester,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Ambil targets
		targetRows, err := database.DB.Query(`
			SELECT 
				et.id, et.class_group_id, et.class_sub_group_id,
				COALESCE(cg.name, '') AS class_group_name,
				COALESCE(csg.name, '') AS class_sub_group_name,
				COALESCE(cg.level, '') AS class_level
			FROM exam_targets et
				LEFT JOIN class_groups cg ON et.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
				WHERE et.exam_id = $1
			`, examID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer targetRows.Close()

		type TargetItem struct {
			ID                 string  `json:"id"`
			ClassGroupID       *string `json:"class_group_id"`
			ClassSubGroupID    *string `json:"class_sub_group_id"`
			ClassGroupName     string  `json:"class_group_name"`
			ClassSubGroupName  string  `json:"class_sub_group_name"`
			ClassLevel         string  `json:"class_level"`
		}

		var targets []TargetItem
		for targetRows.Next() {
			var t TargetItem
			var cgID, csgID sql.NullString
			if err := targetRows.Scan(&t.ID, &cgID, &csgID, &t.ClassGroupName, &t.ClassSubGroupName, &t.ClassLevel); err == nil {
				if cgID.Valid { t.ClassGroupID = &cgID.String }
				if csgID.Valid { t.ClassSubGroupID = &csgID.String }
				targets = append(targets, t)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"exam": gin.H{
				"id":                 e.ID,
				"title":              e.Title,
				"description":        e.Description.String,
				"subject":            e.Subject,
				"exam_type":          e.ExamType,
				"grade_level":        e.GradeLevel.String,
				"phase":              e.Phase.String,
				"academic_year_id":   e.AcademicYearID.String,
				"academic_year_name": e.AcademicYearName.String,
				"semester":           e.Semester.String,
				"duration_minutes":   e.DurationMinutes,
				"total_questions":    e.TotalQuestions,
				"total_score":        e.TotalScore,
				"passing_score":      e.PassingScore.Float64,
				"auto_grade_mc":      e.AutoGradeMC,
				"manual_grade_essay": e.ManualGradeEssay,
				"status":             e.Status,
				"is_active":          e.IsActive,
				"created_at":         e.CreatedAt,
				"updated_at":         e.UpdatedAt,
				"published_at":       e.PublishedAt.Time,
				"questions":          json.RawMessage(e.Questions),
				"scoring_config":     json.RawMessage(e.ScoringConfig),
				"targets":            targets,
			},
		})
}

// POST /api/school-admin/exams
func PostExams(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateSchoolExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi: harus ada minimal 1 target
			if len(req.Targets) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal pilih 1 kelas/sub kelas target"})
				return
			}

			// Hitung total score dari semua questions
			var totalScore float64
			for _, q := range req.Questions {
				totalScore += q.Score
			}
			if totalScore == 0 {
				totalScore = 100 // fallback
			}

			examType := req.ExamType
			if examType == "" { examType = "regular" }

			scoringConfig := req.ScoringConfig
			if len(scoringConfig) == 0 {
				scoringConfig = json.RawMessage(`{"multiple_choice":1,"essay":5,"essay_rubric":"manual"}`)
			}

			questionsJSON, _ := json.Marshal(req.Questions)

			// Mulai transaksi
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Insert exam header
			var examID string
			err = tx.QueryRow(`
				INSERT INTO school_exams (
					school_id, academic_year_id, created_by,
					title, description, subject, exam_type, grade_level, phase,
					duration_minutes, total_questions, total_score, passing_score,
					scoring_config, questions, status
				) VALUES (
					$1, NULLIF($2,'')::uuid, $3,
					$4, NULLIF($5,''), $6, $7, NULLIF($8,''), NULLIF($9,''),
					$10, $11, $12, $13,
					$14::jsonb, $15::jsonb, 'draft'
				) RETURNING id
			`, schoolID, req.AcademicYearID, adminID,
				req.Title, req.Description, req.Subject, examType, req.GradeLevel, req.Phase,
				req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
				scoringConfig, questionsJSON,
			).Scan(&examID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan ujian: " + err.Error()})
				return
			}

			// 2. Insert targets (validasi tiap sub kelas milik sekolah ini)
			for _, t := range req.Targets {
				// Kalau ada sub_group_id, validasi dulu
				if t.ClassSubGroupID != "" {
					var validSchoolID string
					err := tx.QueryRow(`
						SELECT school_id FROM class_sub_groups WHERE id = $1
					`, t.ClassSubGroupID).Scan(&validSchoolID)
					if err != nil || validSchoolID != schoolID {
						c.JSON(http.StatusBadRequest, gin.H{
							"error": fmt.Sprintf("Sub kelas %s tidak valid / bukan milik sekolah Anda", t.ClassSubGroupID),
						})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO exam_targets (exam_id, class_group_id, class_sub_group_id)
					VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid)
					ON CONFLICT (exam_id, class_sub_group_id) DO NOTHING
				`, examID, t.ClassGroupID, t.ClassSubGroupID)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan target: " + err.Error()})
					return
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message": "Ujian berhasil dibuat (draft)",
				"id":      examID,
			})
}

// PUT /api/school-admin/exams/:id
// Hanya bisa update kalau status = 'draft'
func UpdateExams(c *gin.Context) {
		schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Param("id")

			// Cek status dulu
			var currentStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, examID, schoolID).Scan(&currentStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}

			if currentStatus != "draft" {
				c.JSON(http.StatusConflict, gin.H{
					"error": "Hanya ujian berstatus draft yang bisa diubah. Arsipkan atau buat baru.",
				})
				return
			}

			var req CreateSchoolExamRequest
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

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Update header
			_, err = tx.Exec(`
				UPDATE school_exams SET
					academic_year_id = NULLIF($1,'')::uuid,
					title = $2,
					description = NULLIF($3,''),
					subject = $4,
					exam_type = $5,
					grade_level = NULLIF($6,''),
					phase = NULLIF($7,''),
					duration_minutes = $8,
					total_questions = $9,
					total_score = $10,
					passing_score = $11,
					scoring_config = $12::jsonb,
					questions = $13::jsonb,
					updated_at = NOW()
				WHERE id = $14 AND school_id = $15
			`, req.AcademicYearID, req.Title, req.Description, req.Subject, examType,
				req.GradeLevel, req.Phase,
				req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
				scoringConfig, questionsJSON,
				examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update ujian: " + err.Error()})
				return
			}

			// 2. Hapus targets lama, insert ulang
			_, _ = tx.Exec(`DELETE FROM exam_targets WHERE exam_id = $1`, examID)

			for _, t := range req.Targets {
				if t.ClassSubGroupID != "" {
					var validSchoolID string
					err := tx.QueryRow(`
						SELECT school_id FROM class_sub_groups WHERE id = $1
					`, t.ClassSubGroupID).Scan(&validSchoolID)
					if err != nil || validSchoolID != schoolID {
						c.JSON(http.StatusBadRequest, gin.H{
							"error": fmt.Sprintf("Sub kelas %s tidak valid", t.ClassSubGroupID),
						})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO exam_targets (exam_id, class_group_id, class_sub_group_id)
					VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid)
					ON CONFLICT (exam_id, class_sub_group_id) DO NOTHING
				`, examID, t.ClassGroupID, t.ClassSubGroupID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update target: " + err.Error()})
					return
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil diperbarui"})	
}

// PATCH /api/school-admin/exams/:id/publish
func PublishExams(c *gin.Context) { 
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Param("id")

			// Validasi: minimal ada 1 soal & 1 target
			var totalQ int
			var targetCount int
			err = database.DB.QueryRow(`
				SELECT total_questions FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, examID, schoolID).Scan(&totalQ)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}

			_ = database.DB.QueryRow(`SELECT COUNT(*) FROM exam_targets WHERE exam_id = $1`, examID).Scan(&targetCount)

			if totalQ == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak bisa publish: ujian belum punya soal"})
				return
			}
			if targetCount == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak bisa publish: belum ada target kelas"})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET status = 'published', published_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND status = 'draft'
			`, examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Ujian sudah dipublish atau status tidak valid"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil dipublikasikan"})

}

// PATCH /api/school-admin/exams/:id/archive
func ArchiveExams(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET status = 'archived', updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND status IN ('draft', 'published')
			`, c.Param("id"), schoolID)

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

// DELETE /api/school-admin/exams/:id
func DeleteExams(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, c.Param("id"), schoolID)

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

// GET /api/school-admin/exams/target-options
// Return semua class_group + sub_group milik sekolah (untuk dropdown saat buat ujian)
func ExamTargetClass(c *gin.Context) { 
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					cg.id AS class_group_id,
					cg.name AS class_group_name,
					cg.level,
					COALESCE(cg.class_type, 'Umum') AS class_type,
					say.id AS academic_year_id,
					COALESCE(say.name, '-') AS academic_year_name,
					COALESCE(say.semester, '-') AS semester,
					COALESCE(
						(SELECT json_agg(json_build_object(
							'id', csg.id,
							'name', csg.name,
							'code', csg.code,
							'capacity', csg.capacity
						))
						FROM class_sub_groups csg
						WHERE csg.class_group_id = cg.id AND csg.is_active = TRUE),
						'[]'::json
					) AS sub_classes
				FROM class_groups cg
				LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
				WHERE cg.school_id = $1
				ORDER BY cg.level ASC, cg.name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type ClassOption struct {
				ClassGroupID     string          `json:"class_group_id"`
				ClassGroupName   string          `json:"class_group_name"`
				Level            string          `json:"level"`
				ClassType        string          `json:"class_type"`
				AcademicYearID   string          `json:"academic_year_id"`
				AcademicYearName string          `json:"academic_year_name"`
				Semester         string          `json:"semester"`
				SubClasses       json.RawMessage `json:"sub_classes"`
			}

			var list []ClassOption
			for rows.Next() {
				var co ClassOption
				var level, classType, ayID, ayName, semester sql.NullString
				var subClassesJSON []byte

				err := rows.Scan(
					&co.ClassGroupID, &co.ClassGroupName, &level, &classType,
					&ayID, &ayName, &semester, &subClassesJSON,
				)
				if err != nil {
					continue
				}
				co.Level = level.String
				co.ClassType = classType.String
				co.AcademicYearID = ayID.String
				co.AcademicYearName = ayName.String
				co.Semester = semester.String
				co.SubClasses = json.RawMessage(subClassesJSON)

				list = append(list, co)
			}

			c.JSON(http.StatusOK, gin.H{
				"total": len(list),
				"classes": list,
			})
}

// GET /api/school-admin/exam-schedules?filter=upcoming&search=---
func ExamSchedulesList(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			filter := c.Query("filter")   // all | today | week | upcoming | past
			search := c.Query("search")

			query := `
				SELECT 
					es.id, es.exam_id,
					e.title AS exam_title, e.subject, es.duration_minutes,
					es.duration_mode,
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.access_code, es.status,
					cg.name AS class_group_name,
					csg.name AS class_sub_group_name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score, es.highest_score, es.lowest_score
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.school_id = $1 AND es.deleted_at IS NULL
			`
			args := []any{schoolID}
			argIdx := 2

			switch filter {
			case "today":
				query += " AND es.schedule_date = CURRENT_DATE"
			case "week":
				query += " AND es.schedule_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '7 days'"
			case "upcoming":
				query += " AND es.schedule_date >= CURRENT_DATE"
			case "past":
				query += " AND es.schedule_date < CURRENT_DATE"
			}

			if search != "" {
				query += fmt.Sprintf(
					" AND (e.title ILIKE $%d OR es.room ILIKE $%d OR es.supervisor_name ILIKE $%d)",
					argIdx, argIdx, argIdx,
				)
				args = append(args, "%"+search+"%")
				argIdx++
			}

			query += " ORDER BY es.schedule_date ASC, es.start_time ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat jadwal: " + err.Error()})
				return
			}
			defer rows.Close()

			type ScheduleItem struct {
				ID                 string          `json:"id"`
				ExamID             string          `json:"exam_id"`
				ExamTitle          string          `json:"exam_title"`
				Subject            string          `json:"subject"`
				DurationMinutes    int             `json:"duration_minutes"`
				DurationMode       string          `json:"duration_mode"`
				ScheduleDate       string          `json:"schedule_date"`
				StartTime          string          `json:"start_time"`
				EndTime            string          `json:"end_time"`
				Room               *string         `json:"room"`
				SupervisorName     *string         `json:"supervisor_name"`
				AccessCode         *string         `json:"access_code"`
				Status             string          `json:"status"`
				ClassGroupName     *string         `json:"class_group_name"`
				ClassSubGroupName  *string         `json:"class_sub_group_name"`
				TotalStudents      int             `json:"total_students"`
				TotalSubmitted     int             `json:"total_submitted"`
				TotalGraded        int             `json:"total_graded"`
				AverageScore       *float64        `json:"average_score"`
				HighestScore       *float64        `json:"highest_score"`
				LowestScore        *float64        `json:"lowest_score"`
			}

			var schedules []ScheduleItem
			for rows.Next() {
				var s ScheduleItem
				var scheduleDate time.Time
				var startTime, endTime time.Time
				var room, supervisorName, accessCode sql.NullString
				var classGroupName, classSubGroupName sql.NullString
				var avgScore, highScore, lowScore sql.NullFloat64

				err := rows.Scan(
					&s.ID, &s.ExamID,
					&s.ExamTitle, &s.Subject, &s.DurationMinutes,
					&s.DurationMode,  
					&scheduleDate, &startTime, &endTime,
					&room, &supervisorName, &accessCode, &s.Status,
					&classGroupName, &classSubGroupName,
					&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
					&avgScore, &highScore, &lowScore,
				)
				if err != nil {
					fmt.Printf("[GET SCHEDULES] Scan error: %v\n", err)
					continue
				}

				s.ScheduleDate = scheduleDate.Format("2006-01-02")
				s.StartTime = startTime.Format("15:04")
				s.EndTime = endTime.Format("15:04")

				if room.Valid { s.Room = &room.String }
				if supervisorName.Valid { s.SupervisorName = &supervisorName.String }
				if accessCode.Valid { s.AccessCode = &accessCode.String }
				if classGroupName.Valid { s.ClassGroupName = &classGroupName.String }
				if classSubGroupName.Valid { s.ClassSubGroupName = &classSubGroupName.String }
				if avgScore.Valid { s.AverageScore = &avgScore.Float64 }
				if highScore.Valid { s.HighestScore = &highScore.Float64 }
				if lowScore.Valid { s.LowestScore = &lowScore.Float64 }

				schedules = append(schedules, s)
			}

			if schedules == nil {
				schedules = []ScheduleItem{}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":     len(schedules),
				"schedules": schedules,
			})

}

// Schedule Exam Detail GET /api/school-admin/exam-schedules/:id
func ExamSchedulesDetail(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule + exam info
			var s struct {
				ID                string
				ExamID            string
				ExamTitle         string
				ExamDescription   sql.NullString
				Subject           string
				ExamType          string
				GradeLevel        sql.NullString
				Phase             sql.NullString
				TotalQuestions    int
				TotalScore        float64
				PassingScore      sql.NullFloat64
				DurationMinutes   int
				DurationMode 	  string
				ScheduleDate      time.Time
				StartTime         time.Time
				EndTime           time.Time

				Room              sql.NullString
				SupervisorName    sql.NullString
				SessionNotes      sql.NullString

				AccessCode        sql.NullString
				RequireLogin      bool

				Status            string

				ClassGroupID      sql.NullString
				ClassGroupName    sql.NullString
				ClassSubGroupID   sql.NullString
				ClassSubGroupName sql.NullString

				TotalStudents     int
				TotalSubmitted    int
				TotalGraded       int
				AverageScore      sql.NullFloat64
				HighestScore      sql.NullFloat64
				LowestScore       sql.NullFloat64

				CreatedAt         time.Time
				UpdatedAt         time.Time
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id,
					e.title, e.description, e.subject, e.exam_type,
					e.grade_level, e.phase,
					e.total_questions, e.total_score, e.passing_score,
					es.duration_minutes,
					es.duration_mode,      
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.session_notes,
					es.access_code, es.require_login,
					es.status,
					es.class_group_id, cg.name,
					es.class_sub_group_id, csg.name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score, es.highest_score, es.lowest_score,
					es.created_at, es.updated_at
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID,
				&s.ExamTitle, &s.ExamDescription, &s.Subject, &s.ExamType,
				&s.GradeLevel, &s.Phase,
				&s.TotalQuestions, &s.TotalScore, &s.PassingScore,
				&s.DurationMinutes,
				&s.DurationMode,  
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName, &s.SessionNotes,
				&s.AccessCode, &s.RequireLogin,
				&s.Status,
				&s.ClassGroupID, &s.ClassGroupName,
				&s.ClassSubGroupID, &s.ClassSubGroupName,
				&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
				&s.AverageScore, &s.HighestScore, &s.LowestScore,
				&s.CreatedAt, &s.UpdatedAt,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Ambil daftar peserta (semua siswa di class_sub_group ini)
			type StudentParticipant struct {
				StudentID      string   `json:"student_id"`
				FullName       string   `json:"full_name"`
				NISN           string   `json:"nisn"`
				StudentNumber  string   `json:"student_number"`
				SubmissionID   *string  `json:"submission_id"`
				Status         string   `json:"status"`
				TotalScore     *float64 `json:"total_score"`
				Percentage     *float64 `json:"percentage"`
				IsPassed       *bool    `json:"is_passed"`
			}

			students := []StudentParticipant{}

			if s.ClassSubGroupID.Valid {
				rows, err := database.DB.Query(`
					SELECT 
						s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
						sub.id, sub.status,
						sub.total_score, sub.percentage, sub.is_passed
					FROM students s
					LEFT JOIN exam_submissions sub 
						ON sub.student_id = s.id AND sub.schedule_id = $1
					WHERE s.class_sub_group_id = $2 
					AND s.school_id = $3
					AND s.is_active = TRUE
					ORDER BY s.full_name ASC
				`, scheduleID, s.ClassSubGroupID.String, schoolID)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				defer rows.Close()

				for rows.Next() {
					var p StudentParticipant
					var subID, subStatus sql.NullString
					var totalScore, percentage sql.NullFloat64
					var isPassed sql.NullBool

					err := rows.Scan(
						&p.StudentID, &p.FullName, &p.NISN, &p.StudentNumber,
						&subID, &subStatus,
						&totalScore, &percentage, &isPassed,
					)
					if err != nil {
						continue
					}

					if subID.Valid {
						p.SubmissionID = &subID.String
					}
					if subStatus.Valid {
						p.Status = subStatus.String
					} else {
						p.Status = "not_started"
					}
					if totalScore.Valid { p.TotalScore = &totalScore.Float64 }
					if percentage.Valid { p.Percentage = &percentage.Float64 }
					if isPassed.Valid { p.IsPassed = &isPassed.Bool }

					students = append(students, p)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":                   s.ID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"exam_description":     s.ExamDescription.String,
					"subject":              s.Subject,
					"exam_type":            s.ExamType,
					"grade_level":          s.GradeLevel.String,
					"phase":                s.Phase.String,
					"total_questions":      s.TotalQuestions,
					"total_score":          s.TotalScore,
					"passing_score":        s.PassingScore.Float64,
					"duration_minutes":     s.DurationMinutes,
					"schedule_date":        s.ScheduleDate.Format("2006-01-02"),
					"start_time":           s.StartTime.Format("15:04"),
					"end_time":             s.EndTime.Format("15:04"),
					"room":                 s.Room.String,
					"supervisor_name":      s.SupervisorName.String,
					"session_notes":        s.SessionNotes.String,
					"access_code":          s.AccessCode.String,
					"require_login":        s.RequireLogin,
					"status":               s.Status,
					"class_group_id":       s.ClassGroupID.String,
					"class_group_name":     s.ClassGroupName.String,
					"class_sub_group_id":   s.ClassSubGroupID.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"total_students":       s.TotalStudents,
					"total_submitted":      s.TotalSubmitted,
					"total_graded":         s.TotalGraded,
					"average_score":        s.AverageScore.Float64,
					"highest_score":        s.HighestScore.Float64,
					"lowest_score":         s.LowestScore.Float64,
					"created_at":           s.CreatedAt,
					"updated_at":           s.UpdatedAt,
				},
				"students": students,
			})
}

// GET /api/school-admin/exam-schedules/available-targets?exam_id=xxx
func ExamSchedulesTarget(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Query("exam_id")
			if examID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "exam_id wajib diisi"})
				return
			}

			// Validasi exam milik sekolah ini & published
			var examExists bool
			err = database.DB.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM school_exams 
					WHERE id = $1 AND school_id = $2 
					AND deleted_at IS NULL
					AND status = 'published'
				)
			`, examID, schoolID).Scan(&examExists)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal validasi ujian: " + err.Error()})
				return
			}
			if !examExists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan atau belum dipublish"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					et.class_group_id,
					COALESCE(cg.name, '') AS class_group_name,
					et.class_sub_group_id,
					COALESCE(csg.name, '') AS class_sub_group_name,
					COALESCE(cg.level, '') AS class_level,
					(
						SELECT COUNT(*) FROM students s 
						WHERE s.class_sub_group_id = et.class_sub_group_id 
						AND s.is_active = TRUE
					) AS student_count,
					(
						SELECT COUNT(*) FROM exam_schedules es
						WHERE es.exam_id = et.exam_id
						AND es.class_sub_group_id = et.class_sub_group_id
						AND es.status IN ('scheduled', 'ongoing')
						AND es.deleted_at IS NULL
					) AS active_schedule_count
				FROM exam_targets et
				LEFT JOIN class_groups cg ON et.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
				WHERE et.exam_id = $1 
				AND et.class_sub_group_id IS NOT NULL
				AND EXISTS (
					SELECT 1 FROM class_groups 
					WHERE id = et.class_group_id AND school_id = $2
				)
				ORDER BY cg.level ASC, cg.name ASC, csg.name ASC
			`, examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat target: " + err.Error()})
				return
			}
			defer rows.Close()

			type TargetItem struct {
				ClassGroupID          string `json:"class_group_id"`
				ClassGroupName        string `json:"class_group_name"`
				ClassSubGroupID       string `json:"class_sub_group_id"`
				ClassSubGroupName     string `json:"class_sub_group_name"`
				ClassLevel            string `json:"class_level"`
				StudentCount          int    `json:"student_count"`
				ActiveScheduleCount   int    `json:"active_schedule_count"`
				HasActiveSchedule     bool   `json:"has_active_schedule"`
			}

			targets := []TargetItem{}
			for rows.Next() {
				var t TargetItem
				var classGroupID, classSubGroupID sql.NullString

				err := rows.Scan(
					&classGroupID,
					&t.ClassGroupName,
					&classSubGroupID,
					&t.ClassSubGroupName,
					&t.ClassLevel,
					&t.StudentCount,
					&t.ActiveScheduleCount,
				)
				if err != nil {
					fmt.Printf("[AVAILABLE TARGETS] Scan error: %v\n", err)
					continue
				}

				t.ClassGroupID = classGroupID.String
				t.ClassSubGroupID = classSubGroupID.String
				t.HasActiveSchedule = t.ActiveScheduleCount > 0

				targets = append(targets, t)
			}

			if err := rows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterasi rows: " + err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"exam_id": examID,
				"total":   len(targets),
				"targets": targets,
			})

}

// POST /api/school-admin/exam-schedules
func ExamSchedulesCreate(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateScheduleRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// ==========================================
			// Validasi duration_mode
			// ==========================================
			durationMode := req.DurationMode
			if durationMode == "" {
				durationMode = "strict"
			}
			if durationMode != "strict" && durationMode != "flexible" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "duration_mode harus 'strict' atau 'flexible'",
				})
				return
			}

			if durationMode == "strict" {
				startT, err1 := parseTimeString(req.StartTime)
				endT, err2 := parseTimeString(req.EndTime)
				if err1 != nil || err2 != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Format jam tidak valid"})
					return
				}
				diff := int(endT.Sub(startT).Minutes())
				if diff <= 0 {
					diff += 24 * 60
				}
				if diff <= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Jam selesai harus lebih besar dari jam mulai"})
					return
				}
				req.DurationMinutes = diff
			}

			// ==========================================
			// Validasi exam published & milik sekolah
			// ==========================================
			var examStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, req.ExamID, schoolID).Scan(&examStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}
			if examStatus != "published" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya ujian published yang bisa dijadwalkan"})
				return
			}

			// ==========================================
			// Normalize access code
			// ==========================================
			accessCode := strings.ToUpper(strings.TrimSpace(req.AccessCode))
			if accessCode == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Kode akses wajib diisi"})
				return
			}

			// ==========================================
			// Cek duplikat access_code — HILANGKAN filter deleted_at
			// dan hanya cek untuk (access_code, class_sub_group_id) yang akan dipakai.
			// ==========================================
			for _, subGroupID := range req.TargetSubGroupIDs {
				var existingID string
				err := database.DB.QueryRow(`
					SELECT id FROM exam_schedules 
					WHERE access_code = $1 
					AND class_sub_group_id = $2
					AND deleted_at IS NULL
					LIMIT 1
				`, accessCode, subGroupID).Scan(&existingID)

				if err == nil && existingID != "" {
					c.JSON(http.StatusConflict, gin.H{
						"error": fmt.Sprintf(
							"Kode akses '%s' sudah dipakai untuk sub kelas ini. Generate ulang atau pakai kode lain.",
							accessCode,
						),
					})
					return
				}
			}

			// ==========================================
			// Tx + Insert
			// ==========================================
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			createdIDs := []string{}

			for _, subGroupID := range req.TargetSubGroupIDs {
				// Validasi sub group milik sekolah
				var validSchoolID string
				var classGroupID string
				err := tx.QueryRow(`
					SELECT school_id, class_group_id FROM class_sub_groups WHERE id = $1
				`, subGroupID).Scan(&validSchoolID, &classGroupID)
				if err != nil || validSchoolID != schoolID {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak valid: " + subGroupID})
					return
				}

				// Count students
				var totalStudents int
				_ = tx.QueryRow(`
					SELECT COUNT(*) FROM students 
					WHERE class_sub_group_id = $1 AND is_active = TRUE
				`, subGroupID).Scan(&totalStudents)

				// Lock check — SELECT FOR UPDATE biar anti race condition
				var dummy string
				_ = tx.QueryRow(`
					SELECT id FROM exam_schedules
					WHERE access_code = $1 AND class_sub_group_id = $2 AND deleted_at IS NULL
					FOR UPDATE
				`, accessCode, subGroupID).Scan(&dummy)

				var scheduleID string
				err = tx.QueryRow(`
					INSERT INTO exam_schedules (
						school_id, exam_id,
						class_group_id, class_sub_group_id,
						schedule_date, start_time, end_time, duration_minutes,
						duration_mode,
						room, supervisor_name, session_notes,
						access_code, require_login,
						status, total_students,
						created_by
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7, $8,
						$9,
						NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
						$13, $14, 'scheduled', $15, $16
					) RETURNING id
				`, schoolID, req.ExamID,
					classGroupID, subGroupID,
					req.ScheduleDate, req.StartTime, req.EndTime, req.DurationMinutes,
					durationMode,
					req.Room, req.SupervisorName, req.SessionNotes,
					accessCode, req.RequireLogin,
					totalStudents, adminID).Scan(&scheduleID)

				if err != nil {
					// Deteksi unique violation
					if strings.Contains(err.Error(), "23505") ||
					strings.Contains(err.Error(), "duplicate key") {
						c.JSON(http.StatusConflict, gin.H{
							"error": fmt.Sprintf(
								"Kode akses '%s' sudah dipakai untuk sub kelas ini. Generate ulang.",
								accessCode,
							),
						})
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan jadwal: " + err.Error()})
					return
				}
				createdIDs = append(createdIDs, scheduleID)
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":       fmt.Sprintf("%d jadwal berhasil dibuat", len(createdIDs)),
				"schedule_ids":  createdIDs,
				"access_code":   accessCode,
				"duration_mode": durationMode,
			})

}

// Monitoring GET /api/school-admin/exam-schedules
func ExamSchedulesMonitoring(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule info
			var s struct {
				ID                string
				ExamID            string
				ExamTitle         string
				Subject           string
				DurationMinutes   int
				DurationMode      string  
				ScheduleDate      time.Time
				StartTime         time.Time
				EndTime           time.Time
				Room              sql.NullString
				SupervisorName    sql.NullString
				AccessCode        sql.NullString
				Status            string
				ClassGroupName    sql.NullString
				ClassSubGroupName sql.NullString
				TotalStudents     int
				TotalSubmitted    int
				TotalGraded       int
				AverageScore      sql.NullFloat64
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id,
					e.title, e.subject, es.duration_minutes,
					es.duration_mode,   
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.access_code, es.status,
					cg.name, csg.name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID,
				&s.ExamTitle, &s.Subject, &s.DurationMinutes,
				&s.DurationMode,      
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName, &s.AccessCode, &s.Status,
				&s.ClassGroupName, &s.ClassSubGroupName,
				&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
				&s.AverageScore,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Ambil semua siswa target + LEFT JOIN submissions
			rows, err := database.DB.Query(`
				SELECT 
					s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
					COALESCE(csg.name, ''),
					sub.id, sub.status,
					sub.started_at, sub.submitted_at, sub.last_activity_at,
					sub.time_spent_seconds,
					sub.total_score, sub.percentage, sub.is_passed,
					COALESCE(sub.is_flagged, false),
					COALESCE(sub.tab_switch_count, 0),
					sub.ip_address,
					els.id AS session_id,
					els.session_token,
					COALESCE(els.is_blocked, false) AS is_blocked,
					els.blocked_reason,
					els.blocked_at
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN exam_submissions sub 
					ON sub.student_id = s.id AND sub.schedule_id = $1
				LEFT JOIN exam_live_sessions els
					ON els.student_id = s.id 
					AND els.schedule_id = $1
					AND els.submitted_at IS NULL
				WHERE s.school_id = $2 
				AND s.class_sub_group_id = (SELECT class_sub_group_id FROM exam_schedules WHERE id = $1)
				AND s.is_active = TRUE
				ORDER BY s.full_name ASC
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type StudentRow struct {
				StudentID         string     `json:"student_id"`
				StudentName       string     `json:"student_name"`
				NISN              string     `json:"nisn"`
				StudentNumber     string     `json:"student_number"`
				ClassSubGroupName string     `json:"class_sub_group_name"`
				SubmissionID      *string    `json:"submission_id"`
				Status            string     `json:"status"`
				StartedAt         *time.Time `json:"started_at"`
				SubmittedAt       *time.Time `json:"submitted_at"`
				LastActivityAt    *time.Time `json:"last_activity_at"`
				TimeSpentSeconds  *int       `json:"time_spent_seconds"`
				TotalScore        *float64   `json:"total_score"`
				Percentage        *float64   `json:"percentage"`
				IsPassed          *bool      `json:"is_passed"`
				IsFlagged         bool       `json:"is_flagged"`
				TabSwitchCount    int        `json:"tab_switch_count"`
				IPAddress         *string    `json:"ip_address"`

				// Realtime control
				SessionID      *string    `json:"session_id"`
				SessionToken   *string    `json:"session_token"`
				IsBlocked      bool       `json:"is_blocked"`
				BlockedReason  *string    `json:"blocked_reason"`
				BlockedAt      *time.Time `json:"blocked_at"`
			}

			students := []StudentRow{}
			var inProgress, submitted, graded, notStarted, flagged, blocked int
			var totalScoreSum float64
			var scoreCount int

			for rows.Next() {
				var r StudentRow
				var subID sql.NullString
				var status sql.NullString
				var startedAt, submittedAt, lastActivityAt sql.NullTime
				var timeSpent sql.NullInt64
				var totalScore, percentage sql.NullFloat64
				var isPassed sql.NullBool
				var ipAddress sql.NullString

				// New fields
				var sessionID sql.NullString
				var sessionToken sql.NullString
				var isBlocked sql.NullBool
				var blockedReason sql.NullString
				var blockedAt sql.NullTime

				err := rows.Scan(
					&r.StudentID, &r.StudentName, &r.NISN, &r.StudentNumber,
					&r.ClassSubGroupName,
					&subID, &status,
					&startedAt, &submittedAt, &lastActivityAt,
					&timeSpent,
					&totalScore, &percentage, &isPassed,
					&r.IsFlagged, &r.TabSwitchCount,
					&ipAddress,
					&sessionID,
					&sessionToken,
					&isBlocked,
					&blockedReason,
					&blockedAt,
				)
				if err != nil {
					fmt.Printf("[MONITOR] Scan error: %v\n", err)  // ← tambah log
					continue
				}

				// ==========================================
				// ASSIGN SESSION FIELDS DULU
				// ==========================================
				if sessionID.Valid { r.SessionID = &sessionID.String }
				if sessionToken.Valid { r.SessionToken = &sessionToken.String }
				r.IsBlocked = isBlocked.Bool
				if blockedReason.Valid { r.BlockedReason = &blockedReason.String }
				if blockedAt.Valid { r.BlockedAt = &blockedAt.Time }

				// ==========================================
				// TENTUKAN STATUS (URUTAN PENTING!)
				// ==========================================
				if subID.Valid {
					// Sudah submit → pakai status dari submission
					r.SubmissionID = &subID.String
					r.Status = status.String
				} else if sessionID.Valid {
					// Ada session aktif tapi belum submit → SEDANG MENGERJAKAN
					r.Status = "in_progress"
				} else {
					// Tidak ada submission, tidak ada session → belum mulai
					r.Status = "not_started"
				}

				// ==========================================
				// ASSIGN FIELD LAINNYA
				// ==========================================
				if startedAt.Valid { r.StartedAt = &startedAt.Time }
				if submittedAt.Valid { r.SubmittedAt = &submittedAt.Time }
				if lastActivityAt.Valid { r.LastActivityAt = &lastActivityAt.Time }
				if timeSpent.Valid {
					t := int(timeSpent.Int64)
					r.TimeSpentSeconds = &t
				}
				if totalScore.Valid { r.TotalScore = &totalScore.Float64 }
				if percentage.Valid { r.Percentage = &percentage.Float64 }
				if isPassed.Valid { r.IsPassed = &isPassed.Bool }
				if ipAddress.Valid { r.IPAddress = &ipAddress.String }

				// ==========================================
				// AGGREGATE STATS
				// ==========================================
				switch r.Status {
				case "in_progress":
					inProgress++
				case "submitted":
					submitted++
				case "graded", "graded_with_pending":
					graded++
				case "not_started":
					notStarted++
				}
				if r.IsFlagged {
					flagged++
				}
				if r.IsBlocked {
					blocked++
				}
				if r.Percentage != nil {
					totalScoreSum += *r.Percentage
					scoreCount++
				}

				students = append(students, r)
			}

			var avgScore *float64
			if scoreCount > 0 {
				avg := totalScoreSum / float64(scoreCount)
				avgScore = &avg
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":                   s.ID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"subject":              s.Subject,
					"duration_minutes":     s.DurationMinutes,
					"duration_mode":        s.DurationMode,  
					"schedule_date":        s.ScheduleDate.Format("2006-01-02"),
					"start_time":           s.StartTime.Format("15:04"),
					"end_time":             s.EndTime.Format("15:04"),
					"room":                 s.Room.String,
					"supervisor_name":      s.SupervisorName.String,
					"access_code":          s.AccessCode.String,
					"status":               s.Status,
					"class_group_name":     s.ClassGroupName.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"total_students":       s.TotalStudents,
					"total_submitted":      s.TotalSubmitted,
					"total_graded":         s.TotalGraded,
					"average_score":        s.AverageScore.Float64,
				},
				"students": students,
				"stats": gin.H{
					"total_students": len(students),
					"in_progress":    inProgress,
					"submitted":      submitted,
					"graded":         graded,
					"not_started":    notStarted,
					"flagged":        flagged,
					"blocked":        blocked,
					"average_score":  avgScore,
				},
			})

}

// Close Session PATCH /api/school-admin/exam-schedules/:id/close
func ExamCloseSessions(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			var currentStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM exam_schedules 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, scheduleID, schoolID).Scan(&currentStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			if currentStatus == "completed" || currentStatus == "cancelled" {
				c.JSON(http.StatusConflict, gin.H{"error": "Sesi sudah tidak aktif"})
				return
			}

			// 1. Auto-submit semua submission yang masih in_progress
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			result, err := tx.Exec(`
				UPDATE exam_submissions 
				SET status = 'submitted', 
					submitted_at = NOW(),
					last_activity_at = NOW()
				WHERE schedule_id = $1 AND status = 'in_progress'
			`, scheduleID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			autoSubmitted, _ := result.RowsAffected()

			// 2. Set schedule status = completed
			_, err = tx.Exec(`
				UPDATE exam_schedules 
				SET status = 'completed', updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Auto-unblock semua siswa yang masih diblokir
			_, _ = tx.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = false,
					unblocked_at = NOW(),
					updated_at = NOW()
				WHERE schedule_id = $1 AND is_blocked = true
			`, scheduleID)
			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":         "Sesi berhasil ditutup",
				"auto_submitted":  autoSubmitted,
			})

}

// Cancel Exam Schedule PATCH /api//school-admin/exam-schedules/:id/cancel
func ExamSchedulesCancel(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// Ambil body opsional: { reason }
			var req struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&req)

			// 1. Ambil status & info jadwal
			var currentStatus string
			var examTitle, classSubGroupName string
			err = database.DB.QueryRow(`
				SELECT 
					es.status,
					e.title,
					COALESCE(csg.name, '')
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(&currentStatus, &examTitle, &classSubGroupName)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Cek status dasar
			if currentStatus == "cancelled" {
				c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah dibatalkan sebelumnya"})
				return
			}
			if currentStatus == "completed" {
				c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah selesai, tidak bisa dibatalkan"})
				return
			}

			// 3. Cek apakah ada siswa yang sudah mulai mengerjakan
			//    (session aktif tanpa submitted_at, ATAU sudah ada submission)
			var activeSessions int
			var totalSubmissions int

			_ = database.DB.QueryRow(`
				SELECT COUNT(*) 
				FROM exam_live_sessions
				WHERE schedule_id = $1 AND submitted_at IS NULL
			`, scheduleID).Scan(&activeSessions)

			_ = database.DB.QueryRow(`
				SELECT COUNT(*) 
				FROM exam_submissions
				WHERE schedule_id = $1
			`, scheduleID).Scan(&totalSubmissions)

			// 4. Validasi: tidak bisa dibatalkan kalau sudah ada aktivitas siswa
			if activeSessions > 0 || totalSubmissions > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf(
						"Tidak bisa dibatalkan. Sudah ada %d siswa yang memulai dan %d siswa yang submit. Gunakan 'Tutup Sesi' di halaman Monitor untuk menghentikan.",
						activeSessions, totalSubmissions,
					),
					"active_sessions":   activeSessions,
					"total_submissions": totalSubmissions,
				})
				return
			}

			// 5. Cek status jadwal
			if currentStatus == "ongoing" {
				c.JSON(http.StatusConflict, gin.H{
					"error": "Jadwal sudah berlangsung. Gunakan 'Tutup Sesi' di halaman Monitor untuk menghentikan.",
				})
				return
			}

			// 6. Aman dibatalkan → lakukan update
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// Update status jadwal
			result, err := tx.Exec(`
				UPDATE exam_schedules 
				SET status = 'cancelled', updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membatalkan jadwal: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			// Hapus semua live session yang belum submit (kalau ada sisa)
			// — seharusnya kosong, tapi ini safety net
			_, _ = tx.Exec(`
				DELETE FROM exam_live_sessions
				WHERE schedule_id = $1 AND submitted_at IS NULL
			`, scheduleID)

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			// 7. Log aktivitas (opsional)
			adminID, _ := getAdminIDFromUser(c)
			fmt.Printf("[CANCEL SCHEDULE] Admin %s membatalkan jadwal %s (%s) — %s\n",
				adminID, scheduleID, examTitle, classSubGroupName)

			c.JSON(http.StatusOK, gin.H{
				"message":     "Jadwal berhasil dibatalkan",
				"schedule_id": scheduleID,
				"reason":      req.Reason,
			})
}

// Delete Exam Schedule DELETE /api//school-admin/exam-schedules/:id
func ExamSchedulesDelete(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			result, err := database.DB.Exec(`
				UPDATE exam_schedules 
				SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Jadwal berhasil dihapus"})
}

// Detail Submission GET /api/school-admin/exam-submissions/:id
func ExamSubmissionDetail(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			submissionID := c.Param("id")

			var s struct {
				ID                string
				ScheduleID        string
				ExamID            string
				ExamTitle         string
				Subject           string
				StudentID         string
				StudentName       string
				StudentNISN       sql.NullString
				StudentNumber     sql.NullString
				ClassSubGroupName sql.NullString
				Status            string
				StartedAt         sql.NullTime
				SubmittedAt       sql.NullTime
				LastActivityAt    sql.NullTime
				TimeSpentSeconds  sql.NullInt64
				TotalScore        sql.NullFloat64
				MaxScore          sql.NullFloat64
				Percentage        sql.NullFloat64
				IsPassed          sql.NullBool
				TabSwitchCount    int
				IsFlagged         bool
				IPAddress         sql.NullString
				UserAgent         sql.NullString
				Answers           []byte
				PassingScore      sql.NullFloat64
			}

			err = database.DB.QueryRow(`
				SELECT 
					sub.id, sub.schedule_id, sub.exam_id,
					e.title, e.subject,
					sub.student_id, sub.student_name,
					sub.student_nisn, sub.student_number,
					csg.name,
					sub.status,
					sub.started_at, sub.submitted_at, sub.last_activity_at,
					sub.time_spent_seconds,
					sub.total_score, sub.max_score, sub.percentage, sub.is_passed,
					sub.tab_switch_count, sub.is_flagged,
					sub.ip_address, sub.user_agent,
					sub.answers,
					COALESCE(e.passing_score, 70)
				FROM exam_submissions sub
				JOIN school_exams e ON sub.exam_id = e.id
				LEFT JOIN students stu ON stu.id = sub.student_id
				LEFT JOIN class_sub_groups csg ON stu.class_sub_group_id = csg.id
				WHERE sub.id = $1 AND sub.school_id = $2
			`, submissionID, schoolID).Scan(
				&s.ID, &s.ScheduleID, &s.ExamID,
				&s.ExamTitle, &s.Subject,
				&s.StudentID, &s.StudentName,
				&s.StudentNISN, &s.StudentNumber,
				&s.ClassSubGroupName,
				&s.Status,
				&s.StartedAt, &s.SubmittedAt, &s.LastActivityAt,
				&s.TimeSpentSeconds,
				&s.TotalScore, &s.MaxScore, &s.Percentage, &s.IsPassed,
				&s.TabSwitchCount, &s.IsFlagged,
				&s.IPAddress, &s.UserAgent,
				&s.Answers,
				&s.PassingScore,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Submission tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			var answers []any
			_ = json.Unmarshal(s.Answers, &answers)

			c.JSON(http.StatusOK, gin.H{
				"submission": gin.H{
					"id":                   s.ID,
					"schedule_id":          s.ScheduleID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"subject":              s.Subject,
					"student_id":           s.StudentID,
					"student_name":         s.StudentName,
					"student_nisn":         s.StudentNISN.String,
					"student_number":       s.StudentNumber.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"status":               s.Status,
					"started_at":           s.StartedAt.Time,
					"submitted_at":         s.SubmittedAt.Time,
					"last_activity_at":     s.LastActivityAt.Time,
					"time_spent_seconds":   s.TimeSpentSeconds.Int64,
					"total_score":          s.TotalScore.Float64,
					"max_score":            s.MaxScore.Float64,
					"percentage":           s.Percentage.Float64,
					"is_passed":            s.IsPassed.Bool,
					"passing_score":        s.PassingScore.Float64,
					"tab_switch_count":     s.TabSwitchCount,
					"is_flagged":           s.IsFlagged,
					"ip_address":           s.IPAddress.String,
					"user_agent":           s.UserAgent.String,
					"answers":              answers,
					"questions":            []any{}, // optional: include question snapshots
				},
			})
}

// Exam Essay Grade PATCH /api/school-admin/exam-submissions/:id/grade-essay
func ExamGradeEssay(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			submissionID := c.Param("id")

			var req GradeEssayRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Ambil submission
			var scheduleID, examID, studentID string
			var answersJSON []byte
			var passingScore float64

			err = database.DB.QueryRow(`
				SELECT sub.schedule_id, sub.exam_id, sub.student_id, sub.answers,
					COALESCE(e.passing_score, 70)
				FROM exam_submissions sub
				JOIN school_exams e ON sub.exam_id = e.id
				WHERE sub.id = $1 AND sub.school_id = $2
			`, submissionID, schoolID).Scan(
				&scheduleID, &examID, &studentID, &answersJSON, &passingScore,
			)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Submission tidak ditemukan"})
				return
			}

			// 2. Parse answers, update skor essay
			var answers []map[string]any
			if err := json.Unmarshal(answersJSON, &answers); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Format answers error"})
				return
			}

			found := false
			var newTotalScore, newMaxScore float64

			for i, a := range answers {
				qID, _ := a["question_id"].(string)
				maxScore, _ := a["max_score"].(float64)
				newMaxScore += maxScore

				if qID == req.QuestionID {
					answers[i]["score"] = req.Score
					answers[i]["feedback"] = req.Feedback
					answers[i]["graded_at"] = time.Now().Format(time.RFC3339)
					found = true
					newTotalScore += req.Score
				} else {
					if sc, ok := a["score"].(float64); ok {
						newTotalScore += sc
					}
				}
			}

			if !found {
				c.JSON(http.StatusNotFound, gin.H{"error": "Soal tidak ditemukan di submission"})
				return
			}

			// 3. Cek apakah semua essay sudah dinilai
			hasPending := false
			for _, a := range answers {
				if a["type"] == "essay" {
					if _, ok := a["score"]; !ok || a["score"] == nil {
						hasPending = true
						break
					}
				}
			}

			// 4. Hitung ulang persentase
			percentage := 0.0
			if newMaxScore > 0 {
				percentage = (newTotalScore / newMaxScore) * 100
			}

			newStatus := "graded"
			if hasPending {
				newStatus = "graded_with_pending"
			}

			// 5. Update submission
			newAnswersJSON, _ := json.Marshal(answers)
			_, err = database.DB.Exec(`
				UPDATE exam_submissions SET
					answers = $1::jsonb,
					total_score = $2,
					percentage = $3,
					is_passed = $4,
					status = $5,
					updated_at = NOW(),
					fully_graded_at = CASE WHEN $6 = 'graded' THEN NOW() ELSE fully_graded_at END
				WHERE id = $7
			`, newAnswersJSON, newTotalScore, percentage, percentage >= passingScore,
				newStatus, newStatus, submissionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 6. Update schedule stats
			_, _ = database.DB.Exec(`
				UPDATE exam_schedules SET
					total_graded = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1 AND status = 'graded'),
					average_score = (SELECT AVG(percentage) FROM exam_submissions WHERE schedule_id = $1),
					updated_at = NOW()
				WHERE id = $1
			`, scheduleID)

			c.JSON(http.StatusOK, gin.H{
				"message":       "Penilaian berhasil disimpan",
				"total_score":   newTotalScore,
				"percentage":    percentage,
				"is_passed":     percentage >= passingScore,
				"status":        newStatus,
				"has_pending":   hasPending,
			})
}

// Exam Recap Grades GET /api/school-admin/exam-schedules/:id/grades
func ExamGradeRecap(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule info
			var s struct {
				ID              string
				ExamID          string
				ExamTitle       string
				Subject         string
				ScheduleDate    time.Time
				StartTime       time.Time
				EndTime         time.Time
				Room            sql.NullString
				SupervisorName  sql.NullString
				PassingScore    sql.NullFloat64
				ClassSubGroupID sql.NullString
				TotalStudents   int
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id, e.title, e.subject,
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name,
					COALESCE(e.passing_score, 70),
					es.class_sub_group_id,
					es.total_students
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID, &s.ExamTitle, &s.Subject,
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName,
				&s.PassingScore,
				&s.ClassSubGroupID,
				&s.TotalStudents,
			)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			// 2. Query semua siswa target + LEFT JOIN submissions
			rows, err := database.DB.Query(`
				SELECT 
					s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
					COALESCE(csg.name, ''),
					sub.id, sub.status,
					sub.total_score, sub.max_score, sub.percentage, sub.is_passed,
					sub.submitted_at, sub.time_spent_seconds,
					COALESCE(sub.tab_switch_count, 0),
					COALESCE(sub.is_flagged, false),
					sub.answers
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN exam_submissions sub 
					ON sub.student_id = s.id AND sub.schedule_id = $1
				WHERE s.school_id = $2 
				AND s.class_sub_group_id = (SELECT class_sub_group_id FROM exam_schedules WHERE id = $1)
				AND s.is_active = TRUE
				ORDER BY s.full_name ASC
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type StudentRow struct {
				StudentID         string   `json:"student_id"`
				StudentName       string   `json:"student_name"`
				StudentNISN       string   `json:"student_nisn"`
				StudentNumber     string   `json:"student_number"`
				ClassSubGroupName string   `json:"class_sub_group_name"`
				SubmissionID      *string  `json:"submission_id"`
				Status            string   `json:"status"`
				TotalScore        *float64 `json:"total_score"`
				MaxScore          *float64 `json:"max_score"`
				Percentage        *float64 `json:"percentage"`
				IsPassed          *bool    `json:"is_passed"`
				SubmittedAt       *time.Time `json:"submitted_at"`
				TimeSpentSeconds  *int     `json:"time_spent_seconds"`
				TabSwitchCount    int      `json:"tab_switch_count"`
				IsFlagged         bool     `json:"is_flagged"`
				PendingEssayCount int      `json:"pending_essay_count"`
			}

			students := []StudentRow{}
			var submitted, graded, pending, notStarted, passed, failed, flagged int
			var totalPct float64
			var pctCount int
			var highest, lowest *float64

			for rows.Next() {
				var r StudentRow
				var subID sql.NullString
				var status sql.NullString
				var totalScore, maxScore, percentage sql.NullFloat64
				var isPassed sql.NullBool
				var submittedAt sql.NullTime
				var timeSpent sql.NullInt64
				var answersJSON []byte

				err := rows.Scan(
					&r.StudentID, &r.StudentName, &r.StudentNISN, &r.StudentNumber,
					&r.ClassSubGroupName,
					&subID, &status,
					&totalScore, &maxScore, &percentage, &isPassed,
					&submittedAt, &timeSpent,
					&r.TabSwitchCount, &r.IsFlagged,
					&answersJSON,
				)
				if err != nil {
					continue
				}

				if subID.Valid { r.SubmissionID = &subID.String }
				if status.Valid { r.Status = status.String } else { r.Status = "not_started" }
				if totalScore.Valid { r.TotalScore = &totalScore.Float64 }
				if maxScore.Valid { r.MaxScore = &maxScore.Float64 }
				if percentage.Valid {
					r.Percentage = &percentage.Float64
					totalPct += percentage.Float64
					pctCount++
					if highest == nil || percentage.Float64 > *highest { v := percentage.Float64; highest = &v }
					if lowest == nil || percentage.Float64 < *lowest { v := percentage.Float64; lowest = &v }
				}
				if isPassed.Valid { r.IsPassed = &isPassed.Bool }
				if submittedAt.Valid { r.SubmittedAt = &submittedAt.Time }
				if timeSpent.Valid { t := int(timeSpent.Int64); r.TimeSpentSeconds = &t }

				// Count pending essays
				if len(answersJSON) > 0 {
					var answers []map[string]any
					if json.Unmarshal(answersJSON, &answers) == nil {
						for _, a := range answers {
							if a["type"] == "essay" {
								if _, ok := a["score"]; !ok || a["score"] == nil {
									r.PendingEssayCount++
								}
							}
						}
					}
				}

				// Aggregate
				switch r.Status {
				case "in_progress": notStarted++ // in_progress tidak masuk "submitted"
				case "submitted":
					submitted++
				case "graded", "graded_with_pending":
					submitted++
					graded++
					if r.Status == "graded_with_pending" { pending++ }
				case "not_started":
					notStarted++
				}
				if r.IsPassed != nil && *r.IsPassed { passed++ }
				if r.IsPassed != nil && !*r.IsPassed { failed++ }
				if r.IsFlagged { flagged++ }

				students = append(students, r)
			}

			var avg *float64
			if pctCount > 0 {
				a := totalPct / float64(pctCount)
				avg = &a
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":               s.ID,
					"exam_id":          s.ExamID,
					"exam_title":       s.ExamTitle,
					"subject":          s.Subject,
					"schedule_date":    s.ScheduleDate.Format("2006-01-02"),
					"start_time":       s.StartTime.Format("15:04"),
					"end_time":         s.EndTime.Format("15:04"),
					"room":             s.Room.String,
					"supervisor_name":  s.SupervisorName.String,
					"passing_score":    s.PassingScore.Float64,
					"total_students":   s.TotalStudents,
				},
				"students": students,
				"stats": gin.H{
					"total":        len(students),
					"submitted":    submitted,
					"graded":       graded,
					"pending":      pending,
					"not_started":  notStarted,
					"passed":       passed,
					"failed":       failed,
					"flagged":      flagged,
					"average":      avg,
					"highest":      highest,
					"lowest":       lowest,
				},
			})
}

// Exam Schedules Access Code GET /api/school-admin/exam-schedules/check-access-code
func ExamSchedulesAccesCode(c *gin.Context) {
	_, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			code := strings.ToUpper(strings.TrimSpace(c.Query("code")))
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "code wajib diisi"})
				return
			}

			var count int
			err = database.DB.QueryRow(`
				SELECT COUNT(*) FROM exam_schedules 
				WHERE access_code = $1 AND deleted_at IS NULL
			`, code).Scan(&count)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"access_code": code,
				"available":   count == 0,
				"used_count":  count,
			})
}


