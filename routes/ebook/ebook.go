package ebook

import (
	"database/sql"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type EbookPublicItem struct {
	ID            string `json:"id"`
	Judul         string `json:"judul"`
	Jenjang       string `json:"jenjang"`
	MataPelajaran string `json:"mata_pelajaran"`
	Kategori      string `json:"kategori"`
	CoverUrl      string `json:"cover_url"`
	FileUrl       string `json:"file_url"`
}

func ListEbooks(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, judul, jenjang, mata_pelajaran, kategori, cover_url, file_url, created_at 
		FROM ebooks 
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar e-book"})
		return
	}
	defer rows.Close()

	var list []EbookPublicItem
	for rows.Next() {
		var item EbookPublicItem
		var cover sql.NullString
		var createdAt time.Time

		if err := rows.Scan(
			&item.ID, &item.Judul, &item.Jenjang, &item.MataPelajaran,
			&item.Kategori, &cover, &item.FileUrl, &createdAt,
		); err == nil {
			item.CoverUrl = cover.String
			list = append(list, item)
		}
	}

	if list == nil {
		list = []EbookPublicItem{}
	}

	c.JSON(http.StatusOK, gin.H{"ebooks": list})
}

type HistoryRequest struct {
	EbookID    string `json:"ebook_id" binding:"required"`
	ActionType string `json:"action_type" binding:"required"`
}

func RecordHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req HistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data riwayat tidak valid"})
		return
	}

	_, err := database.DB.Exec(
		`INSERT INTO ebook_history (user_id, ebook_id, action_type) VALUES ($1, $2, $3)`,
		userID, req.EbookID, req.ActionType,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat riwayat e-book"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Riwayat berhasil dicatat/disimpan"})
}