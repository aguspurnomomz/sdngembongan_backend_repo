package models

import "time"

type QuestionBank struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Subject       string    `json:"subject"`
	Phase         string    `json:"phase"`
	PriceInTokens int       `json:"price_in_tokens"`
	IsPublic      bool      `json:"is_public"`
	CreatedAt     time.Time `json:"created_at"`
}

type Question struct {
	ID             string `json:"id"`
	QuestionBankID string `json:"question_bank_id"`
	QuestionText   string `json:"question_text"`
	QuestionType   string `json:"question_type"`
	Options        any    `json:"options"`       
	CorrectAnswer  any    `json:"correct_answer"` 
	Explanation    string `json:"explanation"`
	CognitiveLevel string `json:"cognitive_level"`
}