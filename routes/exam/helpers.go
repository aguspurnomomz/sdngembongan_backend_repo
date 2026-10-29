package exam 

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	"github.com/gin-gonic/gin"
	"gembongan_dev_backend/database"
)

func getSchoolIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	// Normalisasi userID ke string
	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	// Ambil email sebagai fallback
	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	// ==========================================
	// 1. Coba cari sebagai school_admin
	// ==========================================
	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE (id = $1 OR (email <> '' AND email = $2)) 
		AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)

	// ==========================================
	// 2. Kalau bukan admin, cek apakah dia teacher di sekolah aktif
	// ==========================================
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

	// ==========================================
	// 3. Kalau dia admin sekolah, pastikan sekolah aktif
	// ==========================================
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

func getAdminIDFromUser(c *gin.Context) (string, error) {
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

func sendExamControlBroadcast(scheduleID string, event map[string]any) error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || serviceRoleKey == "" {
		return fmt.Errorf("SUPABASE_URL atau SUPABASE_SERVICE_ROLE_KEY belum dikonfigurasi")
	}

	// Endpoint Broadcast HTTP di Supabase Realtime v2
	url := fmt.Sprintf("%s/realtime/v1/api/broadcast", supabaseURL)

	topic := fmt.Sprintf("exam-control-%s", scheduleID)

	payload := map[string]any{
		"messages": []map[string]any{
			{
				"topic":   topic,
				"event":   "control",
				"payload": event,
			},
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gagal marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("gagal buat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", serviceRoleKey)
	req.Header.Set("Authorization", "Bearer "+serviceRoleKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal kirim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broadcast gagal %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// parseTimeString — parse "HH:MM" / "HH:MM:SS" 
func parseTimeString(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	// Coba HH:MM:SS dulu
	t, err := time.Parse("15:04:05", s)
	if err == nil {
		return t, nil
	}
	// Fallback HH:MM
	return time.Parse("15:04", s)
}