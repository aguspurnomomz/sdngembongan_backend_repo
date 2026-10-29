package exam

import "encoding/json"

type ExamTargetInput struct {
	ClassGroupID    string `json:"class_group_id"`
	ClassSubGroupID string `json:"class_sub_group_id"`
}

type ExamQuestionInput struct {
	ID             string          `json:"id"`
	Type           string          `json:"type" binding:"required,oneof=multiple_choice essay"`
	Order          int             `json:"order"`
	QuestionText   string          `json:"question_text" binding:"required"`
	Options        json.RawMessage `json:"options"`
	CorrectAnswer  string          `json:"correct_answer"`
	AnswerKey      string          `json:"answer_key"`
	Rubric         json.RawMessage `json:"rubric"`
	Explanation    string          `json:"explanation"`
	CognitiveLevel string          `json:"cognitive_level"`
	Score          float64         `json:"score"`
	MinWords       int             `json:"min_words"`
	ImageURL       string          `json:"image_url"`
}

type CreateSchoolExamRequest struct {
	Title           string              `json:"title" binding:"required"`
	Description     string              `json:"description"`
	Subject         string              `json:"subject" binding:"required"`
	ExamType        string              `json:"exam_type"`
	GradeLevel      string              `json:"grade_level"`
	Phase           string              `json:"phase"`
	AcademicYearID  string              `json:"academic_year_id"`
	DurationMinutes int                 `json:"duration_minutes" binding:"required,min=1"`
	PassingScore    float64             `json:"passing_score"`
	ScoringConfig   json.RawMessage     `json:"scoring_config"`
	Questions       []ExamQuestionInput `json:"questions" binding:"required,min=1"`
	Targets         []ExamTargetInput   `json:"targets"`
}

type CreateScheduleRequest struct {
	ExamID            string   `json:"exam_id" binding:"required"`
	ScheduleDate      string   `json:"schedule_date" binding:"required"`
	StartTime         string   `json:"start_time" binding:"required"`
	EndTime           string   `json:"end_time" binding:"required"`
	DurationMode      string   `json:"duration_mode"`
	DurationMinutes   int      `json:"duration_minutes" binding:"required,min=1"`
	TargetSubGroupIDs []string `json:"target_sub_group_ids" binding:"required,min=1"`
	Room              string   `json:"room"`
	SupervisorName    string   `json:"supervisor_name"`
	SessionNotes      string   `json:"session_notes"`
	AccessCode        string   `json:"access_code" binding:"required"`
	RequireLogin      bool     `json:"require_login"`
}

type GradeEssayRequest struct {
	QuestionID string  `json:"question_id" binding:"required"`
	Score      float64 `json:"score" binding:"min=0"`
	Feedback   string  `json:"feedback"`
}

type BlockStudentRequest struct {
	Reason string `json:"reason" binding:"required,min=3"`
}

type WarnStudentRequest struct {
	Message string `json:"message" binding:"required,min=3"`
}