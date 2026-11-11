package superadmin

import (
	"database/sql"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

func Stats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Selamat datang di Panel Superadmin!",
		"admin":   c.MustGet("admin_email"),
	})
}

type LogItem struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Action    string    `json:"action"`
	IpAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Location  string    `json:"location"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

func ListLogs(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT l.id, a.email, l.action, l.ip_address, l.user_agent, l.location, l.details, l.created_at 
		FROM super_admin_logs l
		LEFT JOIN super_admins a ON l.admin_id = a.id
		ORDER BY l.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log aktivitas: " + err.Error()})
		return
	}
	defer rows.Close()

	var logs []LogItem
	for rows.Next() {
		var item LogItem
		var email, ip, ua, loc, details sql.NullString
		if err := rows.Scan(&item.ID, &email, &item.Action, &ip, &ua, &loc, &details, &item.CreatedAt); err == nil {
			item.Email = email.String
			item.IpAddress = ip.String
			item.UserAgent = ua.String
			item.Location = loc.String
			item.Details = details.String
			logs = append(logs, item)
		}
	}

	if logs == nil {
		logs = []LogItem{}
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}