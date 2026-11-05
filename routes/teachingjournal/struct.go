package teachingjournal

import (
	"encoding/json"
)

// ============================================================
// REQUEST STRUCTS
// ============================================================

type CreateJournalRequest struct {
	TimetableEntryID       string           `json:"timetable_entry_id" binding:"required"`
	JournalDate            string           `json:"journal_date" binding:"required"` // YYYY-MM-DD
	ActualStartTime        string           `json:"actual_start_time"`                // HH:MM
	ActualEndTime          string           `json:"actual_end_time"`                  // HH:MM
	Topic                  string           `json:"topic" binding:"required"`
	LearningObjectives     string           `json:"learning_objectives"`
	Method                 string           `json:"method"`
	MediaUsed              string           `json:"media_used"`
	Activities             string           `json:"activities"`
	Reflection             string           `json:"reflection"`
	StudentAttendanceCount *int             `json:"student_attendance_count"`
	StudentAttendances     []StudentAttendanceInput `json:"student_attendances"`
	DailyScores        	   json.RawMessage `json:"daily_scores"`         
	StudentDevelopments    json.RawMessage `json:"student_developments"` 
	Obstacles          	   string          `json:"obstacles"`            
	Solutions          	   string          `json:"solutions"`            
	BkFollowUp         	   bool            `json:"bk_follow_up"`         
	TotalStudents          *int             `json:"total_students"`
	AssignmentGiven        string           `json:"assignment_given"`
	NextPlan               string           `json:"next_plan"`
	Attachments            []map[string]any `json:"attachments"`
	Tags                   []string         `json:"tags"`
}

type UpdateJournalRequest struct {
	ActualStartTime        string           `json:"actual_start_time"`
	ActualEndTime          string           `json:"actual_end_time"`
	Topic                  string           `json:"topic"`
	LearningObjectives     string           `json:"learning_objectives"`
	Method                 string           `json:"method"`
	MediaUsed              string           `json:"media_used"`
	Activities             string           `json:"activities"`
	Reflection             string           `json:"reflection"`
	StudentAttendanceCount *int             `json:"student_attendance_count"`
	StudentAttendances     []StudentAttendanceInput `json:"student_attendances"`
	DailyScores            json.RawMessage `json:"daily_scores"`
	StudentDevelopments    json.RawMessage `json:"student_developments"`
	Obstacles              string          `json:"obstacles"`
	Solutions              string          `json:"solutions"`
	BkFollowUp             bool            `json:"bk_follow_up"`
	TotalStudents          *int             `json:"total_students"`
	AssignmentGiven        string           `json:"assignment_given"`
	NextPlan               string           `json:"next_plan"`
	Attachments            []map[string]any `json:"attachments"`
	Tags                   []string         `json:"tags"`
}

type VerifyJournalRequest struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Note   string `json:"note"`
}

type StudentAttendanceInput struct {
	StudentID string `json:"student_id"`
	Status    string `json:"status"` // "hadir", "sakit", "izin", "alpa"
}

// ============================================================
// INTERNAL CONTEXT — hasil validasi slot jadwal
// ============================================================

type journalSlotContext struct {
	TeacherID        string
	SchoolID         string
	TimetableID      string
	TimetableEntryID string
	AcademicYearID   string
	ClassGroupID     string
	ClassSubGroupID  string
	SubjectID        string
	SubjectName      string
	ClassName        string
	DayOfWeek        int
	PeriodStart      int
	PeriodEnd        int
	ScheduledStart   string // HH:MM
	ScheduledEnd     string // HH:MM
}