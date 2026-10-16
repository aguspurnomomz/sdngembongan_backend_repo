package teacher

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	
	"teach_partner_dev/database"

	"github.com/gin-gonic/gin"
)


func getUserIDSimple(c *gin.Context) (string, error) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized: user_id tidak ditemukan")
	}

	switch v := userIDRaw.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}


func GetTokenBalance(c *gin.Context) {
	userID, err := getUserIDSimple(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var (
		currentBalance int
		totalCredit    int
		totalDebit     int
		totalTx        int
		lastTxAt       sql.NullTime
	)

	err = database.DB.QueryRow(`
		SELECT 
			COALESCE(current_balance, 0),
			COALESCE(total_credit, 0),
			COALESCE(total_debit, 0),
			COALESCE(total_transactions, 0),
			last_transaction_at
		FROM v_token_balances
		WHERE user_id = $1
	`, userID).Scan(
		&currentBalance, &totalCredit, &totalDebit, &totalTx, &lastTxAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Fallback: user belum ada transaksi, ambil dari profiles
			_ = database.DB.QueryRow(
				`SELECT COALESCE(token_balance, 0) FROM profiles WHERE id = $1`,
				userID,
			).Scan(&currentBalance)

			c.JSON(http.StatusOK, gin.H{
				"current_balance":      currentBalance,
				"total_credit":         0,
				"total_debit":          0,
				"total_transactions":   0,
				"last_transaction_at":  nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"current_balance":     currentBalance,
		"total_credit":        totalCredit,
		"total_debit":         totalDebit,
		"total_transactions":  totalTx,
		"last_transaction_at": nil,
	}

	if lastTxAt.Valid {
		resp["last_transaction_at"] = lastTxAt.Time
	}

	c.JSON(http.StatusOK, resp)
}


func ListTokenTransactions(c *gin.Context) {
	userID, err := getUserIDSimple(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Pagination
	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// Filter opsional
	actionFilter := c.Query("action") // signup_bonus | topup | publish_exam | ...

	query := `
		SELECT 
			id, amount, balance_after, action,
			reference_type, reference_id, metadata, created_at
		FROM token_transactions
		WHERE user_id = $1
	`
	args := []any{userID}
	argIdx := 2

	if actionFilter != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, actionFilter); argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TransactionItem struct {
		ID            string          `json:"id"`
		Amount        int             `json:"amount"`
		BalanceAfter  int             `json:"balance_after"`
		Action        string          `json:"action"`
		ReferenceType *string         `json:"reference_type"`
		ReferenceID   *string         `json:"reference_id"`
		Metadata      json.RawMessage `json:"metadata"`
		CreatedAt     time.Time       `json:"created_at"`
	}

	transactions := []TransactionItem{}
	for rows.Next() {
		var t TransactionItem
		var refType, refID sql.NullString
		var metaJSON []byte

		err := rows.Scan(
			&t.ID, &t.Amount, &t.BalanceAfter, &t.Action,
			&refType, &refID, &metaJSON, &t.CreatedAt,
		)
		if err != nil {
			continue
		}

		if refType.Valid { t.ReferenceType = &refType.String }
		if refID.Valid { t.ReferenceID = &refID.String }
		if len(metaJSON) > 0 {
			t.Metadata = json.RawMessage(metaJSON)
		} else {
			t.Metadata = json.RawMessage("{}")
		}

		transactions = append(transactions, t)
	}

	// Total count untuk pagination info
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM token_transactions WHERE user_id = $1`
	countArgs := []any{userID}

	if actionFilter != "" {
		countQuery += ` AND action = $2`
		countArgs = append(countArgs, actionFilter)
	}

	_ = database.DB.QueryRow(countQuery, countArgs...).Scan(&totalCount)

	c.JSON(http.StatusOK, gin.H{
		"total":        totalCount,
		"limit":        limit,
		"offset":       offset,
		"has_more":     offset+len(transactions) < totalCount,
		"transactions": transactions,
	})
}


func ListTokenPricing(c *gin.Context) {
	userID, _ := getUserIDSimple(c)

	var teacherType string = "b2c"
	if userID != "" {
		_ = database.DB.QueryRow(
			`SELECT COALESCE(teacher_type, 'b2c') FROM profiles WHERE id = $1`,
			userID,
		).Scan(&teacherType)
	}

	rows, err := database.DB.Query(`
		SELECT 
			feature_key, display_name, description, token_cost, applies_to_teacher_type
		FROM token_pricing
		WHERE applies_to_teacher_type = $1 AND is_active = TRUE
		ORDER BY feature_key ASC
	`, teacherType)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type PricingItem struct {
		FeatureKey    string `json:"feature_key"`
		DisplayName   string `json:"display_name"`
		Description   string `json:"description"`
		TokenCost     int    `json:"token_cost"`
		TeacherType   string `json:"applies_to_teacher_type"`
	}

	pricing := []PricingItem{}
	for rows.Next() {
		var p PricingItem
		var desc sql.NullString

		err := rows.Scan(
			&p.FeatureKey, &p.DisplayName, &desc, &p.TokenCost, &p.TeacherType,
		)
		if err != nil {
			continue
		}
		p.Description = desc.String
		pricing = append(pricing, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_type": teacherType,
		"total":        len(pricing),
		"pricing":      pricing,
	})
}