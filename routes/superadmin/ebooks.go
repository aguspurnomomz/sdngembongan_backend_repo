package superadmin

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// GET /api/superadmin/ebooks
func ListEbooks(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, judul, jenjang, mata_pelajaran, kategori, cover_url, file_url, created_at 
		FROM ebooks 
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data e-book: " + err.Error()})
		return
	}
	defer rows.Close()

	type EbookItem struct {
		ID            string    `json:"id"`
		Judul         string    `json:"judul"`
		Jenjang       string    `json:"jenjang"`
		MataPelajaran string    `json:"mata_pelajaran"`
		Kategori      string    `json:"kategori"`
		CoverUrl      string    `json:"cover_url"`
		FileUrl       string    `json:"file_url"`
		CreatedAt     time.Time `json:"created_at"`
	}

	ebooks := []EbookItem{}
	for rows.Next() {
		var e EbookItem
		var coverUrl sql.NullString
		if err := rows.Scan(&e.ID, &e.Judul, &e.Jenjang, &e.MataPelajaran, &e.Kategori, &coverUrl, &e.FileUrl, &e.CreatedAt); err == nil {
			e.CoverUrl = coverUrl.String
			ebooks = append(ebooks, e)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_ebooks": len(ebooks),
		"ebooks":       ebooks,
	})
}

// POST /api/superadmin/ebooks
func CreateEbook(c *gin.Context) {
	var req CreateEbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data e-book tidak valid: " + err.Error()})
		return
	}

	var ebookID string
	err := database.DB.QueryRow(
		`INSERT INTO ebooks (judul, jenjang, mata_pelajaran, kategori, cover_url, file_url) 
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		req.Judul, req.Jenjang, req.MataPelajaran, req.Kategori, req.CoverUrl, req.FileUrl,
	).Scan(&ebookID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan e-book ke database: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "E-book berhasil ditambahkan",
		"ebook_id": ebookID,
	})
}

// DELETE /api/superadmin/ebooks/:id
func DeleteEbook(c *gin.Context) {
	ebookID := c.Param("id")

	var judulBuku string
	_ = database.DB.QueryRow(`SELECT judul FROM ebooks WHERE id = $1`, ebookID).Scan(&judulBuku)

	result, err := database.DB.Exec(`DELETE FROM ebooks WHERE id = $1`, ebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus e-book: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-book tidak ditemukan"})
		return
	}

	if adminID, ok := GetAdminIDFromContext(c); ok {
		LogSuperAdminActivity(adminID, "DELETE_EBOOK", c.ClientIP(),
			c.Request.UserAgent(), "",
			fmt.Sprintf("Menghapus e-book: %s", judulBuku))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "E-book berhasil dihapus",
		"ebook_id": ebookID,
	})
}