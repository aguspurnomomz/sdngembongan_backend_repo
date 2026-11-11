package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

func sendResendEmail(toEmail string, subject string, htmlContent string) {
	apiKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("RESEND_FROM_EMAIL")

	if apiKey == "" || fromEmail == "" || toEmail == "" {
		fmt.Printf("Resend API Key atau From Email belum dikonfigurasi\n")
		return
	}

	payload := map[string]any{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlContent,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Gagal marshal payload Resend: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Gagal membuat request Resend: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Gagal mengirim email via Resend: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("Resend mengembalikan status error: %d\n", resp.StatusCode)
	}
}

func NotifyPasswordChanged(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var userEmail string
	err := database.DB.QueryRow(`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`, userID).Scan(&userEmail)

	if err != nil || userEmail == "" {
		_ = database.DB.QueryRow(`SELECT COALESCE(email_sekolah, '') FROM profiles WHERE id = $1`, userID).Scan(&userEmail)
	}

	if userEmail == "" {
		c.JSON(http.StatusOK, gin.H{"message": "Kata sandi diperbarui, namun email pengguna tidak ditemukan"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	waktuUbah := time.Now().In(loc).Format("02 January 2006 pukul 15:04 WIB")
	ipClient := c.ClientIP()

	subjek := "Keamanan Akun: Kata Sandi Anda Telah Diubah"

	htmlBody := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 10px;">
			<h2 style="color: #10b981;">SkoolaGo Security Alert</h2>
			<p>Halo Pengguna,</p>
			<p>Kami ingin menginformasikan bahwa kata sandi untuk akun Anda baru saja berhasil diubah.</p>
			<div style="background-color: #f8fafc; padding: 15px; border-radius: 8px; margin: 20px 0;">
				<p style="margin: 5px 0;"><strong>Waktu:</strong> %s</p>
				<p style="margin: 5px 0;"><strong>Alamat IP:</strong> %s</p>
			</div>
			<p style="color: #ef4444; font-weight: bold;">Jika Anda merasa TIDAK melakukan perubahan ini, segera hubungi tim terkait melalui support@skoolago.com atau segera amankan akun Anda!</p>
			<hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;" />
			<p style="font-size: 12px; color: #64748b; text-align: center;">
				&copy; 2026 SkoolaGo. All rights reserved.<br />
				Email otomatis ini dikirimkan oleh sistem keamanan SkoolaGo.
			</p>
		</div>
	`, waktuUbah, ipClient)

	go sendResendEmail(userEmail, subjek, htmlBody)

	c.JSON(http.StatusOK, gin.H{"message": "Notifikasi perubahan kata sandi berhasil diproses"})
}