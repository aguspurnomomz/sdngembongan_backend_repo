package attendance

import (
	"database/sql"
	"fmt"
	"net/http"
	//"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type CreateShiftRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	ShiftCategory string `json:"shift_category"` 
	CheckInStart       string `json:"check_in_start" binding:"required"`
	CheckInOnTimeStart string `json:"check_in_on_time_start" binding:"required"`
	CheckInOnTimeEnd   string `json:"check_in_on_time_end" binding:"required"`
	CheckInEnd         string `json:"check_in_end" binding:"required"`
	RequireCheckOut      bool   `json:"require_check_out"`
	CheckOutStart        string `json:"check_out_start"`
	CheckOutOnTimeStart  string `json:"check_out_on_time_start"`
	CheckOutOnTimeEnd    string `json:"check_out_on_time_end"`
	CheckOutEnd          string `json:"check_out_end"`
	AllowEarlyCheckIn  *bool `json:"allow_early_check_in"`
	AllowLateCheckOut  *bool `json:"allow_late_check_out"`
	AutoCloseMinutesAfterCheckIn  *int `json:"auto_close_minutes_after_check_in"`
	AutoCloseMinutesAfterCheckOut *int `json:"auto_close_minutes_after_check_out"`
	EarlyThresholdMinutes      *int `json:"early_threshold_minutes"`
	LateToleranceMinutes       *int `json:"late_tolerance_minutes"`
	EarlyLeaveToleranceMinutes *int `json:"early_leave_tolerance_minutes"`
	ApplicableDays []int `json:"applicable_days"`
	IsActive  *bool `json:"is_active"`
	IsDefault *bool `json:"is_default"`
}

type UpdateShiftRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	CheckInStart       string `json:"check_in_start"`
	CheckInOnTimeStart string `json:"check_in_on_time_start"`
	CheckInOnTimeEnd   string `json:"check_in_on_time_end"`
	CheckInEnd         string `json:"check_in_end"`
	RequireCheckOut     *bool  `json:"require_check_out"`
	CheckOutStart       string `json:"check_out_start"`
	CheckOutOnTimeStart string `json:"check_out_on_time_start"`
	CheckOutOnTimeEnd   string `json:"check_out_on_time_end"`
	CheckOutEnd         string `json:"check_out_end"`
	IsActive *bool `json:"is_active"`
}

