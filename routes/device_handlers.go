package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)


func resolveActiveSessionForDevice(schoolID, deviceCode string) (string, string, error) {
	var deviceID, deviceName, locationNote, sessionMode string
	err := database.DB.QueryRow(`
		SELECT id, device_name, COALESCE(location_note,''), default_session_mode
		FROM attendance_devices
		WHERE school_id = $1 AND device_code = $2 AND is_active = TRUE
	`, schoolID, deviceCode).Scan(&deviceID, &deviceName, &locationNote, &sessionMode)
	if err != nil {
		return "", "", fmt.Errorf("device tidak dikenal atau tidak aktif")
	}

	_, _ = database.DB.Exec(`
		UPDATE attendance_devices SET last_ping_at = NOW() WHERE id = $1
	`, deviceID)

	if sessionMode != "auto_shift" {
		return "", deviceName, fmt.Errorf("device ini dalam mode manual, session_id wajib diisi")
	}

	now := nowJakarta()
	nowTime := now.Format("15:04:05")


	var sessionID string
	err = database.DB.QueryRow(`
		SELECT ases.id
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		WHERE ases.school_id = $1
		  AND ases.session_date = CURRENT_DATE
		  AND ases.status IN ('scheduled', 'open')
		  AND $2::time BETWEEN sh.check_in_start AND sh.check_in_end
		ORDER BY
		  CASE WHEN ases.status = 'open' THEN 0 ELSE 1 END,
		  ases.created_at DESC
		LIMIT 1
	`, schoolID, nowTime).Scan(&sessionID)

	if err == nil && sessionID != "" {
		return sessionID, deviceName, nil
	}

	
	err = database.DB.QueryRow(`
		SELECT id FROM attendance_sessions
		WHERE school_id = $1
		  AND session_date = CURRENT_DATE
		  AND status = 'open'
		ORDER BY COALESCE(opened_at, created_at) DESC
		LIMIT 1
	`, schoolID).Scan(&sessionID)

	if err == nil && sessionID != "" {
		return sessionID, deviceName, nil
	}

	return "", deviceName, fmt.Errorf("tidak ada sesi absensi aktif saat ini")
}


func HandleListDevices(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, device_name, device_code, device_type,
		       COALESCE(location_note,''), default_session_mode,
		       is_active, last_ping_at, created_at
		FROM attendance_devices
		WHERE school_id = $1
		ORDER BY device_name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type DeviceItem struct {
		ID                 string     `json:"id"`
		DeviceName         string     `json:"device_name"`
		DeviceCode         string     `json:"device_code"`
		DeviceType         string     `json:"device_type"`
		LocationNote       string     `json:"location_note"`
		DefaultSessionMode string     `json:"default_session_mode"`
		IsActive           bool       `json:"is_active"`
		LastPingAt         *time.Time `json:"last_ping_at"`
		CreatedAt          time.Time  `json:"created_at"`
	}

	var list []DeviceItem
	for rows.Next() {
		var d DeviceItem
		var lastPing sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.DeviceName, &d.DeviceCode, &d.DeviceType,
			&d.LocationNote, &d.DefaultSessionMode,
			&d.IsActive, &lastPing, &d.CreatedAt,
		); err == nil {
			if lastPing.Valid {
				d.LastPingAt = &lastPing.Time
			}
			list = append(list, d)
		}
	}

	if list == nil {
		list = []DeviceItem{}
	}

	c.JSON(http.StatusOK, gin.H{"total": len(list), "devices": list})
}


func HandleCreateDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := getAdminIDFromUser(c)

	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	deviceType := req.DeviceType
	if deviceType == "" {
		deviceType = "kiosk"
	}
	sessionMode := req.DefaultSessionMode
	if sessionMode == "" {
		sessionMode = "auto_shift"
	}

	base := strings.ToUpper(strings.TrimSpace(req.DeviceName))
	base = strings.ReplaceAll(base, " ", "-")
	var cleaned strings.Builder
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	deviceCode := cleaned.String()
	if deviceCode == "" {
		deviceCode = "DEVICE"
	}

	var existing string
	_ = database.DB.QueryRow(`
		SELECT device_code FROM attendance_devices WHERE device_code = $1
	`, deviceCode).Scan(&existing)
	if existing != "" {
		deviceCode = deviceCode + "-" + randString(4)
	}

	var deviceID string
	err = database.DB.QueryRow(`
		INSERT INTO attendance_devices
			(school_id, device_name, device_code, device_type,
			 location_note, default_session_mode, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7)
		RETURNING id
	`, schoolID, req.DeviceName, deviceCode, deviceType,
		req.LocationNote, sessionMode, adminID).Scan(&deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Device berhasil dibuat",
		"device_id":   deviceID,
		"device_code": deviceCode,
		"kiosk_url":   fmt.Sprintf("/kiosk/%s", deviceCode),
	})
}


func HandleUpdateDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		DeviceName         string `json:"device_name"`
		LocationNote       string `json:"location_note"`
		DefaultSessionMode string `json:"default_session_mode"`
		IsActive           *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
		return
	}

	_, err = database.DB.Exec(`
		UPDATE attendance_devices SET
			device_name = COALESCE(NULLIF($1,''), device_name),
			location_note = COALESCE(NULLIF($2,''), location_note),
			default_session_mode = COALESCE(NULLIF($3,''), default_session_mode),
			is_active = COALESCE($4, is_active),
			updated_at = NOW()
		WHERE id = $5 AND school_id = $6
	`, req.DeviceName, req.LocationNote, req.DefaultSessionMode,
		req.IsActive, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device berhasil diperbarui"})
}


func HandleDeleteDevice(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		DELETE FROM attendance_devices WHERE id = $1 AND school_id = $2
	`, c.Param("id"), schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device berhasil dihapus"})
}


func HandleGetActiveSession(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	deviceCode := c.Param("code")
	sessionID, deviceLabel, resErr := resolveActiveSessionForDevice(schoolID, deviceCode)
	if resErr != nil {
		c.JSON(http.StatusConflict, gin.H{
			"available": false,
			"error":     resErr.Error(),
			"device":    deviceLabel,
		})
		return
	}

	var title, shiftName, className, status string
	var totalStudents, totalCheckIn int
	err = database.DB.QueryRow(`
		SELECT ases.title, sh.name, COALESCE(cg.name,''),
		       ases.status, ases.total_students, ases.total_check_in
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		WHERE ases.id = $1
	`, sessionID).Scan(&title, &shiftName, &className, &status,
		&totalStudents, &totalCheckIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"available": true,
		"session": gin.H{
			"id":             sessionID,
			"title":          title,
			"shift_name":     shiftName,
			"class_name":     className,
			"status":         status,
			"total_students": totalStudents,
			"total_check_in": totalCheckIn,
		},
		"device_label": deviceLabel,
	})
}