package attendance

import (
	//"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type GenerateQRRequest struct {
	StudentIDs []string `json:"student_ids"` 
	ClassSubGroupID string `json:"class_sub_group_id"` 
}

// GET /school-admin/students/qr-list
func GetStudentQRList(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	rows, err := database.DB.Query(`
		SELECT sqc.id, sqc.student_id, sqc.qr_token, sqc.is_active, sqc.generated_at, sqc.regenerated_count,
			s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, '')
		FROM student_qr_codes sqc
		JOIN students s ON sqc.student_id = s.id
		WHERE sqc.school_id = $1 ORDER BY s.full_name ASC
	`, schoolID)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat QR: " + err.Error()}); return }
	defer rows.Close()

	type QRItem struct {
		ID                string    `json:"id"`
		StudentID         string    `json:"student_id"`
		QRToken           string    `json:"qr_token"`
		IsActive          bool      `json:"is_active"`
		GeneratedAt       time.Time `json:"generated_at"`
		RegeneratedCount  int       `json:"regenerated_count"`
		StudentName       string    `json:"student_name"`
		NISN              string    `json:"nisn"`
		StudentNumber     string    `json:"student_number"`
	}

	var list []QRItem
	for rows.Next() {
		var q QRItem
		if err := rows.Scan(
			&q.ID, &q.StudentID, &q.QRToken, &q.IsActive, &q.GeneratedAt, &q.RegeneratedCount,
			&q.StudentName, &q.NISN, &q.StudentNumber,
		); err == nil {
			list = append(list, q)
		}
	}
	c.JSON(http.StatusOK, gin.H{"qr_codes": list, "total": len(list)})
}

// POST /school-admin/students/:id/generate-qr
func GenerateSingleQR(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)
	studentID := c.Param("id")

	var exists string
	err = database.DB.QueryRow(`SELECT id FROM students WHERE id = $1 AND school_id = $2`, studentID, schoolID).Scan(&exists)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan"}); return }

	var existingID string
	err = database.DB.QueryRow(`SELECT id FROM student_qr_codes WHERE student_id = $1`, studentID).Scan(&existingID)
	if err == nil && existingID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Siswa ini sudah punya QR code"}); return
	}

	token := generateQRToken()
	var qrID string
	err = database.DB.QueryRow(`
		INSERT INTO student_qr_codes (student_id, school_id, qr_token, generated_by)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, studentID, schoolID, token, adminID).Scan(&qrID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan QR: " + err.Error()}); return }
	c.JSON(http.StatusCreated, gin.H{"message": "QR berhasil dibuat", "id": qrID, "qr_token": token})
}

// POST /school-admin/students/generate-qr-bulk
func GenerateBulkQR(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)

	var req GenerateQRRequest
	_ = c.ShouldBindJSON(&req)

	query := `
		SELECT s.id FROM students s
		LEFT JOIN student_qr_codes sqc ON sqc.student_id = s.id
		WHERE s.school_id = $1 AND s.is_active = TRUE AND sqc.id IS NULL
	`
	args := []any{schoolID}
	argIdx := 2

	if req.ClassSubGroupID != "" {
		query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx)
		args = append(args, req.ClassSubGroupID)
		argIdx++
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query siswa: " + err.Error()}); return }
	defer rows.Close()

	var studentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil { studentIDs = append(studentIDs, id) }
	}

	if len(studentIDs) == 0 { c.JSON(http.StatusOK, gin.H{"message": "Semua siswa sudah punya QR", "generated": 0}); return }

	tx, err := database.DB.Begin()
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"}); return }
	defer tx.Rollback()

	generated := 0
	for _, sid := range studentIDs {
		token := generateQRToken()
		_, err := tx.Exec(`INSERT INTO student_qr_codes (student_id, school_id, qr_token, generated_by) VALUES ($1, $2, $3, $4)`, sid, schoolID, token, adminID)
		if err == nil { generated++ }
	}

	if err := tx.Commit(); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"}); return }
	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("%d QR berhasil dibuat", generated), "generated": generated})
}

// POST /school-admin/students/:id/regenerate-qr
func RegenerateQR(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	studentID := c.Param("id")

	var qrID string
	err = database.DB.QueryRow(`SELECT id FROM student_qr_codes WHERE student_id = $1 AND school_id = $2`, studentID, schoolID).Scan(&qrID)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "QR tidak ditemukan"}); return }

	newToken := generateQRToken()
	_, err = database.DB.Exec(`
		UPDATE student_qr_codes SET qr_token = $1, regenerated_count = regenerated_count + 1, generated_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, newToken, qrID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal regenerate: " + err.Error()}); return }
	c.JSON(http.StatusOK, gin.H{"message": "QR berhasil di-regenerate", "qr_token": newToken})
}