type CreateClassShiftAssignmentRequest struct {
	ClassGroupID    string `json:"class_group_id"`
	ClassSubGroupID string `json:"class_sub_group_id"`
	ShiftID         string `json:"shift_id" binding:"required"`
	Priority  int   `json:"priority"`
	IsPrimary *bool `json:"is_primary"`
	OverrideCheckInStart  string `json:"override_check_in_start"`
	OverrideCheckInEnd    string `json:"override_check_in_end"`
	OverrideCheckOutStart string `json:"override_check_out_start"`
	OverrideCheckOutEnd   string `json:"override_check_out_end"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 { return "" }
	result := strs[0]
	for i := 1; i < len(strs); i++ { result += sep + strs[i] }
	return result
}

func strPtr(s string) *string { return &s }

// GET /school-admin/attendance/shifts
func GetShifts(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, name, code, COALESCE(description, ''), shift_category,
			check_in_start, check_in_on_time_start, check_in_on_time_end, check_in_end,
			require_check_out, check_out_start, check_out_on_time_start, 
			check_out_on_time_end, check_out_end,
			allow_early_check_in, allow_late_check_out,
			auto_close_minutes_after_check_in, auto_close_minutes_after_check_out,
			early_threshold_minutes, late_tolerance_minutes, early_leave_tolerance_minutes,
			COALESCE(applicable_days, ARRAY[1,2,3,4,5,6]),
			is_active, is_default, created_at
		FROM attendance_shifts WHERE school_id = $1 ORDER BY is_default DESC, code ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat shift: " + err.Error()})
		return
	}
	defer rows.Close()

	type ShiftItem struct {
		ID                            string   `json:"id"`
		Name                          string   `json:"name"`
		Code                          string   `json:"code"`
		Description                   string   `json:"description"`
		ShiftCategory                 string   `json:"shift_category"`
		CheckInStart                  string   `json:"check_in_start"`
		CheckInOnTimeStart            string   `json:"check_in_on_time_start"`
		CheckInOnTimeEnd              string   `json:"check_in_on_time_end"`
		CheckInEnd                    string   `json:"check_in_end"`
		RequireCheckOut               bool     `json:"require_check_out"`
		CheckOutStart                 *string  `json:"check_out_start"`
		CheckOutOnTimeStart           *string  `json:"check_out_on_time_start"`
		CheckOutOnTimeEnd             *string  `json:"check_out_on_time_end"`
		CheckOutEnd                   *string  `json:"check_out_end"`
		AllowEarlyCheckIn             bool     `json:"allow_early_check_in"`
		AllowLateCheckOut             bool     `json:"allow_late_check_out"`
		AutoCloseMinutesAfterCheckIn  int      `json:"auto_close_minutes_after_check_in"`
		AutoCloseMinutesAfterCheckOut int      `json:"auto_close_minutes_after_check_out"`
		EarlyThresholdMinutes         int      `json:"early_threshold_minutes"`
		LateToleranceMinutes          int      `json:"late_tolerance_minutes"`
		EarlyLeaveToleranceMinutes    int      `json:"early_leave_tolerance_minutes"`
		ApplicableDays                []int    `json:"applicable_days"`
		IsActive                      bool     `json:"is_active"`
		IsDefault                     bool     `json:"is_default"`
		CreatedAt                     time.Time `json:"created_at"`
	}

	list := []ShiftItem{}
	for rows.Next() {
		var s ShiftItem
		var ciStart, ciOnTimeStart, ciOnTimeEnd, ciEnd time.Time
		var coStart, coOnTimeStart, coOnTimeEnd, coEnd sql.NullString
		var applicableDays pq.Int64Array

		if err := rows.Scan(
			&s.ID, &s.Name, &s.Code, &s.Description, &s.ShiftCategory,
			&ciStart, &ciOnTimeStart, &ciOnTimeEnd, &ciEnd,
			&s.RequireCheckOut, &coStart, &coOnTimeStart, &coOnTimeEnd, &coEnd,
			&s.AllowEarlyCheckIn, &s.AllowLateCheckOut,
			&s.AutoCloseMinutesAfterCheckIn, &s.AutoCloseMinutesAfterCheckOut,
			&s.EarlyThresholdMinutes, &s.LateToleranceMinutes, &s.EarlyLeaveToleranceMinutes,
			&applicableDays, &s.IsActive, &s.IsDefault, &s.CreatedAt,
		); err != nil {
			continue
		}

		s.ApplicableDays = make([]int, len(applicableDays))
		for i, v := range applicableDays { s.ApplicableDays[i] = int(v) }

		s.CheckInStart = ciStart.Format("15:04")
		s.CheckInOnTimeStart = ciOnTimeStart.Format("15:04")
		s.CheckInOnTimeEnd = ciOnTimeEnd.Format("15:04")
		s.CheckInEnd = ciEnd.Format("15:04")

		if coStart.Valid { t, _ := time.Parse("15:04:05", coStart.String); s.CheckOutStart = strPtr(t.Format("15:04")) }
		if coOnTimeStart.Valid { t, _ := time.Parse("15:04:05", coOnTimeStart.String); s.CheckOutOnTimeStart = strPtr(t.Format("15:04")) }
		if coOnTimeEnd.Valid { t, _ := time.Parse("15:04:05", coOnTimeEnd.String); s.CheckOutOnTimeEnd = strPtr(t.Format("15:04")) }
		if coEnd.Valid { t, _ := time.Parse("15:04:05", coEnd.String); s.CheckOutEnd = strPtr(t.Format("15:04")) }

		list = append(list, s)
	}

	c.JSON(http.StatusOK, gin.H{"shifts": list, "total": len(list)})
}

// POST /school-admin/attendance/shifts
func CreateShift(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)

	var req CreateShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()}); return }

	allowEarly := true
	if req.AllowEarlyCheckIn != nil { allowEarly = *req.AllowEarlyCheckIn }
	allowLateOut := false
	if req.AllowLateCheckOut != nil { allowLateOut = *req.AllowLateCheckOut }
	autoCloseIn := 120
	if req.AutoCloseMinutesAfterCheckIn != nil { autoCloseIn = *req.AutoCloseMinutesAfterCheckIn }
	autoCloseOut := 60
	if req.AutoCloseMinutesAfterCheckOut != nil { autoCloseOut = *req.AutoCloseMinutesAfterCheckOut }
	earlyThreshold := 15
	if req.EarlyThresholdMinutes != nil { earlyThreshold = *req.EarlyThresholdMinutes }
	lateTolerance := 5
	if req.LateToleranceMinutes != nil { lateTolerance = *req.LateToleranceMinutes }
	earlyLeaveTolerance := 15
	if req.EarlyLeaveToleranceMinutes != nil { earlyLeaveTolerance = *req.EarlyLeaveToleranceMinutes }
	isActive := true
	if req.IsActive != nil { isActive = *req.IsActive }
	isDefault := false
	if req.IsDefault != nil { isDefault = *req.IsDefault }
	applicableDays := req.ApplicableDays
	if len(applicableDays) == 0 { applicableDays = []int{1, 2, 3, 4, 5, 6} }
	shiftCategory := req.ShiftCategory
	if shiftCategory == "" { shiftCategory = "regular" }

	var coStart, coOnTimeStart, coOnTimeEnd, coEnd interface{} = nil, nil, nil, nil
	if req.RequireCheckOut {
		if req.CheckOutStart != "" { coStart = req.CheckOutStart }
		if req.CheckOutOnTimeStart != "" { coOnTimeStart = req.CheckOutOnTimeStart }
		if req.CheckOutOnTimeEnd != "" { coOnTimeEnd = req.CheckOutOnTimeEnd }
		if req.CheckOutEnd != "" { coEnd = req.CheckOutEnd }
	}

	if isDefault {
		_, _ = database.DB.Exec(`UPDATE attendance_shifts SET is_default = false WHERE school_id = $1`, schoolID)
	}

	var shiftID string
	err = database.DB.QueryRow(`
		INSERT INTO attendance_shifts (
			school_id, name, code, description, shift_category,
			check_in_start, check_in_on_time_start, check_in_on_time_end, check_in_end,
			require_check_out, check_out_start, check_out_on_time_start,
			check_out_on_time_end, check_out_end,
			allow_early_check_in, allow_late_check_out,
			auto_close_minutes_after_check_in, auto_close_minutes_after_check_out,
			early_threshold_minutes, late_tolerance_minutes, early_leave_tolerance_minutes,
			applicable_days, is_active, is_default, created_by
		) VALUES (
			$1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25
		) RETURNING id
	`, schoolID, req.Name, req.Code, req.Description, shiftCategory,
		req.CheckInStart, req.CheckInOnTimeStart, req.CheckInOnTimeEnd, req.CheckInEnd,
		req.RequireCheckOut, coStart, coOnTimeStart, coOnTimeEnd, coEnd,
		allowEarly, allowLateOut, autoCloseIn, autoCloseOut,
		earlyThreshold, lateTolerance, earlyLeaveTolerance,
		applicableDays, isActive, isDefault, adminID).Scan(&shiftID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan shift: " + err.Error()}); return }
	c.JSON(http.StatusCreated, gin.H{"message": "Shift berhasil dibuat", "id": shiftID})
}

// PUT /school-admin/attendance/shifts/:id
func UpdateShift(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	shiftID := c.Param("id")
	var req UpdateShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"}); return }

	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != "" { updates = append(updates, fmt.Sprintf("name = $%d", argIdx)); args = append(args, req.Name); argIdx++ }
	if req.Code != "" { updates = append(updates, fmt.Sprintf("code = $%d", argIdx)); args = append(args, req.Code); argIdx++ }
	if req.Description != "" { updates = append(updates, fmt.Sprintf("description = $%d", argIdx)); args = append(args, req.Description); argIdx++ }
	if req.CheckInStart != "" { updates = append(updates, fmt.Sprintf("check_in_start = $%d", argIdx)); args = append(args, req.CheckInStart); argIdx++ }
	if req.CheckInOnTimeStart != "" { updates = append(updates, fmt.Sprintf("check_in_on_time_start = $%d", argIdx)); args = append(args, req.CheckInOnTimeStart); argIdx++ }
	if req.CheckInOnTimeEnd != "" { updates = append(updates, fmt.Sprintf("check_in_on_time_end = $%d", argIdx)); args = append(args, req.CheckInOnTimeEnd); argIdx++ }
	if req.CheckInEnd != "" { updates = append(updates, fmt.Sprintf("check_in_end = $%d", argIdx)); args = append(args, req.CheckInEnd); argIdx++ }
	if req.RequireCheckOut != nil { updates = append(updates, fmt.Sprintf("require_check_out = $%d", argIdx)); args = append(args, *req.RequireCheckOut); argIdx++ }
	if req.CheckOutStart != "" { updates = append(updates, fmt.Sprintf("check_out_start = $%d", argIdx)); args = append(args, req.CheckOutStart); argIdx++ }
	if req.CheckOutOnTimeStart != "" { updates = append(updates, fmt.Sprintf("check_out_on_time_start = $%d", argIdx)); args = append(args, req.CheckOutOnTimeStart); argIdx++ }
	if req.CheckOutOnTimeEnd != "" { updates = append(updates, fmt.Sprintf("check_out_on_time_end = $%d", argIdx)); args = append(args, req.CheckOutOnTimeEnd); argIdx++ }
	if req.CheckOutEnd != "" { updates = append(updates, fmt.Sprintf("check_out_end = $%d", argIdx)); args = append(args, req.CheckOutEnd); argIdx++ }
	if req.IsActive != nil { updates = append(updates, fmt.Sprintf("is_active = $%d", argIdx)); args = append(args, *req.IsActive); argIdx++ }

	if len(updates) == 0 { c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada field yang diupdate"}); return }

	updates = append(updates, "updated_at = NOW()")
	query := fmt.Sprintf("UPDATE attendance_shifts SET %s WHERE id = $%d AND school_id = $%d", joinStrings(updates, ", "), argIdx, argIdx+1)
	args = append(args, shiftID, schoolID)

	result, err := database.DB.Exec(query, args...)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }

	affected, _ := result.RowsAffected()
	if affected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "Shift tidak ditemukan"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "Shift berhasil diperbarui"})
}

// DELETE /school-admin/attendance/shifts/:id
func DeleteShift(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	shiftID := c.Param("id")
	var activeCount int
	_ = database.DB.QueryRow(`SELECT count(*) FROM attendance_sessions WHERE shift_id = $1 AND status IN ('open', 'scheduled')`, shiftID).Scan(&activeCount)

	if activeCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Tidak bisa hapus: ada %d sesi aktif pakai shift ini", activeCount)})
		return
	}

	result, err := database.DB.Exec(`DELETE FROM attendance_shifts WHERE id = $1 AND school_id = $2`, shiftID, schoolID)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }

	affected, _ := result.RowsAffected()
	if affected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "Shift tidak ditemukan"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "Shift berhasil dihapus"})
}

// GET /school-admin/attendance/assignments
func GetClassAssignments(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	classGroupID := c.Query("class_group_id")
	shiftID := c.Query("shift_id")

	query := `
		SELECT csa.id, csa.class_group_id, csa.class_sub_group_id, csa.shift_id, csa.priority, csa.is_primary,
			csa.override_check_in_start, csa.override_check_in_end, csa.override_check_out_start, csa.override_check_out_end,
			csa.effective_from, csa.effective_to, csa.is_active, csa.created_at,
			COALESCE(cg.name, '') AS class_group_name, COALESCE(csg.name, '') AS class_sub_group_name,
			COALESCE(s.name, '') AS shift_name, COALESCE(s.code, '') AS shift_code
		FROM class_shift_assignments csa
		LEFT JOIN class_groups cg ON csa.class_group_id = cg.id
		LEFT JOIN class_sub_groups csg ON csa.class_sub_group_id = csg.id
		LEFT JOIN attendance_shifts s ON csa.shift_id = s.id
		WHERE csa.school_id = $1
	`
	args := []any{schoolID}
	argIdx := 2

	if classGroupID != "" { query += fmt.Sprintf(" AND csa.class_group_id = $%d", argIdx); args = append(args, classGroupID); argIdx++ }
	if shiftID != "" { query += fmt.Sprintf(" AND csa.shift_id = $%d", argIdx); args = append(args, shiftID); argIdx++ }
	query += " ORDER BY csa.is_primary DESC, csa.priority ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	defer rows.Close()

	type AssignmentItem struct {
		ID                     string     `json:"id"`
		ClassGroupID           *string    `json:"class_group_id"`
		ClassSubGroupID        *string    `json:"class_sub_group_id"`
		ShiftID                string     `json:"shift_id"`
		Priority               int        `json:"priority"`
		IsPrimary              bool       `json:"is_primary"`
		OverrideCheckInStart   *string    `json:"override_check_in_start"`
		OverrideCheckInEnd     *string    `json:"override_check_in_end"`
		OverrideCheckOutStart  *string    `json:"override_check_out_start"`
		OverrideCheckOutEnd    *string    `json:"override_check_out_end"`
		EffectiveFrom          *string    `json:"effective_from"`
		EffectiveTo            *string    `json:"effective_to"`
		IsActive               bool       `json:"is_active"`
		CreatedAt              time.Time  `json:"created_at"`
		ClassGroupName         string     `json:"class_group_name"`
		ClassSubGroupName      string     `json:"class_sub_group_name"`
		ShiftName              string     `json:"shift_name"`
		ShiftCode              string     `json:"shift_code"`
	}

	var list []AssignmentItem
	for rows.Next() {
		var a AssignmentItem
		var cgID, csgID, ociStart, ociEnd, ocoStart, ocoEnd sql.NullString
		var effFrom, effTo sql.NullTime

		if err := rows.Scan(
			&a.ID, &cgID, &csgID, &a.ShiftID, &a.Priority, &a.IsPrimary,
			&ociStart, &ociEnd, &ocoStart, &ocoEnd, &effFrom, &effTo, &a.IsActive, &a.CreatedAt,
			&a.ClassGroupName, &a.ClassSubGroupName, &a.ShiftName, &a.ShiftCode,
		); err == nil {
			if cgID.Valid { a.ClassGroupID = &cgID.String }
			if csgID.Valid { a.ClassSubGroupID = &csgID.String }
			if ociStart.Valid { t, _ := time.Parse("15:04:05", ociStart.String); s := t.Format("15:04"); a.OverrideCheckInStart = &s }
			if ociEnd.Valid { t, _ := time.Parse("15:04:05", ociEnd.String); s := t.Format("15:04"); a.OverrideCheckInEnd = &s }
			if ocoStart.Valid { t, _ := time.Parse("15:04:05", ocoStart.String); s := t.Format("15:04"); a.OverrideCheckOutStart = &s }
			if ocoEnd.Valid { t, _ := time.Parse("15:04:05", ocoEnd.String); s := t.Format("15:04"); a.OverrideCheckOutEnd = &s }
			if effFrom.Valid { s := effFrom.Time.Format("2006-01-02"); a.EffectiveFrom = &s }
			if effTo.Valid { s := effTo.Time.Format("2006-01-02"); a.EffectiveTo = &s }
			list = append(list, a)
		}
	}
	c.JSON(http.StatusOK, gin.H{"assignments": list, "total": len(list)})
}

// POST /school-admin/attendance/assignments
func CreateClassAssignment(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)

	var req CreateClassShiftAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"}); return }

	if req.ClassGroupID == "" && req.ClassSubGroupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal isi class_group_id atau class_sub_group_id"}); return
	}

	priority := req.Priority
	if priority == 0 { priority = 1 }
	isPrimary := true
	if req.IsPrimary != nil { isPrimary = *req.IsPrimary }

	var classGroupID, classSubGroupID interface{} = nil, nil
	if req.ClassGroupID != "" { classGroupID = req.ClassGroupID }
	if req.ClassSubGroupID != "" { classSubGroupID = req.ClassSubGroupID }

	var ociStart, ociEnd, ocoStart, ocoEnd interface{} = nil, nil, nil, nil
	if req.OverrideCheckInStart != "" { ociStart = req.OverrideCheckInStart }
	if req.OverrideCheckInEnd != "" { ociEnd = req.OverrideCheckInEnd }
	if req.OverrideCheckOutStart != "" { ocoStart = req.OverrideCheckOutStart }
	if req.OverrideCheckOutEnd != "" { ocoEnd = req.OverrideCheckOutEnd }

	var effFrom, effTo interface{} = nil, nil
	if req.EffectiveFrom != "" { effFrom = req.EffectiveFrom }
	if req.EffectiveTo != "" { effTo = req.EffectiveTo }

	var assignmentID string
	err = database.DB.QueryRow(`
		INSERT INTO class_shift_assignments (
			school_id, class_group_id, class_sub_group_id, shift_id, priority, is_primary,
			override_check_in_start, override_check_in_end, override_check_out_start, override_check_out_end,
			effective_from, effective_to, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id
	`, schoolID, classGroupID, classSubGroupID, req.ShiftID, priority, isPrimary,
		ociStart, ociEnd, ocoStart, ocoEnd, effFrom, effTo, adminID).Scan(&assignmentID)

	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan: " + err.Error()}); return }
	c.JSON(http.StatusCreated, gin.H{"message": "Assignment berhasil dibuat", "id": assignmentID})
}

// DELETE /school-admin/attendance/assignments/:id
func DeleteClassAssignment(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }

	result, err := database.DB.Exec(`DELETE FROM class_shift_assignments WHERE id = $1 AND school_id = $2`, c.Param("id"), schoolID)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }

	affected, _ := result.RowsAffected()
	if affected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "Assignment tidak ditemukan"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "Assignment berhasil dihapus"})
}