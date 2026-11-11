package attendance

import (
	//"database/sql"
	"net/http"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type ScanQRRequest struct {
	SessionID     string `json:"session_id" binding:"required"`
	QRToken       string `json:"qr_token" binding:"required"`
	ForceMode     string `json:"force_mode"`
}

// POST /school-admin/attendance/scan
func ScanQR(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil { c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}); return }
	adminID, _ := getAdminIDFromUser(c)

	var req ScanQRRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()}); return }

	result, procErr := processAttendanceScan(schoolID, req.SessionID, "qr", req.QRToken, "", "", adminID, req.ForceMode)
	if procErr != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": procErr.Error()}); return }

	if !result.Success {
		status := http.StatusBadRequest
		switch result.ErrorCode {
		case "not_found": status = http.StatusNotFound
		case "duplicate": status = http.StatusConflict
		case "session_not_found": status = http.StatusNotFound
		case "session_closed": status = http.StatusConflict
		}
		c.JSON(status, gin.H{
			"success": false, "duplicate": result.ErrorCode == "duplicate", "error": result.ErrorMessage, "code": result.ErrorCode,
			"student": gin.H{"full_name": result.StudentName, "nisn": result.StudentNISN},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "mode": result.Mode, "status": result.Status,
		"student": gin.H{"id": result.StudentID, "full_name": result.StudentName, "nisn": result.StudentNISN, "sub_class_name": result.SubClassName},
		"scanned_at": nowJakarta(), "record_id": result.RecordID,
	})
}

// GET /kiosk/:code/active-session
func KioskGetActiveSession(c *gin.Context) {
	code := c.Param("code")

	var schoolID string
	err := database.DB.QueryRow(`SELECT school_id FROM attendance_devices WHERE device_code = $1 AND is_active = TRUE`, code).Scan(&schoolID)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"available": false, "error": "Device tidak dikenal"}); return }

	sessionID, deviceLabel, resErr := resolveActiveSessionForDevice(schoolID, code)
	if resErr != nil {
		c.JSON(http.StatusOK, gin.H{"available": false, "error": resErr.Error(), "device": deviceLabel})
		return
	}

	var title, shiftName, className, status string
	var totalStudents, totalCheckIn int
	_ = database.DB.QueryRow(`
		SELECT ases.title, sh.name, COALESCE(cg.name,''), ases.status, ases.total_students, ases.total_check_in
		FROM attendance_sessions ases
		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
		WHERE ases.id = $1
	`, sessionID).Scan(&title, &shiftName, &className, &status, &totalStudents, &totalCheckIn)

	c.JSON(http.StatusOK, gin.H{
		"available": true,
		"session": gin.H{
			"id": sessionID, "title": title, "shift_name": shiftName, "class_name": className,
			"status": status, "total_students": totalStudents, "total_check_in": totalCheckIn,
		},
		"device_label": deviceLabel,
	})
}

// POST /kiosk/:code/scan-rfid
func KioskScanRFID(c *gin.Context) {
	code := c.Param("code")

	var schoolID string
	err := database.DB.QueryRow(`SELECT school_id FROM attendance_devices WHERE device_code = $1 AND is_active = TRUE`, code).Scan(&schoolID)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Device tidak dikenal"}); return }

	var req struct {
		SessionID string `json:"session_id"`
		RFIDUID   string `json:"rfid_uid" binding:"required"`
		ForceMode string `json:"force_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Format tidak valid"}); return }

	uid := normalizeRFIDUID(req.RFIDUID)
	if uid == "" { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "UID kosong"}); return }

	sessionID := req.SessionID
	deviceLabel := ""
	if sessionID == "" {
		var resErr error
		sessionID, deviceLabel, resErr = resolveActiveSessionForDevice(schoolID, code)
		if resErr != nil {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": resErr.Error(), "code": "no_active_session"})
			return
		}
	}

	result, procErr := processAttendanceScan(schoolID, sessionID, "rfid", uid, code, deviceLabel, "", req.ForceMode)
	if procErr != nil { c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": procErr.Error()}); return }

	if !result.Success {
		status := http.StatusBadRequest
		switch result.ErrorCode {
		case "not_found": status = http.StatusNotFound
		case "duplicate": status = http.StatusConflict
		}
		c.JSON(status, gin.H{
			"success": false, "error": result.ErrorMessage, "code": result.ErrorCode,
			"student": gin.H{"full_name": result.StudentName, "nisn": result.StudentNISN},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "mode": result.Mode, "status": result.Status,
		"student": gin.H{"id": result.StudentID, "full_name": result.StudentName, "nisn": result.StudentNISN, "sub_class_name": result.SubClassName, "photo_url": result.PhotoURL},
		"record_id": result.RecordID, "scanned_at": nowJakarta(),
	})
}