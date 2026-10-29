package school

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/middleware"

	"github.com/gin-gonic/gin"
)

// GET /api/school-admin/time-slots
func ListTimeSlots(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, period_number, start_time, end_time, is_break
		FROM school_time_slots 
		WHERE school_id = $1 
		ORDER BY period_number ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat master jam: " + err.Error()})
		return
	}
	defer rows.Close()

	type TimeSlotItem struct {
		ID           string `json:"id"`
		PeriodNumber int    `json:"period_number"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		IsBreak      bool   `json:"is_break"`
	}

	list := []TimeSlotItem{}
	for rows.Next() {
		var s TimeSlotItem
		var start, end time.Time
		if err := rows.Scan(&s.ID, &s.PeriodNumber, &start, &end, &s.IsBreak); err == nil {
			s.StartTime = start.Format("15:04")
			s.EndTime = end.Format("15:04")
			list = append(list, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"time_slots": list})
}

// POST /api/school-admin/time-slots
func CreateTimeSlot(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req CreateTimeSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Validasi bentrok period_number
	var exists bool
	_ = database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM school_time_slots WHERE school_id = $1 AND period_number = $2)
	`, schoolID, req.PeriodNumber).Scan(&exists)
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Jam ke-%d sudah didefinisikan", req.PeriodNumber)})
		return
	}

	var slotID string
	err = database.DB.QueryRow(`
		INSERT INTO school_time_slots (school_id, period_number, start_time, end_time, is_break)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, schoolID, req.PeriodNumber, req.StartTime, req.EndTime, req.IsBreak).Scan(&slotID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan master jam: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Master jam berhasil ditambahkan", "id": slotID})
}

// PUT /api/school-admin/time-slots/:id
func UpdateTimeSlot(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	slotID := c.Param("id")

	var req UpdateTimeSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Jika period_number diubah, validasi bentrok
	if req.PeriodNumber > 0 {
		var exists bool
		_ = database.DB.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM school_time_slots WHERE school_id = $1 AND period_number = $2 AND id <> $3)
		`, schoolID, req.PeriodNumber, slotID).Scan(&exists)
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Jam ke-%d sudah didefinisikan", req.PeriodNumber)})
			return
		}
	}

	query := `UPDATE school_time_slots SET updated_at = NOW()`
	args := []any{}
	argIdx := 1

	if req.PeriodNumber > 0 {
		query += fmt.Sprintf(", period_number = $%d", argIdx)
		args = append(args, req.PeriodNumber)
		argIdx++
	}
	if req.StartTime != "" {
		query += fmt.Sprintf(", start_time = $%d", argIdx)
		args = append(args, req.StartTime)
		argIdx++
	}
	if req.EndTime != "" {
		query += fmt.Sprintf(", end_time = $%d", argIdx)
		args = append(args, req.EndTime)
		argIdx++
	}
	if req.IsBreak != nil {
		query += fmt.Sprintf(", is_break = $%d", argIdx)
		args = append(args, *req.IsBreak)
		argIdx++
	}

	if len(args) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada data yang diupdate"})
		return
	}

	query += fmt.Sprintf(" WHERE id = $%d AND school_id = $%d", argIdx, argIdx+1)
	args = append(args, slotID, schoolID)

	result, err := database.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui master jam: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Master jam tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Master jam berhasil diperbarui"})
}

// DELETE /api/school-admin/time-slots/:id
func DeleteTimeSlot(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	slotID := c.Param("id")

	result, err := database.DB.Exec(`
		DELETE FROM school_time_slots WHERE id = $1 AND school_id = $2
	`, slotID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus master jam: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Master jam tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Master jam berhasil dihapus"})
}

// GET /api/school-admin/time-slot-overrides?day_of_week=5
func ListTimeSlotOverrides(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	dayFilter := c.Query("day_of_week")

	query := `
		SELECT id, day_of_week, period_number,
			start_time, end_time, is_break,
			COALESCE(notes, ''), created_at, updated_at
		FROM school_time_slot_overrides
		WHERE school_id = $1
	`
	args := []any{schoolID}

	if dayFilter != "" {
		query += " AND day_of_week = $2"
		args = append(args, dayFilter)
	}

	query += " ORDER BY day_of_week ASC, period_number ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memuat override: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	type OverrideItem struct {
		ID           string    `json:"id"`
		DayOfWeek    int       `json:"day_of_week"`
		PeriodNumber int       `json:"period_number"`
		StartTime    string    `json:"start_time"`
		EndTime      string    `json:"end_time"`
		IsBreak      bool      `json:"is_break"`
		Notes        string    `json:"notes"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	list := []OverrideItem{}
	for rows.Next() {
		var o OverrideItem
		var start, end time.Time
		if err := rows.Scan(
			&o.ID, &o.DayOfWeek, &o.PeriodNumber,
			&start, &end, &o.IsBreak,
			&o.Notes, &o.CreatedAt, &o.UpdatedAt,
		); err == nil {
			o.StartTime = start.Format("15:04")
			o.EndTime = end.Format("15:04")
			list = append(list, o)
		}
	}

	c.JSON(http.StatusOK, gin.H{"overrides": list, "total": len(list)})
}

// POST /api/school-admin/time-slot-overrides
func UpsertTimeSlotOverride(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req UpsertTimeSlotOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Format tidak valid: " + err.Error(),
		})
		return
	}

	// Validasi: period_number ini harus ada di default slots
	var defaultExists bool
	_ = database.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM school_time_slots
			WHERE school_id = $1 AND period_number = $2
		)
	`, schoolID, req.PeriodNumber).Scan(&defaultExists)

	if !defaultExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf(
				"Jam ke-%d belum ada di master default. Buat default dulu sebelum override.",
				req.PeriodNumber,
			),
		})
		return
	}

	// Validasi jam
	startT, err1 := parseTimeString(req.StartTime)
	endT, err2 := parseTimeString(req.EndTime)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format jam tidak valid"})
		return
	}
	if timeToMinutes(endT) <= timeToMinutes(startT) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Jam selesai harus lebih besar dari jam mulai",
		})
		return
	}

	var overrideID string
	err = database.DB.QueryRow(`
		INSERT INTO school_time_slot_overrides
			(school_id, day_of_week, period_number,
			start_time, end_time, is_break, notes)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		ON CONFLICT (school_id, day_of_week, period_number)
		DO UPDATE SET
			start_time = EXCLUDED.start_time,
			end_time = EXCLUDED.end_time,
			is_break = EXCLUDED.is_break,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id
	`, schoolID, req.DayOfWeek, req.PeriodNumber,
		req.StartTime, req.EndTime, req.IsBreak, req.Notes).Scan(&overrideID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan override: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Override berhasil disimpan",
		"override_id": overrideID,
	})
}

// PUT /api/school-admin/time-slot-overrides/:id
func UpdateTimeSlotOverride(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	overrideID := c.Param("id")

	var req UpsertTimeSlotOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE school_time_slot_overrides SET
			start_time = $1,
			end_time   = $2,
			is_break   = $3,
			notes      = NULLIF($4, ''),
			updated_at = NOW()
		WHERE id = $5 AND school_id = $6
	`, req.StartTime, req.EndTime, req.IsBreak, req.Notes,
		overrideID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Override tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Override berhasil diperbarui"})
}

// DELETE /api/school-admin/time-slot-overrides/:id
func DeleteTimeSlotOverride(c *gin.Context) {
	schoolID, err := middleware.GetSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		DELETE FROM school_time_slot_overrides
		WHERE id = $1 AND school_id = $2
	`, c.Param("id"), schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Override tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Override berhasil dihapus"})
}

var _ = sql.ErrNoRows