package teacher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"teach_partner_dev/database"
	"teach_partner_dev/routes/helpers"
	tokensvc "teach_partner_dev/routes/token"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

type GenerateAIWithChargeRequest struct {
	Topic          string `json:"topic" binding:"required,min=3,max=200"`
	QuestionType   string `json:"question_type" binding:"required,oneof=multiple_choice essay"`
	NumberOfQ      int    `json:"number_of_q" binding:"required,min=1,max=50"`
	CognitiveLevel string `json:"cognitive_level"`
}

// ==========================================
// POST /api/teacher/ai/generate-questions
//
// Charge model:
//   - B2C/hybrid: 1 token per 5 soal (ceil)
//   - B2B: 0 token (gratis, ditanggung sekolah)
//
// Idempotency:
//   - Pakai request_id dari client (UUID) sebagai idempotency_key
//   - Kalau client retry dengan request_id sama → tidak double-charge
// ==========================================
func GenerateQuestionsWithCharge(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req GenerateAIWithChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter AI tidak valid: " + err.Error()})
		return
	}

	// Validasi API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key Gemini belum dikonfigurasi di server"})
		return
	}

	// Hitung biaya: 1 token per 5 soal (ceil), hanya untuk B2C/hybrid
	// B2B = 0
	costPer5 := tokensvc.GetFeatureCost("ai_generate_5q", scope.TeacherType)
	batches := (req.NumberOfQ + 4) / 5 // ceil(numQ / 5)
	totalCost := costPer5 * batches

	// Generate request_id unik untuk idempotency & tracking
	requestID := uuid.New().String()


	var txID string
	if totalCost > 0 {
		idemKey := fmt.Sprintf("ai_gen_%s", requestID)

		result, chargeErr := tokensvc.ChargeTokens(
			scope.UserID,
			-totalCost,
			"ai_generate",
			"ai_request",
			&requestID,
			&idemKey,
			map[string]interface{}{
				"topic":          req.Topic,
				"question_type":  req.QuestionType,
				"number_of_q":    req.NumberOfQ,
				"batches":        batches,
				"cost_per_batch": costPer5,
			},
		)

		if chargeErr != nil {
			if chargeErr == tokensvc.ErrInsufficientToken {
				var currentBalance int
				_ = database.DB.QueryRow(
					`SELECT token_balance FROM profiles WHERE id = $1`,
					scope.UserID,
				).Scan(&currentBalance)

				c.JSON(http.StatusPaymentRequired, gin.H{
					"error": gin.H{
						"code":    "INSUFFICIENT_TOKEN",
						"message": fmt.Sprintf("Token tidak cukup. Butuh %d token, saldo %d", totalCost, currentBalance),
					},
					"required":        totalCost,
					"current_balance": currentBalance,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal charge token: " + chargeErr.Error()})
			return
		}

		txID = result.TransactionID
	}


	questions, aiErr := callGeminiAI(apiKey, req)

	if aiErr != nil {
		if totalCost > 0 {
			_, newBal, refundErr := tokensvc.RefundTokens(
				scope.UserID,
				totalCost,
				"refund_ai_failed",
				"ai_request",
				requestID,
				"AI generation gagal: "+aiErr.Error(),
			)
			if refundErr != nil {
				fmt.Printf("[CRITICAL] Refund AI failed for user=%s request=%s: %v\n",
					scope.UserID, requestID, refundErr)
			} else {
				fmt.Printf("[AI] Refunded %d token to user=%s (new balance: %d)\n",
					totalCost, scope.UserID, newBal)
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Gagal men-generate soal dari AI: " + aiErr.Error(),
			"refunded":   totalCost > 0,
			"refund_amount": totalCost,
		})
		return
	}

	var newBalance int
	_ = database.DB.QueryRow(
		`SELECT token_balance FROM profiles WHERE id = $1`,
		scope.UserID,
	).Scan(&newBalance)

	c.JSON(http.StatusOK, gin.H{
		"request_id":     requestID,
		"questions":      questions,
		"charged":        totalCost,
		"charged_tx_id":  txID,
		"new_balance":    newBalance,
		"batches":        batches,
		"cost_per_batch": costPer5,
	})
}


func callGeminiAI(apiKey string, req GenerateAIWithChargeRequest) ([]map[string]any, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("gagal inisialisasi client AI: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-exp")
	model.GenerationConfig = genai.GenerationConfig{
		ResponseMIMEType: "application/json",
	}

	cognitiveLevel := req.CognitiveLevel
	if cognitiveLevel == "" {
		cognitiveLevel = "C2 (Memahami)"
	}

	prompt := fmt.Sprintf(
		"Buatkan %d butir soal tipe %s untuk topik '%s' dengan level kognitif %s. "+
			"Format output harus berupa JSON Array murni berisi objek dengan properti: "+
			"question_text (string), question_type (string), options (array of string atau null jika essay), "+
			"correct_answer (string), explanation (string), cognitive_level (string). "+
			"Setiap opsi multiple choice harus punya format 'A. jawaban', 'B. jawaban', dst. "+
			"Untuk correct_answer, isi dengan huruf opsi (A/B/C/D) untuk multiple_choice, "+
			"atau teks jawaban lengkap untuk essay. "+
			"Jangan tambahkan markdown, code block, atau teks apapun selain JSON array.",
		req.NumberOfQ, req.QuestionType, req.Topic, cognitiveLevel,
	)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini API error: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("gemini mengembalikan response kosong")
	}

	var jsonStr string
	for _, part := range resp.Candidates[0].Content.Parts {
		jsonStr += fmt.Sprintf("%v", part)
	}


	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)


	var questions []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &questions); err != nil {
		return nil, fmt.Errorf("format JSON dari AI tidak valid: %w | raw: %s",
			err, truncateString(jsonStr, 200))
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("AI tidak menghasilkan soal apapun")
	}

	return questions, nil
}


func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}


func GetAIGenerateCost(c *gin.Context) {
	scope, err := helpers.MustGetPersonalScope(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	numQStr := c.DefaultQuery("number_of_q", "5")
	var numQ int
	_, err = fmt.Sscanf(numQStr, "%d", &numQ)
	if err != nil || numQ < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "number_of_q harus >= 1"})
		return
	}

	costPer5 := tokensvc.GetFeatureCost("ai_generate_5q", scope.TeacherType)
	batches := (numQ + 4) / 5
	totalCost := costPer5 * batches

	var currentBalance int
	_ = database.DB.QueryRow(
		`SELECT token_balance FROM profiles WHERE id = $1`,
		scope.UserID,
	).Scan(&currentBalance)

	c.JSON(http.StatusOK, gin.H{
		"number_of_q":     numQ,
		"batches":         batches,
		"cost_per_batch":  costPer5,
		"total_cost":      totalCost,
		"current_balance": currentBalance,
		"sufficient":      currentBalance >= totalCost,
		"teacher_type":    scope.TeacherType,
	})
}


var _ = sql.ErrNoRows
var _ = time.Now