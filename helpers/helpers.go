package helpers

import (
	"database/sql"
	"fmt"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

func GetSchoolIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE (id = $1 OR (email <> '' AND email = $2)) 
		AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)

	if err != nil || schoolID == "" {
		var teacherSchoolID string
		errTeacher := database.DB.QueryRow(`
			SELECT sm.school_id
			FROM school_members sm
			JOIN schools s ON sm.school_id = s.id
			WHERE sm.user_id = $1 
			  AND sm.is_active = TRUE 
			  AND sm.left_at IS NULL
			  AND s.is_active = TRUE
			LIMIT 1
		`, userIDStr).Scan(&teacherSchoolID)

		if errTeacher == nil {
			return teacherSchoolID, nil
		}

		return "", fmt.Errorf("akses ditolak: Anda bukan admin sekolah atau guru di sekolah aktif")
	}

	var schoolActive bool
	err = database.DB.QueryRow(
		`SELECT COALESCE(is_active, false) FROM schools WHERE id = $1`,
		schoolID,
	).Scan(&schoolActive)
	if err != nil {
		return "", fmt.Errorf("data sekolah tidak ditemukan")
	}
	if !schoolActive {
		return "", fmt.Errorf("sekolah Anda sedang dinonaktifkan. Hubungi administrator untuk informasi lebih lanjut")
	}

	return schoolID, nil
}

func GetAdminIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}
	return userIDStr, nil
}

func RandString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func NullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}