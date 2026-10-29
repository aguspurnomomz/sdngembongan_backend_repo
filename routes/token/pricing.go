package token

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"gembongan_dev_backend/database"
)


type PublishCost struct {
	BaseCost        int             `json:"base_cost"`
	ManualExtraCost int             `json:"manual_extra_cost"`
	TotalCost       int             `json:"total_cost"`
	Breakdown       json.RawMessage `json:"breakdown"`
}


func GetFeatureCost(featureKey, teacherType string) int {
	var cost int
	err := database.DB.QueryRow(
		`SELECT get_feature_cost($1, $2)`,
		featureKey, teacherType,
	).Scan(&cost)

	if err != nil {
		return 0
	}
	return cost
}


func CalculatePublishCost(teacherType string, totalQuestions int) (*PublishCost, error) {
	var pc PublishCost
	var breakdown []byte

	err := database.DB.QueryRow(`
		SELECT 
			base_cost,
			manual_extra_cost,
			total_cost,
			breakdown
		FROM calculate_publish_cost($1, $2)
	`, teacherType, totalQuestions).Scan(
		&pc.BaseCost,
		&pc.ManualExtraCost,
		&pc.TotalCost,
		&breakdown,
	)

	if err != nil {
		return nil, fmt.Errorf("gagal hitung biaya publish: %w", err)
	}

	pc.Breakdown = json.RawMessage(breakdown)
	return &pc, nil
}


func GetUserTeacherType(userID string) (string, error) {
	var t string
	err := database.DB.QueryRow(
		`SELECT get_user_teacher_type($1)`,
		userID,
	).Scan(&t)

	if err != nil {
		if err == sql.ErrNoRows {
			return "b2c", nil 
		}
		return "", fmt.Errorf("gagal get teacher_type: %w", err)
	}
	return t, nil
}