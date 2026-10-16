package token

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"teach_partner_dev/database"
)

type ChargeResult struct {
	Success       bool   `json:"success"`
	NewBalance    int    `json:"new_balance"`
	Message       string `json:"message"`
	TransactionID string `json:"transaction_id,omitempty"`
}


func ChargeTokens(
	userID string,
	amount int,
	action string,
	referenceType string,
	referenceID *string,
	idempotencyKey *string,
	metadata map[string]interface{},
) (*ChargeResult, error) {

	
	var refID interface{} = nil
	if referenceID != nil && *referenceID != "" {
		refID = *referenceID
	}

	var idemKey interface{} = nil
	if idempotencyKey != nil && *idempotencyKey != "" {
		idemKey = *idempotencyKey
	}

	
	metaBytes, _ := json.Marshal(metadata)
	if metaBytes == nil {
		metaBytes = []byte("{}")
	}

	var result ChargeResult
	var txID sql.NullString

	err := database.DB.QueryRow(`
		SELECT 
			success,
			new_balance,
			message,
			transaction_id
		FROM charge_tokens($1, $2, $3, $4, $5::uuid, $6, $7::jsonb)
	`, userID, amount, action, referenceType, refID, idemKey, string(metaBytes)).Scan(
		&result.Success,
		&result.NewBalance,
		&result.Message,
		&txID,
	)

	if err != nil {
		return nil, fmt.Errorf("charge_tokens call error: %w", err)
	}

	if txID.Valid {
		result.TransactionID = txID.String
	}


	if !result.Success {
		if containsInsuffient(result.Message) {
			return &result, ErrInsufficientToken
		}
		return &result, fmt.Errorf("charge gagal: %s", result.Message)
	}

	return &result, nil
}


var ErrInsufficientToken = fmt.Errorf("token tidak cukup")

func containsInsuffient(msg string) bool {
	return len(msg) > 0 && (contains(msg, "tidak cukup") || contains(msg, "insufficient"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		 s[len(s)-len(substr):] == substr ||
		 containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}


func RefundTokens(
	userID string,
	amount int, 
	action string,
	referenceType string,
	referenceID string,
	reason string,
) (bool, int, error) {

	var success bool
	var newBalance int
	var txID sql.NullString

	err := database.DB.QueryRow(`
		SELECT success, new_balance, transaction_id
		FROM refund_tokens($1, $2, $3, $4, $5::uuid, $6)
	`, userID, amount, action, referenceType, referenceID, reason).Scan(
		&success, &newBalance, &txID,
	)

	if err != nil {
		return false, 0, fmt.Errorf("refund_tokens call error: %w", err)
	}

	if success {
		log.Printf("[REFUND] user=%s amount=+%d action=%s ref=%s/%s",
			userID, amount, action, referenceType, referenceID)
	}

	return success, newBalance, nil
}


func ChargePublishExam(
	userID string,
	teacherType string,
	examID string,
	totalQuestions int,
) (*PublishCost, error) {

	cost, err := CalculatePublishCost(teacherType, totalQuestions)
	if err != nil {
		return nil, err
	}

	if cost.TotalCost == 0 {
		return cost, nil
	}

	idempotencyBase := fmt.Sprintf("publish_base_%s", examID)
	baseResult, err := ChargeTokens(
		userID,
		-cost.BaseCost,
		"publish_exam",
		"exam",
		&examID,
		&idempotencyBase,
		map[string]interface{}{
			"exam_id":         examID,
			"total_questions": totalQuestions,
			"teacher_type":    teacherType,
			"charge_type":     "base",
		},
	)

	if err != nil {
		return nil, fmt.Errorf("gagal charge base publish: %w", err)
	}

	if !baseResult.Success {
		return nil, ErrInsufficientToken
	}

	if cost.ManualExtraCost > 0 {
		idempotencyExtra := fmt.Sprintf("publish_extra_%s", examID)
		extraResult, err := ChargeTokens(
			userID,
			-cost.ManualExtraCost,
			"manual_extra",
			"exam",
			&examID,
			&idempotencyExtra,
			map[string]interface{}{
				"exam_id":             examID,
				"total_questions":     totalQuestions,
				"manual_extra_multiplier": cost.ManualExtraCost / GetFeatureCost("manual_extra_25q", teacherType),
			},
		)

		if err != nil {
			refundAmount := cost.BaseCost
			_, _, refundErr := RefundTokens(
				userID,
				refundAmount,
				"refund_publish_failed",
				"exam",
				examID,
				"Rollback: manual extra charge gagal",
			)
			if refundErr != nil {
				log.Printf("[CRITICAL] Rollback base publish FAILED for exam=%s: %v", examID, refundErr)
			}
			return nil, fmt.Errorf("gagal charge manual extra: %w", err)
		}

		if !extraResult.Success {
			// Rollback base charge!
			refundAmount := cost.BaseCost
			_, _, refundErr := RefundTokens(
				userID,
				refundAmount,
				"refund_publish_failed",
				"exam",
				examID,
				"Rollback: token tidak cukup untuk manual extra",
			)
			if refundErr != nil {
				log.Printf("[CRITICAL] Rollback base publish FAILED for exam=%s: %v", examID, refundErr)
			}
			return nil, ErrInsufficientToken
		}
	}

	return cost, nil
}