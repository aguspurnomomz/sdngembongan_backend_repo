package questionbank

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type QuestionInput struct {
	QuestionText   string `json:"question_text" binding:"required"`
	QuestionType   string `json:"question_type" binding:"required"`
	Options        any    `json:"options"`
	CorrectAnswer  any    `json:"correct_answer" binding:"required"`
	Explanation    string `json:"explanation"`
	CognitiveLevel string `json:"cognitive_level"`
}

type CreateQuestionBankRequest struct {
	Title         string          `json:"title" binding:"required"`
	Description   string          `json:"description"`
	Subject       string          `json:"subject" binding:"required"`
	Phase         string          `json:"phase" binding:"required"`
	PriceInTokens int             `json:"price_in_tokens"`
	IsPublic      bool            `json:"is_public"`
	Questions     []QuestionInput `json:"questions"`
}

func CreateQuestionBank(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req CreateQuestionBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data input tidak valid: " + err.Error()})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi database"})
		return
	}
	defer tx.Rollback()

	var bankID string
	bankQuery := `INSERT INTO question_banks (user_id, title, description, subject, phase, price_in_tokens, is_public) 
	              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`

	err = tx.QueryRow(
		bankQuery,
		userID, req.Title, req.Description, req.Subject,
		req.Phase, req.PriceInTokens, req.IsPublic,
	).Scan(&bankID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan bank soal: " + err.Error()})
		return
	}

	for _, q := range req.Questions {
		var optionsStr string
		if q.Options == nil || q.Options == "" {
			optionsStr = "null"
		} else {
			optionsBytes, err := json.Marshal(q.Options)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memproses format options soal"})
				return
			}
			optionsStr = string(optionsBytes)
		}

		qQuery := `INSERT INTO questions (question_bank_id, question_text, question_type, options, correct_answer, explanation, cognitive_level)
		           VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)`

		_, err = tx.Exec(qQuery,
			bankID, q.QuestionText, q.QuestionType, optionsStr,
			q.CorrectAnswer, q.Explanation, q.CognitiveLevel,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan butir soal: " + err.Error()})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan perubahan secara permanen"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Bank soal dan butir soal berhasil dibuat",
		"bank_id": bankID,
	})
}

type BankItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Subject   string    `json:"subject"`
	Phase     string    `json:"phase"`
	CreatedAt time.Time `json:"created_at"`
}

func ListMyQuestionBanks(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, title, subject, phase, created_at 
		FROM question_banks 
		WHERE user_id = $1 
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar bank soal"})
		return
	}
	defer rows.Close()

	var banks []BankItem
	for rows.Next() {
		var b BankItem
		if err := rows.Scan(&b.ID, &b.Title, &b.Subject, &b.Phase, &b.CreatedAt); err == nil {
			banks = append(banks, b)
		}
	}

	if banks == nil {
		banks = []BankItem{}
	}

	_ = sql.ErrNoRows 

	c.JSON(http.StatusOK, gin.H{"question_banks": banks})
}