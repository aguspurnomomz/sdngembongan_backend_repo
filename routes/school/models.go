package school

type CreateSubjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category" binding:"omitempty,oneof=wajib mulok pilihan"`
}

type UpdateSubjectRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category" binding:"omitempty,oneof=wajib mulok pilihan"`
	IsActive    *bool  `json:"is_active"`
}

type AssignSubjectsToTeacherRequest struct {
	SubjectIDs []string `json:"subject_ids" binding:"required,min=1"`
	IsPrimary  string   `json:"is_primary"` // opsional: subject_id yang jadi mapel utama
}

type CreateTimetableRequest struct {
	ClassGroupID          string `json:"class_group_id"`
	ClassSubGroupID       string `json:"class_sub_group_id" binding:"required"`
	AcademicYearID        string `json:"academic_year_id" binding:"required"`
	Semester              string `json:"semester" binding:"required"`
	CurriculumStructureID string `json:"curriculum_structure_id"`
	EffectiveFrom         string `json:"effective_from"`
	EffectiveTo           string `json:"effective_to"`
	Notes                 string `json:"notes"`
}

type CreateTimetableEntryRequest struct {
	TimetableID string `json:"timetable_id" binding:"required"`
	DayOfWeek   int    `json:"day_of_week" binding:"required,min=1,max=7"`
	PeriodStart int    `json:"period_start" binding:"required,min=1"`
	PeriodEnd   int    `json:"period_end" binding:"required,min=1"`
	SubjectID   string `json:"subject_id"`
	TeacherID   string `json:"teacher_id"`
	Room        string `json:"room"`
	Notes       string `json:"notes"`
}

type CreateTimeSlotRequest struct {
	PeriodNumber int    `json:"period_number" binding:"required,min=1"`
	StartTime    string `json:"start_time" binding:"required"` // format HH:MM
	EndTime      string `json:"end_time" binding:"required"`   // format HH:MM
	IsBreak      bool   `json:"is_break"`
}

type UpdateTimeSlotRequest struct {
	PeriodNumber int    `json:"period_number"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	IsBreak      *bool  `json:"is_break"`
}

type UpsertTimeSlotOverrideRequest struct {
	DayOfWeek    int    `json:"day_of_week" binding:"required,min=1,max=7"`
	PeriodNumber int    `json:"period_number" binding:"required,min=1"`
	StartTime    string `json:"start_time" binding:"required"`
	EndTime      string `json:"end_time" binding:"required"`
	IsBreak      bool   `json:"is_break"`
	Notes        string `json:"notes"`
}