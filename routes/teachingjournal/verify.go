package teachingjournal

import (
	"database/sql"
	"fmt"
	"net/http"
	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// getSchoolIDFromUser di-reimplementasi lokal (sama dengan routes.go)
func getSchoolIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}
	userIDStr := normalizeUserID(userID)

	var userEmail string
	_ = database.DB.QueryRow(`SELECT COALESCE(email,'') FROM auth.users WHERE id = $1`, userIDStr).Scan(&userEmail)

	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins
		WHERE (id = $1 OR (email <> '' AND email = $2)) AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)
	if err != nil {
		return "", fmt.Errorf("akses ditolak: admin sekolah tidak valid")
	}
	return schoolID, nil
}

// ============================================================
// GET /api/school-admin/journals
// ============================================================
func ListSchoolJournals(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	query := `
		SELECT tj.id, tj.journal_date, tj.subject_name_snapshot, tj.class_name_snapshot,
		       tj.period_start, tj.period_end, tj.topic, tj.method, tj.status,
		       tj.student_attendance_count, tj.total_students,
		       COALESCE(t.nama_lengkap,'') AS teacher_name,
		       COALESCE(t.nip,'') AS teacher_nip,
		       tj.submitted_at, tj.verified_at
		FROM teaching_journals tj
		JOIN teachers t ON t.id = tj.teacher_id
		WHERE tj.school_id = $1 AND tj.is_active = TRUE
	`
	args := []any{schoolID}
	idx := 2

	if v := c.Query("teacher_id"); v != "" {
		query += fmt.Sprintf(" AND tj.teacher_id = $%d::uuid", idx)
		args = append(args, v); idx++
	}
	if v := c.Query("class_sub_group_id"); v != "" {
		query += fmt.Sprintf(" AND tj.class_sub_group_id = $%d::uuid", idx)
		args = append(args, v); idx++
	}
	if v := c.Query("from"); v != "" {
		query += fmt.Sprintf(" AND tj.journal_date >= $%d::date", idx)
		args = append(args, v); idx++
	}
	if v := c.Query("to"); v != "" {
		query += fmt.Sprintf(" AND tj.journal_date <= $%d::date", idx)
		args = append(args, v); idx++
	}
	if v := c.Query("status"); v != "" {
		query += fmt.Sprintf(" AND tj.status = $%d", idx)
		args = append(args, v); idx++
	}

	query += " ORDER BY tj.journal_date DESC, tj.period_start ASC LIMIT 1000"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		writeInternalError(c, "Gagal memuat jurnal sekolah", err)
		return
	}
	defer rows.Close()

	type Item struct {
		ID              string `json:"id"`
		JournalDate     string `json:"journal_date"`
		SubjectName     string `json:"subject_name"`
		ClassName       string `json:"class_name"`
		PeriodStart     int    `json:"period_start"`
		PeriodEnd       int    `json:"period_end"`
		Topic           string `json:"topic"`
		Method          string `json:"method"`
		Status          string `json:"status"`
		TeacherName     string `json:"teacher_name"`
		TeacherNIP      string `json:"teacher_nip"`
		SubmittedAt     any    `json:"submitted_at"`
		VerifiedAt      any    `json:"verified_at"`
	}

	list := []Item{}
	for rows.Next() {
		var it Item
		var d, sub, ver sql.NullTime
		if err := rows.Scan(
			&it.ID, &d, &it.SubjectName, &it.ClassName,
			&it.PeriodStart, &it.PeriodEnd, &it.Topic, &it.Method, &it.Status,
			&it.TeacherName, &it.TeacherNIP, &sub, &ver,
		); err == nil {
			it.JournalDate = d.Time.Format("2006-01-02")
			it.SubmittedAt = timeOrNil(sub)
			it.VerifiedAt = timeOrNil(ver)
			list = append(list, it)
		}
	}

	c.JSON(http.StatusOK, gin.H{"journals": list, "total": len(list)})
}

// ============================================================
// GET /api/school-admin/journals/rekap?from=&to=
// ============================================================
func GetSchoolJournalRecap(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter from & to wajib diisi"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT
			t.id, t.nama_lengkap, COALESCE(t.nip,''),
			COUNT(tj.id) AS total,
			COUNT(*) FILTER (WHERE tj.status='draft') AS draft,
			COUNT(*) FILTER (WHERE tj.status='submitted') AS submitted,
			COUNT(*) FILTER (WHERE tj.status='verified') AS verified,
			COUNT(*) FILTER (WHERE tj.status='rejected') AS rejected
		FROM teachers t
		LEFT JOIN teaching_journals tj
		       ON tj.teacher_id = t.id
		      AND tj.journal_date BETWEEN $2::date AND $3::date
		      AND tj.is_active = TRUE
		WHERE t.school_id = $1 AND t.is_active = TRUE
		GROUP BY t.id, t.nama_lengkap, t.nip
		ORDER BY t.nama_lengkap ASC
	`, schoolID, from, to)
	if err != nil {
		writeInternalError(c, "Gagal memuat rekap", err)
		return
	}
	defer rows.Close()

	type Item struct {
		TeacherID    string `json:"teacher_id"`
		TeacherName  string `json:"teacher_name"`
		TeacherNIP   string `json:"teacher_nip"`
		Total        int    `json:"total"`
		Draft        int    `json:"draft"`
		Submitted    int    `json:"submitted"`
		Verified     int    `json:"verified"`
		Rejected     int    `json:"rejected"`
	}

	list := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.TeacherID, &it.TeacherName, &it.TeacherNIP,
			&it.Total, &it.Draft, &it.Submitted, &it.Verified, &it.Rejected); err == nil {
			list = append(list, it)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"from": from, "to": to,
		"rekap": list, "total_teachers": len(list),
	})
}

// ============================================================
// PATCH /api/school-admin/journals/:id/verify
// ============================================================
func VerifyTeachingJournal(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr := normalizeUserID(userID)
	journalID := c.Param("id")

	var req VerifyJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	newStatus := "verified"
	if req.Action == "reject" {
		newStatus = "rejected"
	}

	result, err := database.DB.Exec(`
		UPDATE teaching_journals SET
			status = $1,
			verified_by = $2,
			verified_at = NOW(),
			verification_note = NULLIF($3,''),
			updated_at = NOW()
		WHERE id = $4 AND school_id = $5
		  AND is_active = TRUE
		  AND status = 'submitted'
	`, newStatus, userIDStr, req.Note, journalID, schoolID)

	if err != nil {
		writeInternalError(c, "Gagal verifikasi jurnal", err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Jurnal tidak ditemukan atau belum berstatus submitted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Jurnal berhasil diverifikasi",
		"status":  newStatus,
	})
}