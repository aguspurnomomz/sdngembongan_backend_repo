package routes

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"teach_partner_dev/database"
	"teach_partner_dev/middleware"
	"teach_partner_dev/routes/public"
	//"teach_partner_dev/routes/helpers"
    //"teach_partner_dev/routes/token"
	"teach_partner_dev/routes/teacher"

	"time"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	//"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	//"google.golang.org/api/option"
	"io"
	"log"


)

// --- Struct untuk Superadmin ---
type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Location *LocationData `json:"location"`
}

type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

type CreateEbookRequest struct {
	Judul         string `json:"judul" binding:"required"`
	Jenjang       string `json:"jenjang" binding:"required"`
	MataPelajaran string `json:"mata_pelajaran" binding:"required"`
	Kategori      string `json:"kategori" binding:"required"`
	CoverUrl      string `json:"cover_url"`
	FileUrl       string `json:"file_url" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
	Location    *LocationData `json:"location"`
}

// B2B struct admin school
type CreateSchoolRequest struct {
	SchoolName string `json:"school_name" binding:"required"`
	Npsn       string `json:"npsn" binding:"required"`
	Address    string `json:"address"`
	Jenjang    string `json:"jenjang" binding:"required"` 
}

type AcademicYearRequest struct {
	Name      string `json:"name" binding:"required"`
	Semester  string `json:"semester" binding:"required"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type UpdateSchoolProfileRequest struct {
	SchoolName    string  `json:"school_name" binding:"required"`
	Npsn          string  `json:"npsn" binding:"required"`
	Address       string  `json:"address"`
	City          string  `json:"city"`
	Email         string  `json:"email"`
	Accreditation string  `json:"accreditation"`
	SchoolStatus  string  `json:"school_status"`
	Phone         string  `json:"phone"`
	Website       string  `json:"website"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	FoundationName string `json:"foundation_name"`
}

type CreateClassGroupRequest struct {
	Name           string `json:"name" binding:"required"`
	Level          string `json:"level" binding:"required"`
	Major          string `json:"major"`
	ClassType      string `json:"class_type"`
	AcademicYearID string `json:"academic_year_id" binding:"required"`
}

type CreateSubClassRequest struct {
	ClassGroupID      string `json:"class_group_id" binding:"required"`
	Name              string `json:"name" binding:"required"`
	Code              string `json:"code"`
	Capacity          int    `json:"capacity"`
	HomeroomTeacherID string `json:"homeroom_teacher_id"`
	Notes             string `json:"notes"`
}

type UpdateSubClassRequest struct {
	Name              string `json:"name"`
	Code              string `json:"code"`
	Capacity          int    `json:"capacity"`
	HomeroomTeacherID string `json:"homeroom_teacher_id"`
	Notes             string `json:"notes"`
	IsActive          *bool  `json:"is_active"`
}

type CreateStudentRequest struct {
	ClassSubGroupID string `json:"class_sub_group_id" binding:"required"`
	FullName        string `json:"full_name" binding:"required"`
	StudentNumber   string `json:"student_number"`
	NISN            string `json:"nisn"`
	JenisKelamin 	string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
}

type UpdateStudentRequest struct {
	ClassSubGroupID string `json:"class_sub_group_id"`
	FullName        string `json:"full_name"`
	StudentNumber   string `json:"student_number"`
	NISN            string `json:"nisn"`
	JenisKelamin    string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	IsActive        *bool  `json:"is_active"`
}

type MoveStudentRequest struct {
	TargetSubGroupID string `json:"target_sub_group_id" binding:"required"`
}

// Target kelas untuk ujian
type ExamTargetInput struct {
	ClassGroupID    string `json:"class_group_id"`     
	ClassSubGroupID string `json:"class_sub_group_id"` 
}

// Question item (flexible: PG atau Essay)
type ExamQuestionInput struct {
	ID             string          `json:"id"`
	Type           string          `json:"type" binding:"required,oneof=multiple_choice essay"`
	Order          int             `json:"order"`
	QuestionText   string          `json:"question_text" binding:"required"`
	Options        json.RawMessage `json:"options"`         // null kalau essay
	CorrectAnswer  string          `json:"correct_answer"`  // kosong kalau essay
	AnswerKey      string          `json:"answer_key"`      // untuk essay
	Rubric         json.RawMessage `json:"rubric"`          // opsional
	Explanation    string          `json:"explanation"`
	CognitiveLevel string          `json:"cognitive_level"`
	Score          float64         `json:"score"`
	MinWords       int             `json:"min_words"`
	ImageURL       string          `json:"image_url"`
}

// Create / Update
type CreateSchoolExamRequest struct {
	Title           string              `json:"title" binding:"required"`
	Description     string              `json:"description"`
	Subject         string              `json:"subject" binding:"required"`
	ExamType        string              `json:"exam_type"` // regular|uts|uas|remedial|tryout|quiz
	GradeLevel      string              `json:"grade_level"`
	Phase           string              `json:"phase"`
	AcademicYearID  string              `json:"academic_year_id"`
	DurationMinutes int                 `json:"duration_minutes" binding:"required,min=1"`
	PassingScore    float64             `json:"passing_score"`
	ScoringConfig   json.RawMessage     `json:"scoring_config"`
	Questions       []ExamQuestionInput `json:"questions" binding:"required,min=1"`
	Targets         []ExamTargetInput   `json:"targets"` // WAJIB diisi minimal 1
}

type GradeEssayRequest struct {
    QuestionID string  `json:"question_id" binding:"required"`
    Score      float64 `json:"score" binding:"min=0"`
    Feedback   string  `json:"feedback"`
}

// Members Struct

type JoinSchoolRequest struct {
	SchoolID string `json:"school_id" binding:"required"`
	RoleInSchool string `json:"role_in_school"`  // default 'teacher'
	Notes string `json:"notes"`
}

type InviteTeacherRequest struct {
	Email string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required"`
	NIP string `json:"nip"`
	JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	RoleInSchool string `json:"role_in_school"`  // default 'teacher'
	Notes string `json:"notes"`
}

type CreateTeacherRequest struct {
    Email        string `json:"email" binding:"required,email"`
    FullName     string `json:"full_name" binding:"required"`
    Password     string `json:"password" binding:"required,min=6"`
    NIP          string `json:"nip"`
    JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
	NoWhatsApp   string `json:"no_whatsapp"`
    RoleInSchool string `json:"role_in_school"`  
    Notes        string `json:"notes"`
}

type RemoveTeacherRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type AssignTeacherRequest struct {
	UserID string `json:"user_id" binding:"required"`
	RoleInSchool string `json:"role_in_school"`
	Notes string `json:"notes"`
}

// ==========================================
// Attendance struct
// ==========================================

// --- Academic Calendar ---
type CreateCalendarEventRequest struct {
	CalendarDate       string   `json:"calendar_date" binding:"required"` // YYYY-MM-DD
	DayType            string   `json:"day_type" binding:"required"`      // school_day | holiday | weekend | exam | event | emergency
	Name               string   `json:"name" binding:"required"`
	Description        string   `json:"description"`
	IsAttendanceRequired *bool  `json:"is_attendance_required"`
	IncludeInReport    *bool    `json:"include_in_report"`
	TargetClassGroupIDs []string `json:"target_class_group_ids"`
}

type UpdateCalendarEventRequest struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	IsAttendanceRequired *bool    `json:"is_attendance_required"`
	IncludeInReport      *bool    `json:"include_in_report"`
	TargetClassGroupIDs  []string `json:"target_class_group_ids"`
}

// --- Attendance Shifts ---
type CreateShiftRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`

	ShiftCategory string `json:"shift_category"` // regular | exam | event | extracurricular

	CheckInStart       string `json:"check_in_start" binding:"required"`
	CheckInOnTimeStart string `json:"check_in_on_time_start" binding:"required"`
	CheckInOnTimeEnd   string `json:"check_in_on_time_end" binding:"required"`
	CheckInEnd         string `json:"check_in_end" binding:"required"`

	RequireCheckOut      bool   `json:"require_check_out"`
	CheckOutStart        string `json:"check_out_start"`
	CheckOutOnTimeStart  string `json:"check_out_on_time_start"`
	CheckOutOnTimeEnd    string `json:"check_out_on_time_end"`
	CheckOutEnd          string `json:"check_out_end"`

	AllowEarlyCheckIn  *bool `json:"allow_early_check_in"`
	AllowLateCheckOut  *bool `json:"allow_late_check_out"`

	AutoCloseMinutesAfterCheckIn  *int `json:"auto_close_minutes_after_check_in"`
	AutoCloseMinutesAfterCheckOut *int `json:"auto_close_minutes_after_check_out"`

	EarlyThresholdMinutes      *int `json:"early_threshold_minutes"`
	LateToleranceMinutes       *int `json:"late_tolerance_minutes"`
	EarlyLeaveToleranceMinutes *int `json:"early_leave_tolerance_minutes"`

	ApplicableDays []int `json:"applicable_days"`

	IsActive  *bool `json:"is_active"`
	IsDefault *bool `json:"is_default"`
}

type UpdateShiftRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	
	CheckInStart       string `json:"check_in_start"`
	CheckInOnTimeStart string `json:"check_in_on_time_start"`
	CheckInOnTimeEnd   string `json:"check_in_on_time_end"`
	CheckInEnd         string `json:"check_in_end"`

	RequireCheckOut     *bool  `json:"require_check_out"`
	CheckOutStart       string `json:"check_out_start"`
	CheckOutOnTimeStart string `json:"check_out_on_time_start"`
	CheckOutOnTimeEnd   string `json:"check_out_on_time_end"`
	CheckOutEnd         string `json:"check_out_end"`

	IsActive *bool `json:"is_active"`
}

// --- Class Shift Assignments ---
type CreateClassShiftAssignmentRequest struct {
	ClassGroupID    string `json:"class_group_id"`
	ClassSubGroupID string `json:"class_sub_group_id"`
	ShiftID         string `json:"shift_id" binding:"required"`

	Priority  int   `json:"priority"`
	IsPrimary *bool `json:"is_primary"`

	OverrideCheckInStart  string `json:"override_check_in_start"`
	OverrideCheckInEnd    string `json:"override_check_in_end"`
	OverrideCheckOutStart string `json:"override_check_out_start"`
	OverrideCheckOutEnd   string `json:"override_check_out_end"`

	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
}

// --- Attendance Sessions ---
type CreateAttendanceSessionRequest struct {
	ShiftID string `json:"shift_id" binding:"required"`

	ClassGroupID    string `json:"class_group_id"`
	ClassSubGroupID string `json:"class_sub_group_id"`

	Title       string `json:"title"`
	SessionDate string `json:"session_date"` // YYYY-MM-DD, default today
	Notes       string `json:"notes"`
}

type CloseAttendanceSessionRequest struct {
	Notes string `json:"notes"`
}

// --- Scan ---
type ScanQRRequest struct {
	SessionID     string `json:"session_id" binding:"required"`
	QRToken       string `json:"qr_token" binding:"required"`
	ForceMode     string `json:"force_mode"`
}

// --- Student QR ---
type GenerateQRRequest struct {
	StudentIDs []string `json:"student_ids"` 
	ClassSubGroupID string `json:"class_sub_group_id"` 
}

type RegenerateQRRequest struct {
	Reason string `json:"reason"`
}

type StartExamRequest struct {
	NISN        string `json:"nisn" binding:"required"`
	AccessCode  string `json:"access_code" binding:"required"`
	StudentName string `json:"student_name" binding:"required"`
}

// --- Struct Pembelian Token & Midtrans ---
type CreateTransactionRequest struct {
	PackageName string `json:"package_name" binding:"required"`
	TokenAmount int    `json:"token_amount" binding:"required"`
	Amount      int    `json:"amount" binding:"required"`
}

type CreateSchoolAdminRequest struct {
	SchoolID string `json:"school_id" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Nip      string `json:"nip"` 
}

type LocationData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type UpdateProfileRequest struct {
	NamaGuru               string `json:"nama_guru"`
	NipGuru                string `json:"nip_guru"`
	NamaSekolah            string `json:"nama_sekolah"`
	JenisKelamin 			string `json:"jenis_kelamin"`
	MataPelajaran          string `json:"mata_pelajaran"`
	Fase                   string `json:"fase"`
	Kelas                  string `json:"kelas"`
	Semester               string `json:"semester"`
	TahunPelajaran         string `json:"tahun_pelajaran"`
	NamaKepalaSekolah      string `json:"nama_kepala_sekolah"`
	NipKepalaSekolah       string `json:"nip_kepala_sekolah"`
	KotaKabupaten          string `json:"kota_kabupaten"`
	TanggalPenandatanganan string `json:"tanggal_penandatanganan"`
	AlamatSekolah          string `json:"alamat_sekolah"`
	KecamatanKabupaten     string `json:"kecamatan_kabupaten"`
	KodePos                string `json:"kode_pos"`
	TeleponSekolah         string `json:"telepon_sekolah"`
	EmailSekolah           string `json:"email_sekolah"`
	Npsn                   string `json:"npsn"`
	WebsiteSekolah         string `json:"website_sekolah"`
}

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

type GenerateAIRequest struct {
	Topic          string `json:"topic" binding:"required"`
	QuestionType   string `json:"question_type" binding:"required"`
	NumberOfQ      int    `json:"number_of_q" binding:"required"`
	CognitiveLevel string `json:"cognitive_level"`
}

type CreateExamSessionRequest struct {
	Title            string `json:"title" binding:"required"`
	QuestionBankID   string `json:"question_bank_id" binding:"required"`
	DurationMinutes  int    `json:"duration_minutes"`
}

type SubmitExamRequest struct {
	SessionID     string            `json:"session_id" binding:"required"`
	StudentName   string            `json:"student_name" binding:"required"`
	StudentNumber string            `json:"student_number" binding:"required"`
	NISN          string            `json:"nisn" binding:"required"`
	Answers       map[string]string `json:"answers" binding:"required"`
}

type SubmitLiveExamRequest struct {
	Answers          map[string]string `json:"answers" binding:"required"`
	TabSwitchCount   int               `json:"tab_switch_count"`
	TimeSpentSeconds int               `json:"time_spent_seconds"`
}

type CreateScheduleRequest struct {
    ExamID             string   `json:"exam_id" binding:"required"`
    ScheduleDate       string   `json:"schedule_date" binding:"required"`
    StartTime          string   `json:"start_time" binding:"required"`
    EndTime            string   `json:"end_time" binding:"required"`
    DurationMode       string   `json:"duration_mode"` 
    DurationMinutes    int      `json:"duration_minutes" binding:"required,min=1"`
    TargetSubGroupIDs  []string `json:"target_sub_group_ids" binding:"required,min=1"`
    Room               string   `json:"room"`
    SupervisorName     string   `json:"supervisor_name"`
    SessionNotes       string   `json:"session_notes"`
    AccessCode         string   `json:"access_code" binding:"required"`
    RequireLogin       bool     `json:"require_login"`
}

// ==========================================
// EXAM REALTIME CONTROL STRUCTS
// ==========================================

type BlockStudentRequest struct {
    Reason string `json:"reason" binding:"required,min=3"`
}

type WarnStudentRequest struct {
    Message string `json:"message" binding:"required,min=3"`
}

// ==========================================
// SUBJECTS (Mata Pelajaran)
// ==========================================
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

// Assign mapel ke guru
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

// TIME SLOTS (Master Jam)
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



// getJWTAdminSecret — WAJIB fail-fast kalau env tidak di-set
func getJWTAdminSecret() ([]byte, error) {
	secret := os.Getenv("JWT_ADMIN_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_ADMIN_SECRET tidak dikonfigurasi di environment")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_ADMIN_SECRET terlalu pendek (minimal 32 karakter)")
	}
	return []byte(secret), nil
}

// dummyBcryptHash — dipakai untuk constant-time comparison
var dummyBcryptHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// verifyMidtransSignature memverifikasi signature key dari notifikasi Midtrans
func verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, receivedSignature string) bool {
	if receivedSignature == "" {
		return false
	}

	// Bangun raw string sesuai formula Midtrans
	rawString := orderID + statusCode + grossAmount + serverKey

	// Hitung SHA512
	hash := sha512.Sum512([]byte(rawString))
	calculatedSignature := hex.EncodeToString(hash[:])

	// Constant-time comparison untuk cegah timing attack
	return hmac.Equal([]byte(calculatedSignature), []byte(receivedSignature))
}

// writeInternalError — helper untuk log detail internal TAPI
func writeInternalError(c *gin.Context, publicMsg string, internalErr error) {
	if internalErr != nil {
		log.Printf("[INTERNAL ERROR] %s | detail: %v", publicMsg, internalErr)
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": publicMsg,
		},
	})
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func nowJakarta() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback: UTC+7
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return time.Now().In(loc)
}

func strPtr(s string) *string {
	return &s
}

func logSuperAdminActivity(adminID int, action, ipAddress, userAgent, location, details string) {
	query := `INSERT INTO super_admin_logs (admin_id, action, ip_address, user_agent, location, details) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := database.DB.Exec(query, adminID, action, ipAddress, userAgent, location, details)
	if err != nil {
		fmt.Printf("Gagal mencatat log superadmin: %v\n", err)
	}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_TOKEN",
					"message": "Token tidak disertakan!",
				},
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		jwtSecret, err := getJWTAdminSecret()
		if err != nil {
			log.Printf("[AUTH] FATAL: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "SERVER_MISCONFIGURED",
					"message": "Konfigurasi server tidak valid",
				},
			})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Token tidak valid atau kedaluwarsa",
				},
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "superadmin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Akses ditolak: Bukan level administrator",
				},
			})
			c.Abort()
			return
		}

		adminIDFloat, ok := claims["admin_id"].(float64)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "INVALID_TOKEN",
						"message": "Token tidak valid",
					},
				})
				c.Abort()
				return
			}
			adminID := int(adminIDFloat)

			jwtVersionFloat, ok := claims["token_version"].(float64)
			if !ok {
				// Token lama (sebelum implementasi token_version) → paksa re-login
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "TOKEN_EXPIRED",
						"message": "Sesi Anda telah berakhir. Silakan login ulang.",
					},
				})
				c.Abort()
				return
			}
			jwtVersion := int(jwtVersionFloat)

			var dbVersion int
			err = database.DB.QueryRow(
				`SELECT COALESCE(token_version, 1) FROM super_admins WHERE id = $1`,
				adminID,
			).Scan(&dbVersion)
			if err != nil {
				// Fail-safe: kalau tidak bisa cek, reject
				log.Printf("[AUTH] Gagal cek token_version untuk admin_id=%d: %v", adminID, err)
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "INVALID_TOKEN",
						"message": "Token tidak valid",
					},
				})
				c.Abort()
				return
			}

			if jwtVersion != dbVersion {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "TOKEN_REVOKED",
						"message": "Sesi Anda telah berakhir karena kata sandi telah diubah. Silakan login ulang.",
					},
				})
				c.Abort()
				return
			}

			c.Set("admin_email", claims["email"])
			c.Set("admin_id", adminID)

			c.Next()
	}
}

func sendWhatsAppNotification(targetPhone string, message string) {
	apiToken := os.Getenv("WA_GATEWAY_TOKEN")
	apiURL := os.Getenv("WA_GATEWAY_URL") 

	if apiToken == "" || apiURL == "" || targetPhone == "" {
		return 
	}

	payload := map[string]string{
		"target":  targetPhone,
		"message": message,
	}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	_, _ = client.Do(req)
}

func sendResendEmail(toEmail string, subject string, htmlContent string) {
	apiKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("RESEND_FROM_EMAIL") 

	if apiKey == "" || fromEmail == "" || toEmail == "" {
		fmt.Printf("Resend API Key atau From Email belum dikonfigurasi\n")
		return
	}

	payload := map[string]any{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlContent,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Gagal marshal payload Resend: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Gagal membuat request Resend: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Gagal mengirim email via Resend: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("Resend mengembalikan status error: %d\n", resp.StatusCode)
	}
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func getSchoolIDFromUserA(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE (id = $1 OR (email <> '' AND email = $2)) 
		AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)

	if err != nil {
		return "", fmt.Errorf("akses ditolak: admin sekolah tidak valid")
	}

	return schoolID, nil
}

func getSchoolIDFromUserB(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE (id = $1 OR (email <> '' AND email = $2)) 
		AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)

	if err != nil {
		return "", fmt.Errorf("akses ditolak: admin sekolah tidak valid")
	}

	

	var schoolActive bool
	err = database.DB.QueryRow(
		`SELECT COALESCE(is_active, false) FROM schools WHERE id = $1`,
		schoolID,
	).Scan(&schoolActive)

	if err != nil {
		return "", fmt.Errorf("data sekolah tidak ditemukan")
	}

	if !schoolActive {
		return "", fmt.Errorf("sekolah Anda sedang dinonaktifkan. Hubungi administrator untuk informasi lebih lanjut")
	}

	return schoolID, nil
}

func getSchoolIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	// Normalisasi userID ke string
	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	// Ambil email sebagai fallback
	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	// ==========================================
	// 1. Coba cari sebagai school_admin
	// ==========================================
	var schoolID string
	err := database.DB.QueryRow(`
		SELECT school_id FROM school_admins 
		WHERE (id = $1 OR (email <> '' AND email = $2)) 
		AND is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID)

	// ==========================================
	// 2. Kalau bukan admin, cek apakah dia teacher di sekolah aktif
	// ==========================================
	if err != nil || schoolID == "" {
		var teacherSchoolID string
		errTeacher := database.DB.QueryRow(`
			SELECT sm.school_id
			FROM school_members sm
			JOIN schools s ON sm.school_id = s.id
			WHERE sm.user_id = $1 
			  AND sm.is_active = TRUE 
			  AND sm.left_at IS NULL
			  AND s.is_active = TRUE
			LIMIT 1
		`, userIDStr).Scan(&teacherSchoolID)

		if errTeacher == nil {
			return teacherSchoolID, nil
		}

		return "", fmt.Errorf("akses ditolak: Anda bukan admin sekolah atau guru di sekolah aktif")
	}

	// ==========================================
	// 3. Kalau dia admin sekolah, pastikan sekolah aktif
	// ==========================================
	var schoolActive bool
	err = database.DB.QueryRow(
		`SELECT COALESCE(is_active, false) FROM schools WHERE id = $1`,
		schoolID,
	).Scan(&schoolActive)

	if err != nil {
		return "", fmt.Errorf("data sekolah tidak ditemukan")
	}

	if !schoolActive {
		return "", fmt.Errorf("sekolah Anda sedang dinonaktifkan. Hubungi administrator untuk informasi lebih lanjut")
	}

	return schoolID, nil
}

// Menghandle int, int64, float64, dan string
func getAdminIDFromContext(c *gin.Context) (int, bool) {
	raw, exists := c.Get("admin_id")
	if !exists {
		return 0, false
	}

	switch v := raw.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

// getAdminIDFromUser — helper 
func getAdminIDFromUser(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized")
	}

	var userIDStr string
	switch v := userID.(type) {
	case string:
		userIDStr = v
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	return userIDStr, nil
}

// generateQRToken 
func generateQRToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1 * time.Nanosecond) // ensure different seed
	}
	return "STU-" + string(b)
}

// parseTimeString — parse "HH:MM" / "HH:MM:SS" 
func parseTimeString(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	// Coba HH:MM:SS dulu
	t, err := time.Parse("15:04:05", s)
	if err == nil {
		return t, nil
	}
	// Fallback HH:MM
	return time.Parse("15:04", s)
}

// timeToMinutes 
func timeToMinutes(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// determineCheckInStatus 
func determineCheckInStatus(checkInStart, checkInEnd, scanTime time.Time) string {
	startMin := timeToMinutes(checkInStart)
	endMin := timeToMinutes(checkInEnd)
	scanMin := timeToMinutes(scanTime)

	if scanMin < startMin {
		return "early"
	}
	if scanMin > endMin {
		return "late"
	}
	// Di tengah: on_time atau early (tergantung konfigurasi)
	// Untuk simple: pakai 1/3 dan 2/3 sebagai threshold
	rangeMin := endMin - startMin
	if rangeMin > 0 {
		firstThird := startMin + rangeMin/3
		if scanMin < firstThird {
			return "early"
		}
	}
	return "on_time"
}

// determineCheckOutStatus 
func determineCheckOutStatus(checkOutStart, checkOutEnd, scanTime time.Time) string {
	startMin := timeToMinutes(checkOutStart)
	endMin := timeToMinutes(checkOutEnd)
	scanMin := timeToMinutes(scanTime)

	if scanMin < startMin {
		return "early_leave"
	}
	if scanMin > endMin {
		return "late_leave"
	}
	// Di tengah: on_time_leave atau early_leave
	rangeMin := endMin - startMin
	if rangeMin > 0 {
		firstThird := startMin + rangeMin/3
		if scanMin < firstThird {
			return "early_leave"
		}
	}
	return "on_time_leave"
}

func generateSessionToken() string {
	b := make([]byte, 24)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[time.Now().UnixNano()%62]
		time.Sleep(1 * time.Nanosecond)
	}
	return "SES-" + string(b)
}

func GeneratePresignedPutURL(ctx context.Context, objectKey, contentType string) (string, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucketName == "" {
		return "", fmt.Errorf("konfigurasi R2 belum lengkap (cek R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME)")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
		config.WithRegion("auto"),
	)
	if err != nil {
		return "", fmt.Errorf("gagal load config R2: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID))
	})

	presignClient := s3.NewPresignClient(client)

	result, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(10*time.Minute))

	if err != nil {
		return "", fmt.Errorf("gagal generate presigned URL: %w", err)
	}

	return result.URL, nil
}

// sendExamControlBroadcast mengirim event realtime ke channel `exam-control-{scheduleID}`
// lewat Supabase Realtime REST API.
func sendExamControlBroadcast(scheduleID string, event map[string]any) error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if supabaseURL == "" || serviceRoleKey == "" {
		return fmt.Errorf("SUPABASE_URL atau SUPABASE_SERVICE_ROLE_KEY belum dikonfigurasi")
	}

	// Endpoint Broadcast HTTP di Supabase Realtime v2
	url := fmt.Sprintf("%s/realtime/v1/api/broadcast", supabaseURL)

	topic := fmt.Sprintf("exam-control-%s", scheduleID)

	payload := map[string]any{
		"messages": []map[string]any{
			{
				"topic":   topic,
				"event":   "control",
				"payload": event,
			},
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gagal marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("gagal buat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", serviceRoleKey)
	req.Header.Set("Authorization", "Bearer "+serviceRoleKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal kirim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broadcast gagal %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func SetupRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "Server berjalan dengan baik"})
	})

	r.GET("/debug-sentry", func(c *gin.Context) {
		panic("Test Sentry Error dari Backend Golang!")
	})

	r.POST("/api/payment/webhook", func(c *gin.Context) {
		var notification map[string]any
		if err := c.ShouldBindJSON(&notification); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVALID_PAYLOAD",
					"message": "Format notifikasi tidak valid",
				},
			})
			return
		}

		orderID, _           := notification["order_id"].(string)
		transactionStatus, _ := notification["transaction_status"].(string)
		fraudStatus, _       := notification["fraud_status"].(string)
		statusCode, _        := notification["status_code"].(string)
		grossAmount, _       := notification["gross_amount"].(string)
		receivedSignature, _ := notification["signature_key"].(string)

		if orderID == "" || transactionStatus == "" || statusCode == "" || grossAmount == "" {
			log.Printf("[WEBHOOK] Field wajib tidak lengkap: orderID=%q status=%q statusCode=%q grossAmount=%q",
				orderID, transactionStatus, statusCode, grossAmount)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "MISSING_FIELDS",
					"message": "Field wajib tidak lengkap",
				},
			})
			return
		}


		serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
		if serverKey == "" {
			log.Printf("[WEBHOOK] FATAL: MIDTRANS_SERVER_KEY tidak dikonfigurasi di environment")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "SERVER_MISCONFIGURED",
					"message": "Konfigurasi server tidak valid",
				},
			})
			return
		}

		if !verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, receivedSignature) {
			log.Printf("[WEBHOOK] Signature TIDAK VALID untuk order_id=%q dari IP=%s — MUNGKIN SERANGAN!",
				orderID, c.ClientIP())
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "INVALID_SIGNATURE",
					"message": "Signature tidak valid",
				},
			})
			return
		}

		log.Printf("[WEBHOOK] Signature valid untuk order_id=%q, status=%q, fraud=%q",
			orderID, transactionStatus, fraudStatus)

		switch transactionStatus {

		// ---------- Pembayaran SUKSES ----------
		case "capture", "settlement":
			if fraudStatus != "" && fraudStatus != "accept" {
				log.Printf("[WEBHOOK] Order %s fraud_status=%q, skip proses", orderID, fraudStatus)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "fraud not accepted"})
				return
			}

			// Ambil data order untuk verifikasi
			var userID string
			var tokenAmount int
			var dbAmount int
			var currentStatus string
			err := database.DB.QueryRow(
				`SELECT user_id, token_amount, amount, status FROM token_orders WHERE order_id = $1`,
				orderID,
			).Scan(&userID, &tokenAmount, &dbAmount, &currentStatus)

			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("[WEBHOOK] Order %s tidak ditemukan di database", orderID)
					// Return 200 supaya Midtrans tidak retry terus-menerus
					c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "order not found"})
					return
				}
				log.Printf("[WEBHOOK] Error query order %s: %v", orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			// ---------- IDEMPOTENCY CHECK ----------
			if currentStatus == "success" {
				log.Printf("[WEBHOOK] Order %s sudah diproses sebelumnya, skip (idempotent)", orderID)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "already processed"})
				return
			}

			if currentStatus != "pending" {
				log.Printf("[WEBHOOK] Order %s status tidak valid untuk diproses: %q", orderID, currentStatus)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "invalid status transition"})
				return
			}

			// ---------- VERIFIKASI AMOUNT ----------
			midtransAmountFloat, err := strconv.ParseFloat(grossAmount, 64)
			if err != nil {
				log.Printf("[WEBHOOK] Order %s gross_amount tidak valid: %q", orderID, grossAmount)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "invalid amount format"})
				return
			}
			if int(midtransAmountFloat) != dbAmount {
				log.Printf("[WEBHOOK] ⚠️ Amount MISMATCH untuk order %s: Midtrans=%v, DB=%d",
					orderID, midtransAmountFloat, dbAmount)
				// Jangan proses! Amount tidak cocok bisa jadi indikasi tampering
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "amount mismatch"})
				return
			}

			// ---------- PROSES DALAM TRANSACTION ----------
			tx, err := database.DB.Begin()
			if err != nil {
				log.Printf("[WEBHOOK] Gagal mulai transaksi untuk order %s: %v", orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}
			defer tx.Rollback()

			result, err := tx.Exec(
				`UPDATE token_orders SET status = 'success', updated_at = NOW() 
				WHERE order_id = $1 AND status = 'pending'`,
				orderID,
			)
			if err != nil {
				log.Printf("[WEBHOOK] Gagal update order %s: %v", orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				log.Printf("[WEBHOOK] Order %s sudah di-proses oleh request lain (race condition)", orderID)
				c.JSON(http.StatusOK, gin.H{"status": "ok", "note": "concurrent processing detected"})
				return
			}

			_, err = tx.Exec(
				`UPDATE profiles SET token_balance = token_balance + $1, updated_at = NOW() 
				WHERE id = $2`,
				tokenAmount, userID,
			)
			if err != nil {
				log.Printf("[WEBHOOK] Gagal tambah token untuk user %s: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			if err := tx.Commit(); err != nil {
				log.Printf("[WEBHOOK] Gagal commit transaksi order %s: %v", orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			log.Printf("[WEBHOOK] Order %s BERHASIL diproses: user=%s +%d token",
				orderID, userID, tokenAmount)

		// ---------- Pembayaran GAGAL/DIBATALKAN ----------
		case "cancel", "deny", "expire":
			result, err := database.DB.Exec(
				`UPDATE token_orders SET status = $1, updated_at = NOW() 
				WHERE order_id = $2 AND status = 'pending'`,
				transactionStatus, orderID,
			)
			if err != nil {
				log.Printf("[WEBHOOK] Gagal update order %s ke status %s: %v", orderID, transactionStatus, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			log.Printf("[WEBHOOK] Order %s status di-update ke %q (rows affected: %d)",
				orderID, transactionStatus, rowsAffected)

		// ---------- Status tidak dikenal ----------
		default:
			log.Printf("[WEBHOOK] Order %s transaction_status tidak dikenal: %q", orderID, transactionStatus)
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/api/superadmin/superadmin-login", func(c *gin.Context) {
		var req AdminLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVALID_INPUT",
					"message": "Format input tidak valid",
					"details": err.Error(),
				},
			})
			return
		}

		const genericCredentialError = "Email atau kata sandi salah"

		var adminID int
		var hashedPassword string
		var adminName string
		var failedAttempts int
		var lockedUntil sql.NullTime
		var waNumber sql.NullString
		var tokenVersion int

		query := `SELECT id, password_hash, nama, failed_login_attempts, locked_until, no_whatsapp, token_version FROM super_admins WHERE email = $1`
		err := database.DB.QueryRow(query, req.Email).Scan(
			&adminID, &hashedPassword, &adminName, &failedAttempts, &lockedUntil, &waNumber, &tokenVersion,  
		)

		// Siapkan lokasi (dipakai di log & WA)
		lokasiStr := "Tidak diizinkan / Tidak tersedia"
		mapsLink := "Tidak tersedia"
		if req.Location != nil {
			lokasiStr = fmt.Sprintf("%.6f, %.6f", req.Location.Latitude, req.Location.Longitude)
			mapsLink = fmt.Sprintf("%.6f, %.6f (https://maps.google.com/?q=%.6f,%.6f)",
				req.Location.Latitude, req.Location.Longitude,
				req.Location.Latitude, req.Location.Longitude)
		}

		
		// Email tidak ditemukan
		if err == sql.ErrNoRows {
			_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_CREDENTIALS",
					"message": genericCredentialError,
				},
			})
			return
		}

		// Error database lain 
		if err != nil {
			writeInternalError(c, "Terjadi kesalahan pada server", err)
			return
		}

		// Cek lock — dengan reset jika sudah expired
		if lockedUntil.Valid {
			var isLocked bool
			var sisaDetik int

			err = database.DB.QueryRow(`
				SELECT 
					COALESCE(locked_until > NOW(), false) AS is_locked,
					COALESCE(GREATEST(0, EXTRACT(EPOCH FROM (locked_until - NOW()))::int), 0) AS sisa_detik
				FROM super_admins 
				WHERE id = $1
			`, adminID).Scan(&isLocked, &sisaDetik)

			if err != nil {
				writeInternalError(c, "Terjadi kesalahan pada server", err)
				return
			}

			if isLocked {
				c.Header("Retry-After", strconv.Itoa(sisaDetik))
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": gin.H{
						"code":                "ACCOUNT_LOCKED",
						"message":             fmt.Sprintf("Akun dikunci sementara. Coba lagi dalam %d detik.", sisaDetik),
						"retry_after_seconds": sisaDetik,
					},
				})
				return
			}

			// Reset counter biar user punya 3 kesempatan baru
			if lockedUntil.Valid {
				if _, resetErr := database.DB.Exec(
					`UPDATE super_admins SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`,
					adminID,
				); resetErr != nil {
					log.Printf("[LOGIN] Gagal reset lock counter untuk admin_id=%d: %v", adminID, resetErr)
				}
				failedAttempts = 0
			}
		}

		// Verifikasi password
		err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
		if err != nil {
			failedAttempts++

			if failedAttempts >= 3 {
				_, dbErr := database.DB.Exec(
					`UPDATE super_admins SET failed_login_attempts = $1, locked_until = NOW() + INTERVAL '1 minute' WHERE id = $2`,
					failedAttempts, adminID,
				)
				if dbErr != nil {
					log.Printf("[LOGIN] Gagal update lock untuk admin_id=%d: %v", adminID, dbErr)
				}

				// Kirim notifikasi WA (async, non-blocking)
				if waNumber.Valid && waNumber.String != "" {
					ipClient := c.ClientIP()
					userAgent := c.Request.UserAgent()
					pesanAlert := fmt.Sprintf(
						"PERINGATAN KEAMANAN\n\nTerdeteksi 3 kali percobaan login yang gagal pada akun Superadmin Anda.\nAkun telah dikunci sementara selama 1 menit.\n\nDetail Akses:\n- Lokasi: %s\n- Alamat IP: %s\n- Perangkat: %s",
						mapsLink, ipClient, userAgent,
					)
					go sendWhatsAppNotification(waNumber.String, pesanAlert)
				}

				logSuperAdminActivity(adminID, "LOGIN_LOCKED", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Akun dikunci 1 menit karena 3x salah password")

				c.Header("Retry-After", "60")
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": gin.H{
						"code":                "ACCOUNT_LOCKED",
						"message":             "Terlalu banyak percobaan gagal. Akun dikunci selama 60 detik.",
						"retry_after_seconds": 60,
					},
				})
				return
			}

			
			_, dbErr := database.DB.Exec(
				`UPDATE super_admins SET failed_login_attempts = $1 WHERE id = $2`,
				failedAttempts, adminID,
			)
			if dbErr != nil {
				log.Printf("[LOGIN] Gagal update failed_attempts untuk admin_id=%d: %v", adminID, dbErr)
			}

			logSuperAdminActivity(adminID, "LOGIN_FAILED", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Password salah")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_CREDENTIALS",
					"message": genericCredentialError,
				},
			})
			return
		}

		_, _ = database.DB.Exec(
			`UPDATE super_admins SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`,
			adminID,
		)

		logSuperAdminActivity(adminID, "LOGIN_SUCCESS", c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Login berhasil")

		if waNumber.Valid && waNumber.String != "" {
			ipClient := c.ClientIP()
			userAgent := c.Request.UserAgent()
			pesanWA := fmt.Sprintf(
				"INFORMASI LOGIN SUPERADMIN\n\nAkun Superadmin Anda baru saja masuk pada %s.\n\nDetail Akses:\n- Lokasi (Koordinat): %s\n- Alamat IP: %s\n- Perangkat: %s\n\nJika ini bukan Anda, segera amankan akun Anda! \n\nCopyright © 2025 - 2026 SkoolaGo. All Rights Reserved.",
				time.Now().Format("02-01-2006 15:04:05"),
				mapsLink, ipClient, userAgent,
			)
			go sendWhatsAppNotification(waNumber.String, pesanWA)
		}

		jwtSecret, err := getJWTAdminSecret()
		if err != nil {
			log.Printf("[LOGIN] FATAL: %v", err)
			writeInternalError(c, "Terjadi kesalahan pada server", err)
			return
		}

		const tokenTTL = 24 * time.Hour
		claims := jwt.MapClaims{
			"admin_id": adminID,
			"email":    req.Email,
			"role":     "superadmin",
			"token_version": tokenVersion,
			"exp":      time.Now().Add(tokenTTL).Unix(),
			"iat":      time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			writeInternalError(c, "Gagal menghasilkan token admin", err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"token":        tokenString,
				"token_type":   "Bearer",
				"expires_in":   int(tokenTTL.Seconds()),
				"admin": gin.H{
					"id":    adminID,
					"name":  adminName,
					"email": req.Email,
					"role":  "superadmin",
				},
			},
		})
	})

	adminApi := r.Group("/api/superadmin")
	adminApi.Use(AdminAuthMiddleware())
	{
		adminApi.GET("/stats", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Selamat datang di Panel Superadmin!",
				"admin":   c.MustGet("admin_email"),
			})
		})

		adminApi.GET("/logs", func(c *gin.Context) {
			rows, err := database.DB.Query(`
				SELECT l.id, a.email, l.action, l.ip_address, l.user_agent, l.location, l.details, l.created_at 
				FROM super_admin_logs l
				LEFT JOIN super_admins a ON l.admin_id = a.id
				ORDER BY l.created_at DESC
				LIMIT 50
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log aktivitas: " + err.Error()})
				return
			}
			defer rows.Close()

			type LogItem struct {
				ID        int       `json:"id"`
				Email     string    `json:"email"`
				Action    string    `json:"action"`
				IpAddress string    `json:"ip_address"`
				UserAgent string    `json:"user_agent"`
				Location  string    `json:"location"`
				Details   string    `json:"details"`
				CreatedAt time.Time `json:"created_at"`
			}

			var logs []LogItem
			for rows.Next() {
				var item LogItem
				var email, ip, ua, loc, details sql.NullString
				if err := rows.Scan(&item.ID, &email, &item.Action, &ip, &ua, &loc, &details, &item.CreatedAt); err == nil {
					item.Email = email.String
					item.IpAddress = ip.String
					item.UserAgent = ua.String
					item.Location = loc.String
					item.Details = details.String
					logs = append(logs, item)
				}
			}

			c.JSON(http.StatusOK, gin.H{"logs": logs})
		})

		adminApi.POST("/change-password", func(c *gin.Context) {
			var adminID int

			if adminIDRaw, exists := c.Get("admin_id"); exists {
				switch v := adminIDRaw.(type) {
				case int:
					adminID = v
				case int64:
					adminID = int(v)
				case float64:
					adminID = int(v)
				default:
					writeInternalError(c, "Format admin_id tidak dikenali", 
						fmt.Errorf("unexpected admin_id type: %T", adminIDRaw))
					return
				}
			} else {
				emailRaw, ok := c.Get("admin_email")
				if !ok {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": gin.H{
							"code":    "MISSING_SESSION",
							"message": "Sesi admin tidak dikenali",
						},
					})
					return
				}
				email, ok := emailRaw.(string)
				if !ok || email == "" {
					writeInternalError(c, "Format admin_email tidak dikenali", nil)
					return
				}
				if err := database.DB.QueryRow(
					`SELECT id FROM super_admins WHERE email = $1`, email,
				).Scan(&adminID); err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": gin.H{
							"code":    "MISSING_SESSION",
							"message": "Sesi admin tidak dikenali",
						},
					})
					return
				}
			}

			var req ChangePasswordRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"code":    "INVALID_INPUT",
						"message": "Format input tidak valid (minimal 12 karakter untuk superadmin)",
						"details": err.Error(),
					},
				})
				return
			}

			if len(req.NewPassword) < 12 {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"code":    "PASSWORD_TOO_SHORT",
						"message": "Kata sandi baru minimal 12 karakter untuk superadmin",
					},
				})
				return
			}

			if req.NewPassword == req.OldPassword {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"code":    "PASSWORD_UNCHANGED",
						"message": "Kata sandi baru harus berbeda dari kata sandi lama",
					},
				})
				return
			}

			var currentHash string
			var waNumber sql.NullString

			if err := database.DB.QueryRow(
				`SELECT password_hash, no_whatsapp FROM super_admins WHERE id = $1`,
				adminID,
			).Scan(&currentHash, &waNumber); err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{
						"error": gin.H{
							"code":    "ADMIN_NOT_FOUND",
							"message": "Data admin tidak ditemukan",
						},
					})
					return
				}
				writeInternalError(c, "Gagal mengambil data admin", err)
				return
			}


			err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword))
			if err != nil {
				logSuperAdminActivity(adminID, "CHANGE_PASSWORD_FAILED",
					c.ClientIP(), c.Request.UserAgent(), "", "Kata sandi lama salah")

				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"code":    "INVALID_OLD_PASSWORD",
						"message": "Kata sandi lama salah",
					},
				})
				return
			}

			newHashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
			if err != nil {
				writeInternalError(c, "Gagal memproses kata sandi baru", err)
				return
			}

			tx, err := database.DB.Begin()
			if err != nil {
				writeInternalError(c, "Gagal mulai transaksi", err)
				return
			}
			defer tx.Rollback()

			_, err = tx.Exec(`
				UPDATE super_admins SET
					password_hash = $1,
					failed_login_attempts = 0,    
					locked_until = NULL, 
					token_version = token_version + 1,        
					updated_at = NOW()
				WHERE id = $2
			`, string(newHashBytes), adminID)
			if err != nil {
				writeInternalError(c, "Gagal menyimpan kata sandi baru", err)
				return
			}

			if err := tx.Commit(); err != nil {
				writeInternalError(c, "Gagal commit perubahan", err)
				return
			}

	
			lokasiStr := "Tidak diizinkan / Tidak tersedia"
			mapsLink := "Tidak tersedia"
			if req.Location != nil {
				lokasiStr = fmt.Sprintf("%.6f, %.6f", req.Location.Latitude, req.Location.Longitude)
				mapsLink = fmt.Sprintf("%.6f, %.6f (https://maps.google.com/?q=%.6f,%.6f)",
					req.Location.Latitude, req.Location.Longitude,
					req.Location.Latitude, req.Location.Longitude)
			}

			logSuperAdminActivity(adminID, "CHANGE_PASSWORD",
				c.ClientIP(), c.Request.UserAgent(), lokasiStr, "Berhasil mengubah kata sandi")

	
			if waNumber.Valid && waNumber.String != "" {
				pesanWA := fmt.Sprintf(
					"PERINGATAN KEAMANAN\n\nKata sandi akun Superadmin Anda baru saja diubah pada %s WIB.\n\nDetail Akses:\n- Lokasi: %s\n- Alamat IP: %s\n- Perangkat: %s\n\nJika Anda tidak merasa melakukan ini, segera amankan akun Anda!",
					nowJakarta().Format("02-01-2006 15:04:05"),
					mapsLink,
					c.ClientIP(),
					c.Request.UserAgent(),
				)
				go sendWhatsAppNotification(waNumber.String, pesanWA)
			}

			c.JSON(http.StatusOK, gin.H{
				"message":         "Kata sandi berhasil diperbarui. Silakan login ulang.",
				"session_ended":   true,  
			})
		})

		adminApi.GET("/registered-users", func(c *gin.Context) {
			// Filter tab: ?type=all|b2c|b2b|hybrid|admin
			filterType := c.DefaultQuery("type", "all")

			query := `
				SELECT 
					p.id, 
					COALESCE(p.nama_guru, ''), 
					COALESCE(p.nip_guru, ''), 
					COALESCE(p.nama_sekolah, ''), 
					COALESCE(p.mata_pelajaran, ''), 
					p.token_balance, 
					p.is_active, 
					p.last_login, 
					p.updated_at,
					COALESCE(p.teacher_type, 'b2c') AS teacher_type,
					COALESCE(
						(SELECT json_agg(json_build_object(
							'school_id', sm.school_id,
							'school_name', s.school_name,
							'school_active', s.is_active,
							'role_in_school', sm.role_in_school
						))
						FROM school_members sm
						JOIN schools s ON sm.school_id = s.id
						WHERE sm.user_id = p.id 
						AND sm.is_active = TRUE 
						AND sm.left_at IS NULL), 
						'[]'::json
					) AS memberships
				FROM profiles p
				WHERE COALESCE(p.role, 'independent_teacher') <> 'school_admin'
			`

			args := []any{}
			if filterType != "all" {
				query += ` AND COALESCE(p.teacher_type, 'b2c') = $1`
				args = append(args, filterType)
			}

			query += ` ORDER BY p.updated_at DESC`

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pengguna: " + err.Error()})
				return
			}
			defer rows.Close()

			type UserListItem struct {
				ID            string          `json:"id"`
				NamaGuru      string          `json:"nama_guru"`
				NipGuru       string          `json:"nip_guru"`
				NamaSekolah   string          `json:"nama_sekolah"`
				MataPelajaran string          `json:"mata_pelajaran"`
				TokenBalance  int             `json:"token_balance"`
				IsActive      bool            `json:"is_active"`
				LastLogin     *time.Time      `json:"last_login"`
				UpdatedAt     time.Time       `json:"updated_at"`
				TeacherType   string          `json:"teacher_type"`
				Memberships   json.RawMessage `json:"memberships"`
			}

			var users []UserListItem
			for rows.Next() {
				var u UserListItem
				var namaGuru, nipGuru, namaSekolah, mataPelajaran sql.NullString
				var lastLogin sql.NullTime
				var membershipsJSON []byte

				err := rows.Scan(
					&u.ID, &namaGuru, &nipGuru, &namaSekolah, &mataPelajaran,
					&u.TokenBalance, &u.IsActive, &lastLogin, &u.UpdatedAt,
					&u.TeacherType, &membershipsJSON,
				)
				if err != nil {
					continue
				}

				u.NamaGuru = namaGuru.String
				u.NipGuru = nipGuru.String
				u.NamaSekolah = namaSekolah.String
				u.MataPelajaran = mataPelajaran.String
				if lastLogin.Valid {
					u.LastLogin = &lastLogin.Time
				}
				u.Memberships = json.RawMessage(membershipsJSON)

				users = append(users, u)
			}

			// Hitung statistik
			var totalB2C, totalB2B, totalHybrid int
			for _, u := range users {
				switch u.TeacherType {
				case "b2c":
					totalB2C++
				case "b2b":
					totalB2B++
				case "hybrid":
					totalHybrid++
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total_users":  len(users),
				"total_b2c":    totalB2C,
				"total_b2b":    totalB2B,
				"total_hybrid": totalHybrid,
				"users":        users,
			})
		})

		adminApi.PATCH("/users/:id/status", func(c *gin.Context) {
			userID := c.Param("id")
			var req UpdateUserStatusRequest

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			query := `UPDATE profiles SET is_active = $1, updated_at = NOW() WHERE id = $2`
			result, err := database.DB.Exec(query, req.IsActive, userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status pengguna: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pengguna tidak ditemukan"})
				return
			}

			statusText := "diaktifkan"
			if !req.IsActive {
				statusText = "diblokir"
			}

			c.JSON(http.StatusOK, gin.H{
				"message":   fmt.Sprintf("Pengguna berhasil %s", statusText),
				"user_id":   userID,
				"is_active": req.IsActive,
			})
		})

		adminApi.GET("/ebooks", func(c *gin.Context) {
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

			var ebooks []EbookItem
			for rows.Next() {
				var e EbookItem
				var coverUrl sql.NullString
				
				err := rows.Scan(&e.ID, &e.Judul, &e.Jenjang, &e.MataPelajaran, &e.Kategori, &coverUrl, &e.FileUrl, &e.CreatedAt)
				if err != nil {
					continue
				}
				e.CoverUrl = coverUrl.String
				ebooks = append(ebooks, e)
			}

			c.JSON(http.StatusOK, gin.H{
				"total_ebooks": len(ebooks),
				"ebooks":       ebooks,
			})
		})

		adminApi.POST("/ebooks", func(c *gin.Context) {
			var req CreateEbookRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data e-book tidak valid: " + err.Error()})
				return
			}

			var ebookID string
			query := `INSERT INTO ebooks (judul, jenjang, mata_pelajaran, kategori, cover_url, file_url) 
			          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
			
			err := database.DB.QueryRow(query, req.Judul, req.Jenjang, req.MataPelajaran, req.Kategori, req.CoverUrl, req.FileUrl).Scan(&ebookID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan e-book ke database: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":  "E-book berhasil ditambahkan",
				"ebook_id": ebookID,
			})
		})

		adminApi.DELETE("/ebooks/:id", func(c *gin.Context) {
			ebookID := c.Param("id")

			var judulBuku string
			_ = database.DB.QueryRow(`SELECT judul FROM ebooks WHERE id = $1`, ebookID).Scan(&judulBuku)

			query := `DELETE FROM ebooks WHERE id = $1`
			result, err := database.DB.Exec(query, ebookID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus e-book: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "E-book tidak ditemukan"})
				return
			}

			if adminID, ok := getAdminIDFromContext(c); ok {
				logSuperAdminActivity(adminID, "DELETE_EBOOK", c.ClientIP(), 
				c.Request.UserAgent(), "", 
				fmt.Sprintf("Menghapus e-book: %s", judulBuku))
			}

			c.JSON(http.StatusOK, gin.H{
				"message":  "E-book berhasil dihapus",
				"ebook_id": ebookID,
			})
		})

		// Superadmin daftarkan Sekolah Baru (B2B role)
		adminApi.POST("/schools", func(c *gin.Context) {
			var req CreateSchoolRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data sekolah tidak valid: " + err.Error()})
				return
			}

			// pengecekan npsn
			var existingID string
			err := database.DB.QueryRow(`SELECT id FROM schools WHERE npsn = $1`, req.Npsn).Scan(&existingID)
			if err == nil && existingID != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Sekolah dengan NPSN tersebut sudah terdaftar di sistem"})
				return
			}

			var schoolID string
			query := `INSERT INTO schools (school_name, npsn, address, jenjang, is_active) VALUES ($1, $2, $3, $4, TRUE) RETURNING id`
			err = database.DB.QueryRow(query, req.SchoolName, req.Npsn, req.Address, req.Jenjang).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data sekolah ke database: " + err.Error()})
				return
			}

			// log
			if adminID, ok := getAdminIDFromContext(c); ok {
				logSuperAdminActivity(adminID, "CREATE_SCHOOL", c.ClientIP(), 
				c.Request.UserAgent(), "", 
				fmt.Sprintf("Mendaftarkan sekolah baru: %s (NPSN: %s)", 
				req.SchoolName, req.Npsn))
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":     "Sekolah berhasil didaftarkan",
				"school_id":   schoolID,
				"school_name": req.SchoolName,
				"npsn":        req.Npsn,
			})
		})

		// Superadmin lihat Daftar Sekolah B2B
		adminApi.GET("/schools", func(c *gin.Context) {
			rows, err := database.DB.Query(`
				SELECT id, school_name, npsn, address, is_active, created_at 
				FROM schools 
				ORDER BY created_at DESC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar sekolah: " + err.Error()})
				return
			}
			defer rows.Close()

			type SchoolItem struct {
				ID         string    `json:"id"`
				SchoolName string    `json:"school_name"`
				Npsn       string    `json:"npsn"`
				Address    string    `json:"address"`
				IsActive   bool      `json:"is_active"`
				CreatedAt  time.Time `json:"created_at"`
			}

			var schools []SchoolItem
			for rows.Next() {
				var s SchoolItem
				var address sql.NullString
				if err := rows.Scan(&s.ID, &s.SchoolName, &s.Npsn, &address, &s.IsActive, &s.CreatedAt); err == nil {
					s.Address = address.String
					schools = append(schools, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total_schools": len(schools),
				"schools":       schools,
			})
		})

		// Superadmin daftarkan Akun Admin Sekolah (B2B)
		adminApi.POST("/school-admins", func(c *gin.Context) {
			var req CreateSchoolAdminRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid: " + err.Error()})
				return
			}

			// Pastikan school_id yang dipilih valid dan ada di database
			var schoolExists string
			err := database.DB.QueryRow(`SELECT id FROM schools WHERE id = $1`, req.SchoolID).Scan(&schoolExists)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Institusi sekolah tidak ditemukan"})
				return
			}

			// Daftarkan user ke Supabase Auth menggunakan Admin API
			supabaseURL := os.Getenv("SUPABASE_URL")
			serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

			if supabaseURL == "" || serviceRoleKey == "" {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Konfigurasi Supabase URL atau Service Role Key belum lengkap di server"})
				return
			}

			supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)
			
			payloadAuth := map[string]any{
				"email":          req.Email,
				"password":       req.Password,
				"email_confirm":  true, 
				"user_metadata": map[string]any{
					"full_name": req.FullName,
					"role":      "school_admin",
				},
			}
			jsonAuthBody, _ := json.Marshal(payloadAuth)

			httpReq, err := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan request ke Supabase Auth"})
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("apikey", serviceRoleKey)
			httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

			client := &http.Client{Timeout: 15 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke layanan Supabase Auth"})
				return
			}
			defer resp.Body.Close()

			var authResp map[string]any
			json.NewDecoder(resp.Body).Decode(&authResp)

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				errMsg := "Gagal membuat akun di Supabase Auth"
				if msg, ok := authResp["msg"].(string); ok {
					errMsg = msg
				} else if message, ok := authResp["message"].(string); ok {
					errMsg = message
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}

			// Ambil UUID user yang baru dibuat dari respon Supabase Auth
			userID, ok := authResp["id"].(string)
			if !ok || userID == "" {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase Auth"})
				return
			}

			//Insert ke tabel school_admins
			query := `
				INSERT INTO school_admins (id, school_id, full_name, email, nip, user_type, is_active, updated_at) 
				VALUES ($1, $2, $3, $4, $5, 'school_admin', TRUE, NOW())
			`

			_, err = database.DB.Exec(query, userID, req.SchoolID, req.FullName, req.Email, req.Nip)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data admin sekolah ke database: " + err.Error()})
				return
			}

			// Upsert ke profiles
			_, profErr := database.DB.Exec(`
				INSERT INTO profiles (
					id, email_sekolah, token_balance, is_active,
					role, teacher_type, school_id, registration_source, updated_at
				)
				VALUES ($1, $2, 0, TRUE, 'school_admin', 'b2b', $3, 'created_by_superadmin', NOW())
				ON CONFLICT (id) DO UPDATE SET
					email_sekolah = EXCLUDED.email_sekolah,
					role = 'school_admin',
					teacher_type = 'b2b',
					school_id = EXCLUDED.school_id,
					registration_source = 'created_by_superadmin',
					updated_at = NOW()
			`, userID, req.Email, req.SchoolID)

			if profErr != nil {
				log.Printf("[CREATE SCHOOL ADMIN] Gagal upsert profile untuk user=%s: %v", userID, profErr)
			} else {
				log.Printf("[CREATE SCHOOL ADMIN] Profile upserted untuk user=%s (school_id=%s)", userID, req.SchoolID)
			}

			// Log activity superadmin
			if adminID, ok := getAdminIDFromContext(c); ok {
				logSuperAdminActivity(adminID, "CREATE_SCHOOL_ADMIN", c.ClientIP(), 
				c.Request.UserAgent(), "", 
				fmt.Sprintf("Membuat akun admin sekolah untuk: %s (%s)", 
				req.FullName, req.Email))
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":   "Akun Admin Sekolah berhasil dibuat dan tersimpan.",
				"admin_id":  userID,
				"email":     req.Email,
				"school_id": req.SchoolID,
			})
		})

		// Superadmin lihat Daftar Admin Sekolah B2B
		adminApi.GET("/school-admins", func(c *gin.Context) {
			rows, err := database.DB.Query(`
				SELECT sa.id, sa.full_name, sa.email, sa.nip, sa.is_active, sa.created_at, s.school_name, s.npsn
				FROM school_admins sa
				JOIN schools s ON sa.school_id = s.id
				ORDER BY sa.created_at DESC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar admin sekolah: " + err.Error()})
				return
			}
			defer rows.Close()

			type SchoolAdminItem struct {
				ID         string    `json:"id"`
				FullName   string    `json:"full_name"`
				Email      string    `json:"email"`
				Nip        string    `json:"nip"`
				IsActive   bool      `json:"is_active"`
				CreatedAt  time.Time `json:"created_at"`
				SchoolName string    `json:"school_name"`
				Npsn       string    `json:"npsn"`
			}

			var admins []SchoolAdminItem
			for rows.Next() {
				var a SchoolAdminItem
				var nip sql.NullString
				if err := rows.Scan(&a.ID, &a.FullName, &a.Email, &nip, &a.IsActive, &a.CreatedAt, &a.SchoolName, &a.Npsn); err == nil {
					a.Nip = nip.String
					admins = append(admins, a)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total_admins": len(admins),
				"admins":       admins,
			})
		})

		// Endpoint Nonaktifkan/Aktifkan Sekolah (Soft Disable)
		adminApi.PATCH("/schools/:id/status", func(c *gin.Context) {
			// adminIDVal, _ := c.Get("admin_id")
			// if adminIDVal == nil {
			// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi superadmin tidak valid"})
			// 	return
			// }
			// adminID := int(adminIDVal.(float64))

			// schoolID := c.Param("id")
			adminID, ok := getAdminIDFromContext(c)
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "MISSING_SESSION",
						"message": "Sesi superadmin tidak valid",
					},
				})
				return
			}

			schoolID := c.Param("id")

			var req struct {
				IsActive bool   `json:"is_active"`
				Reason   string `json:"reason" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			if len(strings.TrimSpace(req.Reason)) < 5 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Alasan wajib diisi (minimal 5 karakter)"})
				return
			}

			// Cek sekolah ada
			var currentActive bool
			var schoolName string
			err := database.DB.QueryRow(`
				SELECT COALESCE(is_active, false), school_name 
				FROM schools WHERE id = $1
			`, schoolID).Scan(&currentActive, &schoolName)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sekolah tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal cek sekolah: " + err.Error()})
				return
			}

			// Cek kalau status sama (idempotent)
			if currentActive == req.IsActive {
				statusText := "aktif"
				if !req.IsActive { statusText = "nonaktif" }
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("Sekolah sudah berstatus %s", statusText),
				})
				return
			}

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Update status sekolah
			_, err = tx.Exec(`
				UPDATE schools SET is_active = $1, updated_at = NOW() WHERE id = $2
			`, req.IsActive, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status sekolah: " + err.Error()})
				return
			}

			// Hitung jumlah admin dan siswa terdampak
			var adminCount, studentCount int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM school_admins WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&adminCount)
			_ = tx.QueryRow(`SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`, schoolID).Scan(&studentCount)

			autoClosedSessions := 0

			// 2. Kalau dinonaktifkan, auto-close sesi absensi yang masih open/scheduled
						// 2. Kalau dinonaktifkan, auto-close sesi absensi + nonaktifkan school_members
			if !req.IsActive {
				// Cari semua sesi yang masih open/scheduled
				rows, err := tx.Query(`
					SELECT id FROM attendance_sessions
					WHERE school_id = $1 AND status IN ('open', 'scheduled')
				`, schoolID)
				if err == nil {
					var sessionIDs []string
					for rows.Next() {
						var sid string
						if err := rows.Scan(&sid); err == nil {
							sessionIDs = append(sessionIDs, sid)
						}
					}
					rows.Close()

					for _, sid := range sessionIDs {
						_, _ = tx.Exec(`SELECT auto_mark_absent_for_session($1)`, sid)
						_, _ = tx.Exec(`
							UPDATE attendance_sessions SET
								status = 'auto_closed',
								closed_at = NOW(),
								updated_at = NOW()
							WHERE id = $1
						`, sid)
						_, _ = tx.Exec(`SELECT recalculate_session_stats($1)`, sid)
					}
					autoClosedSessions = len(sessionIDs)
				}

				// Nonaktifkan semua school_members
				_, _ = tx.Exec(`
					UPDATE school_members 
					SET is_active = FALSE, updated_at = NOW()
					WHERE school_id = $1 AND is_active = TRUE
				`, schoolID)
			} else {
				// Aktifkan kembali semua school_members yang left_at NULL
				_, _ = tx.Exec(`
					UPDATE school_members 
					SET is_active = TRUE, updated_at = NOW()
					WHERE school_id = $1 AND left_at IS NULL
				`, schoolID)
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit: " + err.Error()})
				return
			}

			// 3. Catat log
			action := "SCHOOL_DEACTIVATED"
			if req.IsActive {
				action = "SCHOOL_ACTIVATED"
			}
			details := fmt.Sprintf(
				"[%s] %s | Reason: %s | Admins: %d | Students: %d | Auto-closed sessions: %d",
				action, schoolName, req.Reason, adminCount, studentCount, autoClosedSessions,
			)
			logSuperAdminActivity(adminID, action, c.ClientIP(), c.Request.UserAgent(), "", details)

			statusText := "dinonaktifkan"
			if req.IsActive {
				statusText = "diaktifkan"
			}

			c.JSON(http.StatusOK, gin.H{
				"message":              fmt.Sprintf("Sekolah berhasil %s", statusText),
				"school_id":            schoolID,
				"school_name":          schoolName,
				"is_active":            req.IsActive,
				"affected_admins":      adminCount,
				"affected_students":    studentCount,
				"auto_closed_sessions": autoClosedSessions,
			})
		})

		// ==========================================
		// SCHOOL ROLES (Peran di Sekolah) — Superadmin
		// ==========================================

		// GET /api/superadmin/school-roles
		adminApi.GET("/school-roles", func(c *gin.Context) {
			includeInactive := c.Query("include_inactive") == "true"

			query := `
				SELECT id, code, label, COALESCE(description, ''), is_active, sort_order, created_at, updated_at
				FROM school_roles
			`
			if !includeInactive {
				query += ` WHERE is_active = TRUE`
			}
			query += ` ORDER BY sort_order ASC, label ASC`

			rows, err := database.DB.Query(query)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar peran: " + err.Error()})
				return
			}
			defer rows.Close()

			type RoleItem struct {
				ID          string    `json:"id"`
				Code        string    `json:"code"`
				Label       string    `json:"label"`
				Description string    `json:"description"`
				IsActive    bool      `json:"is_active"`
				SortOrder   int       `json:"sort_order"`
				CreatedAt   time.Time `json:"created_at"`
				UpdatedAt   time.Time `json:"updated_at"`
			}

			list := []RoleItem{}
			for rows.Next() {
				var r RoleItem
				var desc sql.NullString
				if err := rows.Scan(&r.ID, &r.Code, &r.Label, &desc, &r.IsActive, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err == nil {
					r.Description = desc.String
					list = append(list, r)
				}
			}

			c.JSON(http.StatusOK, gin.H{"total": len(list), "roles": list})
		})

		// POST /api/superadmin/school-roles
		adminApi.POST("/school-roles", func(c *gin.Context) {
			var req struct {
				Code        string `json:"code" binding:"required,min=2,max=50"`
				Label       string `json:"label" binding:"required,min=2,max=100"`
				Description string `json:"description"`
				SortOrder   int    `json:"sort_order"`
				IsActive    *bool  `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Normalize code: lowercase, ganti spasi jadi underscore
			code := strings.ToLower(strings.TrimSpace(req.Code))
			code = strings.ReplaceAll(code, " ", "_")

			isActive := true
			if req.IsActive != nil {
				isActive = *req.IsActive
			}

			// Cek duplikat code
			var existingID string
			err := database.DB.QueryRow(`SELECT id FROM school_roles WHERE code = $1`, code).Scan(&existingID)
			if err == nil && existingID != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Kode peran sudah dipakai"})
				return
			}

			var roleID string
			err = database.DB.QueryRow(`
				INSERT INTO school_roles (code, label, description, is_active, sort_order)
				VALUES ($1, $2, NULLIF($3, ''), $4, $5)
				RETURNING id
			`, code, strings.TrimSpace(req.Label), req.Description, isActive, req.SortOrder).Scan(&roleID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan peran: " + err.Error()})
				return
			}

			if adminID, ok := getAdminIDFromContext(c); ok {
				logSuperAdminActivity(adminID, "CREATE_SCHOOL_ROLE", c.ClientIP(),
					c.Request.UserAgent(), "",
					fmt.Sprintf("Menambahkan peran: %s (%s)", req.Label, code))
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Peran berhasil ditambahkan", "id": roleID, "code": code})
		})

		// PUT /api/superadmin/school-roles/:id
		adminApi.PUT("/school-roles/:id", func(c *gin.Context) {
			roleID := c.Param("id")

			var req struct {
				Label       string `json:"label"`
				Description string `json:"description"`
				SortOrder   *int   `json:"sort_order"`
				IsActive    *bool  `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_roles SET
					label       = COALESCE(NULLIF($1, ''), label),
					description = NULLIF($2, ''),
					sort_order  = COALESCE($3, sort_order),
					is_active   = COALESCE($4, is_active),
					updated_at  = NOW()
				WHERE id = $5
			`, strings.TrimSpace(req.Label), req.Description, req.SortOrder, req.IsActive, roleID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui peran: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Peran tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Peran berhasil diperbarui"})
		})

		// DELETE /api/superadmin/school-roles/:id
		adminApi.DELETE("/school-roles/:id", func(c *gin.Context) {
			roleID := c.Param("id")

			// Ambil code dulu untuk cek pemakaian
			var code, label string
			err := database.DB.QueryRow(`SELECT code, label FROM school_roles WHERE id = $1`, roleID).Scan(&code, &label)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Peran tidak ditemukan"})
				return
			}

			// Cek apakah masih dipakai di school_members
			var usedCount int
			_ = database.DB.QueryRow(
				`SELECT COUNT(*) FROM school_members WHERE role_in_school = $1`,
				code,
			).Scan(&usedCount)

			if usedCount > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf(
						"Peran ini masih dipakai oleh %d anggota. Nonaktifkan saja, atau ubah dulu peran mereka.",
						usedCount,
					),
				})
				return
			}

			_, err = database.DB.Exec(`DELETE FROM school_roles WHERE id = $1`, roleID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus peran: " + err.Error()})
				return
			}

			if adminID, ok := getAdminIDFromContext(c); ok {
				logSuperAdminActivity(adminID, "DELETE_SCHOOL_ROLE", c.ClientIP(),
					c.Request.UserAgent(), "",
					fmt.Sprintf("Menghapus peran: %s (%s)", label, code))
			}

			c.JSON(http.StatusOK, gin.H{"message": "Peran berhasil dihapus"})
		})

		// adminApi.GET("/academic-years", func(c *gin.Context) {
		// 	adminID := c.MustGet("admin_id") // Atau ambil berdasarkan sesi school_admin yang login

		// 	// Ambil school_id berdasarkan admin yang sedang login
		// 	var schoolID string
		// 	err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1`, adminID).Scan(&schoolID)
		// 	if err != nil {
		// 		c.JSON(http.StatusNotFound, gin.H{"error": "Data institusi admin tidak ditemukan"})
		// 		return
		// 	}

		// 	rows, err := database.DB.Query(`
		// 		SELECT id, name, semester, start_date, end_date, is_active, created_at 
		// 		FROM school_academic_years 
		// 		WHERE school_id = $1 
		// 		ORDER BY created_at DESC
		// 	`, schoolID)
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tahun akademik"})
		// 		return
		// 	}
		// 	defer rows.Close()

		// 	type AcademicYear struct {
		// 		ID        string     `json:"id"`
		// 		Name      string     `json:"name"`
		// 		Semester  string     `json:"semester"`
		// 		StartDate *string    `json:"start_date"`
		// 		EndDate   *string    `json:"end_date"`
		// 		IsActive  bool       `json:"is_active"`
		// 		CreatedAt time.Time  `json:"created_at"`
		// 	}

		// 	var years []AcademicYear
		// 	for rows.Next() {
		// 		var y AcademicYear
		// 		var start, end sql.NullString
		// 		if err := rows.Scan(&y.ID, &y.Name, &y.Semester, &start, &end, &y.IsActive, &y.CreatedAt); err == nil {
		// 			if start.Valid { y.StartDate = &start.String }
		// 			if end.Valid { y.EndDate = &end.String }
		// 			years = append(years, y)
		// 		}
		// 	}

		// 	c.JSON(http.StatusOK, gin.H{"academic_years": years})
		// })

		// adminApi.POST("/academic-years", func(c *gin.Context) {
		// 	adminID := c.MustGet("admin_id")
		// 	var schoolID string
		// 	_ = database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1`, adminID).Scan(&schoolID)

		// 	var req AcademicYearRequest
		// 	if err := c.ShouldBindJSON(&req); err != nil {
		// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		// 		return
		// 	}

		// 	var yearID string
		// 	err := database.DB.QueryRow(`
		// 		INSERT INTO school_academic_years (school_id, name, semester, start_date, end_date, is_active)
		// 		VALUES ($1, $2, $3, NULLIF($4, '')::date, NULLIF($5, '')::date, FALSE)
		// 		RETURNING id
		// 	`, schoolID, req.Name, req.Semester, req.StartDate, req.EndDate).Scan(&yearID)

		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan tahun akademik: " + err.Error()})
		// 		return
		// 	}

		// 	c.JSON(http.StatusCreated, gin.H{"message": "Tahun akademik berhasil ditambahkan", "id": yearID})
		// })

		// adminApi.PATCH("/academic-years/:id/activate", func(c *gin.Context) {
		// 	adminID := c.MustGet("admin_id")
		// 	var schoolID string
		// 	_ = database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1`, adminID).Scan(&schoolID)
		// 	yearID := c.Param("id")

		// 	// Nonaktifkan semua tahun akademik di sekolah ini terlebih dahulu
		// 	_, _ = database.DB.Exec(`UPDATE school_academic_years SET is_active = FALSE WHERE school_id = $1`, schoolID)

		// 	// Aktifkan tahun akademik yang dipilih
		// 	_, err := database.DB.Exec(`UPDATE school_academic_years SET is_active = TRUE WHERE id = $1 AND school_id = $2`, yearID, schoolID)
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan tahun akademik"})
		// 		return
		// 	}

		// 	c.JSON(http.StatusOK, gin.H{"message": "Tahun akademik aktif berhasil diperbarui"})
		// })

		// GET /api/superadmin/permissions
		adminApi.GET("/permissions", func(c *gin.Context) {
			rows, err := database.DB.Query(`
				SELECT id, code, label, module, COALESCE(description, '')
				FROM permissions
				ORDER BY module ASC, label ASC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat permissions: " + err.Error()})
				return
			}
			defer rows.Close()

			type PermissionItem struct {
				ID          string `json:"id"`
				Code        string `json:"code"`
				Label       string `json:"label"`
				Module      string `json:"module"`
				Description string `json:"description"`
			}

			list := []PermissionItem{}
			for rows.Next() {
				var p PermissionItem
				if err := rows.Scan(&p.ID, &p.Code, &p.Label, &p.Module, &p.Description); err == nil {
					list = append(list, p)
				}
			}
			c.JSON(http.StatusOK, gin.H{"permissions": list})
		})

		// GET /api/superadmin/school-roles/:id/permissions
		adminApi.GET("/school-roles/:id/permissions", func(c *gin.Context) {
			roleID := c.Param("id")

			rows, err := database.DB.Query(`
				SELECT permission_id 
				FROM school_role_permissions 
				WHERE role_id = $1
			`, roleID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			var permissionIDs []string
			for rows.Next() {
				var pid string
				if err := rows.Scan(&pid); err == nil {
					permissionIDs = append(permissionIDs, pid)
				}
			}

			c.JSON(http.StatusOK, gin.H{"permission_ids": permissionIDs})
		})

		// PUT /api/superadmin/school-roles/:id/permissions
		adminApi.PUT("/school-roles/:id/permissions", func(c *gin.Context) {
			roleID := c.Param("id")

			// Cek apakah role ini is_system (tidak boleh diubah)
			var isSystem bool
			err := database.DB.QueryRow(`SELECT is_system FROM school_roles WHERE id = $1`, roleID).Scan(&isSystem)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Role tidak ditemukan"})
				return
			}
			if isSystem {
				c.JSON(http.StatusForbidden, gin.H{"error": "Role sistem tidak dapat diubah permission-nya"})
				return
			}

			var req struct {
				PermissionIDs []string `json:"permission_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// Hapus permission lama
			_, _ = tx.Exec(`DELETE FROM school_role_permissions WHERE role_id = $1`, roleID)

			// Insert permission baru
			for _, pid := range req.PermissionIDs {
				_, _ = tx.Exec(`
					INSERT INTO school_role_permissions (role_id, permission_id)
					VALUES ($1, $2)
					ON CONFLICT DO NOTHING
				`, roleID, pid)
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Permission role berhasil diperbarui"})
		})
	}

	// ==========================================
	// Endpoint yang diakses siswa (uji coba ujian)
	// ==========================================
	apiPub := r.Group("/api")
	{

		apiPub.GET("/public/personal-exam/access/:code", public.GetPersonalExamByCode)
		apiPub.POST("/public/personal-exam/start", public.StartPersonalExam)
		apiPub.POST("/public/personal-exam/submit", public.SubmitPersonalExam)
		apiPub.GET("/public/personal-exam/result/:sessionToken", public.GetPersonalExamResult)
		// Endpoint Siswa Memuat Soal Berdasarkan Token QR
		apiPub.GET("/exam/session-questions", func(c *gin.Context) {
			token := c.Query("token")
			if token == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Token sesi tidak disertakan"})
				return
			}

			var sessionID, examTitle string
			var questionBankID sql.NullString
			
			err := database.DB.QueryRow(`
				SELECT id, title, question_bank_id 
				FROM exam_sessions 
				WHERE (qr_code_token = $1 OR id::text = $1) AND deleted_at IS NULL
			`, token).Scan(&sessionID, &examTitle, &questionBankID)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak ditemukan atau QR Code tidak valid"})
				return
			}

			if !questionBankID.Valid {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Sesi ujian ini tidak memiliki bank soal yang terhubung"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT id, question_text, question_type, options, cognitive_level 
				FROM questions 
				WHERE question_bank_id = $1
			`, questionBankID.String)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat butir soal dari database: " + err.Error()})
				return
			}
			defer rows.Close()

			type QuestionItem struct {
				ID             string `json:"id"`
				QuestionText   string `json:"question_text"`
				QuestionType   string `json:"question_type"`
				Options        any    `json:"options"`
				CognitiveLevel string `json:"cognitive_level"`
			}

			var questions []QuestionItem
			for rows.Next() {
				var q QuestionItem
				var optionsJson []byte
				var cogLevel sql.NullString
				
				if err := rows.Scan(&q.ID, &q.QuestionText, &q.QuestionType, &optionsJson, &cogLevel); err != nil {
					continue
				}

				var parsedOptions any
				if len(optionsJson) > 0 {
					_ = json.Unmarshal(optionsJson, &parsedOptions)
				}
				q.Options = parsedOptions
				q.CognitiveLevel = cogLevel.String
				
				questions = append(questions, q)
			}

			c.JSON(http.StatusOK, gin.H{
				"session_id": sessionID,
				"title":      examTitle,
				"questions":  questions,
			})
		})

		// Endpoint Siswa Mengumpulkan Jawaban Ujian
		apiPub.POST("/exam/submit", func(c *gin.Context) {
			var req SubmitExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengumpulan ujian tidak lengkap: " + err.Error()})
				return
			}

			var existingID string
			err := database.DB.QueryRow(`
				SELECT id FROM exam_submissions WHERE exam_session_id = $1 AND nisn = $2
			`, req.SessionID, req.NISN).Scan(&existingID)

			if err == nil && existingID != "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak! Anda (NISN " + req.NISN + ") sudah pernah mengumpulkan ujian ini."})
				return
			}

			var questionBankID string
			err = database.DB.QueryRow(`
				SELECT question_bank_id FROM exam_sessions WHERE id = $1
			`, req.SessionID).Scan(&questionBankID)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak valid"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT id, correct_answer FROM questions WHERE question_bank_id = $1
			`, questionBankID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi kunci jawaban"})
				return
			}
			defer rows.Close()

			correctAnswersMap := make(map[string]string)
			totalQuestions := 0
			for rows.Next() {
				var qID string
				var correctAnsJSON []byte
				if err := rows.Scan(&qID, &correctAnsJSON); err == nil {
					var rawAns string
					if json.Unmarshal(correctAnsJSON, &rawAns) == nil {
						correctAnswersMap[qID] = rawAns
					} else {
						correctAnswersMap[qID] = string(correctAnsJSON)
					}
					totalQuestions++
				}
			}

			correctCount := 0
			for qID, studentAns := range req.Answers {
				if correctAns, exists := correctAnswersMap[qID]; exists {
					cleanedCorrect := strings.Trim(correctAns, `"`)
					if strings.EqualFold(strings.TrimSpace(studentAns), strings.TrimSpace(cleanedCorrect)) {
						correctCount++
					}
				}
			}

			var score float64 = 0
			if totalQuestions > 0 {
				score = (float64(correctCount) / float64(totalQuestions)) * 100
			}

			answersBytes, _ := json.Marshal(req.Answers)
			queryInsert := `
				INSERT INTO exam_submissions (exam_session_id, student_name, student_number, nisn, answers, score)
				VALUES ($1, $2, $3, $4, $5::jsonb, $6)
			`

			_, err = database.DB.Exec(queryInsert, req.SessionID, req.StudentName, req.StudentNumber, req.NISN, string(answersBytes), score)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan jawaban: Siswa dengan NISN ini sudah tercatat mengumpulkan ujian."})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Ujian berhasil dikumpulkan",
				"score":   score,
			})
		})

		// Endpoint Cek Status Siswa
		apiPub.GET("/exam/check-student", func(c *gin.Context) {
			sessionID := c.Query("session_id")
			nisn := c.Query("nisn")

			if sessionID == "" || nisn == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter tidak lengkap"})
				return
			}

			var existingID string
			err := database.DB.QueryRow(`
				SELECT id FROM exam_submissions WHERE exam_session_id = $1 AND nisn = $2
			`, sessionID, nisn).Scan(&existingID)

			if err == nil && existingID != "" {
				c.JSON(http.StatusOK, gin.H{"has_submitted": true, "message": "NISN ini sudah pernah mengumpulkan ujian."})
				return
			}

			c.JSON(http.StatusOK, gin.H{"has_submitted": false})
		})

		apiPub.POST("/exam/start", func(c *gin.Context) {
			var req StartExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak lengkap: " + err.Error()})
				return
			}

			// 1. Cari schedule dari access code
			var scheduleID, examID, schoolID, status string
			var scheduleDate time.Time
			var startTime, endTime time.Time
			var classSubGroupID, classGroupID sql.NullString
			var requireLogin bool
			var examTitle, subject string
			var passingScore float64
			var durationMinutes int
			var durationMode string

			err := database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id, es.school_id, es.status,
					es.schedule_date, es.start_time, es.end_time,
					es.class_group_id, es.class_sub_group_id,
					es.require_login, es.duration_minutes,
					es.duration_mode, 
					e.title, e.subject, COALESCE(e.passing_score, 0)
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				WHERE es.access_code = $1 
				AND es.deleted_at IS NULL
				AND es.status IN ('scheduled', 'ongoing')
			`, strings.ToUpper(req.AccessCode)).Scan(
				&scheduleID, &examID, &schoolID, &status,
				&scheduleDate, &startTime, &endTime,
				&classGroupID, &classSubGroupID,
				&requireLogin, &durationMinutes,
				&durationMode,
				&examTitle, &subject, &passingScore,
			)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Kode ujian tidak valid atau ujian sudah berakhir"})
				return
			}

			// 2. Cek waktu (boleh mulai kalau sekarang >= start_time - 10 menit dan <= end_time)
			now := nowJakarta()
			scheduleStart := time.Date(
				scheduleDate.Year(), scheduleDate.Month(), scheduleDate.Day(),
				startTime.Hour(), startTime.Minute(), 0, 0, now.Location(),
			)
			scheduleEnd := time.Date(
				scheduleDate.Year(), scheduleDate.Month(), scheduleDate.Day(),
				endTime.Hour(), endTime.Minute(), 0, 0, now.Location(),
			)

			var expiresAt time.Time

			if durationMode == "flexible" {
				// ==========================================
				// FLEXIBLE: window waktu buka
				// ==========================================
				if now.Before(scheduleStart) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": fmt.Sprintf("Ujian belum dibuka. Dibuka pukul %s", startTime.Format("15:04")),
					})
					return
				}
				if now.After(scheduleEnd) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Window ujian sudah ditutup"})
					return
				}
				

				// expires = min(now + duration, scheduleEnd)
				durationEnd := now.Add(time.Duration(durationMinutes) * time.Minute)
				if durationEnd.Before(scheduleEnd) {
					expiresAt = durationEnd
				} else {
					expiresAt = scheduleEnd
				}

			} else {
				// ==========================================
				// STRICT: seperti sebelumnya
				// ==========================================
				if now.Before(scheduleStart.Add(-10 * time.Minute)) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": fmt.Sprintf("Ujian belum dimulai. Mulai pukul %s", startTime.Format("15:04")),
					})
					return
				}
				if now.After(scheduleEnd) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Waktu ujian sudah berakhir"})
					return
				}

				// expires = end_time (durasi strict = sampai jam selesai)
				expiresAt = scheduleEnd
			}

			
			

			// 3. Cari student di class_sub_group ini pakai NISN + nama
			var studentID, actualName, actualNISN string
			var studentClassSubGroupID sql.NullString

			err = database.DB.QueryRow(`
				SELECT id, full_name, COALESCE(nisn, ''), class_sub_group_id
				FROM students 
				WHERE school_id = $1 
				AND nisn = $2
				AND LOWER(full_name) = LOWER($3)
				AND is_active = TRUE
			`, schoolID, req.NISN, req.StudentName).Scan(
				&studentID, &actualName, &actualNISN, &studentClassSubGroupID,
			)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Data siswa tidak cocok. Periksa NISN dan nama Anda.",
				})
				return
			}

			// Validasi siswa di sub kelas yang tepat
			if classSubGroupID.Valid && studentClassSubGroupID.Valid {
				if classSubGroupID.String != studentClassSubGroupID.String {
					c.JSON(http.StatusForbidden, gin.H{"error": "Anda bukan peserta ujian ini"})
					return
				}
			}

			// 4. Cek apakah sudah submit
			var existingSubID string
			err = database.DB.QueryRow(`
				SELECT id FROM exam_submissions 
				WHERE schedule_id = $1 AND student_id = $2
			`, scheduleID, studentID).Scan(&existingSubID)
			if err == nil && existingSubID != "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Anda sudah mengumpulkan ujian ini"})
				return
			}

			// 5. Cek existing live session (kalau ada, reuse token-nya — siswa resume)
			var existingToken string
			var existingExpires time.Time
			err = database.DB.QueryRow(`
				SELECT session_token, expires_at
				FROM exam_live_sessions
				WHERE schedule_id = $1 AND student_id = $2 AND submitted_at IS NULL
			`, scheduleID, studentID).Scan(&existingToken, &existingExpires)

			var sessionToken string
			

			if err == nil && existingToken != "" {
				// Resume
				sessionToken = existingToken
				expiresAt = existingExpires
			} else {
				// Buat session baru
				sessionToken = generateSessionToken()
				// expires = min(end_time, now + duration)
				durationEnd := now.Add(time.Duration(durationMinutes) * time.Minute)
				if durationEnd.Before(scheduleEnd) {
					expiresAt = durationEnd
				} else {
					expiresAt = scheduleEnd
				}

				// Ambil questions dari exam
				var questionsJSON []byte
				var scoringConfig []byte
				err = database.DB.QueryRow(`
					SELECT questions, scoring_config FROM school_exams WHERE id = $1
				`, examID).Scan(&questionsJSON, &scoringConfig)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat soal"})
					return
				}

				// ==========================================
				// SANITIZE: hapus correct_answer & answer_key dari soal!
				// ==========================================
				var questions []map[string]any
				if err := json.Unmarshal(questionsJSON, &questions); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Format soal error"})
					return
				}
				for _, q := range questions {
					delete(q, "correct_answer")
					delete(q, "answer_key")
					delete(q, "explanation")
					delete(q, "rubric")
				}
				sanitizedJSON, _ := json.Marshal(questions)

				examConfig := map[string]any{
					"duration_minutes": durationMinutes,
					"passing_score":    passingScore,
					"total_score":      len(questions),
				}
				configJSON, _ := json.Marshal(examConfig)

				_, err = database.DB.Exec(`
					INSERT INTO exam_live_sessions (
						schedule_id, student_id, school_id,
						session_token,
						student_name_snapshot, student_nisn_snapshot,
						questions_snapshot, exam_config_snapshot,
						started_at, expires_at,
						ip_address, user_agent
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb,
						NOW(), $9, $10, $11
					)
				`, scheduleID, studentID, schoolID,
					sessionToken,
					actualName, actualNISN,
					sanitizedJSON, configJSON,
					expiresAt, c.ClientIP(), c.Request.UserAgent(),
				)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi ujian: " + err.Error()})
					return
				}

				// Update schedule status ke ongoing
				if status == "scheduled" {
					_, _ = database.DB.Exec(`
						UPDATE exam_schedules SET status = 'ongoing', updated_at = NOW()
						WHERE id = $1
					`, scheduleID)
				}
			}

			// 6. Ambil questions_snapshot dari session
			var questionsSnapshot []byte
			var examConfigSnapshot []byte
			err = database.DB.QueryRow(`
				SELECT questions_snapshot, exam_config_snapshot
				FROM exam_live_sessions WHERE session_token = $1
			`, sessionToken).Scan(&questionsSnapshot, &examConfigSnapshot)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat soal"})
				return
			}

			var questions any
			var examConfig struct {
				DurationMinutes int     `json:"duration_minutes"`
				PassingScore    float64 `json:"passing_score"`
				TotalScore      int     `json:"total_score"`
			}
			_ = json.Unmarshal(questionsSnapshot, &questions)
			_ = json.Unmarshal(examConfigSnapshot, &examConfig)

			c.JSON(http.StatusOK, gin.H{
				"session_token":    sessionToken,
				"schedule_id":      scheduleID,
				"exam_id":          examID,
				"student_id":       studentID,
				"student_name":     actualName,
				"student_nisn":     actualNISN,
				"exam_title":       examTitle,
				"exam_subject":     subject,
				"duration_minutes": examConfig.DurationMinutes,
				"passing_score":    examConfig.PassingScore,
				"total_score":      examConfig.TotalScore,
				"started_at":       now.Format(time.RFC3339),
				"expires_at":       expiresAt.Format(time.RFC3339),
				"questions":        questions,
			})
		})

		apiPub.POST("/exam/:sessionToken/submit", func(c *gin.Context) {
			sessionToken := c.Param("sessionToken")

			var req SubmitLiveExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// ==========================================
			// 1. Ambil session + exam_id + started_at
			// ==========================================
			var sessionID, scheduleID, examID, schoolID, studentID string
			var studentName, studentNISN string
			var questionsSnapshot []byte
			var startedAt time.Time
			var submittedAt sql.NullTime

			err := database.DB.QueryRow(`
				SELECT 
					els.id, els.schedule_id,
					es.exam_id,
					els.school_id, els.student_id,
					els.student_name_snapshot, els.student_nisn_snapshot,
					els.questions_snapshot,
					els.started_at,
					els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.session_token = $1
			`, sessionToken).Scan(
				&sessionID, &scheduleID,
				&examID,
				&schoolID, &studentID,
				&studentName, &studentNISN,
				&questionsSnapshot,
				&startedAt,
				&submittedAt,
			)
			if err != nil {
				fmt.Printf("[SUBMIT] Gagal ambil session: %v\n", err)
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi ujian tidak valid"})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusForbidden, gin.H{"error": "Ujian sudah dikumpulkan sebelumnya"})
				return
			}

			// ==========================================
			// 2. Ambil exam info (KKM, total score)
			// ==========================================
			var passingScore float64
			var examQuestionsJSON []byte
			err = database.DB.QueryRow(`
				SELECT COALESCE(passing_score, 0), questions 
				FROM school_exams 
				WHERE id = $1
			`, examID).Scan(&passingScore, &examQuestionsJSON)
			if err != nil {
				fmt.Printf("[SUBMIT] Gagal ambil exam info: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil data ujian"})
				return
			}

			// ==========================================
			// 3. Auto-grade PG
			// ==========================================
			var examQuestions []struct {
				ID            string  `json:"id"`
				Type          string  `json:"type"`
				CorrectAnswer string  `json:"correct_answer"`
				AnswerKey     string  `json:"answer_key"`
				Score         float64 `json:"score"`
			}
			if err := json.Unmarshal(examQuestionsJSON, &examQuestions); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Format soal error"})
				return
			}

			var totalScore float64
			var maxScore float64
			var hasEssay bool

			answersDetail := []map[string]any{}

			for _, q := range examQuestions {
				maxScore += q.Score
				studentAns := strings.TrimSpace(req.Answers[q.ID])

				detail := map[string]any{
					"question_id": q.ID,
					"type":        q.Type,
					"answer":      studentAns,
					"max_score":   q.Score,
				}

				if q.Type == "multiple_choice" {
					isCorrect := strings.EqualFold(studentAns, strings.TrimSpace(q.CorrectAnswer))
					detail["is_correct"] = isCorrect
					if isCorrect {
						detail["score"] = q.Score
						totalScore += q.Score
					} else {
						detail["score"] = 0.0
					}
				} else {
					detail["score"] = nil
					detail["is_correct"] = nil
					hasEssay = true
				}

				answersDetail = append(answersDetail, detail)
			}

			percentage := 0.0
			if maxScore > 0 {
				percentage = (totalScore / maxScore) * 100
			}

			status := "graded"
			if hasEssay {
				status = "graded_with_pending"
			}

			// ==========================================
			// 4. Insert submission
			// ==========================================
			answersJSON, _ := json.Marshal(answersDetail)

			var submissionID string
			err = database.DB.QueryRow(`
				INSERT INTO exam_submissions (
					schedule_id, exam_id, school_id, student_id,
					student_name, student_nisn,
					started_at, submitted_at,
					answers, total_score, max_score, percentage, is_passed,
					status, tab_switch_count, time_spent_seconds,
					ip_address, user_agent
				) VALUES (
					$1, $2, $3, $4, $5, $6,
					$7, NOW(),
					$8::jsonb, $9, $10, $11, $12,
					$13, $14, $15, $16, $17
				) RETURNING id
			`, scheduleID, examID, schoolID, studentID,
				studentName, studentNISN,
				startedAt,   // ← pakai variabel, bukan subquery
				answersJSON, totalScore, maxScore, percentage, percentage >= passingScore,
				status, req.TabSwitchCount, req.TimeSpentSeconds,
				c.ClientIP(), c.Request.UserAgent(),
			).Scan(&submissionID)
			if err != nil {
				fmt.Printf("[SUBMIT] Gagal insert submission: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan jawaban: " + err.Error()})
				return
			}

			// ==========================================
			// 5. Mark session submitted
			// ==========================================
			_, _ = database.DB.Exec(`
				UPDATE exam_live_sessions 
				SET submitted_at = NOW() 
				WHERE id = $1
			`, sessionID)

			// ==========================================
			// 6. Update schedule stats
			// ==========================================
			_, _ = database.DB.Exec(`
				UPDATE exam_schedules SET
					total_submitted = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1),
					total_graded = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1 AND status = 'graded'),
					average_score = (SELECT AVG(percentage) FROM exam_submissions WHERE schedule_id = $1),
					highest_score = (SELECT MAX(percentage) FROM exam_submissions WHERE schedule_id = $1),
					lowest_score = (SELECT MIN(percentage) FROM exam_submissions WHERE schedule_id = $1),
					updated_at = NOW()
				WHERE id = $1
			`, scheduleID)

			c.JSON(http.StatusOK, gin.H{
				"submission_id": submissionID,
				"total_score":   totalScore,
				"max_score":     maxScore,
				"percentage":    percentage,
				"is_passed":     percentage >= passingScore,
				"message":       "Jawaban berhasil dikumpulkan. Soal essay akan dikoreksi manual oleh guru.",
			})
		})
	}

	// Route Protected 
	// Hanya guru yang login yang bisa mengakses data miliknya sendiri
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{

		// api.GET("/auth/check-role", func(c *gin.Context) {
		// 	userID, exists := c.Get("user_id")
		// 	if !exists {
		// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		// 		return
		// 	}

		// 	var userIDStr string
		// 	switch v := userID.(type) {
		// 	case string:
		// 		userIDStr = v
		// 	case fmt.Stringer:
		// 		userIDStr = v.String()
		// 	default:
		// 		userIDStr = fmt.Sprintf("%v", v)
		// 	}

		// 	fmt.Printf("[check-role] userID=%s\n", userIDStr)

		// 	var userEmail string
		// 	err := database.DB.QueryRow(
		// 		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		// 		userIDStr,
		// 	).Scan(&userEmail)

		// 	if err != nil {
		// 		fmt.Printf("[check-role] Gagal ambil email dari auth.users: %v\n", err)
		// 		if emailFromCtx, ok := c.Get("email"); ok {
		// 			userEmail, _ = emailFromCtx.(string)
		// 		}
		// 	}
		// 	fmt.Printf("[check-role] email=%s\n", userEmail)

	
		// 	var userType string
		// 	var schoolActive bool
		// 	var schoolID sql.NullString

		// 	err = database.DB.QueryRow(`
		// 		SELECT 
		// 			sa.user_type,
		// 			COALESCE(s.is_active, false) AS school_active,
		// 			sa.school_id
		// 		FROM school_admins sa
		// 		LEFT JOIN schools s ON sa.school_id = s.id
		// 		WHERE (sa.id = $1 OR (sa.email <> '' AND sa.email = $2)) 
		// 		AND sa.is_active = TRUE
		// 	`, userIDStr, userEmail).Scan(&userType, &schoolActive, &schoolID)

		// 	// *artifact 14 oct 2026
		// 	// if err == nil {
		// 	// 	fmt.Printf("[check-role] User adalah school_admin (%s), school_active=%v\n", userType, schoolActive)

		// 	// 	// Pastikan row profiles juga ada
		// 	// 	_, _ = database.DB.Exec(`
		// 	// 		INSERT INTO profiles (id, email_sekolah, token_balance, is_active, updated_at)
		// 	// 		VALUES ($1, $2, 0, TRUE, NOW())
		// 	// 		ON CONFLICT (id) DO NOTHING
		// 	// 	`, userIDStr, userEmail)

		// 	// 	c.JSON(http.StatusOK, gin.H{
		// 	// 		"role":            userType,
		// 	// 		"school_inactive": !schoolActive, // ← Flag baru
		// 	// 		"school_id":       schoolID.String,
		// 	// 	})
		// 	// 	return
		// 	// }

		// 	if err == nil {
		// 		fmt.Printf("[check-role] User adalah school_admin (%s), school_active=%v\n", userType, schoolActive)

		// 		// Pastikan row profiles ada dengan metadata admin yang BENAR
		// 		// (bukan default 'independent_teacher'/'b2c')
		// 		_, profErr := database.DB.Exec(`
		// 			INSERT INTO profiles (
		// 				id, email_sekolah, token_balance, is_active,
		// 				role, teacher_type, school_id, registration_source, updated_at
		// 			)
		// 			VALUES ($1, $2, 0, TRUE, 'school_admin', 'b2b', $3, 'created_by_superadmin', NOW())
		// 			ON CONFLICT (id) DO UPDATE SET
		// 				role = 'school_admin',
		// 				teacher_type = 'b2b',
		// 				school_id = COALESCE(EXCLUDED.school_id, profiles.school_id),
		// 				registration_source = 'created_by_superadmin',
		// 				updated_at = NOW()
		// 		`, userIDStr, userEmail, schoolID.String)

		// 		if profErr != nil {
		// 			fmt.Printf("[check-role] Gagal upsert profile admin: %v\n", profErr)
		// 		} else {
		// 			fmt.Printf("[check-role] Profile admin upserted (school_id=%s)\n", schoolID.String)
		// 		}

		// 		c.JSON(http.StatusOK, gin.H{
		// 			"role":            userType,
		// 			"school_inactive": !schoolActive,
		// 			"school_id":       schoolID.String,
		// 		})
		// 		return
		// 	}

		// 	// Upsert sebagai teacher

		// 	// fmt.Println("[check-role] Bukan school_admin, upsert sebagai teacher")

		// 	// _, err = database.DB.Exec(`
		// 	// 	INSERT INTO profiles (id, email_sekolah, token_balance, is_active, updated_at)
		// 	// 	VALUES ($1, $2, 0, TRUE, NOW())
		// 	// 	ON CONFLICT (id) DO NOTHING
		// 	// `, userIDStr, userEmail)  

		// 	// if err != nil {
		// 	// 	fmt.Printf("[check-role] Gagal upsert profile: %v\n", err)
		// 	// } else {
		// 	// 	fmt.Println("[check-role] Upsert profile OK")
		// 	// }

		// 	// c.JSON(http.StatusOK, gin.H{"role": "teacher"})

		// 	fmt.Println("[check-role] Bukan school_admin, cek/upsert sebagai teacher")

		// 	var existingProfile bool
		// 	err = database.DB.QueryRow(`
		// 		SELECT EXISTS(SELECT 1 FROM profiles WHERE id = $1)
		// 	`, userIDStr).Scan(&existingProfile)

		// 	if !existingProfile {
		// 		_, err = database.DB.Exec(`
		// 			INSERT INTO profiles (id, email_sekolah, token_balance, is_active, teacher_type, registration_source, updated_at)
		// 			VALUES ($1, $2, 2, TRUE, 'b2c', 'self_register', NOW())
		// 			ON CONFLICT (id) DO NOTHING
		// 		`, userIDStr, userEmail)

		// 		if err != nil {
		// 			fmt.Printf("[check-role] Gagal upsert profile: %v\n", err)
		// 		} else {
		// 			fmt.Println("[check-role] Upsert profile OK (new user)")
		// 		}
		// 	} else {
		// 		fmt.Println("[check-role] Profile sudah ada, skip insert")
		// 	}

		// 	var teacherType string
		// 	_ = database.DB.QueryRow(`
		// 		SELECT COALESCE(teacher_type, 'b2c') FROM profiles WHERE id = $1
		// 	`, userIDStr).Scan(&teacherType)

		// 	// Ambil daftar membership sekolah (kalau ada)
		// 	type MembershipInfo struct {
		// 		SchoolID     string `json:"school_id"`
		// 		SchoolName   string `json:"school_name"`
		// 		RoleInSchool string `json:"role_in_school"`
		// 		IsActive     bool   `json:"is_active"`
		// 	}

		// 	var memberships []MembershipInfo
		// 	rows, err := database.DB.Query(`
		// 		SELECT sm.school_id, s.school_name, sm.role_in_school, sm.is_active
		// 		FROM school_members sm
		// 		JOIN schools s ON sm.school_id = s.id
		// 		WHERE sm.user_id = $1 AND sm.left_at IS NULL
		// 	`, userIDStr)

		// 	if err == nil {
		// 		defer rows.Close()
		// 		for rows.Next() {
		// 			var m MembershipInfo
		// 			if err := rows.Scan(&m.SchoolID, &m.SchoolName, &m.RoleInSchool, &m.IsActive); err == nil {
		// 				memberships = append(memberships, m)
		// 			}
		// 		}
		// 	}

		// 	c.JSON(http.StatusOK, gin.H{
		// 		"role":         "teacher",
		// 		"teacher_type": teacherType,
		// 		"memberships":  memberships,
		// 		"school_inactive": false,
		// 	})

			
		// })

		api.GET("/auth/check-role", CheckRoleHandler)
		// SCHOOL ADMIN MANAGE TEACHER ROLES
		api.POST("/school-admin/teachers/:id/roles", AssignRoleToTeacher)
		api.DELETE("/school-admin/teachers/:id/roles/:roleId", RemoveRoleFromTeacher)
		api.GET("/school-admin/teachers/:id/roles", GetTeacherRoles)

		api.POST("/auth/notify-password-changedNotUssed", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Ambil email guru dari tabel profiles
			var userEmail string
			err := database.DB.QueryRow(`SELECT email_sekolah FROM profiles WHERE id = $1`, userID).Scan(&userEmail)
			if err != nil || userEmail == "" {
				c.JSON(http.StatusOK, gin.H{"message": "Kata sandi diperbarui, namun email profil tidak ditemukan"})
				return
			}


			loc, _ := time.LoadLocation("Asia/Jakarta")
			waktuUbah := time.Now().In(loc).Format("02 January 2006 pukul 15:04 WIB")
			ipClient := c.ClientIP()

			subjek := "Keamanan Akun: Kata Sandi Anda Telah Diubah"
			
			// template HTML email notifikasi ganti kata sandi *ini bisa di handle di frontend
			htmlBody := fmt.Sprintf(`
				<div style="font-family: Arial, sans-serif; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 10px;">
					<h2 style="color: #10b981;">SkoolaGo Security Alert</h2>
					<p>Halo Guru,</p>
					<p>Kami ingin menginformasikan bahwa kata sandi untuk akun SkoolaGo Anda baru saja berhasil diubah.</p>
					<div style="background-color: #f8fafc; padding: 15px; border-radius: 8px; margin: 20px 0;">
						<p style="margin: 5px 0;"><strong>Waktu:</strong> %s</p>
						<p style="margin: 5px 0;"><strong>Alamat IP:</strong> %s</p>
					</div>
					<p>Jika Anda yang melakukan perubahan ini, Anda dapat mengabaikan email ini dengan aman.</p>
					<p style="color: #ef4444; font-weight: bold;">Jika Anda TIDAK merasa melakukan perubahan ini, segera hubungi administrator atau amankan akun Anda!</p>
					<hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;" />
					<p style="font-size: 12px; color: #64748b;">Email otomatis ini dikirimkan oleh sistem keamanan SkoolaGo.</p>
				</div>
			`, waktuUbah, ipClient)

			go sendResendEmail(userEmail, subjek, htmlBody)

			c.JSON(http.StatusOK, gin.H{"message": "Notifikasi perubahan kata sandi berhasil diproses"})
		})

		// ambil dari tabel auth supabasenya
		api.POST("/auth/notify-password-changed", func(c *gin.Context) {
            userID, exists := c.Get("user_id")
            if !exists {
                c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
                return
            }

            // Ambil email langsung dari tabel auth.users Supabase berdasarkan ID user
            var userEmail string
            err := database.DB.QueryRow(`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`, userID).Scan(&userEmail)
            
            // Fallback: Jika gagal di auth.users, coba cek ke tabel profiles
            if err != nil || userEmail == "" {
                _ = database.DB.QueryRow(`SELECT COALESCE(email_sekolah, '') FROM profiles WHERE id = $1`, userID).Scan(&userEmail)
            }

            if userEmail == "" {
                c.JSON(http.StatusOK, gin.H{"message": "Kata sandi diperbarui, namun email pengguna tidak ditemukan"})
                return
            }

            loc, _ := time.LoadLocation("Asia/Jakarta")
            waktuUbah := time.Now().In(loc).Format("02 January 2006 pukul 15:04 WIB")
            ipClient := c.ClientIP()

            subjek := "Keamanan Akun: Kata Sandi Anda Telah Diubah"
            
            htmlBody := fmt.Sprintf(`
                <div style="font-family: Arial, sans-serif; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 10px;">
                    <h2 style="color: #10b981;">SkoolaGo Security Alert</h2>
                    <p>Halo Pengguna,</p>
                    <p>Kami ingin menginformasikan bahwa kata sandi untuk akun Anda baru saja berhasil diubah.</p>
                    <div style="background-color: #f8fafc; padding: 15px; border-radius: 8px; margin: 20px 0;">
                        <p style="margin: 5px 0;"><strong>Waktu:</strong> %s</p>
                        <p style="margin: 5px 0;"><strong>Alamat IP:</strong> %s</p>
                    </div>
                    <p style="color: #ef4444; font-weight: bold;">Jika Anda merasa TIDAK melakukan perubahan ini, segera hubungi tim terkait melalui support@skoolago.com atau segera amankan akun Anda!</p>
                    <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;" />
                    <p style="font-size: 12px; color: #64748b; text-align: center;">
                        &copy; 2026 SkoolaGo. All rights reserved.<br />
                        Email otomatis ini dikirimkan oleh sistem keamanan SkoolaGo.
                    </p>
                </div>
            `, waktuUbah, ipClient)

            go sendResendEmail(userEmail, subjek, htmlBody)

            c.JSON(http.StatusOK, gin.H{"message": "Notifikasi perubahan kata sandi berhasil diproses"})
        })
		
		//TODO matikan dulu MIDTRANS
		// api.POST("/payment/create-snap", func(c *gin.Context) {
		// 	userID, _ := c.Get("user_id")

		// 	var req CreateTransactionRequest
		// 	if err := c.ShouldBindJSON(&req); err != nil {
		// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Data paket tidak valid: " + err.Error()})
		// 		return
		// 	}

		// 	orderID := fmt.Sprintf("TP-%d-%s", time.Now().Unix(), strings.ToUpper(randString(4)))

		// 	_, err := database.DB.Exec(
		// 		`INSERT INTO token_orders (user_id, order_id, package_name, token_amount, amount, status) VALUES ($1, $2, $3, $4, $5, 'pending')`,
		// 		userID, orderID, req.PackageName, req.TokenAmount, req.Amount,
		// 	)
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat pesanan ke database"})
		// 		return
		// 	}

		// 	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
		// 	isProduction := os.Getenv("MIDTRANS_IS_PRODUCTION") == "true" 
			
		// 	midtransURL := "https://app.sandbox.midtrans.com/snap/v1/transactions"
		// 	if isProduction {
		// 		midtransURL = "https://app.midtrans.com/snap/v1/transactions"
		// 	}

		// 	payload := map[string]any{
		// 		"transaction_details": map[string]any{
		// 			"order_id":     orderID,
		// 			"gross_amount": req.Amount,
		// 		},
		// 		"item_details": []map[string]any{
		// 			{
		// 				"id":       req.PackageName,
		// 				"price":    req.Amount,
		// 				"quantity": 1,
		// 				"name":     fmt.Sprintf("Paket Token %s (%d Token)", req.PackageName, req.TokenAmount),
		// 			},
		// 		},
		// 	}

		// 	jsonBody, _ := json.Marshal(payload)
		// 	httpReq, _ := http.NewRequest("POST", midtransURL, bytes.NewBuffer(jsonBody))
		// 	httpReq.Header.Set("Content-Type", "application/json")
		// 	httpReq.Header.Set("Accept", "application/json")
		// 	httpReq.SetBasicAuth(serverKey, "")

		// 	client := &http.Client{Timeout: 15 * time.Second}
		// 	resp, err := client.Do(httpReq)
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke gateway pembayaran Midtrans"})
		// 		return
		// 	}
		// 	defer resp.Body.Close()

		// 	var midtransResp map[string]any
		// 	json.NewDecoder(resp.Body).Decode(&midtransResp)

		// 	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// 		errMsg := "Gagal membuat transaksi dari Midtrans"
		// 		if errs, ok := midtransResp["error_messages"].([]any); ok && len(errs) > 0 {
		// 			errMsg = fmt.Sprintf("%v", errs[0])
		// 		}
		// 		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		// 		return
		// 	}

		// 	snapToken, _ := midtransResp["token"].(string)
		// 	redirectURL, _ := midtransResp["redirect_url"].(string)

		// 	if snapToken == "" {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token Snap tidak ditemukan dari Midtrans"})
		// 		return
		// 	}

		// 	_, _ = database.DB.Exec(
		// 		`UPDATE token_orders SET snap_token = $1, snap_redirect_url = $2 WHERE order_id = $3`,
		// 		snapToken, redirectURL, orderID,
		// 	)

		// 	c.JSON(http.StatusOK, gin.H{
		// 		"order_id":     orderID,
		// 		"snap_token":   snapToken,
		// 		"redirect_url": redirectURL,
		// 	})
		// })

		api.GET("/profile", func(c *gin.Context) {
			fmt.Println("========================================")
			fmt.Println(">>> [1] Handler /profile MULAI")

			userID, exists := c.Get("user_id")
			if !exists {
				fmt.Println(">>> [1b] ERROR: user_id tidak ada di context!")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}
			fmt.Printf(">>> [2] userID=%v (type=%T)\n", userID, userID)

			// --- UPDATE last_login ---
			fmt.Println(">>> [3] Akan UPDATE last_login...")
			res, err := database.DB.Exec(`UPDATE profiles SET last_login = NOW() WHERE id = $1`, userID)
			if err != nil {
				fmt.Printf(">>> [3b] UPDATE last_login ERROR: %v\n", err)
			} else {
				rowsAffected, _ := res.RowsAffected()
				fmt.Printf(">>> [3c] UPDATE last_login OK, rows affected=%d\n", rowsAffected)
			}

			var p UpdateProfileRequest
			var tokenBalance int

			queryFixed := `SELECT 
				COALESCE(nama_guru, ''), COALESCE(nip_guru, ''), COALESCE(nama_sekolah, ''), 
				COALESCE(mata_pelajaran, ''), COALESCE(fase, ''), COALESCE(kelas, ''), 
				COALESCE(jenis_kelamin, ''),
				COALESCE(semester, ''), COALESCE(tahun_pelajaran, ''), COALESCE(nama_kepala_sekolah, ''), 
				COALESCE(nip_kepala_sekolah, ''), COALESCE(kota_kabupaten, ''), COALESCE(tanggal_penandatanganan, ''), 
				COALESCE(alamat_sekolah, ''), COALESCE(kecamatan_kabupaten, ''), COALESCE(kode_pos, ''), 
				COALESCE(telepon_sekolah, ''), COALESCE(email_sekolah, ''), COALESCE(npsn, ''), 
				COALESCE(website_sekolah, ''), token_balance 
				FROM profiles WHERE id = $1`

			dollarCount := strings.Count(queryFixed, "$")
			fmt.Printf(">>> [4] Query SELECT siap. Jumlah placeholder $N = %d\n", dollarCount)
			fmt.Printf(">>> [4b] Query full:\n%s\n", queryFixed)

			args := []any{userID}
			fmt.Printf(">>> [5] Jumlah argumen yang dikirim = %d\n", len(args))
			for i, a := range args {
				fmt.Printf(">>> [5.%d] arg[%d] = %v (type=%T)\n", i, i, a, a)
			}

			if dollarCount != len(args) {
				fmt.Printf(">>> [5b] MISMATCH! Query butuh %d param, tapi dikirim %d param\n", dollarCount, len(args))
			}

			fmt.Println(">>> [6] Akan eksekusi QueryRow...")
			err = database.DB.QueryRow(queryFixed, args...).Scan(
				&p.NamaGuru, &p.NipGuru, &p.NamaSekolah, &p.MataPelajaran, &p.Fase, &p.Kelas,
				&p.JenisKelamin,
				&p.Semester, &p.TahunPelajaran, &p.NamaKepalaSekolah, &p.NipKepalaSekolah,
				&p.KotaKabupaten, &p.TanggalPenandatanganan, &p.AlamatSekolah, &p.KecamatanKabupaten,
				&p.KodePos, &p.TeleponSekolah, &p.EmailSekolah, &p.Npsn, &p.WebsiteSekolah, &tokenBalance,
			)

			if err != nil {
				fmt.Printf(">>> [6b] QueryRow ERROR: %v\n", err)
				fmt.Printf(">>> [6c] Error type: %T\n", err)

				if err == sql.ErrNoRows {
					fmt.Println(">>> [6d] Tidak ada row di profiles untuk userID ini")
					c.JSON(http.StatusNotFound, gin.H{"error": "Profil tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			fmt.Printf(">>> [7] QueryRow SUKSES! token_balance=%d, nama_guru=%s\n", tokenBalance, p.NamaGuru)
			fmt.Println(">>> [8] Handler /profile SELESAI")
			fmt.Println("========================================")

			c.JSON(http.StatusOK, gin.H{
				"user_id":       userID,
				"profile":       p,
				"token_balance": tokenBalance,
			})
		})

		api.PUT("/profile", func(c *gin.Context) {
			userID, _ := c.Get("user_id")

			var req UpdateProfileRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			query := `UPDATE profiles SET 
				nama_guru = $1, nip_guru = $2, jenis_kelamin = $3, nama_sekolah = $4, 
				mata_pelajaran = $5, fase = $6, kelas = $7, semester = $8, tahun_pelajaran = $9, 
				nama_kepala_sekolah = $10, nip_kepala_sekolah = $11, kota_kabupaten = $12, 
				tanggal_penandatanganan = $13, alamat_sekolah = $14, kecamatan_kabupaten = $15, 
				kode_pos = $16, telepon_sekolah = $17, email_sekolah = $18, npsn = $19, 
				website_sekolah = $20, updated_at = NOW() 
				WHERE id = $21`

			_, err := database.DB.Exec(query,
				req.NamaGuru, req.NipGuru, req.JenisKelamin, req.NamaSekolah,    // ← urutan harus match $1..$4
				req.MataPelajaran, req.Fase, req.Kelas, req.Semester, req.TahunPelajaran,
				req.NamaKepalaSekolah, req.NipKepalaSekolah, req.KotaKabupaten,
				req.TanggalPenandatanganan, req.AlamatSekolah, req.KecamatanKabupaten,
				req.KodePos, req.TeleponSekolah, req.EmailSekolah, req.Npsn,
				req.WebsiteSekolah, userID,   
			)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui profil: " + err.Error()})
				return
			}

            c.JSON(http.StatusOK, gin.H{"message": "Identitas perangkat berhasil diperbarui"})
		})
	
		api.POST("/question-banks", func(c *gin.Context) {
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
			
			err = tx.QueryRow(bankQuery, userID, req.Title, req.Description, req.Subject, req.Phase, req.PriceInTokens, req.IsPublic).Scan(&bankID)
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
				
				_, err = tx.Exec(qQuery, bankID, q.QuestionText, q.QuestionType, optionsStr, q.CorrectAnswer, q.Explanation, q.CognitiveLevel)
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
		})
		
		api.GET("/my-question-banks", func(c *gin.Context) {
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

			type BankItem struct {
				ID        string    `json:"id"`
				Title     string    `json:"title"`
				Subject   string    `json:"subject"`
				Phase     string    `json:"phase"`
				CreatedAt time.Time `json:"created_at"`
			}

			var banks []BankItem
			for rows.Next() {
				var b BankItem
				if err := rows.Scan(&b.ID, &b.Title, &b.Subject, &b.Phase, &b.CreatedAt); err == nil {
					banks = append(banks, b)
				}
			}

			c.JSON(http.StatusOK, gin.H{"question_banks": banks})
		})

		//DEPRECATED /ai/generate-questions
		// api.POST("/ai/generate-questions", func(c *gin.Context) {
		// 	var req GenerateAIRequest
		// 	if err := c.ShouldBindJSON(&req); err != nil {
		// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter AI tidak valid: " + err.Error()})
		// 		return
		// 	}

		// 	ctx := context.Background()
		// 	apiKey := os.Getenv("GEMINI_API_KEY")
		// 	if apiKey == "" {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key Gemini belum dikonfigurasi di server"})
		// 		return
		// 	}

		// 	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal inisialisasi client AI"})
		// 		return
		// 	}
		// 	defer client.Close()

		// 	model := client.GenerativeModel("gemini-3.7-flash")
		// 	model.GenerationConfig = genai.GenerationConfig{
		// 		ResponseMIMEType: "application/json",
		// 	}

		// 	prompt := fmt.Sprintf(
		// 		"Buatkan %d butir soal tipe %s untuk topik '%s' dengan level kognitif %s. "+
		// 			"Format output harus berupa JSON Array murni berisi objek dengan properti: "+
		// 			"question_text (string), question_type (string), options (object atau null jika essay/isian singkat), "+
		// 			"correct_answer (string atau array), explanation (string), cognitive_level (string).",
		// 		req.NumberOfQ, req.QuestionType, req.Topic, req.CognitiveLevel,
		// 	)

		// 	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		// 	if err != nil {
		// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal men-generate soal dari AI: " + err.Error()})
		// 		return
		// 	}

		// 	var jsonResult string
		// 	for _, part := range resp.Candidates[0].Content.Parts {
		// 		jsonResult += fmt.Sprintf("%v", part)
		// 	}

		// 	c.Data(http.StatusOK, "application/json", []byte(jsonResult))
		// })

		api.GET("/ebooks-list", func(c *gin.Context) {
			rows, err := database.DB.Query(`
				SELECT id, judul, jenjang, mata_pelajaran, kategori, cover_url, file_url, created_at 
				FROM ebooks 
				ORDER BY created_at DESC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar e-book"})
				return
			}
			defer rows.Close()

			type EbookPublicItem struct {
				ID            string    `json:"id"`
				Judul         string    `json:"judul"`
				Jenjang       string    `json:"jenjang"`
				MataPelajaran string    `json:"mata_pelajaran"`
				Kategori      string    `json:"kategori"`
				CoverUrl      string    `json:"cover_url"`
				FileUrl       string    `json:"file_url"`
			}

			var list []EbookPublicItem
			for rows.Next() {
				var item EbookPublicItem
				var cover sql.NullString
				var createdAt time.Time
				
				if err := rows.Scan(&item.ID, &item.Judul, &item.Jenjang, &item.MataPelajaran, &item.Kategori, &cover, &item.FileUrl, &createdAt); err == nil {
					item.CoverUrl = cover.String
					list = append(list, item)
				}
			}

			c.JSON(http.StatusOK, gin.H{"ebooks": list})
		})

		api.POST("/ebooks-list/history", func(c *gin.Context) {
			userID, _ := c.Get("user_id")

			var req struct {
				EbookID    string `json:"ebook_id" binding:"required"`
				ActionType string `json:"action_type" binding:"required"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Data riwayat tidak valid"})
				return
			}

			_, err := database.DB.Exec(
				`INSERT INTO ebook_history (user_id, ebook_id, action_type) VALUES ($1, $2, $3)`,
				userID, req.EbookID, req.ActionType,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat riwayat e-book"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Riwayat berhasil dicatat/disimpan"})
		})

		// Cloudflare R2 Object Storage — generate presigned URL untuk direct upload dari frontend
		api.GET("/upload/presigned-url", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Normalisasi userID ke string
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			case fmt.Stringer:
				userIDStr = v.String()
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// Tentukan extension + content-type berdasarkan file_type
			fileType := c.Query("file_type")
			if fileType == "" {
				fileType = "image"
			}

			var ext, contentType string
			switch fileType {
			case "image":
				ext = ".webp"
				contentType = "image/webp"
			case "pdf":
				ext = ".pdf"
				contentType = "application/pdf"
			case "table":
				// tabel dirender jadi gambar juga
				ext = ".webp"
				contentType = "image/webp"
			default:
				ext = ".bin"
				contentType = "application/octet-stream"
			}

			// Object key dengan prefix per-user supaya tidak bentrok & gampang audit
			objectKey := fmt.Sprintf("exams/%s/%s%s", userIDStr, uuid.New().String(), ext)

			url, err := GeneratePresignedPutURL(c.Request.Context(), objectKey, contentType)
			if err != nil {
				fmt.Printf("[PRESIGN] error untuk user=%s objectKey=%s: %v\n", userIDStr, objectKey, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate presigned URL: " + err.Error()})
				return
			}

			// PUBLIC_URL opsional — kalau di-set, frontend bisa langsung simpan URL final
			publicURL := ""
			if base := os.Getenv("R2_PUBLIC_URL"); base != "" {
				publicURL = strings.TrimRight(base, "/") + "/" + objectKey
			}

			c.JSON(http.StatusOK, gin.H{
				"upload_url":   url,
				"object_key":   objectKey,
				"content_type": contentType,
				"public_url":   publicURL,
				"expires_in":   600, // detik — sesuai WithPresignExpires(10*time.Minute)
			})
		})

		api.GET("/school-admin/academic-years", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			fmt.Printf("DEBUG userID yang mencoba akses academic-years: %v\n", userID)

			var schoolID string
			err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
			if err != nil {
				fmt.Printf("DEBUG Error ambil school_id: %v\n", err) // <-- Lihat error ini di terminal Go Anda
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Data admin sekolah tidak valid atau tidak aktif"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT id, name, semester, start_date, end_date, is_active, created_at 
				FROM school_academic_years 
				WHERE school_id = $1 
				ORDER BY created_at DESC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tahun akademik"})
				return
			}
			defer rows.Close()

			type AcademicYear struct {
				ID        string     `json:"id"`
				Name      string     `json:"name"`
				Semester  string     `json:"semester"`
				StartDate *string    `json:"start_date"`
				EndDate   *string    `json:"end_date"`
				IsActive  bool       `json:"is_active"`
				CreatedAt time.Time  `json:"created_at"`
			}

			var years []AcademicYear
			for rows.Next() {
				var y AcademicYear
				var start, end sql.NullTime   
				if err := rows.Scan(&y.ID, &y.Name, &y.Semester, &start, &end, &y.IsActive, &y.CreatedAt); err == nil {
					if start.Valid {
						s := start.Time.Format("2006-01-02")  
						y.StartDate = &s
					}
					if end.Valid {
						s := end.Time.Format("2006-01-02")
						y.EndDate = &s
					}
					years = append(years, y)
				}
			}

			c.JSON(http.StatusOK, gin.H{"academic_years": years})
		})

		api.POST("/school-admin/academic-years", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}

			var req AcademicYearRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			var yearID string
			err = database.DB.QueryRow(`
				INSERT INTO school_academic_years (school_id, name, semester, start_date, end_date, is_active)
				VALUES ($1, $2, $3, NULLIF($4, '')::date, NULLIF($5, '')::date, FALSE)
				RETURNING id
			`, schoolID, req.Name, req.Semester, req.StartDate, req.EndDate).Scan(&yearID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan tahun akademik: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Tahun akademik berhasil ditambahkan", "id": yearID})
		})

		api.PATCH("/school-admin/academic-years/:id/activate", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}

			yearID := c.Param("id")

			// Nonaktifkan semua tahun akademik di sekolah ini terlebih dahulu
			_, _ = database.DB.Exec(`UPDATE school_academic_years SET is_active = FALSE WHERE school_id = $1`, schoolID)

			// Aktifkan tahun akademik yang dipilih
			_, err = database.DB.Exec(`UPDATE school_academic_years SET is_active = TRUE WHERE id = $1 AND school_id = $2`, yearID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan tahun akademik"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Tahun akademik aktif berhasil diperbarui"})
		})
	
		api.GET("/school-admin/profile", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Normalisasi userID
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// Ambil email sebagai fallback
			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			// Ambil school_id dengan dua cara
			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)

			if err != nil {
				fmt.Printf("[GET PROFILE] Gagal ambil school_id: %v (userID=%s, email=%s)\n", err, userIDStr, userEmail)
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}

			var id, schoolName, npsn, address, jenjang, city, email, accreditation, schoolStatus, phone, website, foundationName sql.NullString
			var latitude, longitude sql.NullFloat64
			var isActive bool
			var createdAt time.Time

			err = database.DB.QueryRow(`
				SELECT 
					id, 
					school_name, 
					npsn, 
					COALESCE(address, ''), 
					COALESCE(city, ''),     
					COALESCE(email, ''),
					COALESCE(accreditation, ''),
					COALESCE(school_status, 'Negeri'),
					COALESCE(phone, ''),
					COALESCE(website, ''),
					latitude,
					longitude,
					COALESCE(foundation_name, ''),
					COALESCE(jenjang, 'SMP'), 
					is_active, 
					created_at 
				FROM schools WHERE id = $1
			`, schoolID).Scan(
				&id, &schoolName, &npsn, &address, &city, &email, 
				&accreditation, &schoolStatus, &phone, &website, &latitude, &longitude, &foundationName,
				&jenjang, &isActive, &createdAt,
			)

			if err != nil {
				fmt.Printf("[GET PROFILE] Gagal ambil data sekolah: %v (schoolID=%s)\n", err, schoolID)
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Data sekolah tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"school": gin.H{
					"id":              id.String,
					"school_name":     schoolName.String,
					"npsn":            npsn.String,
					"address":         address.String,
					"city":            city.String,     
					"email":           email.String, 
					"accreditation":   accreditation.String,
					"school_status":   schoolStatus.String,
					"phone":           phone.String,
					"website":         website.String,
					"latitude":        latitude.Float64,
					"longitude":       longitude.Float64,
					"foundation_name": foundationName.String,
					"jenjang":         jenjang.String,
					"is_active":       isActive,
					"created_at":      createdAt,
				},
			})
		})

		api.PUT("/school-admin/profile", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}

			var req UpdateSchoolProfileRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			_, err = database.DB.Exec(`
				UPDATE schools 
				SET 
					school_name = $1, 
					npsn = $2, 
					address = $3, 
					city = $4, 
					email = $5, 
					accreditation = $6, 
					school_status = $7, 
					phone = $8, 
					website = $9, 
					latitude = $10, 
					longitude = $11, 
					foundation_name = $12,
					updated_at = NOW() 
				WHERE id = $13
			`, req.SchoolName, req.Npsn, req.Address, req.City, req.Email, 
			req.Accreditation, req.SchoolStatus, req.Phone, req.Website, 
			req.Latitude, req.Longitude, req.FoundationName, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data sekolah: " + err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Profil sekolah berhasil diperbarui"})
		})

		// Admin Sekolah Mengambil Daftar Kelas
		api.GET("/school-admin/classes", func(c *gin.Context) {
			// GUNAKAN FUNGSI INI (Sudah ada cek is_active sekolah)
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				// Error akan berisi "sekolah Anda sedang dinonaktifkan..." jika nonaktif
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			// Ambil nama sekolah (karena getSchoolIDFromUser hanya return ID)
			var schoolName string
			_ = database.DB.QueryRow(`SELECT school_name FROM schools WHERE id = $1`, schoolID).Scan(&schoolName)

			rows, err := database.DB.Query(`
				SELECT cg.id, cg.name, cg.level, COALESCE(cg.class_type, 'Umum'), 
					cg.academic_year_id, 
					COALESCE(say.name, '-'), COALESCE(say.semester, '-'), cg.created_at
				FROM class_groups cg
				LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
				WHERE cg.school_id = $1
				ORDER BY cg.level ASC, cg.name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data kelas"})
				return
			}
			defer rows.Close()

			type ClassItem struct {
				ID               string    `json:"id"`
				SchoolName       string    `json:"school_name"`
				Name             string    `json:"name"`
				Level            string    `json:"level"`
				ClassType        string    `json:"class_type"`
				AcademicYearID   string    `json:"academic_year_id"`
				AcademicYearName string    `json:"academic_year_name"`
				Semester         string    `json:"semester"`
				CreatedAt        time.Time `json:"created_at"`
			}

			// Inisialisasi slice agar JSON mengembalikan [] bukan null jika kosong
			classes := []ClassItem{}
			
			for rows.Next() {
				var cl ClassItem
				cl.SchoolName = schoolName
				if err := rows.Scan(&cl.ID, &cl.Name, &cl.Level, &cl.ClassType, &cl.AcademicYearID, &cl.AcademicYearName, &cl.Semester, &cl.CreatedAt); err == nil {
					classes = append(classes, cl)
				}
			}

			c.JSON(http.StatusOK, gin.H{"classes": classes})
		})

		// Admin Sekolah Menambah Kelas Baru
		api.POST("/school-admin/classes", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

		
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			case fmt.Stringer:
				userIDStr = v.String()
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// fmt.Println("════════════════════════════════════════")
			// fmt.Println("[CREATE CLASS] ▶ MULAI")
			// fmt.Printf("[CREATE CLASS] userIDStr = %q\n", userIDStr)

			// Ambil email dari auth.users
			var userEmail string
			err := database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)
			if err != nil {
				fmt.Printf("[CREATE CLASS] ⚠️ Gagal ambil email: %v\n", err)
			}
			fmt.Printf("[CREATE CLASS] userEmail = %q\n", userEmail)

			// Ambil school_id dengan DUA cara
			var schoolID string
			err = database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)

			if err != nil {
				fmt.Printf("[CREATE CLASS] ❌ Gagal ambil school_id: %v\n", err)
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}
			// fmt.Printf("[CREATE CLASS] ✅ schoolID = %q\n", schoolID)

			var req CreateClassGroupRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}
			// fmt.Printf("[CREATE CLASS] req.AcademicYearID = %q\n", req.AcademicYearID)
			// fmt.Printf("[CREATE CLASS] req.Name = %q, req.Level = %q, req.ClassType = %q\n", 
			// 	req.Name, req.Level, req.ClassType)

			// ==========================================
			// VALIDASI: Cek academic_year_id milik sekolah ini
			// ==========================================
			var validYearSchoolID string
			err = database.DB.QueryRow(`
				SELECT school_id FROM school_academic_years 
				WHERE id = $1
			`, req.AcademicYearID).Scan(&validYearSchoolID)

			if err != nil {
				if err == sql.ErrNoRows {
					fmt.Printf("[CREATE CLASS] ❌ academic_year_id=%s TIDAK ADA di tabel\n", req.AcademicYearID)
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "Tahun akademik tidak ditemukan di sistem.",
					})
					return
				}
				fmt.Printf("[CREATE CLASS] ❌ Query error: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal validasi tahun akademik: " + err.Error()})
				return
			}

			// fmt.Printf("[CREATE CLASS] academic_year milik school_id = %q\n", validYearSchoolID)
			// fmt.Printf("[CREATE CLASS] admin school_id           = %q\n", schoolID)

			if validYearSchoolID != schoolID {
				fmt.Println("[CREATE CLASS] ❌❌❌ MISMATCH SCHOOL_ID! ❌❌❌")
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf(
						"Tahun akademik ini milik sekolah lain. (school_id Anda: %s, school_id tahun: %s)",
						schoolID, validYearSchoolID,
					),
				})
				return
			}

			// fmt.Println("[CREATE CLASS] ✅ Validasi OK, akan INSERT...")

			// ==========================================
			// INSERT
			// ==========================================
			var classID string
			err = database.DB.QueryRow(`
				INSERT INTO class_groups (school_id, academic_year_id, name, level, class_type)
				VALUES ($1, $2, $3, $4, NULLIF($5, ''))
				RETURNING id
			`, schoolID, req.AcademicYearID, req.Name, req.Level, req.ClassType).Scan(&classID)

			if err != nil {
				// fmt.Printf("[CREATE CLASS] ❌ INSERT ERROR: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data kelas: " + err.Error()})
				return
			}

			// fmt.Printf("[CREATE CLASS] ✅ SUKSES! classID = %s\n", classID)
			// fmt.Println("════════════════════════════════════════")

			c.JSON(http.StatusCreated, gin.H{
				"message": "Kelas berhasil ditambahkan",
				"id":      classID,
			})
		})

		// Admin Sekolah Menghapus Kelas
		api.DELETE("/school-admin/classes/:id", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`SELECT school_id FROM school_admins WHERE id = $1 AND is_active = TRUE`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Anda bukan admin sekolah yang aktif"})
				return
			}

			classID := c.Param("id")
			result, err := database.DB.Exec(`DELETE FROM class_groups WHERE id = $1 AND school_id = $2`, classID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus kelas"})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Kelas tidak ditemukan atau bukan milik institusi Anda"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Kelas berhasil dihapus"})
		})

		// GET: List sub kelas berdasarkan class_group_id
		api.GET("/school-admin/classes/:classId/sub-classes", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE id = $1 AND is_active = TRUE
			`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			classID := c.Param("classId")

			// Validasi class_group milik sekolah ini
			var classExists string
			err = database.DB.QueryRow(`
				SELECT id FROM class_groups WHERE id = $1 AND school_id = $2
			`, classID, schoolID).Scan(&classExists)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Kelas tidak ditemukan"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					csg.id, csg.name, COALESCE(csg.code, ''), 
					COALESCE(csg.capacity, 0), COALESCE(csg.notes, ''),
					csg.homeroom_teacher_id, 
					COALESCE(p.nama_guru, '') AS homeroom_teacher_name,
					csg.is_active, csg.created_at
				FROM class_sub_groups csg
				LEFT JOIN profiles p ON csg.homeroom_teacher_id = p.id
				WHERE csg.class_group_id = $1 AND csg.school_id = $2
				ORDER BY csg.created_at ASC
			`, classID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil sub kelas: " + err.Error()})
				return
			}
			defer rows.Close()

			type SubClassItem struct {
				ID                  string    `json:"id"`
				Name                string    `json:"name"`
				Code                string    `json:"code"`
				Capacity            int       `json:"capacity"`
				Notes               string    `json:"notes"`
				HomeroomTeacherID   *string   `json:"homeroom_teacher_id"`
				HomeroomTeacherName string    `json:"homeroom_teacher_name"`
				IsActive            bool      `json:"is_active"`
				CreatedAt           time.Time `json:"created_at"`
			}

			var subs []SubClassItem
			for rows.Next() {
				var s SubClassItem
				var teacherID sql.NullString
				if err := rows.Scan(
					&s.ID, &s.Name, &s.Code, &s.Capacity, &s.Notes,
					&teacherID, &s.HomeroomTeacherName, &s.IsActive, &s.CreatedAt,
				); err == nil {
					if teacherID.Valid {
						s.HomeroomTeacherID = &teacherID.String
					}
					subs = append(subs, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"class_group_id": classID,
				"sub_classes":    subs,
			})
		})

		// sub kelas baru
		api.POST("/school-admin/sub-classes", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Normalisasi userID
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// Ambil school_id (fallback via email)
			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			var req CreateSubClassRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			// Validasi class_group milik sekolah ini
			var classExists string
			err = database.DB.QueryRow(`
				SELECT id FROM class_groups WHERE id = $1 AND school_id = $2
			`, req.ClassGroupID, schoolID).Scan(&classExists)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Kelas induk tidak ditemukan atau bukan milik sekolah Anda"})
				return
			}

			// Cek duplikat nama
			var existingID string
			err = database.DB.QueryRow(`
				SELECT id FROM class_sub_groups 
				WHERE class_group_id = $1 AND name = $2
			`, req.ClassGroupID, req.Name).Scan(&existingID)
			if err == nil && existingID != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Sub kelas dengan nama tersebut sudah ada"})
				return
			}

			// Insert
			var subID string
			var homeroomTeacher interface{} = nil
			if req.HomeroomTeacherID != "" {
				homeroomTeacher = req.HomeroomTeacherID
			}

			err = database.DB.QueryRow(`
				INSERT INTO class_sub_groups 
					(class_group_id, school_id, name, code, capacity, homeroom_teacher_id, notes)
				VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''))
				RETURNING id
			`, req.ClassGroupID, schoolID, req.Name, req.Code, req.Capacity, homeroomTeacher, req.Notes).Scan(&subID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan sub kelas: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message": "Sub kelas berhasil ditambahkan",
				"id":      subID,
			})
		})

		// Update sub kelas
		api.PUT("/school-admin/sub-classes/:id", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE id = $1 AND is_active = TRUE
			`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			subID := c.Param("id")

			var req UpdateSubClassRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			var homeroomTeacher interface{} = nil
			if req.HomeroomTeacherID != "" {
				homeroomTeacher = req.HomeroomTeacherID
			}

			var isActive interface{} = nil
			if req.IsActive != nil {
				isActive = *req.IsActive
			}

			result, err := database.DB.Exec(`
				UPDATE class_sub_groups SET
					name = COALESCE(NULLIF($1, ''), name),
					code = NULLIF($2, ''),
					capacity = $3,
					homeroom_teacher_id = $4,
					notes = NULLIF($5, ''),
					is_active = COALESCE($6, is_active),
					updated_at = NOW()
				WHERE id = $7 AND school_id = $8
			`, req.Name, req.Code, req.Capacity, homeroomTeacher, req.Notes, isActive, subID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui sub kelas: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sub kelas tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Sub kelas berhasil diperbarui"})
		})

		// Hapus sub kelas
		api.DELETE("/school-admin/sub-classes/:id", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE id = $1 AND is_active = TRUE
			`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			subID := c.Param("id")

			result, err := database.DB.Exec(`
				DELETE FROM class_sub_groups WHERE id = $1 AND school_id = $2
			`, subID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus sub kelas: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sub kelas tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Sub kelas berhasil dihapus"})
		})

		// List semua sub kelas milik sekolah
		api.GET("/school-admin/sub-classes", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Normalisasi userID
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// Ambil email sebagai fallback
			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			// Ambil school_id
			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			// Query semua sub kelas dengan info master kelasnya
			rows, err := database.DB.Query(`
				SELECT 
					csg.id,
					csg.name,
					COALESCE(csg.code, ''),
					COALESCE(csg.capacity, 0),
					COALESCE(csg.notes, ''),
					csg.homeroom_teacher_id,
					COALESCE(p.nama_guru, '') AS homeroom_teacher_name,
					csg.is_active,
					csg.created_at,
					cg.id AS class_group_id,
					cg.name AS class_group_name,
					cg.level AS class_group_level,
					COALESCE(cg.class_type, 'Umum') AS class_group_type,
					cg.academic_year_id,
					COALESCE(say.name, '-') AS academic_year_name,
					COALESCE(say.semester, '-') AS semester
				FROM class_sub_groups csg
				JOIN class_groups cg ON csg.class_group_id = cg.id
				LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
				LEFT JOIN profiles p ON csg.homeroom_teacher_id = p.id
				WHERE csg.school_id = $1
				ORDER BY cg.level ASC, cg.name ASC, csg.name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar sub kelas: " + err.Error()})
				return
			}
			defer rows.Close()

			type SubClassWithClass struct {
				ID                  string    `json:"id"`
				Name                string    `json:"name"`
				Code                string    `json:"code"`
				Capacity            int       `json:"capacity"`
				Notes               string    `json:"notes"`
				HomeroomTeacherID   *string   `json:"homeroom_teacher_id"`
				HomeroomTeacherName string    `json:"homeroom_teacher_name"`
				IsActive            bool      `json:"is_active"`
				CreatedAt           time.Time `json:"created_at"`
				ClassGroupID        string    `json:"class_group_id"`
				ClassGroupName      string    `json:"class_group_name"`
				ClassGroupLevel     string    `json:"class_group_level"`
				ClassGroupType      string    `json:"class_group_type"`
				AcademicYearID      string    `json:"academic_year_id"`
				AcademicYearName    string    `json:"academic_year_name"`
				Semester            string    `json:"semester"`
			}

			var subClasses []SubClassWithClass
			for rows.Next() {
				var s SubClassWithClass
				var teacherID sql.NullString
				if err := rows.Scan(
					&s.ID, &s.Name, &s.Code, &s.Capacity, &s.Notes,
					&teacherID, &s.HomeroomTeacherName, &s.IsActive, &s.CreatedAt,
					&s.ClassGroupID, &s.ClassGroupName, &s.ClassGroupLevel, &s.ClassGroupType,
					&s.AcademicYearID, &s.AcademicYearName, &s.Semester,
				); err == nil {
					if teacherID.Valid {
						s.HomeroomTeacherID = &teacherID.String
					}
					subClasses = append(subClasses, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":       len(subClasses),
				"sub_classes": subClasses,
			})
		})

		// List semua murid (dengan filter opsional by sub class)
		api.GET("/school-admin/students", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			// Normalisasi userID
			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			// Filter opsional: ?sub_class_id=xxx
			subClassID := c.Query("sub_class_id")

			query := `
				SELECT 
					s.id, s.full_name, COALESCE(s.student_number, ''), 
					COALESCE(s.nisn, ''), COALESCE(s.jenis_kelamin, ''),
					s.is_active, s.created_at,
					s.class_sub_group_id,
					COALESCE(csg.name, '') AS sub_class_name,
					csg.class_group_id,
					COALESCE(cg.name, '') AS class_group_name,
					COALESCE(cg.class_type, 'Umum') AS class_group_type
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN class_groups cg ON csg.class_group_id = cg.id
				WHERE s.school_id = $1
			`
			args := []any{schoolID}

			if subClassID != "" {
				query += ` AND s.class_sub_group_id = $2`
				args = append(args, subClassID)
			}

			query += ` ORDER BY cg.level ASC, csg.name ASC, NULLIF(s.student_number, '')::int ASC`

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data murid: " + err.Error()})
				return
			}
			defer rows.Close()

			type StudentItem struct {
				ID               string    `json:"id"`
				FullName         string    `json:"full_name"`
				StudentNumber    string    `json:"student_number"`
				NISN             string    `json:"nisn"`
				JenisKelamin     string    `json:"jenis_kelamin"`
				IsActive         bool      `json:"is_active"`
				CreatedAt        time.Time `json:"created_at"`
				ClassSubGroupID  *string   `json:"class_sub_group_id"`
				SubClassName     string    `json:"sub_class_name"`
				ClassGroupID     *string   `json:"class_group_id"`
				ClassGroupName   string    `json:"class_group_name"`
				ClassGroupType   string    `json:"class_group_type"`
			}

			var students []StudentItem
			for rows.Next() {
				var s StudentItem
				var subGroupID, classGroupID sql.NullString
				if err := rows.Scan(
						&s.ID, &s.FullName, &s.StudentNumber, &s.NISN, &s.JenisKelamin, 
						&s.IsActive, &s.CreatedAt,
						&subGroupID, &s.SubClassName, &classGroupID, &s.ClassGroupName, &s.ClassGroupType,
					); err == nil {
					if subGroupID.Valid {
						s.ClassSubGroupID = &subGroupID.String
					}
					if classGroupID.Valid {
						s.ClassGroupID = &classGroupID.String
					}
					students = append(students, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":    len(students),
				"students": students,
			})
		})

		// Tambah murid baru & distribusikan ke sub kelas
		api.POST("/school-admin/students", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			var req CreateStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			// Validasi sub class milik sekolah ini
			var classGroupID string
			err = database.DB.QueryRow(`
				SELECT class_group_id FROM class_sub_groups 
				WHERE id = $1 AND school_id = $2
			`, req.ClassSubGroupID, schoolID).Scan(&classGroupID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak ditemukan atau bukan milik sekolah Anda"})
				return
			}

			// Cek duplikat NISN (jika diisi)
			if req.NISN != "" {
				var existingID string
				err = database.DB.QueryRow(`
					SELECT id FROM students WHERE school_id = $1 AND nisn = $2
				`, schoolID, req.NISN).Scan(&existingID)
				if err == nil && existingID != "" {
					c.JSON(http.StatusConflict, gin.H{"error": "NISN sudah terdaftar di sekolah ini"})
					return
				}
			}

			// Insert
			var studentID string
			err = database.DB.QueryRow(`
				INSERT INTO students 
					(school_id, class_group_id, class_sub_group_id, full_name, 
					student_number, nisn, jenis_kelamin, is_active)
				VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), TRUE)
				RETURNING id
			`, schoolID, classGroupID, req.ClassSubGroupID, req.FullName, 
			req.StudentNumber, req.NISN, req.JenisKelamin).Scan(&studentID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data murid: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message": "Murid berhasil ditambahkan",
				"id":      studentID,
			})
		})

		// Update data murid (termasuk pindah sub kelas)
		api.PUT("/school-admin/students/:id", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			studentID := c.Param("id")

			var req UpdateStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			// Jika pindah sub kelas, validasi dulu
			var newClassGroupID interface{} = nil
			if req.ClassSubGroupID != "" {
				var cgID string
				err = database.DB.QueryRow(`
					SELECT class_group_id FROM class_sub_groups 
					WHERE id = $1 AND school_id = $2
				`, req.ClassSubGroupID, schoolID).Scan(&cgID)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tujuan tidak valid"})
					return
				}
				newClassGroupID = cgID
			}

			var isActive interface{} = nil
			if req.IsActive != nil {
				isActive = *req.IsActive
			}

			var subGroupID interface{} = nil
			if req.ClassSubGroupID != "" {
				subGroupID = req.ClassSubGroupID
			}

			result, err := database.DB.Exec(`
				UPDATE students SET
					full_name = COALESCE(NULLIF($1, ''), full_name),
					student_number = NULLIF($2, ''),
					nisn = NULLIF($3, ''),
					jenis_kelamin = COALESCE(NULLIF($4, ''), jenis_kelamin), 
					class_sub_group_id = $5,
					class_group_id = COALESCE($6, class_group_id),
					is_active = COALESCE($7, is_active),
					updated_at = NOW()
				WHERE id = $8 AND school_id = $9
			`, req.FullName, req.StudentNumber, req.NISN, req.JenisKelamin,
			subGroupID, newClassGroupID, isActive, studentID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data murid: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Murid tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Data murid berhasil diperbarui"})
		})

		// Hapus murid
		api.DELETE("/school-admin/students/:id", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE id = $1 AND is_active = TRUE
			`, userID).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			studentID := c.Param("id")

			result, err := database.DB.Exec(`
				DELETE FROM students WHERE id = $1 AND school_id = $2
			`, studentID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus murid: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Murid tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Murid berhasil dihapus"})
		})

		// Statistik distribusi per sub kelas
		api.GET("/school-admin/students/stats", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var userEmail string
			_ = database.DB.QueryRow(
				`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
				userIDStr,
			).Scan(&userEmail)

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2)) 
				AND is_active = TRUE
				LIMIT 1
			`, userIDStr, userEmail).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					csg.id, csg.name, COALESCE(csg.capacity, 0),
					cg.name AS class_group_name,
					COUNT(s.id) AS student_count
				FROM class_sub_groups csg
				JOIN class_groups cg ON csg.class_group_id = cg.id
				LEFT JOIN students s ON s.class_sub_group_id = csg.id AND s.is_active = TRUE
				WHERE csg.school_id = $1
				GROUP BY csg.id, csg.name, csg.capacity, cg.name
				ORDER BY cg.name ASC, csg.name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil statistik: " + err.Error()})
				return
			}
			defer rows.Close()

			type StatItem struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				Capacity       int    `json:"capacity"`
				ClassGroupName string `json:"class_group_name"`
				StudentCount   int    `json:"student_count"`
			}

			var stats []StatItem
			for rows.Next() {
				var s StatItem
				if err := rows.Scan(&s.ID, &s.Name, &s.Capacity, &s.ClassGroupName, &s.StudentCount); err == nil {
					stats = append(stats, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{"stats": stats})
		})

		api.GET("/school-admin/calendar", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			// Filter opsional
			fromDate := c.Query("from")     // YYYY-MM-DD
			toDate := c.Query("to")         // YYYY-MM-DD
			dayType := c.Query("type")      // school_day | holiday | dll

			query := `
				SELECT id, calendar_date, day_type, name, COALESCE(description, ''),
					is_attendance_required, include_in_report,
					COALESCE(target_class_group_ids, ARRAY[]::uuid[]),
					created_at
				FROM academic_calendar
				WHERE school_id = $1
			`
			args := []any{schoolID}
			argIdx := 2

			if fromDate != "" {
				query += fmt.Sprintf(" AND calendar_date >= $%d", argIdx)
				args = append(args, fromDate)
				argIdx++
			}
			if toDate != "" {
				query += fmt.Sprintf(" AND calendar_date <= $%d", argIdx)
				args = append(args, toDate)
				argIdx++
			}
			if dayType != "" {
				query += fmt.Sprintf(" AND day_type = $%d", argIdx)
				args = append(args, dayType)
				argIdx++
			}

			query += " ORDER BY calendar_date ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil kalender: " + err.Error()})
				return
			}
			defer rows.Close()

			type CalendarItem struct {
				ID                   string    `json:"id"`
				CalendarDate         string    `json:"calendar_date"`
				DayType              string    `json:"day_type"`
				Name                 string    `json:"name"`
				Description          string    `json:"description"`
				IsAttendanceRequired bool      `json:"is_attendance_required"`
				IncludeInReport      bool      `json:"include_in_report"`
				TargetClassGroupIDs  []string  `json:"target_class_group_ids"`
				CreatedAt            time.Time `json:"created_at"`
			}

			var list []CalendarItem
			for rows.Next() {
				var item CalendarItem
				var dateVal time.Time
				var targetIDs []string

				if err := rows.Scan(
					&item.ID, &dateVal, &item.DayType, &item.Name, &item.Description,
					&item.IsAttendanceRequired, &item.IncludeInReport, &targetIDs, &item.CreatedAt,
				); err == nil {
					item.CalendarDate = dateVal.Format("2006-01-02")
					item.TargetClassGroupIDs = targetIDs
					list = append(list, item)
				}
			}

			c.JSON(http.StatusOK, gin.H{"calendar": list, "total": len(list)})
		})

		api.POST("/school-admin/calendar", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateCalendarEventRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			isAttendanceRequired := true
			if req.IsAttendanceRequired != nil {
				isAttendanceRequired = *req.IsAttendanceRequired
			}
			includeInReport := true
			if req.IncludeInReport != nil {
				includeInReport = *req.IncludeInReport
			}

			var targetIDs interface{} = nil
			if len(req.TargetClassGroupIDs) > 0 {
				targetIDs = req.TargetClassGroupIDs
			}

			var newID string
			err = database.DB.QueryRow(`
				INSERT INTO academic_calendar (
					school_id, calendar_date, day_type, name, description,
					is_attendance_required, include_in_report,
					target_class_group_ids, created_by
				) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9)
				RETURNING id
			`, schoolID, req.CalendarDate, req.DayType, req.Name, req.Description,
				isAttendanceRequired, includeInReport, targetIDs, adminID).Scan(&newID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Event kalender berhasil dibuat", "id": newID})
		})

		// Update kalender event
		api.PUT("/school-admin/calendar/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			eventID := c.Param("id")
			var req UpdateCalendarEventRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
				return
			}

			var targetIDs interface{} = nil
			if len(req.TargetClassGroupIDs) > 0 {
				targetIDs = req.TargetClassGroupIDs
			}

			result, err := database.DB.Exec(`
				UPDATE academic_calendar SET
					name = COALESCE(NULLIF($1, ''), name),
					description = COALESCE(NULLIF($2, ''), description),
					is_attendance_required = COALESCE($3, is_attendance_required),
					include_in_report = COALESCE($4, include_in_report),
					target_class_group_ids = COALESCE($5, target_class_group_ids),
					updated_at = NOW()
				WHERE id = $6 AND school_id = $7
			`, req.Name, req.Description, req.IsAttendanceRequired, req.IncludeInReport,
				targetIDs, eventID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Event tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Event berhasil diperbarui"})
		})

		// Hapus kalender event
		api.DELETE("/school-admin/calendar/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(
				`DELETE FROM academic_calendar WHERE id = $1 AND school_id = $2`,
				c.Param("id"), schoolID,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Event tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Event berhasil dihapus"})
		})

		// ==========================================
		// SCHOOL ADMIN: DASHBOARD STATS
		// ==========================================
		api.GET("/school-admin/dashboard-stats", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			var totalStudents, maleStudents, femaleStudents int
			var totalTeachers, maleTeachers, femaleTeachers int

			// Query statistik SISWA
			_ = database.DB.QueryRow(`
				SELECT 
					COUNT(*),
					COUNT(*) FILTER (WHERE jenis_kelamin = 'L'),
					COUNT(*) FILTER (WHERE jenis_kelamin = 'P')
				FROM students 
				WHERE school_id = $1 AND is_active = TRUE
			`, schoolID).Scan(&totalStudents, &maleStudents, &femaleStudents)

			// Query statistik GURU
			_ = database.DB.QueryRow(`
				SELECT 
					COUNT(*),
					COUNT(*) FILTER (WHERE jenis_kelamin = 'L'),
					COUNT(*) FILTER (WHERE jenis_kelamin = 'P')
				FROM teachers 
				WHERE school_id = $1 AND is_active = TRUE
			`, schoolID).Scan(&totalTeachers, &maleTeachers, &femaleTeachers)

			c.JSON(http.StatusOK, gin.H{
				"students": gin.H{
					"total":  totalStudents,
					"male":   maleStudents,
					"female": femaleStudents,
				},
				"teachers": gin.H{
					"total":  totalTeachers,
					"male":   maleTeachers,
					"female": femaleTeachers,
				},
			})
		})

		// List shifts
		api.GET("/school-admin/attendance/shifts", func(c *gin.Context) {
			fmt.Println("════════════════════════════════════════")
			fmt.Println("[GET SHIFTS] ▶ START")

			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				fmt.Printf("[GET SHIFTS] ❌ getSchoolIDFromUser error: %v\n", err)
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			fmt.Printf("[GET SHIFTS] ✅ schoolID = %q\n", schoolID)

			rows, err := database.DB.Query(`
				SELECT id, name, code, COALESCE(description, ''), shift_category,
					check_in_start, check_in_on_time_start, check_in_on_time_end, check_in_end,
					require_check_out, check_out_start, check_out_on_time_start, 
					check_out_on_time_end, check_out_end,
					allow_early_check_in, allow_late_check_out,
					auto_close_minutes_after_check_in, auto_close_minutes_after_check_out,
					early_threshold_minutes, late_tolerance_minutes, early_leave_tolerance_minutes,
					COALESCE(applicable_days, ARRAY[1,2,3,4,5,6]),
					is_active, is_default, created_at
				FROM attendance_shifts
				WHERE school_id = $1
				ORDER BY is_default DESC, code ASC
			`, schoolID)
			if err != nil {
				fmt.Printf("[GET SHIFTS] ❌ Query error: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat shift: " + err.Error()})
				return
			}
			defer rows.Close()

			type ShiftItem struct {
				ID                            string   `json:"id"`
				Name                          string   `json:"name"`
				Code                          string   `json:"code"`
				Description                   string   `json:"description"`
				ShiftCategory                 string   `json:"shift_category"`
				CheckInStart                  string   `json:"check_in_start"`
				CheckInOnTimeStart            string   `json:"check_in_on_time_start"`
				CheckInOnTimeEnd              string   `json:"check_in_on_time_end"`
				CheckInEnd                    string   `json:"check_in_end"`
				RequireCheckOut               bool     `json:"require_check_out"`
				CheckOutStart                 *string  `json:"check_out_start"`
				CheckOutOnTimeStart           *string  `json:"check_out_on_time_start"`
				CheckOutOnTimeEnd             *string  `json:"check_out_on_time_end"`
				CheckOutEnd                   *string  `json:"check_out_end"`
				AllowEarlyCheckIn             bool     `json:"allow_early_check_in"`
				AllowLateCheckOut             bool     `json:"allow_late_check_out"`
				AutoCloseMinutesAfterCheckIn  int      `json:"auto_close_minutes_after_check_in"`
				AutoCloseMinutesAfterCheckOut int      `json:"auto_close_minutes_after_check_out"`
				EarlyThresholdMinutes         int      `json:"early_threshold_minutes"`
				LateToleranceMinutes          int      `json:"late_tolerance_minutes"`
				EarlyLeaveToleranceMinutes    int      `json:"early_leave_tolerance_minutes"`
				ApplicableDays                []int    `json:"applicable_days"`
				IsActive                      bool     `json:"is_active"`
				IsDefault                     bool     `json:"is_default"`
				CreatedAt                     time.Time `json:"created_at"`
			}

			list := []ShiftItem{}
			for rows.Next() {
				var s ShiftItem
				var ciStart, ciOnTimeStart, ciOnTimeEnd, ciEnd time.Time
				var coStart, coOnTimeStart, coOnTimeEnd, coEnd sql.NullString
				var applicableDays pq.Int64Array  // ← PAKAI INI

				if err := rows.Scan(
					&s.ID, &s.Name, &s.Code, &s.Description, &s.ShiftCategory,
					&ciStart, &ciOnTimeStart, &ciOnTimeEnd, &ciEnd,
					&s.RequireCheckOut, &coStart, &coOnTimeStart, &coOnTimeEnd, &coEnd,
					&s.AllowEarlyCheckIn, &s.AllowLateCheckOut,
					&s.AutoCloseMinutesAfterCheckIn, &s.AutoCloseMinutesAfterCheckOut,
					&s.EarlyThresholdMinutes, &s.LateToleranceMinutes, &s.EarlyLeaveToleranceMinutes,
					&applicableDays,  // ← SCAN ke pq.Int64Array
					&s.IsActive, &s.IsDefault, &s.CreatedAt,
				); err != nil {
					fmt.Printf("[GET SHIFTS] ⚠️ Scan error: %v\n", err)
					continue
				}

				// Convert pq.Int64Array → []int
				s.ApplicableDays = make([]int, len(applicableDays))
				for i, v := range applicableDays {
					s.ApplicableDays[i] = int(v)
				}

				s.CheckInStart = ciStart.Format("15:04")
				s.CheckInOnTimeStart = ciOnTimeStart.Format("15:04")
				s.CheckInOnTimeEnd = ciOnTimeEnd.Format("15:04")
				s.CheckInEnd = ciEnd.Format("15:04")

				if coStart.Valid { t, _ := time.Parse("15:04:05", coStart.String); s.CheckOutStart = strPtr(t.Format("15:04")) }
				if coOnTimeStart.Valid { t, _ := time.Parse("15:04:05", coOnTimeStart.String); s.CheckOutOnTimeStart = strPtr(t.Format("15:04")) }
				if coOnTimeEnd.Valid { t, _ := time.Parse("15:04:05", coOnTimeEnd.String); s.CheckOutOnTimeEnd = strPtr(t.Format("15:04")) }
				if coEnd.Valid { t, _ := time.Parse("15:04:05", coEnd.String); s.CheckOutEnd = strPtr(t.Format("15:04")) }

				list = append(list, s)
			}

			fmt.Printf("[GET SHIFTS] ✅ Total shifts returned: %d\n", len(list))
			c.JSON(http.StatusOK, gin.H{"shifts": list, "total": len(list)})
		})

		// Create shift
		api.POST("/school-admin/attendance/shifts", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateShiftRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Defaults
			allowEarly := true
			if req.AllowEarlyCheckIn != nil { allowEarly = *req.AllowEarlyCheckIn }
			allowLateOut := false
			if req.AllowLateCheckOut != nil { allowLateOut = *req.AllowLateCheckOut }
			autoCloseIn := 120
			if req.AutoCloseMinutesAfterCheckIn != nil { autoCloseIn = *req.AutoCloseMinutesAfterCheckIn }
			autoCloseOut := 60
			if req.AutoCloseMinutesAfterCheckOut != nil { autoCloseOut = *req.AutoCloseMinutesAfterCheckOut }
			earlyThreshold := 15
			if req.EarlyThresholdMinutes != nil { earlyThreshold = *req.EarlyThresholdMinutes }
			lateTolerance := 5
			if req.LateToleranceMinutes != nil { lateTolerance = *req.LateToleranceMinutes }
			earlyLeaveTolerance := 15
			if req.EarlyLeaveToleranceMinutes != nil { earlyLeaveTolerance = *req.EarlyLeaveToleranceMinutes }
			isActive := true
			if req.IsActive != nil { isActive = *req.IsActive }
			isDefault := false
			if req.IsDefault != nil { isDefault = *req.IsDefault }
			applicableDays := req.ApplicableDays
			if len(applicableDays) == 0 { applicableDays = []int{1, 2, 3, 4, 5, 6} }

			shiftCategory := req.ShiftCategory
			if shiftCategory == "" { shiftCategory = "regular" }

			// Prepare nullable check-out
			var coStart, coOnTimeStart, coOnTimeEnd, coEnd interface{} = nil, nil, nil, nil
			if req.RequireCheckOut {
				if req.CheckOutStart != "" { coStart = req.CheckOutStart }
				if req.CheckOutOnTimeStart != "" { coOnTimeStart = req.CheckOutOnTimeStart }
				if req.CheckOutOnTimeEnd != "" { coOnTimeEnd = req.CheckOutOnTimeEnd }
				if req.CheckOutEnd != "" { coEnd = req.CheckOutEnd }
			}

			// Kalau set is_default = true, nonaktifkan default lain
			if isDefault {
				_, _ = database.DB.Exec(
					`UPDATE attendance_shifts SET is_default = false WHERE school_id = $1`,
					schoolID,
				)
			}

			var shiftID string
			err = database.DB.QueryRow(`
				INSERT INTO attendance_shifts (
					school_id, name, code, description, shift_category,
					check_in_start, check_in_on_time_start, check_in_on_time_end, check_in_end,
					require_check_out, check_out_start, check_out_on_time_start,
					check_out_on_time_end, check_out_end,
					allow_early_check_in, allow_late_check_out,
					auto_close_minutes_after_check_in, auto_close_minutes_after_check_out,
					early_threshold_minutes, late_tolerance_minutes, early_leave_tolerance_minutes,
					applicable_days, is_active, is_default, created_by
				) VALUES (
					$1, $2, $3, NULLIF($4, ''), $5,
					$6, $7, $8, $9,
					$10, $11, $12, $13, $14,
					$15, $16,
					$17, $18,
					$19, $20, $21,
					$22, $23, $24, $25
				) RETURNING id
			`, schoolID, req.Name, req.Code, req.Description, shiftCategory,
				req.CheckInStart, req.CheckInOnTimeStart, req.CheckInOnTimeEnd, req.CheckInEnd,
				req.RequireCheckOut, coStart, coOnTimeStart, coOnTimeEnd, coEnd,
				allowEarly, allowLateOut,
				autoCloseIn, autoCloseOut,
				earlyThreshold, lateTolerance, earlyLeaveTolerance,
				applicableDays, isActive, isDefault, adminID).Scan(&shiftID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan shift: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Shift berhasil dibuat", "id": shiftID})
		})

		// Update shift
		api.PUT("/school-admin/attendance/shifts/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			shiftID := c.Param("id")
			var req UpdateShiftRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
				return
			}

			// Build dynamic query — hanya update field yang diisi
			updates := []string{}
			args := []interface{}{}
			argIdx := 1

			if req.Name != "" {
				updates = append(updates, fmt.Sprintf("name = $%d", argIdx))
				args = append(args, req.Name); argIdx++
			}
			if req.Code != "" {
				updates = append(updates, fmt.Sprintf("code = $%d", argIdx))
				args = append(args, req.Code); argIdx++
			}
			if req.Description != "" {
				updates = append(updates, fmt.Sprintf("description = $%d", argIdx))
				args = append(args, req.Description); argIdx++
			}
			if req.CheckInStart != "" {
				updates = append(updates, fmt.Sprintf("check_in_start = $%d", argIdx))
				args = append(args, req.CheckInStart); argIdx++
			}
			if req.CheckInOnTimeStart != "" {
				updates = append(updates, fmt.Sprintf("check_in_on_time_start = $%d", argIdx))
				args = append(args, req.CheckInOnTimeStart); argIdx++
			}
			if req.CheckInOnTimeEnd != "" {
				updates = append(updates, fmt.Sprintf("check_in_on_time_end = $%d", argIdx))
				args = append(args, req.CheckInOnTimeEnd); argIdx++
			}
			if req.CheckInEnd != "" {
				updates = append(updates, fmt.Sprintf("check_in_end = $%d", argIdx))
				args = append(args, req.CheckInEnd); argIdx++
			}
			if req.RequireCheckOut != nil {
				updates = append(updates, fmt.Sprintf("require_check_out = $%d", argIdx))
				args = append(args, *req.RequireCheckOut); argIdx++
			}
			if req.CheckOutStart != "" {
				updates = append(updates, fmt.Sprintf("check_out_start = $%d", argIdx))
				args = append(args, req.CheckOutStart); argIdx++
			}
			if req.CheckOutOnTimeStart != "" {
				updates = append(updates, fmt.Sprintf("check_out_on_time_start = $%d", argIdx))
				args = append(args, req.CheckOutOnTimeStart); argIdx++
			}
			if req.CheckOutOnTimeEnd != "" {
				updates = append(updates, fmt.Sprintf("check_out_on_time_end = $%d", argIdx))
				args = append(args, req.CheckOutOnTimeEnd); argIdx++
			}
			if req.CheckOutEnd != "" {
				updates = append(updates, fmt.Sprintf("check_out_end = $%d", argIdx))
				args = append(args, req.CheckOutEnd); argIdx++
			}
			if req.IsActive != nil {
				updates = append(updates, fmt.Sprintf("is_active = $%d", argIdx))
				args = append(args, *req.IsActive); argIdx++
			}

			if len(updates) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada field yang diupdate"})
				return
			}

			updates = append(updates, "updated_at = NOW()")
			query := fmt.Sprintf(
				"UPDATE attendance_shifts SET %s WHERE id = $%d AND school_id = $%d",
				joinStrings(updates, ", "), argIdx, argIdx+1,
			)
			args = append(args, shiftID, schoolID)

			result, err := database.DB.Exec(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Shift tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Shift berhasil diperbarui"})
		})

		// Hapus shift
		api.DELETE("/school-admin/attendance/shifts/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			shiftID := c.Param("id")

			// Cek apakah ada session aktif pakai shift ini
			var activeCount int
			_ = database.DB.QueryRow(`
				SELECT count(*) FROM attendance_sessions
				WHERE shift_id = $1 AND status IN ('open', 'scheduled')
			`, shiftID).Scan(&activeCount)

			if activeCount > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("Tidak bisa hapus: ada %d sesi aktif pakai shift ini", activeCount),
				})
				return
			}

			result, err := database.DB.Exec(
				`DELETE FROM attendance_shifts WHERE id = $1 AND school_id = $2`,
				shiftID, schoolID,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Shift tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Shift berhasil dihapus"})
		})

		// List assignments
		api.GET("/school-admin/attendance/assignments", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			classGroupID := c.Query("class_group_id")
			shiftID := c.Query("shift_id")

			query := `
				SELECT 
					csa.id, csa.class_group_id, csa.class_sub_group_id, csa.shift_id,
					csa.priority, csa.is_primary,
					csa.override_check_in_start, csa.override_check_in_end,
					csa.override_check_out_start, csa.override_check_out_end,
					csa.effective_from, csa.effective_to, csa.is_active,
					csa.created_at,
					COALESCE(cg.name, '') AS class_group_name,
					COALESCE(csg.name, '') AS class_sub_group_name,
					COALESCE(s.name, '') AS shift_name,
					COALESCE(s.code, '') AS shift_code
				FROM class_shift_assignments csa
				LEFT JOIN class_groups cg ON csa.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON csa.class_sub_group_id = csg.id
				LEFT JOIN attendance_shifts s ON csa.shift_id = s.id
				WHERE csa.school_id = $1
			`
			args := []any{schoolID}
			argIdx := 2

			if classGroupID != "" {
				query += fmt.Sprintf(" AND csa.class_group_id = $%d", argIdx)
				args = append(args, classGroupID); argIdx++
			}
			if shiftID != "" {
				query += fmt.Sprintf(" AND csa.shift_id = $%d", argIdx)
				args = append(args, shiftID); argIdx++
			}

			query += " ORDER BY csa.is_primary DESC, csa.priority ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type AssignmentItem struct {
				ID                     string     `json:"id"`
				ClassGroupID           *string    `json:"class_group_id"`
				ClassSubGroupID        *string    `json:"class_sub_group_id"`
				ShiftID                string     `json:"shift_id"`
				Priority               int        `json:"priority"`
				IsPrimary              bool       `json:"is_primary"`
				OverrideCheckInStart   *string    `json:"override_check_in_start"`
				OverrideCheckInEnd     *string    `json:"override_check_in_end"`
				OverrideCheckOutStart  *string    `json:"override_check_out_start"`
				OverrideCheckOutEnd    *string    `json:"override_check_out_end"`
				EffectiveFrom          *string    `json:"effective_from"`
				EffectiveTo            *string    `json:"effective_to"`
				IsActive               bool       `json:"is_active"`
				CreatedAt              time.Time  `json:"created_at"`
				ClassGroupName         string     `json:"class_group_name"`
				ClassSubGroupName      string     `json:"class_sub_group_name"`
				ShiftName              string     `json:"shift_name"`
				ShiftCode              string     `json:"shift_code"`
			}

			var list []AssignmentItem
			for rows.Next() {
				var a AssignmentItem
				var cgID, csgID sql.NullString
				var ociStart, ociEnd, ocoStart, ocoEnd sql.NullString
				var effFrom, effTo sql.NullTime

				if err := rows.Scan(
					&a.ID, &cgID, &csgID, &a.ShiftID,
					&a.Priority, &a.IsPrimary,
					&ociStart, &ociEnd, &ocoStart, &ocoEnd,
					&effFrom, &effTo, &a.IsActive, &a.CreatedAt,
					&a.ClassGroupName, &a.ClassSubGroupName, &a.ShiftName, &a.ShiftCode,
				); err == nil {
					if cgID.Valid { a.ClassGroupID = &cgID.String }
					if csgID.Valid { a.ClassSubGroupID = &csgID.String }
					if ociStart.Valid { t, _ := time.Parse("15:04:05", ociStart.String); s := t.Format("15:04"); a.OverrideCheckInStart = &s }
					if ociEnd.Valid { t, _ := time.Parse("15:04:05", ociEnd.String); s := t.Format("15:04"); a.OverrideCheckInEnd = &s }
					if ocoStart.Valid { t, _ := time.Parse("15:04:05", ocoStart.String); s := t.Format("15:04"); a.OverrideCheckOutStart = &s }
					if ocoEnd.Valid { t, _ := time.Parse("15:04:05", ocoEnd.String); s := t.Format("15:04"); a.OverrideCheckOutEnd = &s }
					if effFrom.Valid { s := effFrom.Time.Format("2006-01-02"); a.EffectiveFrom = &s }
					if effTo.Valid { s := effTo.Time.Format("2006-01-02"); a.EffectiveTo = &s }

					list = append(list, a)
				}
			}

			c.JSON(http.StatusOK, gin.H{"assignments": list, "total": len(list)})
		})

		// Create assignment
		api.POST("/school-admin/attendance/assignments", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateClassShiftAssignmentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid"})
				return
			}

			if req.ClassGroupID == "" && req.ClassSubGroupID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal isi class_group_id atau class_sub_group_id"})
				return
			}

			priority := req.Priority
			if priority == 0 { priority = 1 }
			isPrimary := true
			if req.IsPrimary != nil { isPrimary = *req.IsPrimary }

			var classGroupID, classSubGroupID interface{} = nil, nil
			if req.ClassGroupID != "" { classGroupID = req.ClassGroupID }
			if req.ClassSubGroupID != "" { classSubGroupID = req.ClassSubGroupID }

			var ociStart, ociEnd, ocoStart, ocoEnd interface{} = nil, nil, nil, nil
			if req.OverrideCheckInStart != "" { ociStart = req.OverrideCheckInStart }
			if req.OverrideCheckInEnd != "" { ociEnd = req.OverrideCheckInEnd }
			if req.OverrideCheckOutStart != "" { ocoStart = req.OverrideCheckOutStart }
			if req.OverrideCheckOutEnd != "" { ocoEnd = req.OverrideCheckOutEnd }

			var effFrom, effTo interface{} = nil, nil
			if req.EffectiveFrom != "" { effFrom = req.EffectiveFrom }
			if req.EffectiveTo != "" { effTo = req.EffectiveTo }

			var assignmentID string
			err = database.DB.QueryRow(`
				INSERT INTO class_shift_assignments (
					school_id, class_group_id, class_sub_group_id, shift_id,
					priority, is_primary,
					override_check_in_start, override_check_in_end,
					override_check_out_start, override_check_out_end,
					effective_from, effective_to, created_by
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				RETURNING id
			`, schoolID, classGroupID, classSubGroupID, req.ShiftID,
				priority, isPrimary,
				ociStart, ociEnd, ocoStart, ocoEnd,
				effFrom, effTo, adminID).Scan(&assignmentID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Assignment berhasil dibuat", "id": assignmentID})
		})

		// Hapus assignment
		api.DELETE("/school-admin/attendance/assignments/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(
				`DELETE FROM class_shift_assignments WHERE id = $1 AND school_id = $2`,
				c.Param("id"), schoolID,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Assignment tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Assignment berhasil dihapus"})
		})

		// List QR codes siswa
		api.GET("/school-admin/students/qr-list", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					sqc.id, sqc.student_id, sqc.qr_token, sqc.is_active,
					sqc.generated_at, sqc.regenerated_count,
					s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, '')
				FROM student_qr_codes sqc
				JOIN students s ON sqc.student_id = s.id
				WHERE sqc.school_id = $1
				ORDER BY s.full_name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat QR: " + err.Error()})
				return
			}
			defer rows.Close()

			type QRItem struct {
				ID                string    `json:"id"`
				StudentID         string    `json:"student_id"`
				QRToken           string    `json:"qr_token"`
				IsActive          bool      `json:"is_active"`
				GeneratedAt       time.Time `json:"generated_at"`
				RegeneratedCount  int       `json:"regenerated_count"`
				StudentName       string    `json:"student_name"`
				NISN              string    `json:"nisn"`
				StudentNumber     string    `json:"student_number"`
			}

			var list []QRItem
			for rows.Next() {
				var q QRItem
				if err := rows.Scan(
					&q.ID, &q.StudentID, &q.QRToken, &q.IsActive,
					&q.GeneratedAt, &q.RegeneratedCount,
					&q.StudentName, &q.NISN, &q.StudentNumber,
				); err == nil {
					list = append(list, q)
				}
			}

			c.JSON(http.StatusOK, gin.H{"qr_codes": list, "total": len(list)})
		})

		api.POST("/school-admin/students/:id/generate-qr", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)
			studentID := c.Param("id")

			var exists string
			err = database.DB.QueryRow(`
				SELECT id FROM students WHERE id = $1 AND school_id = $2
			`, studentID, schoolID).Scan(&exists)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Siswa tidak ditemukan"})
				return
			}

			var existingID string
			err = database.DB.QueryRow(`
				SELECT id FROM student_qr_codes WHERE student_id = $1
			`, studentID).Scan(&existingID)
			if err == nil && existingID != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa ini sudah punya QR code"})
				return
			}

			token := generateQRToken()

			var qrID string
			err = database.DB.QueryRow(`
				INSERT INTO student_qr_codes (student_id, school_id, qr_token, generated_by)
				VALUES ($1, $2, $3, $4)
				RETURNING id
			`, studentID, schoolID, token, adminID).Scan(&qrID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan QR: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":  "QR berhasil dibuat",
				"id":       qrID,
				"qr_token": token,
			})
		})

		api.POST("/school-admin/students/generate-qr-bulk", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req GenerateQRRequest
			_ = c.ShouldBindJSON(&req)

			query := `
				SELECT s.id FROM students s
				LEFT JOIN student_qr_codes sqc ON sqc.student_id = s.id
				WHERE s.school_id = $1 AND s.is_active = TRUE AND sqc.id IS NULL
			`
			args := []any{schoolID}
			argIdx := 2

			if req.ClassSubGroupID != "" {
				query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx)
				args = append(args, req.ClassSubGroupID)
				argIdx++
			}

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query siswa: " + err.Error()})
				return
			}
			defer rows.Close()

			var studentIDs []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					studentIDs = append(studentIDs, id)
				}
			}

			if len(studentIDs) == 0 {
				c.JSON(http.StatusOK, gin.H{"message": "Semua siswa sudah punya QR", "generated": 0})
				return
			}

			// Batch insert
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			generated := 0
			for _, sid := range studentIDs {
				token := generateQRToken()
				_, err := tx.Exec(`
					INSERT INTO student_qr_codes (student_id, school_id, qr_token, generated_by)
					VALUES ($1, $2, $3, $4)
				`, sid, schoolID, token, adminID)
				if err == nil {
					generated++
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":   fmt.Sprintf("%d QR berhasil dibuat", generated),
				"generated": generated,
			})
		})

		// Regenerate QR siswa
		api.POST("/school-admin/students/:id/regenerate-qr", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			studentID := c.Param("id")

			// Cek QR existing
			var qrID string
			err = database.DB.QueryRow(`
				SELECT id FROM student_qr_codes 
				WHERE student_id = $1 AND school_id = $2
			`, studentID, schoolID).Scan(&qrID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "QR tidak ditemukan"})
				return
			}

			newToken := generateQRToken()
			_, err = database.DB.Exec(`
				UPDATE student_qr_codes SET
					qr_token = $1,
					regenerated_count = regenerated_count + 1,
					generated_at = NOW(),
					updated_at = NOW()
				WHERE id = $2
			`, newToken, qrID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal regenerate: " + err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":  "QR berhasil di-regenerate",
				"qr_token": newToken,
			})
		})

		// List semua sessions
		api.GET("/school-admin/attendance/sessions", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			// Filter opsional
			dateFrom := c.Query("from")
			dateTo := c.Query("to")
			status := c.Query("status")
			limit := c.DefaultQuery("limit", "100")

			query := `
				SELECT 
					ases.id, ases.title, ases.session_date, ases.status,
					ases.shift_id, COALESCE(sh.name, '') AS shift_name, COALESCE(sh.code, '') AS shift_code,
					ases.class_group_id, COALESCE(cg.name, '') AS class_group_name,
					ases.class_sub_group_id, COALESCE(csg.name, '') AS class_sub_group_name,
					ases.total_students, ases.total_check_in, ases.total_check_out,
					ases.total_absent,
					ases.opened_at, ases.closed_at,
					ases.created_at
				FROM attendance_sessions ases
				LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
				LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON ases.class_sub_group_id = csg.id
				WHERE ases.school_id = $1
			`
			args := []any{schoolID}
			argIdx := 2

			if dateFrom != "" {
				query += fmt.Sprintf(" AND ases.session_date >= $%d", argIdx)
				args = append(args, dateFrom); argIdx++
			}
			if dateTo != "" {
				query += fmt.Sprintf(" AND ases.session_date <= $%d", argIdx)
				args = append(args, dateTo); argIdx++
			}
			if status != "" {
				query += fmt.Sprintf(" AND ases.status = $%d", argIdx)
				args = append(args, status); argIdx++
			}

			query += fmt.Sprintf(" ORDER BY ases.session_date DESC, ases.created_at DESC LIMIT $%d", argIdx)
			args = append(args, limit)

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat sesi: " + err.Error()})
				return
			}
			defer rows.Close()

			type SessionItem struct {
				ID                 string     `json:"id"`
				Title              string     `json:"title"`
				SessionDate        string     `json:"session_date"`
				Status             string     `json:"status"`
				ShiftID            string     `json:"shift_id"`
				ShiftName          string     `json:"shift_name"`
				ShiftCode          string     `json:"shift_code"`
				ClassGroupID       *string    `json:"class_group_id"`
				ClassGroupName     string     `json:"class_group_name"`
				ClassSubGroupID    *string    `json:"class_sub_group_id"`
				ClassSubGroupName  string     `json:"class_sub_group_name"`
				TotalStudents      int        `json:"total_students"`
				TotalCheckIn       int        `json:"total_check_in"`
				TotalCheckOut      int        `json:"total_check_out"`
				TotalAbsent        int        `json:"total_absent"`
				OpenedAt           *time.Time `json:"opened_at"`
				ClosedAt           *time.Time `json:"closed_at"`
				CreatedAt          time.Time  `json:"created_at"`
			}

			var sessions []SessionItem
			for rows.Next() {
				var s SessionItem
				var dateVal time.Time
				var cgID, csgID sql.NullString
				var openedAt, closedAt sql.NullTime

				if err := rows.Scan(
					&s.ID, &s.Title, &dateVal, &s.Status,
					&s.ShiftID, &s.ShiftName, &s.ShiftCode,
					&cgID, &s.ClassGroupName,
					&csgID, &s.ClassSubGroupName,
					&s.TotalStudents, &s.TotalCheckIn, &s.TotalCheckOut,
					&s.TotalAbsent,
					&openedAt, &closedAt, &s.CreatedAt,
				); err == nil {
					s.SessionDate = dateVal.Format("2006-01-02")
					if cgID.Valid { s.ClassGroupID = &cgID.String }
					if csgID.Valid { s.ClassSubGroupID = &csgID.String }
					if openedAt.Valid { s.OpenedAt = &openedAt.Time }
					if closedAt.Valid { s.ClosedAt = &closedAt.Time }
					sessions = append(sessions, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
		})

		// Sessions today
		api.GET("/school-admin/attendance/sessions/today", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					ases.id, ases.title, ases.session_date, ases.status,
					ases.shift_id, COALESCE(sh.name, '') AS shift_name,
					ases.class_group_id, COALESCE(cg.name, '') AS class_group_name,
					ases.total_students, ases.total_check_in, ases.total_absent
				FROM attendance_sessions ases
				LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
				LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
				WHERE ases.school_id = $1 
					AND ases.session_date = CURRENT_DATE
					AND ases.status IN ('scheduled', 'open')
				ORDER BY ases.created_at ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type TodayItem struct {
				ID              string  `json:"id"`
				Title           string  `json:"title"`
				SessionDate     string  `json:"session_date"`
				Status          string  `json:"status"`
				ShiftID         string  `json:"shift_id"`
				ShiftName       string  `json:"shift_name"`
				ClassGroupID    *string `json:"class_group_id"`
				ClassGroupName  string  `json:"class_group_name"`
				TotalStudents   int     `json:"total_students"`
				TotalCheckIn    int     `json:"total_check_in"`
				TotalAbsent     int     `json:"total_absent"`
			}

			var sessions []TodayItem
			for rows.Next() {
				var s TodayItem
				var dateVal time.Time
				var cgID sql.NullString
				if err := rows.Scan(
					&s.ID, &s.Title, &dateVal, &s.Status,
					&s.ShiftID, &s.ShiftName,
					&cgID, &s.ClassGroupName,
					&s.TotalStudents, &s.TotalCheckIn, &s.TotalAbsent,
				); err == nil {
					s.SessionDate = dateVal.Format("2006-01-02")
					if cgID.Valid { s.ClassGroupID = &cgID.String }
					sessions = append(sessions, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
		})

		// Detail session
		api.GET("/school-admin/attendance/sessions/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			sessionID := c.Param("id")

			var s struct {
				ID                 string
				Title              string
				SessionDate        time.Time
				Status             string
				ShiftID            string
				ShiftName          string
				ShiftCode          string
				ClassGroupID       sql.NullString
				ClassGroupName     string
				ClassSubGroupID    sql.NullString
				ClassSubGroupName  string
				TotalStudents      int
				TotalCheckIn       int
				TotalCheckOut      int
				TotalAbsent        int
				OpenedAt           sql.NullTime
				ClosedAt           sql.NullTime
			}

			err = database.DB.QueryRow(`
				SELECT 
					ases.id, ases.title, ases.session_date, ases.status,
					ases.shift_id, COALESCE(sh.name, ''), COALESCE(sh.code, ''),
					ases.class_group_id, COALESCE(cg.name, ''),
					ases.class_sub_group_id, COALESCE(csg.name, ''),
					ases.total_students, ases.total_check_in, ases.total_check_out,
					ases.total_absent, ases.opened_at, ases.closed_at
				FROM attendance_sessions ases
				LEFT JOIN attendance_shifts sh ON ases.shift_id = sh.id
				LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON ases.class_sub_group_id = csg.id
				WHERE ases.id = $1 AND ases.school_id = $2
			`, sessionID, schoolID).Scan(
				&s.ID, &s.Title, &s.SessionDate, &s.Status,
				&s.ShiftID, &s.ShiftName, &s.ShiftCode,
				&s.ClassGroupID, &s.ClassGroupName,
				&s.ClassSubGroupID, &s.ClassSubGroupName,
				&s.TotalStudents, &s.TotalCheckIn, &s.TotalCheckOut,
				&s.TotalAbsent, &s.OpenedAt, &s.ClosedAt,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			result := gin.H{
				"id":                  s.ID,
				"title":               s.Title,
				"session_date":        s.SessionDate.Format("2006-01-02"),
				"status":              s.Status,
				"shift_id":            s.ShiftID,
				"shift_name":          s.ShiftName,
				"shift_code":          s.ShiftCode,
				"class_group_name":    s.ClassGroupName,
				"class_sub_group_name": s.ClassSubGroupName,
				"total_students":      s.TotalStudents,
				"total_check_in":      s.TotalCheckIn,
				"total_check_out":     s.TotalCheckOut,
				"total_absent":        s.TotalAbsent,
			}
			if s.ClassGroupID.Valid { result["class_group_id"] = s.ClassGroupID.String }
			if s.ClassSubGroupID.Valid { result["class_sub_group_id"] = s.ClassSubGroupID.String }
			if s.OpenedAt.Valid { result["opened_at"] = s.OpenedAt.Time }
			if s.ClosedAt.Valid { result["closed_at"] = s.ClosedAt.Time }

			c.JSON(http.StatusOK, gin.H{"session": result})
		})

		// Buat session baru
		api.POST("/school-admin/attendance/sessions", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateAttendanceSessionRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Ambil shift config
			var shiftName, shiftCode string
			var checkInStart, checkInEnd time.Time
			var requireCheckOut bool
			var shiftConfigJSON []byte

			err = database.DB.QueryRow(`
				SELECT name, code, check_in_start, check_in_end, require_check_out,
					row_to_json(attendance_shifts)
				FROM attendance_shifts
				WHERE id = $1 AND school_id = $2
			`, req.ShiftID, schoolID).Scan(
				&shiftName, &shiftCode, &checkInStart, &checkInEnd, &requireCheckOut, &shiftConfigJSON,
			)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Shift tidak ditemukan"})
				return
			}

			// Session date (default today)
			sessionDate := req.SessionDate
			if sessionDate == "" {
				sessionDate = nowJakarta().Format("2006-01-02")
			}

			// Title (auto-generate jika kosong)
			title := req.Title
			if title == "" {
				title = fmt.Sprintf("%s - %s", shiftName, sessionDate)
			}

			// Count total_students berdasarkan filter kelas
			countQuery := `SELECT COUNT(*) FROM students WHERE school_id = $1 AND is_active = TRUE`
			countArgs := []any{schoolID}
			countIdx := 2

			var classGroupID, classSubGroupID interface{} = nil, nil
			if req.ClassGroupID != "" {
				classGroupID = req.ClassGroupID
				countQuery += fmt.Sprintf(" AND class_group_id = $%d", countIdx)
				countArgs = append(countArgs, req.ClassGroupID)
				countIdx++
			}
			if req.ClassSubGroupID != "" {
				classSubGroupID = req.ClassSubGroupID
				countQuery += fmt.Sprintf(" AND class_sub_group_id = $%d", countIdx)
				countArgs = append(countArgs, req.ClassSubGroupID)
				countIdx++
			}

			var totalStudents int
			_ = database.DB.QueryRow(countQuery, countArgs...).Scan(&totalStudents)

			// Auto-close time = session_date + check_in_end + auto_close_minutes
			autoCloseMinutes := 120
			var acm sql.NullInt64
			_ = database.DB.QueryRow(`
				SELECT auto_close_minutes_after_check_in FROM attendance_shifts WHERE id = $1
			`, req.ShiftID).Scan(&acm)
			if acm.Valid {
				autoCloseMinutes = int(acm.Int64)
			}

			now := nowJakarta()
			autoCloseTime := time.Date(
				now.Year(), now.Month(), now.Day(),
				checkInEnd.Hour(), checkInEnd.Minute(), 0, 0, now.Location(),
			).Add(time.Duration(autoCloseMinutes) * time.Minute)

			var sessionID string
			err = database.DB.QueryRow(`
				INSERT INTO attendance_sessions (
					school_id, shift_id, class_group_id, class_sub_group_id,
					title, session_date, shift_config_snapshot,
					status, opened_by, auto_close_at, total_students, notes
				) VALUES ($1, $2, $3, $4, $5, $6, $7, 'scheduled', $8, $9, $10, NULLIF($11, ''))
				RETURNING id
			`, schoolID, req.ShiftID, classGroupID, classSubGroupID,
				title, sessionDate, shiftConfigJSON,
				adminID, autoCloseTime, totalStudents, req.Notes).Scan(&sessionID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":    "Sesi berhasil dibuat",
				"id":         sessionID,
				"session_id": sessionID,
			})
		})

		// Open session
		api.POST("/school-admin/attendance/sessions/:id/open", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			sessionID := c.Param("id")
			adminID, _ := getAdminIDFromUser(c)

			result, err := database.DB.Exec(`
				UPDATE attendance_sessions SET
					status = 'open',
					opened_at = COALESCE(opened_at, NOW()),
					opened_by = COALESCE(opened_by, $1),
					updated_at = NOW()
				WHERE id = $2 AND school_id = $3 AND status IN ('scheduled', 'open')
			`, adminID, sessionID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan atau sudah ditutup"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Sesi berhasil dibuka"})
		})

		// Close session (dengan auto-absent)
		api.POST("/school-admin/attendance/sessions/:id/close", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			sessionID := c.Param("id")
			adminID, _ := getAdminIDFromUser(c)

			// Validasi session milik sekolah
			var sStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM attendance_sessions 
				WHERE id = $1 AND school_id = $2
			`, sessionID, schoolID).Scan(&sStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"})
				return
			}

			if sStatus == "closed" || sStatus == "auto_closed" {
				c.JSON(http.StatusConflict, gin.H{"error": "Sesi sudah ditutup"})
				return
			}

			// Panggil function auto-mark absent
			var absentCount int
			err = database.DB.QueryRow(
				`SELECT auto_mark_absent_for_session($1)`,
				sessionID,
			).Scan(&absentCount)
			if err != nil {
				fmt.Printf("[CLOSE SESSION] Auto absent error: %v\n", err)
				absentCount = 0
			}

			// Update status
			_, _ = database.DB.Exec(`
				UPDATE attendance_sessions SET
					status = 'closed',
					closed_by = $1,
					closed_at = NOW(),
					updated_at = NOW()
				WHERE id = $2
			`, adminID, sessionID)

			// Recalculate stats
			_, _ = database.DB.Exec(`SELECT recalculate_session_stats($1)`, sessionID)

			c.JSON(http.StatusOK, gin.H{
				"message":      "Sesi berhasil ditutup",
				"absent_count": absentCount,
			})
		})

		// Hapus session
		api.DELETE("/school-admin/attendance/sessions/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				DELETE FROM attendance_sessions WHERE id = $1 AND school_id = $2
			`, c.Param("id"), schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Sesi berhasil dihapus"})
		})

		// Records untuk session (untuk report)
		api.GET("/school-admin/attendance/sessions/:id/records", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			sessionID := c.Param("id")

			// Ambil session info dulu
			var classGroupID, classSubGroupID sql.NullString
			err = database.DB.QueryRow(`
				SELECT class_group_id, class_sub_group_id 
				FROM attendance_sessions 
				WHERE id = $1 AND school_id = $2
			`, sessionID, schoolID).Scan(&classGroupID, &classSubGroupID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"})
				return
			}

			// Query semua siswa target + LEFT JOIN ke records
			query := `
				SELECT 
					s.id AS student_id,
					s.full_name AS student_name,
					COALESCE(s.nisn, '') AS nisn,
					COALESCE(csg.name, '') AS sub_class_name,
					ci.status AS check_in_status,
					ci.scanned_at AS check_in_at,
					co.status AS check_out_status,
					co.scanned_at AS check_out_at,
					COALESCE(ci.status = 'absent', false) AS is_absent
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN attendance_records ci ON ci.student_id = s.id 
					AND ci.session_id = $1 AND ci.record_type = 'check_in'
				LEFT JOIN attendance_records co ON co.student_id = s.id 
					AND co.session_id = $1 AND co.record_type = 'check_out'
				WHERE s.school_id = $2 AND s.is_active = TRUE
			`
			args := []any{sessionID, schoolID}
			argIdx := 3

			if classGroupID.Valid {
				query += fmt.Sprintf(" AND s.class_group_id = $%d", argIdx)
				args = append(args, classGroupID.String)
				argIdx++
			}
			if classSubGroupID.Valid {
				query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx)
				args = append(args, classSubGroupID.String)
				argIdx++
			}

			query += " ORDER BY s.full_name ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query records: " + err.Error()})
				return
			}
			defer rows.Close()

			type RecordItem struct {
				StudentID        string  `json:"student_id"`
				StudentName      string  `json:"student_name"`
				NISN             string  `json:"nisn"`
				SubClassName     string  `json:"sub_class_name"`
				CheckInStatus    *string `json:"check_in_status"`
				CheckInTime      *string `json:"check_in_time"`
				CheckOutStatus   *string `json:"check_out_status"`
				CheckOutTime     *string `json:"check_out_time"`
				IsAbsent         bool    `json:"is_absent"`
			}

			var records []RecordItem
			for rows.Next() {
				var r RecordItem
				var ciStatus, coStatus sql.NullString
				var ciAt, coAt sql.NullTime

				if err := rows.Scan(
					&r.StudentID, &r.StudentName, &r.NISN, &r.SubClassName,
					&ciStatus, &ciAt, &coStatus, &coAt, &r.IsAbsent,
				); err == nil {
					if ciStatus.Valid { r.CheckInStatus = &ciStatus.String }
					if coStatus.Valid { r.CheckOutStatus = &coStatus.String }
					if ciAt.Valid {
						t := ciAt.Time.Format("15:04")
						r.CheckInTime = &t
					}
					if coAt.Valid {
						t := coAt.Time.Format("15:04")
						r.CheckOutTime = &t
					}
					records = append(records, r)
				}
			}

			c.JSON(http.StatusOK, gin.H{"records": records, "total": len(records)})
		})

		// ==========================================
		// QR SCAN — thin wrapper ke processAttendanceScan
		// ==========================================

		api.POST("/school-admin/attendance/scan", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req ScanQRRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			result, procErr := processAttendanceScan(
				schoolID, req.SessionID, "qr", req.QRToken,
				"", "", adminID, req.ForceMode,
			)
			if procErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": procErr.Error()})
				return
			}

			if !result.Success {
				status := http.StatusBadRequest
				switch result.ErrorCode {
				case "not_found":
					status = http.StatusNotFound
				case "duplicate":
					status = http.StatusConflict
				case "session_not_found":
					status = http.StatusNotFound
				case "session_closed":
					status = http.StatusConflict
				}
				c.JSON(status, gin.H{
					"success": false,
					"duplicate": result.ErrorCode == "duplicate",
					"error":   result.ErrorMessage,
					"code":    result.ErrorCode,
					"student": gin.H{
						"full_name": result.StudentName,
						"nisn":      result.StudentNISN,
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"mode":    result.Mode,
				"status":  result.Status,
				"student": gin.H{
					"id":             result.StudentID,
					"full_name":      result.StudentName,
					"nisn":           result.StudentNISN,
					"sub_class_name": result.SubClassName,
				},
				"scanned_at": nowJakarta(),
				"record_id":  result.RecordID,
			})
		})

		api.POST("/school-admin/attendance/scan-rfid", HandleScanRFID)

		// ==========================================
		// RFID CRUD
		// ==========================================
		api.GET("/school-admin/rfid-cards", HandleListRFIDCards)
		api.POST("/school-admin/rfid-cards", HandleRegisterRFID)
		api.POST("/school-admin/rfid-cards/bulk-register", HandleBulkRegisterRFID)
		api.DELETE("/school-admin/rfid-cards/:id", HandleDeleteRFID)
		api.POST("/school-admin/rfid-cards/:id/revoke", HandleRevokeRFID)

		// ==========================================
		// DEVICES / KIOSK
		// ==========================================
		api.GET("/school-admin/attendance/devices", HandleListDevices)
		api.POST("/school-admin/attendance/devices", HandleCreateDevice)
		api.PATCH("/school-admin/attendance/devices/:id", HandleUpdateDevice)
		api.DELETE("/school-admin/attendance/devices/:id", HandleDeleteDevice)

		// Public endpoint untuk kiosk (device-code auth, tidak butuh login admin)
		apiPub.GET("/kiosk/:code/active-session", func(c *gin.Context) {
			// handle khusus kiosk: auth via device code, bukan user login
			code := c.Param("code")

			// Ambil school_id dari device code
			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM attendance_devices
				WHERE device_code = $1 AND is_active = TRUE
			`, code).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"available": false, "error": "Device tidak dikenal"})
				return
			}

			// Reuse logic resolve
			sessionID, deviceLabel, resErr := resolveActiveSessionForDevice(schoolID, code)
			if resErr != nil {
				c.JSON(http.StatusOK, gin.H{
					"available": false,
					"error":     resErr.Error(),
					"device":    deviceLabel,
				})
				return
			}

			var title, shiftName, className, status string
			var totalStudents, totalCheckIn int
			_ = database.DB.QueryRow(`
				SELECT ases.title, sh.name, COALESCE(cg.name,''),
				       ases.status, ases.total_students, ases.total_check_in
				FROM attendance_sessions ases
				JOIN attendance_shifts sh ON ases.shift_id = sh.id
				LEFT JOIN class_groups cg ON ases.class_group_id = cg.id
				WHERE ases.id = $1
			`, sessionID).Scan(&title, &shiftName, &className, &status,
				&totalStudents, &totalCheckIn)

			c.JSON(http.StatusOK, gin.H{
				"available": true,
				"session": gin.H{
					"id":             sessionID,
					"title":          title,
					"shift_name":     shiftName,
					"class_name":     className,
					"status":         status,
					"total_students": totalStudents,
					"total_check_in": totalCheckIn,
				},
				"device_label": deviceLabel,
			})
		})

		// Public endpoint untuk kiosk scan (device-code auth)
		apiPub.POST("/kiosk/:code/scan-rfid", func(c *gin.Context) {
			code := c.Param("code")

			var schoolID string
			err := database.DB.QueryRow(`
				SELECT school_id FROM attendance_devices
				WHERE device_code = $1 AND is_active = TRUE
			`, code).Scan(&schoolID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Device tidak dikenal"})
				return
			}

			var req struct {
				SessionID string `json:"session_id"`
				RFIDUID   string `json:"rfid_uid" binding:"required"`
				ForceMode string `json:"force_mode"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Format tidak valid"})
				return
			}

			uid := normalizeRFIDUID(req.RFIDUID)
			if uid == "" {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "UID kosong"})
				return
			}

			sessionID := req.SessionID
			deviceLabel := ""
			if sessionID == "" {
				var resErr error
				sessionID, deviceLabel, resErr = resolveActiveSessionForDevice(schoolID, code)
				if resErr != nil {
					c.JSON(http.StatusConflict, gin.H{
						"success": false,
						"error":   resErr.Error(),
						"code":    "no_active_session",
					})
					return
				}
			}

			// Untuk kiosk, adminID kosong (bukan admin yang scan)
			result, procErr := processAttendanceScan(
				schoolID, sessionID, "rfid", uid,
				code, deviceLabel, "", req.ForceMode,
			)
			if procErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": procErr.Error()})
				return
			}

			if !result.Success {
				status := http.StatusBadRequest
				switch result.ErrorCode {
				case "not_found":
					status = http.StatusNotFound
				case "duplicate":
					status = http.StatusConflict
				}
				c.JSON(status, gin.H{
					"success": false,
					"error":   result.ErrorMessage,
					"code":    result.ErrorCode,
					"student": gin.H{
						"full_name": result.StudentName,
						"nisn":      result.StudentNISN,
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"mode":    result.Mode,
				"status":  result.Status,
				"student": gin.H{
					"id":             result.StudentID,
					"full_name":      result.StudentName,
					"nisn":           result.StudentNISN,
					"sub_class_name": result.SubClassName,
					"photo_url":      result.PhotoURL,
				},
				"record_id":  result.RecordID,
				"scanned_at": nowJakarta(),
			})
		})

		

		// api.POST("/school-admin/attendance/scan", func(c *gin.Context) {
		// 	schoolID, err := getSchoolIDFromUser(c)
		// 	if err != nil {
		// 		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		// 		return
		// 	}
		// 	adminID, _ := getAdminIDFromUser(c)

		// 	var req ScanQRRequest
		// 	if err := c.ShouldBindJSON(&req); err != nil {
		// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		// 		return
		// 	}

		// 	// 1. Validasi session
		// 	var sessionStatus string
		// 	var shiftID string
		// 	var sessionClassGroupID, sessionClassSubGroupID sql.NullString
		// 	var checkInStart, checkInEnd time.Time
		// 	var requireCheckOut bool
		// 	var checkOutStart, checkOutEnd sql.NullTime

		// 	err = database.DB.QueryRow(`
		// 		SELECT 
		// 			ases.status, ases.shift_id,
		// 			ases.class_group_id, ases.class_sub_group_id,
		// 			sh.check_in_start, sh.check_in_end,
		// 			sh.require_check_out, sh.check_out_start, sh.check_out_end
		// 		FROM attendance_sessions ases
		// 		JOIN attendance_shifts sh ON ases.shift_id = sh.id
		// 		WHERE ases.id = $1 AND ases.school_id = $2
		// 	`, req.SessionID, schoolID).Scan(
		// 		&sessionStatus, &shiftID,
		// 		&sessionClassGroupID, &sessionClassSubGroupID,
		// 		&checkInStart, &checkInEnd,
		// 		&requireCheckOut, &checkOutStart, &checkOutEnd,
		// 	)

		// 	if err != nil {
		// 		c.JSON(http.StatusNotFound, gin.H{"error": "Sesi tidak ditemukan"})
		// 		return
		// 	}

		// 	if sessionStatus != "open" && sessionStatus != "scheduled" {
		// 		c.JSON(http.StatusConflict, gin.H{"error": "Sesi sudah ditutup"})
		// 		return
		// 	}

		// 	// Auto-open session jika masih scheduled
		// 	if sessionStatus == "scheduled" {
		// 		_, _ = database.DB.Exec(`
		// 			UPDATE attendance_sessions SET status = 'open', opened_at = NOW(), opened_by = $1
		// 			WHERE id = $2
		// 		`, adminID, req.SessionID)
		// 	}

		// 	// 2. Cari student by QR token
		// 	var studentID, studentName, studentNISN string
		// 	var studentClassGroupID, studentClassSubGroupID sql.NullString

		// 	err = database.DB.QueryRow(`
		// 		SELECT 
		// 			s.id, s.full_name, COALESCE(s.nisn, ''),
		// 			s.class_group_id, s.class_sub_group_id
		// 		FROM student_qr_codes sqc
		// 		JOIN students s ON sqc.student_id = s.id
		// 		WHERE sqc.qr_token = $1 AND sqc.school_id = $2 AND sqc.is_active = TRUE
		// 	`, req.QRToken, schoolID).Scan(
		// 		&studentID, &studentName, &studentNISN,
		// 		&studentClassGroupID, &studentClassSubGroupID,
		// 	)

		// 	if err != nil {
		// 		c.JSON(http.StatusNotFound, gin.H{
		// 			"error":   "QR tidak dikenal",
		// 			"qr_token": req.QRToken,
		// 		})
		// 		return
		// 	}

		// 	// 3. Validasi kelas
		// 	if sessionClassGroupID.Valid && studentClassGroupID.Valid {
		// 		if sessionClassGroupID.String != studentClassGroupID.String {
		// 			c.JSON(http.StatusBadRequest, gin.H{
		// 				"error":   "Siswa bukan bagian dari kelas yang di-absensi",
		// 				"student": gin.H{"full_name": studentName, "nisn": studentNISN},
		// 			})
		// 			return
		// 		}
		// 	}
		// 	if sessionClassSubGroupID.Valid && studentClassSubGroupID.Valid {
		// 		if sessionClassSubGroupID.String != studentClassSubGroupID.String {
		// 			c.JSON(http.StatusBadRequest, gin.H{
		// 				"error":   "Siswa bukan bagian dari sub kelas yang di-absensi",
		// 				"student": gin.H{"full_name": studentName, "nisn": studentNISN},
		// 			})
		// 			return
		// 		}
		// 	}

		// 	// 4. Tentukan mode: check_in atau check_out
		// 	var hasCheckIn, hasCheckOut bool
		// 	_ = database.DB.QueryRow(`
		// 		SELECT 
		// 			EXISTS(SELECT 1 FROM attendance_records WHERE session_id = $1 AND student_id = $2 AND record_type = 'check_in'),
		// 			EXISTS(SELECT 1 FROM attendance_records WHERE session_id = $1 AND student_id = $2 AND record_type = 'check_out')
		// 	`, req.SessionID, studentID).Scan(&hasCheckIn, &hasCheckOut)

		// 	// Force mode dari request (override)
		// 	mode := ""
		// 	if req.ForceMode == "check_in" || req.ForceMode == "check_out" {
		// 		mode = req.ForceMode
		// 	} else {
		// 		if !hasCheckIn {
		// 			mode = "check_in"
		// 		} else if requireCheckOut && !hasCheckOut {
		// 			mode = "check_out"
		// 		} else {
		// 			// Sudah check-in & tidak butuh check-out ATAU sudah check-out
		// 			c.JSON(http.StatusConflict, gin.H{
		// 				"duplicate": true,
		// 				"error":     "Siswa sudah tercatat sebelumnya",
		// 				"student":   gin.H{"full_name": studentName, "nisn": studentNISN},
		// 			})
		// 			return
		// 		}
		// 	}

		// 	// 5. Hitung status berdasarkan waktu
		// 	now := nowJakarta()
		// 	currentTime := time.Date(2000, 1, 1, now.Hour(), now.Minute(), now.Second(), 0, time.UTC)

		// 	var status string
		// 	var deviation int

		// 	if mode == "check_in" {
		// 		status = determineCheckInStatus(
		// 			checkInStart, checkInEnd, currentTime,
		// 		)
		// 		// Hitung deviation
		// 		if currentTime.Before(checkInStart) {
		// 			deviation = -int(checkInStart.Sub(currentTime).Minutes())
		// 		} else if currentTime.After(checkInEnd) {
		// 			deviation = int(currentTime.Sub(checkInEnd).Minutes())
		// 		}
		// 	} else {
		// 		if !checkOutStart.Valid || !checkOutEnd.Valid {
		// 			c.JSON(http.StatusBadRequest, gin.H{"error": "Shift ini tidak support check-out"})
		// 			return
		// 		}
		// 		status = determineCheckOutStatus(
		// 			checkOutStart.Time, checkOutEnd.Time, currentTime,
		// 		)
		// 	}

		// 	// 6. Insert record
		// 	var recordID string
		// 	err = database.DB.QueryRow(`
		// 		INSERT INTO attendance_records (
		// 			session_id, student_id, school_id,
		// 			record_type, status, scanned_at, scanned_by,
		// 			scan_method, deviation_minutes
		// 		) VALUES ($1, $2, $3, $4, $5, NOW(), $6, 'qr', $7)
		// 		RETURNING id
		// 	`, req.SessionID, studentID, schoolID,
		// 		mode, status, adminID, deviation).Scan(&recordID)

		// 	if err != nil {
		// 		// Kemungkinan duplikat
		// 		fmt.Printf("[SCAN] Insert error: %v\n", err)
		// 		c.JSON(http.StatusConflict, gin.H{
		// 			"duplicate": true,
		// 			"error":     "Siswa sudah tercatat di sesi ini",
		// 			"student":   gin.H{"full_name": studentName, "nisn": studentNISN},
		// 		})
		// 		return
		// 	}

		// 	// 7. Update counter di session
		// 	if mode == "check_in" {
		// 		_, _ = database.DB.Exec(`
		// 			UPDATE attendance_sessions 
		// 			SET total_check_in = total_check_in + 1,
		// 				total_on_time = CASE WHEN $2 IN ('on_time', 'early') THEN total_on_time + 1 ELSE total_on_time END,
		// 				total_late = CASE WHEN $2 IN ('late', 'very_late') THEN total_late + 1 ELSE total_late END,
		// 				updated_at = NOW()
		// 			WHERE id = $1
		// 		`, req.SessionID, status)
		// 	} else {
		// 		_, _ = database.DB.Exec(`
		// 			UPDATE attendance_sessions 
		// 			SET total_check_out = total_check_out + 1,
		// 				updated_at = NOW()
		// 			WHERE id = $1
		// 		`, req.SessionID)
		// 	}

		// 	// 8. Upsert summary
		// 	_, _ = database.DB.Exec(`
		// 		INSERT INTO student_attendance_summary 
		// 			(student_id, school_id, session_id, summary_date, 
		// 			 check_in_status, check_in_time,
		// 			 check_out_status, check_out_time,
		// 			 is_complete, is_absent)
		// 		VALUES ($1, $2, $3, CURRENT_DATE,
		// 			CASE WHEN $4 = 'check_in' THEN $5::text ELSE NULL END,
		// 			CASE WHEN $4 = 'check_in' THEN $6::time ELSE NULL END,
		// 			CASE WHEN $4 = 'check_out' THEN $5::text ELSE NULL END,
		// 			CASE WHEN $4 = 'check_out' THEN $6::time ELSE NULL END,
		// 			false, false)
		// 		ON CONFLICT (student_id, session_id) DO UPDATE SET
		// 			check_in_status = CASE WHEN $4 = 'check_in' THEN $5 ELSE student_attendance_summary.check_in_status END,
		// 			check_in_time = CASE WHEN $4 = 'check_in' THEN $6::time ELSE student_attendance_summary.check_in_time END,
		// 			check_out_status = CASE WHEN $4 = 'check_out' THEN $5 ELSE student_attendance_summary.check_out_status END,
		// 			check_out_time = CASE WHEN $4 = 'check_out' THEN $6::time ELSE student_attendance_summary.check_out_time END,
		// 			is_complete = CASE WHEN $4 = 'check_out' THEN true ELSE student_attendance_summary.is_complete END,
		// 			updated_at = NOW()
		// 	`, studentID, schoolID, req.SessionID,
		// 		mode, status, now.Format("15:04:05"))

		// 	c.JSON(http.StatusOK, gin.H{
		// 		"success": true,
		// 		"mode":    mode,
		// 		"status":  status,
		// 		"student": gin.H{
		// 			"id":              studentID,
		// 			"full_name":       studentName,
		// 			"nisn":            studentNISN,
		// 			"sub_class_name":  studentClassSubGroupID.String,
		// 		},
		// 		"scanned_at": now,
		// 		"record_id":  recordID,
		// 	})
		// })

		// ==========================================
		// TEACHER: Join School (Self-service)
		// B2C teacher bisa klaim dirinya sebagai guru di sekolah tertentu
		// ==========================================
		api.POST("/teacher/join-school", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var req JoinSchoolRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi sekolah ada & aktif
			var schoolExists bool
			var schoolName string
			err := database.DB.QueryRow(`
				SELECT COALESCE(is_active, false), school_name
				FROM schools WHERE id = $1
			`, req.SchoolID).Scan(&schoolExists, &schoolName)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sekolah tidak ditemukan"})
				return
			}

			if !schoolExists {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Sekolah sedang dinonaktifkan, tidak bisa join"})
				return
			}

			roleInSchool := req.RoleInSchool
			if roleInSchool == "" {
				roleInSchool = "teacher"
			}

			// Insert atau update membership
			var membershipID string
			err = database.DB.QueryRow(`
				INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes)
				VALUES ($1, $2, $3, TRUE, NULLIF($4, ''))
				ON CONFLICT (user_id, school_id) DO UPDATE SET
					role_in_school = EXCLUDED.role_in_school,
					is_active = TRUE,
					left_at = NULL,
					notes = EXCLUDED.notes,
					updated_at = NOW()
				RETURNING id
			`, userIDStr, req.SchoolID, roleInSchool, req.Notes).Scan(&membershipID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal bergabung dengan sekolah: " + err.Error()})
				return
			}

			// Update teacher_type menjadi hybrid/b2b
			_, _ = database.DB.Exec(`
				UPDATE profiles 
				SET teacher_type = CASE 
					WHEN teacher_type = 'b2c' THEN 'hybrid'
					WHEN teacher_type = 'b2b' THEN 'b2b'
					ELSE 'b2b'
				END,
				updated_at = NOW()
				WHERE id = $1
			`, userIDStr)

			c.JSON(http.StatusCreated, gin.H{
				"message":        fmt.Sprintf("Berhasil bergabung dengan %s", schoolName),
				"membership_id":  membershipID,
				"school_id":      req.SchoolID,
				"school_name":    schoolName,
				"role_in_school": roleInSchool,
			})
		})

		// ==========================================
		// TEACHER: Leave School (Self-service)
		// ==========================================
		api.POST("/teacher/leave-school", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			var req struct {
				SchoolID string `json:"school_id" binding:"required"`
				Reason   string `json:"reason"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Set is_active=false dan left_at (soft leave, history tetap ada)
			result, err := database.DB.Exec(`
				UPDATE school_members 
				SET is_active = FALSE, 
					left_at = NOW(), 
					notes = COALESCE(notes, '') || ' | Left: ' || COALESCE(NULLIF($1, ''), 'no reason'),
					updated_at = NOW()
				WHERE user_id = $2 AND school_id = $3 AND is_active = TRUE
			`, req.Reason, userIDStr, req.SchoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal keluar dari sekolah: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Membership tidak ditemukan atau sudah tidak aktif"})
				return
			}

			// Cek apakah masih punya membership aktif lain
			var activeMemberships int
			_ = database.DB.QueryRow(`
				SELECT COUNT(*) FROM school_members 
				WHERE user_id = $1 AND is_active = TRUE AND left_at IS NULL
			`, userIDStr).Scan(&activeMemberships)

			// Update teacher_type: kalau sudah tidak ada membership → b2c
			if activeMemberships == 0 {
				_, _ = database.DB.Exec(`
					UPDATE profiles SET teacher_type = 'b2c', updated_at = NOW()
					WHERE id = $1
				`, userIDStr)
			}

			c.JSON(http.StatusOK, gin.H{
				"message":             "Berhasil keluar dari sekolah",
				"remaining_membership": activeMemberships,
			})
		})

		// ==========================================
		// TEACHER: Get My Memberships
		// ==========================================
		api.GET("/teacher/my-schools", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			rows, err := database.DB.Query(`
				SELECT 
					sm.id, sm.school_id, s.school_name, s.npsn, s.jenjang,
					sm.role_in_school, sm.is_active, sm.joined_at, sm.left_at,
					COALESCE(s.is_active, false) AS school_active
				FROM school_members sm
				JOIN schools s ON sm.school_id = s.id
				WHERE sm.user_id = $1
				ORDER BY sm.joined_at DESC
			`, userIDStr)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type MembershipItem struct {
				ID            string     `json:"id"`
				SchoolID      string     `json:"school_id"`
				SchoolName    string     `json:"school_name"`
				NPSN          string     `json:"npsn"`
				Jenjang       string     `json:"jenjang"`
				RoleInSchool  string     `json:"role_in_school"`
				IsActive      bool       `json:"is_active"`
				SchoolActive  bool       `json:"school_active"`
				JoinedAt      time.Time  `json:"joined_at"`
				LeftAt        *time.Time `json:"left_at"`
			}

			var list []MembershipItem
			for rows.Next() {
				var m MembershipItem
				var npsn, jenjang sql.NullString
				var leftAt sql.NullTime
				if err := rows.Scan(&m.ID, &m.SchoolID, &m.SchoolName, &npsn, &jenjang, &m.RoleInSchool, &m.IsActive, &m.JoinedAt, &leftAt, &m.SchoolActive); err == nil {
					m.NPSN = npsn.String
					m.Jenjang = jenjang.String
					if leftAt.Valid {
						m.LeftAt = &leftAt.Time
					}
					list = append(list, m)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":       len(list),
				"memberships": list,
			})
		})

		// ==========================================
		// TEACHER: List semua sekolah (untuk klaim/join)
		// ==========================================
		api.GET("/teacher/available-schools", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}

			var userIDStr string
			switch v := userID.(type) {
			case string:
				userIDStr = v
			default:
				userIDStr = fmt.Sprintf("%v", v)
			}

			// Tampilkan sekolah aktif yang belum di-join user ini
			rows, err := database.DB.Query(`
				SELECT s.id, s.school_name, s.npsn, COALESCE(s.jenjang, 'SMP'), s.address
				FROM schools s
				WHERE s.is_active = TRUE
				  AND NOT EXISTS (
					SELECT 1 FROM school_members sm 
					WHERE sm.user_id = $1 
					  AND sm.school_id = s.id 
					  AND sm.is_active = TRUE
				  )
				ORDER BY s.school_name ASC
			`, userIDStr)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type SchoolItem struct {
				ID         string `json:"id"`
				SchoolName string `json:"school_name"`
				NPSN       string `json:"npsn"`
				Jenjang    string `json:"jenjang"`
				Address    string `json:"address"`
			}

			var list []SchoolItem
			for rows.Next() {
				var s SchoolItem
				var npsn, address sql.NullString
				if err := rows.Scan(&s.ID, &s.SchoolName, &npsn, &s.Jenjang, &address); err == nil {
					s.NPSN = npsn.String
					s.Address = address.String
					list = append(list, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":   len(list),
				"schools": list,
			})
		})

		// ==========================================
		// SCHOOL ADMIN: List semua guru di sekolah
		// ==========================================
		api.GET("/school-admin/teachers", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					t.id AS teacher_id,
					t.user_id,
					t.nama_lengkap AS nama_guru,
					COALESCE(t.nip, '') AS nip_guru,
					COALESCE(t.jenis_kelamin, '') AS jenis_kelamin,
					COALESCE(p.email_sekolah, '') AS email,
					COALESCE(sm.role_in_school, 'teacher') AS role_in_school,
					t.is_active,
					t.joined_at,
					sm.left_at,
					COALESCE(p.teacher_type, 'b2b') AS teacher_type
				FROM teachers t
				JOIN profiles p ON t.user_id = p.id
				LEFT JOIN school_members sm 
					ON sm.user_id = t.user_id AND sm.school_id = t.school_id
				WHERE t.school_id = $1
				ORDER BY t.is_active DESC, t.joined_at DESC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type TeacherItem struct {
				TeacherID    string     `json:"teacher_id"`    
				UserID       string     `json:"user_id"`
				NamaGuru     string     `json:"nama_guru"`
				NIPGuru      string     `json:"nip_guru"`
				JenisKelamin string     `json:"jenis_kelamin"`
				Email        string     `json:"email"`
				RoleInSchool string     `json:"role_in_school"`
				IsActive     bool       `json:"is_active"`
				TeacherType  string     `json:"teacher_type"`
				JoinedAt     time.Time  `json:"joined_at"`
				LeftAt       *time.Time `json:"left_at"`
			}

			var list []TeacherItem
			for rows.Next() {
				var t TeacherItem
				var teacherType sql.NullString
				var leftAt sql.NullTime
				if err := rows.Scan(
					&t.TeacherID, &t.UserID, &t.NamaGuru, &t.NIPGuru, &t.JenisKelamin,
					&t.Email, &t.RoleInSchool, &t.IsActive, &t.JoinedAt, &leftAt, &teacherType,
				); err == nil {
					t.TeacherType = teacherType.String
					if leftAt.Valid {
						t.LeftAt = &leftAt.Time
					}
					list = append(list, t)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":    len(list),
				"teachers": list,
			})
		})

		// SCHOOL ADMIN: Invite teacher via email
		api.POST("/school-admin/teachers/invite", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req InviteTeacherRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Cek user dengan email tersebut sudah ada di profiles
			var existingUserID string
			err = database.DB.QueryRow(`
				SELECT id FROM profiles WHERE email_sekolah = $1
			`, req.Email).Scan(&existingUserID)

			if err != nil {
				// Belum ada → buat akun baru di Supabase Auth
				supabaseURL := os.Getenv("SUPABASE_URL")
				serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
				
				supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)
				
				// Generate password default (bisa diganti user via forgot-password)
				tempPassword := randString(12)
				
				payloadAuth := map[string]any{
					"email":         req.Email,
					"password":      tempPassword,
					"email_confirm": true,
					"user_metadata": map[string]any{
						"full_name": req.FullName,
						"role":      "teacher",
					},
				}
				jsonAuthBody, _ := json.Marshal(payloadAuth)

				httpReq, _ := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("apikey", serviceRoleKey)
				httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

				client := &http.Client{Timeout: 15 * time.Second}
				resp, err := client.Do(httpReq)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke Supabase Auth"})
					return
				}
				defer resp.Body.Close()

				var authResp map[string]any
				json.NewDecoder(resp.Body).Decode(&authResp)

				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
					errMsg := "Gagal membuat akun guru di Supabase Auth"
					if msg, ok := authResp["msg"].(string); ok {
						errMsg = msg
					}
					c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
					return
				}

				existingUserID, _ = authResp["id"].(string)
				if existingUserID == "" {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase Auth"})
					return
				}

				// Insert ke profiles
				_, _ = database.DB.Exec(`
					INSERT INTO profiles (
						id, nama_guru, nip_guru, jenis_kelamin, email_sekolah, 
						token_balance, is_active, teacher_type, registration_source, updated_at
					)
					VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, 2, TRUE, 'b2b', 'invited_by_school', NOW())
					ON CONFLICT (id) DO UPDATE SET
						nama_guru = EXCLUDED.nama_guru,
						nip_guru = COALESCE(NULLIF(EXCLUDED.nip_guru, ''), profiles.nip_guru),
						jenis_kelamin = COALESCE(NULLIF(EXCLUDED.jenis_kelamin, ''), profiles.jenis_kelamin),
						updated_at = NOW()
				`, existingUserID, req.FullName, req.NIP, req.JenisKelamin, req.Email)
			}

			roleInSchool := req.RoleInSchool
			if roleInSchool == "" {
				roleInSchool = "teacher"
			}

			// Insert membership
			var membershipID string
			err = database.DB.QueryRow(`
				INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
				VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
				ON CONFLICT (user_id, school_id) DO UPDATE SET
					role_in_school = EXCLUDED.role_in_school,
					is_active = TRUE,
					left_at = NULL,
					notes = EXCLUDED.notes,
					updated_at = NOW()
				RETURNING id
			`, existingUserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan membership: " + err.Error()})
				return
			}

			// Update teacher_type jadi b2b/hybrid
			_, _ = database.DB.Exec(`
				UPDATE profiles 
				SET teacher_type = CASE 
					WHEN teacher_type = 'b2c' THEN 'hybrid'
					ELSE 'b2b'
				END,
				updated_at = NOW()
				WHERE id = $1
			`, existingUserID)

			c.JSON(http.StatusCreated, gin.H{
				"message":       "Guru berhasil ditambahkan ke sekolah",
				"user_id":       existingUserID,
				"membership_id": membershipID,
				"email":         req.Email,
			})
		})

		api.GET("/school-admin/school-roles", func(c *gin.Context) {
			// Validasi apakah user adalah admin sekolah aktif
			_, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT code, label 
				FROM school_roles 
				WHERE is_active = TRUE 
				ORDER BY sort_order ASC, label ASC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar peran: " + err.Error()})
				return
			}
			defer rows.Close()

			type RoleOption struct {
				Code  string `json:"code"`
				Label string `json:"label"`
			}

			var roles []RoleOption
			for rows.Next() {
				var r RoleOption
				if err := rows.Scan(&r.Code, &r.Label); err == nil {
					roles = append(roles, r)
				}
			}

			// Pastikan mengembalikan array kosong, bukan null, jika tidak ada data
			if roles == nil {
				roles = []RoleOption{}
			}

			c.JSON(http.StatusOK, gin.H{"roles": roles})
		})
		// ==========================================
		// SCHOOL ADMIN: Buat akun guru baru (tanpa email)
		// POST /api/school-admin/teachers
		// ==========================================
		api.POST("/school-admin/teachers", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateTeacherRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			emailLower := strings.ToLower(strings.TrimSpace(req.Email))

			// ==========================================
			// 1. Validasi: cek duplikat email di Supabase Auth
			// ==========================================
			var existingProfileID string
			err = database.DB.QueryRow(`
				SELECT id FROM profiles WHERE LOWER(email_sekolah) = $1
			`, emailLower).Scan(&existingProfileID)

			if err == nil && existingProfileID != "" {
				// Sudah ada di profiles — cek apakah sudah jadi member sekolah ini
				var alreadyMember bool
				_ = database.DB.QueryRow(`
					SELECT EXISTS(
						SELECT 1 FROM school_members 
						WHERE user_id = $1 AND school_id = $2 
						AND is_active = TRUE AND left_at IS NULL
					)
				`, existingProfileID, schoolID).Scan(&alreadyMember)

				if alreadyMember {
					c.JSON(http.StatusConflict, gin.H{
						"error": "Guru dengan email ini sudah terdaftar di sekolah Anda",
					})
					return
				}

				// Sudah ada di sistem tapi belum jadi member sekolah ini → tambahkan saja
				roleInSchool := req.RoleInSchool
				if roleInSchool == "" {
					roleInSchool = "teacher"
				}

				var membershipID string
				err = database.DB.QueryRow(`
					INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
					VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
					ON CONFLICT (user_id, school_id) DO UPDATE SET
						is_active = TRUE,
						left_at = NULL,
						role_in_school = EXCLUDED.role_in_school,
						notes = EXCLUDED.notes,
						updated_at = NOW()
					RETURNING id
				`, existingProfileID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan guru: " + err.Error()})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"message":       "Guru sudah punya akun, berhasil ditambahkan ke sekolah.",
					"user_id":       existingProfileID,
					"membership_id": membershipID,
					"email":         emailLower,
					"is_new_user":   false,
				})
				return
			}

			// ==========================================
			// 2. Belum ada → buat akun baru di Supabase Auth
			// ==========================================
			supabaseURL := os.Getenv("SUPABASE_URL")
			serviceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

			if supabaseURL == "" || serviceRoleKey == "" {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Konfigurasi Supabase belum lengkap di server",
				})
				return
			}

			supabaseAdminURL := fmt.Sprintf("%s/auth/v1/admin/users", supabaseURL)

			payloadAuth := map[string]any{
				"email":         emailLower,
				"password":      req.Password,
				"email_confirm": true, // langsung aktif, tidak perlu verifikasi
				"user_metadata": map[string]any{
					"full_name":     req.FullName,
					"role":          "teacher",
					"jenis_kelamin": req.JenisKelamin,
				},
			}
			jsonAuthBody, _ := json.Marshal(payloadAuth)

			httpReq, err := http.NewRequest("POST", supabaseAdminURL, bytes.NewBuffer(jsonAuthBody))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan request ke Supabase"})
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("apikey", serviceRoleKey)
			httpReq.Header.Set("Authorization", "Bearer "+serviceRoleKey)

			client := &http.Client{Timeout: 15 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal terhubung ke Supabase Auth"})
				return
			}
			defer resp.Body.Close()

			var authResp map[string]any
			json.NewDecoder(resp.Body).Decode(&authResp)

			// Supabase return 200/201 kalau sukses
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				errMsg := "Gagal membuat akun guru di Supabase Auth"
				if msg, ok := authResp["msg"].(string); ok {
					errMsg = msg
				} else if message, ok := authResp["message"].(string); ok {
					errMsg = message
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
				return
			}

			newUserID, ok := authResp["id"].(string)
			if !ok || newUserID == "" {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendapatkan UUID dari Supabase"})
				return
			}

			// ==========================================
			// 3. Insert ke profiles
			// ==========================================
			_, err = database.DB.Exec(`
				INSERT INTO profiles (
					id, email_sekolah, token_balance, is_active, teacher_type, 
					registration_source, updated_at
				)
				VALUES ($1, $2, 2, TRUE, 'b2b', 'created_by_school_admin', NOW())
				ON CONFLICT (id) DO UPDATE SET
					email_sekolah = EXCLUDED.email_sekolah,
					is_active = TRUE,
					updated_at = NOW()
			`, newUserID, emailLower)

			if err != nil {
				log.Printf("[CREATE TEACHER] Gagal insert profiles untuk user=%s: %v", newUserID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Akun dibuat tapi gagal menyimpan profil: " + err.Error(),
				})
				return
			}

			// ==========================================
			// 4. ✅ INSERT KE TABEL TEACHERS (BARU)
			// ==========================================
			roleInSchool := req.RoleInSchool
			if roleInSchool == "" {
				roleInSchool = "teacher"
			}

			var teacherID string
			err = database.DB.QueryRow(`
				INSERT INTO teachers (
					user_id, school_id, nama_lengkap, nip, jenis_kelamin, no_whatsapp, is_active
				)
				VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), TRUE)
				ON CONFLICT (user_id) DO UPDATE SET
					school_id = EXCLUDED.school_id,
					nama_lengkap = EXCLUDED.nama_lengkap,
					nip = COALESCE(NULLIF(EXCLUDED.nip, ''), teachers.nip),
					jenis_kelamin = COALESCE(NULLIF(EXCLUDED.jenis_kelamin, ''), teachers.jenis_kelamin),
					no_whatsapp = COALESCE(NULLIF(EXCLUDED.no_whatsapp, ''), teachers.no_whatsapp),
					is_active = TRUE,
					updated_at = NOW()
				RETURNING id
			`, newUserID, schoolID, req.FullName, req.NIP, req.JenisKelamin, req.NoWhatsApp).Scan(&teacherID)

			if err != nil {
				log.Printf("[CREATE TEACHER] Gagal insert teachers untuk user=%s: %v", newUserID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Akun dibuat tapi gagal menyimpan data guru: " + err.Error(),
				})
				return
			}

			// ==========================================
			// 5. Insert ke school_members (relasi role & audit)
			// ==========================================
			var membershipID string
			err = database.DB.QueryRow(`
				INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
				VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
				ON CONFLICT (user_id, school_id) DO UPDATE SET
					is_active = TRUE,
					left_at = NULL,
					role_in_school = EXCLUDED.role_in_school,
					notes = EXCLUDED.notes,
					updated_at = NOW()
				RETURNING id
			`, newUserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

			if err != nil {
				log.Printf("[CREATE TEACHER] Gagal insert school_members untuk user=%s: %v", newUserID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Akun & profil dibuat, tapi gagal mendaftarkan ke sekolah: " + err.Error(),
				})
				return
			}

			// ==========================================
			// 5. Success — return data guru + password untuk dikirim manual
			// ==========================================
			c.JSON(http.StatusCreated, gin.H{
				"message":       "Guru berhasil ditambahkan. Kirim email & password ke guru secara manual.",
				"user_id":       newUserID,
				"teacher_id":    teacherID,
				"membership_id": membershipID,
				"email":         emailLower,
				"password":      req.Password,      
				"full_name":     req.FullName,
				"nip":           req.NIP,
				"jenis_kelamin": req.JenisKelamin,
				"no_whatsapp":   req.NoWhatsApp,
				"role_in_school": roleInSchool,
				"is_new_user":   true,
			})
		})

		// ==========================================
		// SCHOOL ADMIN: Update Teacher Data
		api.PUT("/school-admin/teachers/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			teacherID := c.Param("id")

			// Struct khusus untuk request update (bisa ditaruh di atas bersama struct lain)
			var req struct {
				NamaGuru     string `json:"nama_guru" binding:"required"`
				NIP          string `json:"nip"`
				JenisKelamin string `json:"jenis_kelamin" binding:"omitempty,oneof=L P"`
				NoWhatsApp   string `json:"no_whatsapp"`
				RoleInSchool string `json:"role_in_school"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
				return
			}

			// Cek apakah guru ini ada dan milik sekolah yang sedang login
			var userID string
			err = database.DB.QueryRow(`
				SELECT user_id FROM teachers 
				WHERE id = $1 AND school_id = $2
			`, teacherID, schoolID).Scan(&userID)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan di sekolah ini"})
				return
			}

			// Mulai Transaksi
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Update tabel teachers
			_, err = tx.Exec(`
				UPDATE teachers 
				SET nama_lengkap = $1, nip = NULLIF($2, ''), jenis_kelamin = NULLIF($3, ''), no_whatsapp = NULLIF($4, ''), updated_at = NOW()
				WHERE id = $5 AND school_id = $6
			`, req.NamaGuru, req.NIP, req.JenisKelamin, req.NoWhatsApp, teacherID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update data guru: " + err.Error()})
				return
			}

			// 2. Update tabel profiles (opsional, agar sinkron di seluruh sistem)
			_, err = tx.Exec(`
				UPDATE profiles 
				SET nama_guru = $1, nip_guru = NULLIF($2, ''), jenis_kelamin = NULLIF($3, ''), updated_at = NOW()
				WHERE id = $4
			`, req.NamaGuru, req.NIP, req.JenisKelamin, userID)

			if err != nil {
				fmt.Printf("[UPDATE TEACHER] Gagal update profil untuk user=%s: %v\n", userID, err)
			}

			if req.RoleInSchool != "" {
				_, err = tx.Exec(`
					UPDATE school_members 
					SET role_in_school = $1, updated_at = NOW()
					WHERE user_id = $2 AND school_id = $3
				`, req.RoleInSchool, userID, schoolID)
				
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update peran guru: " + err.Error()})
					return
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan perubahan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Data guru berhasil diperbarui"})
		})

		// SCHOOL ADMIN: Assign guru existing (dari user_id)
		api.POST("/school-admin/teachers/assign", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req AssignTeacherRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi user ada di profiles
			var exists string
			err = database.DB.QueryRow(`SELECT id FROM profiles WHERE id = $1`, req.UserID).Scan(&exists)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
				return
			}

			roleInSchool := req.RoleInSchool
			if roleInSchool == "" {
				roleInSchool = "teacher"
			}

			var membershipID string
			err = database.DB.QueryRow(`
				INSERT INTO school_members (user_id, school_id, role_in_school, is_active, notes, added_by)
				VALUES ($1, $2, $3, TRUE, NULLIF($4, ''), $5)
				ON CONFLICT (user_id, school_id) DO UPDATE SET
					role_in_school = EXCLUDED.role_in_school,
					is_active = TRUE,
					left_at = NULL,
					notes = EXCLUDED.notes,
					updated_at = NOW()
				RETURNING id
			`, req.UserID, schoolID, roleInSchool, req.Notes, adminID).Scan(&membershipID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Update teacher_type
			_, _ = database.DB.Exec(`
				UPDATE profiles 
				SET teacher_type = CASE 
					WHEN teacher_type = 'b2c' THEN 'hybrid'
					ELSE 'b2b'
				END,
				updated_at = NOW()
				WHERE id = $1
			`, req.UserID)

			c.JSON(http.StatusCreated, gin.H{
				"message":       "Guru berhasil di-assign ke sekolah",
				"membership_id": membershipID,
			})
		})

		// SCHOOL ADMIN: Remove teacher (soft leave)
		api.DELETE("/school-admin/teachers/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			teacherID := c.Param("id") // ← sekarang ini teacher_id, bukan membership_id

			var req RemoveTeacherRequest
			_ = c.ShouldBindJSON(&req)

			// Cek apakah teacher_id valid & milik sekolah ini
			var userID string
			err = database.DB.QueryRow(`
				SELECT user_id FROM teachers 
				WHERE id = $1 AND school_id = $2 AND is_active = TRUE
			`, teacherID, schoolID).Scan(&userID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan di sekolah ini"})
				return
			}

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Soft-delete di tabel teachers
			_, err = tx.Exec(`
				UPDATE teachers 
				SET is_active = FALSE, updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, teacherID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Soft-delete di school_members
			_, err = tx.Exec(`
				UPDATE school_members 
				SET is_active = FALSE,
					left_at = NOW(),
					notes = COALESCE(notes, '') || ' | Removed: ' || COALESCE(NULLIF($1, ''), 'no reason'),
					updated_at = NOW()
				WHERE user_id = $2 AND school_id = $3 AND is_active = TRUE
			`, req.Reason, userID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Guru berhasil di-remove dari sekolah"})
		})

		api.GET("/school-admin/students/gender-stats", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			var totalL, totalP, totalNull int
			_ = database.DB.QueryRow(`
				SELECT 
					COUNT(*) FILTER (WHERE jenis_kelamin = 'L') AS total_l,
					COUNT(*) FILTER (WHERE jenis_kelamin = 'P') AS total_p,
					COUNT(*) FILTER (WHERE jenis_kelamin IS NULL OR jenis_kelamin = '') AS total_null
				FROM students 
				WHERE school_id = $1 AND is_active = TRUE
			`, schoolID).Scan(&totalL, &totalP, &totalNull)

			c.JSON(http.StatusOK, gin.H{
				"laki_laki": totalL,
				"perempuan": totalP,
				"belum_diisi": totalNull,
				"total": totalL + totalP + totalNull,
			})
		})

		// ==========================================
		// SCHOOL ADMIN: SUBJECTS (Mata Pelajaran)
		// ==========================================

		// -------------------- LIST --------------------
		// GET /api/school-admin/subjects
		api.GET("/school-admin/subjects", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			// Filter opsional: ?is_active=true
			isActiveFilter := c.Query("is_active")

			query := `
				SELECT 
					s.id, s.name, COALESCE(s.code, '') AS code,
					COALESCE(s.description, '') AS description,
					s.category,  
					s.is_active, s.created_at, s.updated_at,
					(
						SELECT COUNT(*) FROM teacher_subjects ts
						WHERE ts.subject_id = s.id
					) AS teacher_count
				FROM subjects s
				WHERE s.school_id = $1
			`
			args := []any{schoolID}

			switch isActiveFilter {
			case "true":
				query += " AND s.is_active = TRUE"
			case "false":
				query += " AND s.is_active = FALSE"
			}

			query += " ORDER BY s.name ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat mapel: " + err.Error()})
				return
			}
			defer rows.Close()

			type SubjectItem struct {
				ID           string    `json:"id"`
				Name         string    `json:"name"`
				Code         string    `json:"code"`
				Description  string    `json:"description"`
				Category     string    `json:"category"`   
				IsActive     bool      `json:"is_active"`
				CreatedAt    time.Time `json:"created_at"`
				UpdatedAt    time.Time `json:"updated_at"`
				TeacherCount int       `json:"teacher_count"`
			}

			list := []SubjectItem{}
			for rows.Next() {
				var s SubjectItem
				if err := rows.Scan(
					&s.ID, &s.Name, &s.Code, &s.Description,
					&s.Category, 
					&s.IsActive, &s.CreatedAt, &s.UpdatedAt, &s.TeacherCount,
				); err == nil {
					list = append(list, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":    len(list),
				"subjects": list,
			})
		})

		// -------------------- CREATE --------------------
		// POST /api/school-admin/subjects
		api.POST("/school-admin/subjects", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			var req CreateSubjectRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Cek duplikat nama di sekolah yang sama
			var existingID string
			err = database.DB.QueryRow(`
				SELECT id FROM subjects 
				WHERE school_id = $1 AND LOWER(name) = LOWER($2)
			`, schoolID, strings.TrimSpace(req.Name)).Scan(&existingID)

			if err == nil && existingID != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Mapel dengan nama tersebut sudah ada"})
				return
			}

			var subjectID string
			err = database.DB.QueryRow(`
				INSERT INTO subjects (school_id, name, code, description, category)
				VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), COALESCE(NULLIF($5, ''), 'wajib'))
				RETURNING id
			`, schoolID, strings.TrimSpace(req.Name), req.Code, req.Description, req.Category).Scan(&subjectID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan mapel: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":    "Mata pelajaran berhasil ditambahkan",
				"subject_id": subjectID,
			})
		})

		// -------------------- UPDATE --------------------
		// PUT /api/school-admin/subjects/:id
		api.PUT("/school-admin/subjects/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			subjectID := c.Param("id")

			var req UpdateSubjectRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Cek duplikat nama (kecuali dirinya sendiri)
			if req.Name != "" {
				var existingID string
				err = database.DB.QueryRow(`
					SELECT id FROM subjects 
					WHERE school_id = $1 AND LOWER(name) = LOWER($2) AND id <> $3
				`, schoolID, strings.TrimSpace(req.Name), subjectID).Scan(&existingID)

				if err == nil && existingID != "" {
					c.JSON(http.StatusConflict, gin.H{"error": "Mapel dengan nama tersebut sudah ada"})
					return
				}
			}

			result, err := database.DB.Exec(`
				UPDATE subjects SET
					name = COALESCE(NULLIF($1, ''), name),
					code = NULLIF($2, ''),
					description = NULLIF($3, ''),
					category = COALESCE(NULLIF($4, ''), category),
					is_active = COALESCE($5, is_active),
					updated_at = NOW()
				WHERE id = $6 AND school_id = $7
			`, strings.TrimSpace(req.Name), req.Code, req.Description, req.Category, req.IsActive, subjectID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update mapel: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil diperbarui"})
		})

		// -------------------- DELETE (soft) --------------------
		// DELETE /api/school-admin/subjects/:id
		api.DELETE("/school-admin/subjects/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			subjectID := c.Param("id")

			// Cek apakah masih dipakai guru
			var usedCount int
			_ = database.DB.QueryRow(`
				SELECT COUNT(*) FROM teacher_subjects WHERE subject_id = $1
			`, subjectID).Scan(&usedCount)

			if usedCount > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf(
						"Mapel ini masih diampu oleh %d guru. Hapus relasi dulu atau nonaktifkan saja.",
						usedCount,
					),
				})
				return
			}

			// Soft delete
			result, err := database.DB.Exec(`
				UPDATE subjects SET is_active = FALSE, updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, subjectID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil dinonaktifkan"})
		})

		// -------------------- DETAIL (opsional) --------------------
		// GET /api/school-admin/subjects/:id
		api.GET("/school-admin/subjects/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			subjectID := c.Param("id")

			var s struct {
				ID          string
				Name        string
				Code        sql.NullString
				Description sql.NullString
				Category    string 
				IsActive    bool
				CreatedAt   time.Time
				UpdatedAt   time.Time
			}

			err = database.DB.QueryRow(`
				SELECT id, name, code, description, category, is_active, created_at, updated_at
				FROM subjects WHERE id = $1 AND school_id = $2
			`, subjectID, schoolID).Scan(
				&s.ID, &s.Name, &s.Code, &s.Description, &s.Category,
				&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Mapel tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Ambil guru yang mengampu
			teacherRows, err := database.DB.Query(`
				SELECT 
					t.id AS teacher_id, t.nama_lengkap, 
					ts.is_primary
				FROM teacher_subjects ts
				JOIN teachers t ON ts.teacher_id = t.id
				WHERE ts.subject_id = $1 AND t.is_active = TRUE
				ORDER BY t.nama_lengkap ASC
			`, subjectID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer teacherRows.Close()

			type TeacherInfo struct {
				TeacherID   string `json:"teacher_id"`
				TeacherName string `json:"teacher_name"`
				IsPrimary   bool   `json:"is_primary"`
			}

			teachers := []TeacherInfo{}
			for teacherRows.Next() {
				var t TeacherInfo
				if err := teacherRows.Scan(&t.TeacherID, &t.TeacherName, &t.IsPrimary); err == nil {
					teachers = append(teachers, t)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"subject": gin.H{
					"id":          s.ID,
					"name":        s.Name,
					"code":        s.Code.String,
					"description": s.Description.String,
					"category":    s.Category,
					"is_active":   s.IsActive,
					"created_at":  s.CreatedAt,
					"updated_at":  s.UpdatedAt,
					"teachers":    teachers,
				},
			})
		})

		// ==========================================
		// SCHOOL ADMIN: Assign Subjects to Teacher
		// ==========================================
		api.POST("/school-admin/teachers/:id/subjects", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			teacherID := c.Param("id")

			var req AssignSubjectsToTeacherRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi teacher milik sekolah ini
			var exists bool
			err = database.DB.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM teachers WHERE id = $1 AND school_id = $2
				)
			`, teacherID, schoolID).Scan(&exists)
			if err != nil || !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan"})
				return
			}

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// Hapus relasi lama (replace all)
			_, _ = tx.Exec(`DELETE FROM teacher_subjects WHERE teacher_id = $1`, teacherID)

			// Insert relasi baru
			insertedCount := 0
			for _, subjectID := range req.SubjectIDs {
				// Validasi subject milik sekolah ini
				var validSchoolID string
				err := tx.QueryRow(`
					SELECT school_id FROM subjects WHERE id = $1 AND is_active = TRUE
				`, subjectID).Scan(&validSchoolID)
				if err != nil || validSchoolID != schoolID {
					log.Printf("[ASSIGN SUBJECTS] Skip subject %s: tidak valid", subjectID)
					continue
				}

				isPrimary := (subjectID == req.IsPrimary)

				_, err = tx.Exec(`
					INSERT INTO teacher_subjects 
						(teacher_id, subject_id, school_id, is_primary, assigned_by)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (teacher_id, subject_id) DO UPDATE SET
						is_primary = EXCLUDED.is_primary,
						assigned_at = NOW()
				`, teacherID, subjectID, schoolID, isPrimary, adminID)

				if err == nil {
					insertedCount++
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":        fmt.Sprintf("%d mapel berhasil di-assign ke guru", insertedCount),
				"assigned_count": insertedCount,
			})
		})

		// GET /api/school-admin/teachers/:teacherId/subjects
		// Lihat mapel yang diampu 1 guru
		api.GET("/school-admin/teachers/:id/subjects", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			teacherID := c.Param("id")

			// Validasi teacher
			var teacherName string
			err = database.DB.QueryRow(`
				SELECT nama_lengkap FROM teachers 
				WHERE id = $1 AND school_id = $2
			`, teacherID, schoolID).Scan(&teacherName)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					s.id, s.name, COALESCE(s.code, '') AS code,
					ts.is_primary, ts.assigned_at
				FROM teacher_subjects ts
				JOIN subjects s ON ts.subject_id = s.id
				WHERE ts.teacher_id = $1
				ORDER BY ts.is_primary DESC, s.name ASC
			`, teacherID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type SubjectAssignment struct {
				SubjectID    string    `json:"subject_id"`
				SubjectName  string    `json:"subject_name"`
				SubjectCode  string    `json:"subject_code"`
				IsPrimary    bool      `json:"is_primary"`
				AssignedAt   time.Time `json:"assigned_at"`
			}

			list := []SubjectAssignment{}
			for rows.Next() {
				var s SubjectAssignment
				if err := rows.Scan(
					&s.SubjectID, &s.SubjectName, &s.SubjectCode,
					&s.IsPrimary, &s.AssignedAt,
				); err == nil {
					list = append(list, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"teacher_id":   teacherID,
				"teacher_name": teacherName,
				"total":        len(list),
				"subjects":     list,
			})
		})

		// Hapus 1 mapel dari guru
		api.DELETE("/school-admin/teachers/:id/subjects/:subjectId", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			teacherID := c.Param("id")
			subjectID := c.Param("subjectId")

			result, err := database.DB.Exec(`
				DELETE FROM teacher_subjects 
				WHERE teacher_id = $1 AND subject_id = $2 AND school_id = $3
			`, teacherID, subjectID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Relasi tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Mapel berhasil dihapus dari guru"})
		})

		// --- ENDPOINT: List Timetables ---
		// GET /api/school-admin/timetables
		api.GET("/school-admin/timetables", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			// Filter opsional
			classSubGroupID := c.Query("class_sub_group_id")
			academicYearID := c.Query("academic_year_id")

			query := `
				SELECT 
					ct.id, ct.class_group_id, ct.class_sub_group_id, ct.academic_year_id,
					ct.semester, ct.status, ct.effective_from, ct.effective_to, ct.notes,
					ct.created_at, ct.updated_at,
					COALESCE(cg.name, '') AS class_group_name,
					COALESCE(csg.name, '') AS class_sub_group_name,
					COALESCE(say.name, '') AS academic_year_name
				FROM class_timetables ct
				LEFT JOIN class_groups cg ON ct.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON ct.class_sub_group_id = csg.id
				LEFT JOIN school_academic_years say ON ct.academic_year_id = say.id
				WHERE ct.school_id = $1
			`
			args := []any{schoolID}
			argIdx := 2

			if classSubGroupID != "" {
				query += fmt.Sprintf(" AND ct.class_sub_group_id = $%d", argIdx)
				args = append(args, classSubGroupID)
				argIdx++
			}
			if academicYearID != "" {
				query += fmt.Sprintf(" AND ct.academic_year_id = $%d", argIdx)
				args = append(args, academicYearID)
				argIdx++
			}

			query += " ORDER BY ct.created_at DESC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat jadwal: " + err.Error()})
				return
			}
			defer rows.Close()

			type TimetableItem struct {
				ID                   string     `json:"id"`
				ClassGroupID         *string    `json:"class_group_id"`
				ClassSubGroupID      string     `json:"class_sub_group_id"`
				AcademicYearID       string     `json:"academic_year_id"`
				Semester             string     `json:"semester"`
				Status               string     `json:"status"`
				EffectiveFrom        *string    `json:"effective_from"`
				EffectiveTo          *string    `json:"effective_to"`
				Notes                string     `json:"notes"`
				CreatedAt            time.Time  `json:"created_at"`
				UpdatedAt            time.Time  `json:"updated_at"`
				ClassGroupName       string     `json:"class_group_name"`
				ClassSubGroupName    string     `json:"class_sub_group_name"`
				AcademicYearName     string     `json:"academic_year_name"`
			}

			list := []TimetableItem{}
			for rows.Next() {
				var t TimetableItem
				var cgID sql.NullString
				var effFrom, effTo sql.NullTime
				var notes sql.NullString

				err := rows.Scan(
					&t.ID, &cgID, &t.ClassSubGroupID, &t.AcademicYearID,
					&t.Semester, &t.Status, &effFrom, &effTo, &notes,
					&t.CreatedAt, &t.UpdatedAt,
					&t.ClassGroupName, &t.ClassSubGroupName, &t.AcademicYearName,
				)
				if err != nil {
					continue
				}
				if cgID.Valid { t.ClassGroupID = &cgID.String }
				if effFrom.Valid { s := effFrom.Time.Format("2006-01-02"); t.EffectiveFrom = &s }
				if effTo.Valid { s := effTo.Time.Format("2006-01-02"); t.EffectiveTo = &s }
				t.Notes = notes.String

				list = append(list, t)
			}

			c.JSON(http.StatusOK, gin.H{"total": len(list), "timetables": list})
		})

		// --- ENDPOINT: Create Timetable (Header) ---
		// POST /api/school-admin/timetables
		api.POST("/school-admin/timetables", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateTimetableRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi sub-kelas milik sekolah ini
			var exists bool
			err = database.DB.QueryRow(`
				SELECT EXISTS(SELECT 1 FROM class_sub_groups WHERE id = $1 AND school_id = $2)
			`, req.ClassSubGroupID, schoolID).Scan(&exists)
			if err != nil || !exists {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak valid"})
				return
			}

			// Insert
			var timetableID string
			err = database.DB.QueryRow(`
				INSERT INTO class_timetables (
					school_id, class_group_id, class_sub_group_id, academic_year_id,
					semester, curriculum_structure_id, status, effective_from, effective_to, notes, created_by
				) VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,'')::uuid, 'draft', 
						NULLIF($7,'')::date, NULLIF($8,'')::date, NULLIF($9,''), $10)
				RETURNING id
			`, schoolID, req.ClassGroupID, req.ClassSubGroupID, req.AcademicYearID,
				req.Semester, req.CurriculumStructureID, req.EffectiveFrom, req.EffectiveTo, req.Notes, adminID).Scan(&timetableID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jadwal: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Jadwal berhasil dibuat", "id": timetableID})
		})

		// --- ENDPOINT: Get Detail Timetable (dengan Entries) ---
		api.GET("/school-admin/timetables/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")

			// ==========================================
			// 1. Ambil header
			// ==========================================
			var t struct {
				ID              string
				ClassSubGroupID string
				AcademicYearID  string
				Semester        string
				Status          string
				Notes           sql.NullString
			}
			err = database.DB.QueryRow(`
				SELECT id, class_sub_group_id, academic_year_id, semester, status, notes
				FROM class_timetables WHERE id = $1 AND school_id = $2
			`, timetableID, schoolID).Scan(
				&t.ID, &t.ClassSubGroupID, &t.AcademicYearID,
				&t.Semester, &t.Status, &t.Notes,
			)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			// ==========================================
			// 2. Ambil entries
			// ==========================================
			rows, err := database.DB.Query(`
				SELECT 
					te.id, te.day_of_week, te.period_start, te.period_end,
					te.subject_id, te.subject_name_snapshot, te.teacher_id,
					COALESCE(t.nama_lengkap, '') AS teacher_name,
					COALESCE(te.room, ''), COALESCE(te.notes, '')
				FROM timetable_entries te
				LEFT JOIN teachers t ON te.teacher_id = t.id
				WHERE te.timetable_id = $1
				ORDER BY te.day_of_week ASC, te.period_start ASC
			`, timetableID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type EntryItem struct {
				ID                  string  `json:"id"`
				DayOfWeek           int     `json:"day_of_week"`
				PeriodStart         int     `json:"period_start"`
				PeriodEnd           int     `json:"period_end"`
				SubjectID           *string `json:"subject_id"`
				SubjectNameSnapshot string  `json:"subject_name_snapshot"`
				TeacherID           *string `json:"teacher_id"`
				TeacherName         string  `json:"teacher_name"`
				Room                string  `json:"room"`
				Notes               string  `json:"notes"`
			}

			entries := []EntryItem{}
			for rows.Next() {
				var e EntryItem
				var subID, teacherID sql.NullString
				if err := rows.Scan(
					&e.ID, &e.DayOfWeek, &e.PeriodStart, &e.PeriodEnd,
					&subID, &e.SubjectNameSnapshot, &teacherID, &e.TeacherName,
					&e.Room, &e.Notes,
				); err == nil {
					if subID.Valid {
						e.SubjectID = &subID.String
					}
					if teacherID.Valid {
						e.TeacherID = &teacherID.String
					}
					entries = append(entries, e)
				}
			}

			// ==========================================
			// 3. Ambil time_slots DEFAULT
			// ==========================================
			type TimeSlotItem struct {
				PeriodNumber int    `json:"period_number"`
				StartTime    string `json:"start_time"`
				EndTime      string `json:"end_time"`
				IsBreak      bool   `json:"is_break"`
				IsOverride   bool   `json:"is_override"`
			}

			defaultMap := map[int]TimeSlotItem{}

			defaultSlotRows, err := database.DB.Query(`
				SELECT period_number, start_time, end_time, is_break
				FROM school_time_slots
				WHERE school_id = $1
				ORDER BY period_number ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer defaultSlotRows.Close()

			for defaultSlotRows.Next() {
				var s TimeSlotItem
				var start, end time.Time
				if err := defaultSlotRows.Scan(&s.PeriodNumber, &start, &end, &s.IsBreak); err == nil {
					s.StartTime = start.Format("15:04")
					s.EndTime = end.Format("15:04")
					s.IsOverride = false
					defaultMap[s.PeriodNumber] = s
				}
			}

			// ==========================================
			// 4. Ambil OVERRIDE per hari
			// ==========================================
			type overrideKey struct {
				Day    int
				Period int
			}
			overrideMap := map[overrideKey]TimeSlotItem{}

			overrideRows, err := database.DB.Query(`
				SELECT day_of_week, period_number, start_time, end_time, is_break
				FROM school_time_slot_overrides
				WHERE school_id = $1
			`, schoolID)
			// Sengaja tidak return error kalau tabel belum ada / kosong —
			// anggap saja tidak ada override
			if err == nil {
				defer overrideRows.Close()
				for overrideRows.Next() {
					var day, period int
					var start, end time.Time
					var isBreak bool
					if err := overrideRows.Scan(&day, &period, &start, &end, &isBreak); err == nil {
						overrideMap[overrideKey{Day: day, Period: period}] = TimeSlotItem{
							PeriodNumber: period,
							StartTime:    start.Format("15:04"),
							EndTime:      end.Format("15:04"),
							IsBreak:      isBreak,
							IsOverride:   true,
						}
					}
				}
			}

			// ==========================================
			// 5. Bangun list default (flat, backward-compat)
			// ==========================================
			defaultList := []TimeSlotItem{}
			for period := 1; ; period++ {
				if s, ok := defaultMap[period]; ok {
					defaultList = append(defaultList, s)
				} else {
					// stop kalau sudah tidak ada period berikutnya
					if _, hasNext := defaultMap[period+1]; !hasNext {
						break
					}
				}
			}

			// ==========================================
			// 6. Bangun time_slots_by_day (merge override > default)
			//    key: "1".."7" (string, biar aman untuk JSON object)
			// ==========================================
			timeSlotsByDay := map[string][]TimeSlotItem{}

			for day := 1; day <= 7; day++ {
				daySlots := []TimeSlotItem{}
				for period := 1; ; period++ {
					defSlot, hasDefault := defaultMap[period]

					ov, hasOverride := overrideMap[overrideKey{Day: day, Period: period}]

					if hasOverride {
						daySlots = append(daySlots, ov)
						continue
					}
					if hasDefault {
						daySlots = append(daySlots, defSlot)
						continue
					}

					// kalau tidak ada di period ini, cek apakah masih ada period berikutnya
					_, hasNextDefault := defaultMap[period+1]
					_, hasNextOverride := overrideMap[overrideKey{Day: day, Period: period + 1}]
					if !hasNextDefault && !hasNextOverride {
						break
					}
				}
				timeSlotsByDay[strconv.Itoa(day)] = daySlots
			}

			// ==========================================
			// 7. Response
			// ==========================================
			c.JSON(http.StatusOK, gin.H{
				"timetable": gin.H{
					"id":                 t.ID,
					"class_sub_group_id": t.ClassSubGroupID,
					"academic_year_id":   t.AcademicYearID,
					"semester":           t.Semester,
					"status":             t.Status,
					"notes":              t.Notes.String,
				},
				"entries":           entries,
				"time_slots":        defaultList,     // backward compat
				"time_slots_by_day": timeSlotsByDay,  // baru
			})
		})
		// --- ENDPOINT: Delete Timetable (Header) ---
		api.DELETE("/school-admin/timetables/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")

			// Karena di skema database Anda, tabel `timetable_entries` memiliki
			// constraint `ON DELETE CASCADE` ke `class_timetables`, 
			// maka menghapus header otomatis akan menghapus semua baris jadwal di dalamnya.
			result, err := database.DB.Exec(`
				DELETE FROM class_timetables 
				WHERE id = $1 AND school_id = $2
			`, timetableID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus jadwal: " + err.Error()})
				return
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan atau bukan milik sekolah Anda"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Jadwal beserta seluruh baris pelajaran berhasil dihapus"})
		})

		// --- ENDPOINT: Activate Timetable ---
		api.PATCH("/school-admin/timetables/:id/activate", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")

			var currentStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM class_timetables WHERE id = $1 AND school_id = $2
			`, timetableID, schoolID).Scan(&currentStatus)

			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			if currentStatus == "active" {
				c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah aktif"})
				return
			}

	
			var entryCount int
			_ = database.DB.QueryRow(`SELECT COUNT(*) FROM timetable_entries WHERE timetable_id = $1`, timetableID).Scan(&entryCount)
			
			// (Abaikan validasi ini jika Anda ingin mengizinkan jadwal kosong diaktifkan)
			// if entryCount == 0 {
			// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak bisa mengaktifkan jadwal yang masih kosong"})
			// 	return
			// }

			// 3. (PENTING) Nonaktifkan jadwal lain di sub-kelas dan tahun akademik yang sama
			// Karena 1 kelas hanya boleh punya 1 jadwal aktif di semester yang sama.
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
				return
			}
			defer tx.Rollback()

			var subGroupID, academicYearID, semester string
			_ = tx.QueryRow(`SELECT class_sub_group_id, academic_year_id, semester FROM class_timetables WHERE id = $1`, timetableID).Scan(&subGroupID, &academicYearID, &semester)

			_, err = tx.Exec(`
				UPDATE class_timetables 
				SET status = 'archived', updated_at = NOW()
				WHERE class_sub_group_id = $1 AND academic_year_id = $2 AND semester = $3 AND id <> $4
			`, subGroupID, academicYearID, semester, timetableID)
			
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengarsipkan jadwal lama: " + err.Error()})
				return
			}

			// 4. Aktifkan jadwal ini
			_, err = tx.Exec(`
				UPDATE class_timetables 
				SET status = 'active', updated_at = NOW()
				WHERE id = $1
			`, timetableID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan jadwal: " + err.Error()})
				return
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Jadwal berhasil diaktifkan"})
		})

		// --- ENDPOINT: Create Entry (Baris Jadwal) ---
		api.POST("/school-admin/timetables/:id/entries", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")

			// Validasi timetable milik sekolah
			var exists bool
			_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM class_timetables WHERE id = $1 AND school_id = $2)`, timetableID, schoolID).Scan(&exists)
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			var req CreateTimetableEntryRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Ambil subject name untuk snapshot
			var subjectName string
			if req.SubjectID != "" {
				_ = database.DB.QueryRow(`SELECT name FROM subjects WHERE id = $1`).Scan(&subjectName)
			}

			var entryID string
			err = database.DB.QueryRow(`
				INSERT INTO timetable_entries (
					timetable_id, day_of_week, period_start, period_end,
					subject_id, subject_name_snapshot, teacher_id, room, notes
				) VALUES ($1, $2, $3, $4, NULLIF($5,'')::uuid, $6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,''))
				RETURNING id
			`, timetableID, req.DayOfWeek, req.PeriodStart, req.PeriodEnd,
				req.SubjectID, subjectName, req.TeacherID, req.Room, req.Notes).Scan(&entryID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan baris jadwal: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Baris jadwal berhasil ditambahkan", "id": entryID})
		})

		// --- ENDPOINT: Delete Entry ---
		api.DELETE("/school-admin/timetables/:id/entries/:entryId", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")
			entryID := c.Param("entryId")

			// Pastikan entry ini milik timetable yang memang milik sekolah
			result, err := database.DB.Exec(`
				DELETE FROM timetable_entries 
				WHERE id = $1 AND timetable_id = $2 
				AND timetable_id IN (SELECT id FROM class_timetables WHERE school_id = $3)
			`, entryID, timetableID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Baris jadwal tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Baris jadwal berhasil dihapus"})
		})

		// --- ENDPOINT: Validate Timetable (Sederhana v1) ---
		api.POST("/school-admin/timetables/:id/validate", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			timetableID := c.Param("id")

			// Validasi 1: Cek bentrok guru (guru yang sama di jam yang sama di kelas lain)
			// Validasi 2: Cek bentrok ruangan (opsional)
			// Untuk MVP, kita hanya cek bentrok guru di sekolah yang sama.

			rows, err := database.DB.Query(`
				SELECT te.day_of_week, te.period_start, te.period_end, te.teacher_id, t.nama_lengkap
				FROM timetable_entries te
				JOIN teachers t ON te.teacher_id = t.id            
				WHERE te.timetable_id = $1 AND te.teacher_id IS NOT NULL
			`, timetableID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type Conflict struct {
				Day        int    `json:"day"`
				Period     string `json:"period"`
				Teacher    string `json:"teacher"`
				Message    string `json:"message"`
			}
			conflicts := []Conflict{}

			for rows.Next() {
				var day, start, end int
				var teacherID, teacherName string
				if err := rows.Scan(&day, &start, &end, &teacherID, &teacherName); err != nil {
					continue
				}

				// Cari di timetable LAIN (sekolah sama) yang punya guru sama & jam overlap
				var conflictCount int
				_ = database.DB.QueryRow(`
					SELECT COUNT(*) FROM timetable_entries te2
					JOIN class_timetables ct2 ON te2.timetable_id = ct2.id
					WHERE ct2.school_id = $1
					AND te2.timetable_id <> $2
					AND te2.teacher_id = $3
					AND te2.day_of_week = $4
					AND te2.period_start <= $5  -- end jam baru
					AND te2.period_end >= $6    -- start jam baru
				`, schoolID, timetableID, teacherID, day, end, start).Scan(&conflictCount)

				if conflictCount > 0 {
					conflicts = append(conflicts, Conflict{
						Day:     day,
						Period:  fmt.Sprintf("%d-%d", start, end),
						Teacher: teacherName,
						Message: fmt.Sprintf("Guru %s bentrok dengan jadwal lain di jam ini", teacherName),
					})
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"valid":     len(conflicts) == 0,
				"conflicts": conflicts,
			})
		})

		// ==========================================
		// SCHOOL ADMIN: TIME SLOTS (Master Jam)
		// ==========================================
		api.GET("/school-admin/time-slots", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT id, period_number, start_time, end_time, is_break
				FROM school_time_slots 
				WHERE school_id = $1 
				ORDER BY period_number ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat master jam: " + err.Error()})
				return
			}
			defer rows.Close()

			type TimeSlotItem struct {
				ID           string `json:"id"`
				PeriodNumber int    `json:"period_number"`
				StartTime    string `json:"start_time"`
				EndTime      string `json:"end_time"`
				IsBreak      bool   `json:"is_break"`
			}

			list := []TimeSlotItem{}
			for rows.Next() {
				var s TimeSlotItem
				var start, end time.Time
				if err := rows.Scan(&s.ID, &s.PeriodNumber, &start, &end, &s.IsBreak); err == nil {
					s.StartTime = start.Format("15:04")
					s.EndTime = end.Format("15:04")
					list = append(list, s)
				}
			}

			c.JSON(http.StatusOK, gin.H{"time_slots": list})
		})

		api.POST("/school-admin/time-slots", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			var req CreateTimeSlotRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi bentrok period_number
			var exists bool
			_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM school_time_slots WHERE school_id = $1 AND period_number = $2)`, schoolID, req.PeriodNumber).Scan(&exists)
			if exists {
				c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Jam ke-%d sudah didefinisikan", req.PeriodNumber)})
				return
			}

			var slotID string
			err = database.DB.QueryRow(`
				INSERT INTO school_time_slots (school_id, period_number, start_time, end_time, is_break)
				VALUES ($1, $2, $3, $4, $5) RETURNING id
			`, schoolID, req.PeriodNumber, req.StartTime, req.EndTime, req.IsBreak).Scan(&slotID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan master jam: " + err.Error()})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "Master jam berhasil ditambahkan", "id": slotID})
		})

		api.PUT("/school-admin/time-slots/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			slotID := c.Param("id")

			var req UpdateTimeSlotRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Jika period_number diubah, validasi bentrok
			if req.PeriodNumber > 0 {
				var exists bool
				_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM school_time_slots WHERE school_id = $1 AND period_number = $2 AND id <> $3)`, schoolID, req.PeriodNumber, slotID).Scan(&exists)
				if exists {
					c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Jam ke-%d sudah didefinisikan", req.PeriodNumber)})
					return
				}
			}

			query := `UPDATE school_time_slots SET updated_at = NOW()`
			args := []any{}
			argIdx := 1

			if req.PeriodNumber > 0 {
				query += fmt.Sprintf(", period_number = $%d", argIdx)
				args = append(args, req.PeriodNumber); argIdx++
			}
			if req.StartTime != "" {
				query += fmt.Sprintf(", start_time = $%d", argIdx)
				args = append(args, req.StartTime); argIdx++
			}
			if req.EndTime != "" {
				query += fmt.Sprintf(", end_time = $%d", argIdx)
				args = append(args, req.EndTime); argIdx++
			}
			if req.IsBreak != nil {
				query += fmt.Sprintf(", is_break = $%d", argIdx)
				args = append(args, *req.IsBreak); argIdx++
			}

			if len(args) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada data yang diupdate"})
				return
			}

			query += fmt.Sprintf(" WHERE id = $%d AND school_id = $%d", argIdx, argIdx+1)
			args = append(args, slotID, schoolID)

			result, err := database.DB.Exec(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui master jam: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Master jam tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Master jam berhasil diperbarui"})
		})

		api.DELETE("/school-admin/time-slots/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			slotID := c.Param("id")

			result, err := database.DB.Exec(`DELETE FROM school_time_slots WHERE id = $1 AND school_id = $2`, slotID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus master jam: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Master jam tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Master jam berhasil dihapus"})
		})

	// ==========================================
	// SCHOOL ADMIN: TIME SLOT OVERRIDES (per hari)
	// ==========================================

	// GET /api/school-admin/time-slot-overrides?day_of_week=5
	api.GET("/school-admin/time-slot-overrides", func(c *gin.Context) {
		schoolID, err := getSchoolIDFromUser(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		dayFilter := c.Query("day_of_week") // opsional

		query := `
			SELECT id, day_of_week, period_number,
				start_time, end_time, is_break,
				COALESCE(notes, ''), created_at, updated_at
			FROM school_time_slot_overrides
			WHERE school_id = $1
		`
		args := []any{schoolID}

		if dayFilter != "" {
			query += " AND day_of_week = $2"
			args = append(args, dayFilter)
		}

		query += " ORDER BY day_of_week ASC, period_number ASC"

		rows, err := database.DB.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal memuat override: " + err.Error(),
			})
			return
		}
		defer rows.Close()

		type OverrideItem struct {
			ID           string    `json:"id"`
			DayOfWeek    int       `json:"day_of_week"`
			PeriodNumber int       `json:"period_number"`
			StartTime    string    `json:"start_time"`
			EndTime      string    `json:"end_time"`
			IsBreak      bool      `json:"is_break"`
			Notes        string    `json:"notes"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
		}

		list := []OverrideItem{}
		for rows.Next() {
			var o OverrideItem
			var start, end time.Time
			if err := rows.Scan(
				&o.ID, &o.DayOfWeek, &o.PeriodNumber,
				&start, &end, &o.IsBreak,
				&o.Notes, &o.CreatedAt, &o.UpdatedAt,
			); err == nil {
				o.StartTime = start.Format("15:04")
				o.EndTime = end.Format("15:04")
				list = append(list, o)
			}
		}

		c.JSON(http.StatusOK, gin.H{"overrides": list, "total": len(list)})
	})

	// POST /api/school-admin/time-slot-overrides
	// Upsert: kalau (school, day, period) sudah ada → update
	api.POST("/school-admin/time-slot-overrides", func(c *gin.Context) {
		schoolID, err := getSchoolIDFromUser(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		var req UpsertTimeSlotOverrideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Format tidak valid: " + err.Error(),
			})
			return
		}

		// Validasi: period_number ini harus ada di default slots
		// (biar tidak override JP yang tidak ada)
		var defaultExists bool
		_ = database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM school_time_slots
				WHERE school_id = $1 AND period_number = $2
			)
		`, schoolID, req.PeriodNumber).Scan(&defaultExists)

		if !defaultExists {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf(
					"Jam ke-%d belum ada di master default. Buat default dulu sebelum override.",
					req.PeriodNumber,
				),
			})
			return
		}

		// Validasi jam
		startT, err1 := parseTimeString(req.StartTime)
		endT, err2 := parseTimeString(req.EndTime)
		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format jam tidak valid"})
			return
		}
		if timeToMinutes(endT) <= timeToMinutes(startT) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Jam selesai harus lebih besar dari jam mulai",
			})
			return
		}

		var overrideID string
		err = database.DB.QueryRow(`
			INSERT INTO school_time_slot_overrides
				(school_id, day_of_week, period_number,
				start_time, end_time, is_break, notes)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
			ON CONFLICT (school_id, day_of_week, period_number)
			DO UPDATE SET
				start_time = EXCLUDED.start_time,
				end_time = EXCLUDED.end_time,
				is_break = EXCLUDED.is_break,
				notes = EXCLUDED.notes,
				updated_at = NOW()
			RETURNING id
		`, schoolID, req.DayOfWeek, req.PeriodNumber,
			req.StartTime, req.EndTime, req.IsBreak, req.Notes).Scan(&overrideID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal menyimpan override: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":     "Override berhasil disimpan",
			"override_id": overrideID,
		})
	})

	// PUT /api/school-admin/time-slot-overrides/:id
	api.PUT("/school-admin/time-slot-overrides/:id", func(c *gin.Context) {
		schoolID, err := getSchoolIDFromUser(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		overrideID := c.Param("id")

		var req UpsertTimeSlotOverrideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
			return
		}

		result, err := database.DB.Exec(`
			UPDATE school_time_slot_overrides SET
				start_time = $1,
				end_time   = $2,
				is_break   = $3,
				notes      = NULLIF($4, ''),
				updated_at = NOW()
			WHERE id = $5 AND school_id = $6
		`, req.StartTime, req.EndTime, req.IsBreak, req.Notes,
			overrideID, schoolID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Override tidak ditemukan"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Override berhasil diperbarui"})
	})

	// DELETE /api/school-admin/time-slot-overrides/:id
	api.DELETE("/school-admin/time-slot-overrides/:id", func(c *gin.Context) {
		schoolID, err := getSchoolIDFromUser(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		result, err := database.DB.Exec(`
			DELETE FROM school_time_slot_overrides
			WHERE id = $1 AND school_id = $2
		`, c.Param("id"), schoolID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Override tidak ditemukan"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Override berhasil dihapus"})
	})

		// SCHOOL ADMIN: SCHOOL EXAMS (CRUD)
		// -------------------- LIST --------------------
		// GET /api/school-admin/exams?status=draft&subject=Matematika&search=uts
		api.GET("/school-admin/exams", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			status := c.Query("status")
			subject := c.Query("subject")
			search := c.Query("search")
			examType := c.Query("exam_type")

			query := `
				SELECT 
					e.id, e.title, COALESCE(e.description, ''), e.subject, e.exam_type,
					COALESCE(e.grade_level, ''), COALESCE(e.phase, ''),
					e.duration_minutes, e.total_questions, e.total_score, 
					COALESCE(e.passing_score, 0),
					e.status, e.is_active,
					e.created_at, e.updated_at, e.published_at,
					COALESCE(say.name, '') AS academic_year_name,
					COALESCE(say.semester, '') AS semester,
					COALESCE(
						(SELECT json_agg(json_build_object(
							'id', et.id,
							'class_group_id', et.class_group_id,
							'class_sub_group_id', et.class_sub_group_id,
							'class_group_name', cg.name,
							'class_sub_group_name', csg.name
						))
						FROM exam_targets et
						LEFT JOIN class_groups cg ON et.class_group_id = cg.id
						LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
						WHERE et.exam_id = e.id),
						'[]'::json
					) AS targets
				FROM school_exams e
				LEFT JOIN school_academic_years say ON e.academic_year_id = say.id
				WHERE e.school_id = $1 AND e.deleted_at IS NULL
			`
			args := []any{schoolID}
			argIdx := 2

			if status != "" {
				query += fmt.Sprintf(" AND e.status = $%d", argIdx)
				args = append(args, status)
				argIdx++
			}
			if subject != "" {
				query += fmt.Sprintf(" AND e.subject = $%d", argIdx)
				args = append(args, subject)
				argIdx++
			}
			if examType != "" {
				query += fmt.Sprintf(" AND e.exam_type = $%d", argIdx)
				args = append(args, examType)
				argIdx++
			}
			if search != "" {
				query += fmt.Sprintf(" AND (e.title ILIKE $%d OR e.description ILIKE $%d)", argIdx, argIdx)
				args = append(args, "%"+search+"%")
				argIdx++
			}

			query += " ORDER BY e.created_at DESC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar ujian: " + err.Error()})
				return
			}
			defer rows.Close()

			type ExamListItem struct {
				ID                string          `json:"id"`
				Title             string          `json:"title"`
				Description       string          `json:"description"`
				Subject           string          `json:"subject"`
				ExamType          string          `json:"exam_type"`
				GradeLevel        string          `json:"grade_level"`
				Phase             string          `json:"phase"`
				DurationMinutes   int             `json:"duration_minutes"`
				TotalQuestions    int             `json:"total_questions"`
				TotalScore        float64         `json:"total_score"`
				PassingScore      float64         `json:"passing_score"`
				Status            string          `json:"status"`
				IsActive          bool            `json:"is_active"`
				CreatedAt         time.Time       `json:"created_at"`
				UpdatedAt         time.Time       `json:"updated_at"`
				PublishedAt       *time.Time      `json:"published_at"`
				AcademicYearName  string          `json:"academic_year_name"`
				Semester          string          `json:"semester"`
				Targets           json.RawMessage `json:"targets"`
			}

			var exams []ExamListItem
			for rows.Next() {
				var e ExamListItem
				var desc, gradeLevel, phase, ayName, semester sql.NullString
				var passingScore sql.NullFloat64
				var publishedAt sql.NullTime
				var targetsJSON []byte

				err := rows.Scan(
					&e.ID, &e.Title, &desc, &e.Subject, &e.ExamType,
					&gradeLevel, &phase,
					&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &passingScore,
					&e.Status, &e.IsActive,
					&e.CreatedAt, &e.UpdatedAt, &publishedAt,
					&ayName, &semester, &targetsJSON,
				)
				if err != nil {
					continue
				}

				e.Description = desc.String
				e.GradeLevel = gradeLevel.String
				e.Phase = phase.String
				e.AcademicYearName = ayName.String
				e.Semester = semester.String
				if passingScore.Valid {
					e.PassingScore = passingScore.Float64
				}
				if publishedAt.Valid {
					e.PublishedAt = &publishedAt.Time
				}
				e.Targets = json.RawMessage(targetsJSON)

				exams = append(exams, e)
			}

			c.JSON(http.StatusOK, gin.H{
				"total": len(exams),
				"exams": exams,
			})
		})

		// -------------------- DETAIL --------------------
		// GET /api/school-admin/exams/:id
		api.GET("/school-admin/exams/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Param("id")

			var e struct {
				ID                string
				Title             string
				Description       sql.NullString
				Subject           string
				ExamType          string
				GradeLevel        sql.NullString
				Phase             sql.NullString
				AcademicYearID    sql.NullString
				DurationMinutes   int
				TotalQuestions    int
				TotalScore        float64
				PassingScore      sql.NullFloat64
				AutoGradeMC       bool
				ManualGradeEssay  bool
				Status            string
				IsActive          bool
				CreatedAt         time.Time
				UpdatedAt         time.Time
				PublishedAt       sql.NullTime
				Questions         []byte
				ScoringConfig     []byte
				AcademicYearName  sql.NullString
				Semester          sql.NullString
			}

			err = database.DB.QueryRow(`
				SELECT 
					e.id, e.title, e.description, e.subject, e.exam_type,
					e.grade_level, e.phase, e.academic_year_id,
					e.duration_minutes, e.total_questions, e.total_score, e.passing_score,
					e.auto_grade_mc, e.manual_grade_essay,
					e.status, e.is_active,
					e.created_at, e.updated_at, e.published_at,
					e.questions, e.scoring_config,
					say.name, say.semester
				FROM school_exams e
				LEFT JOIN school_academic_years say ON e.academic_year_id = say.id
				WHERE e.id = $1 AND e.school_id = $2 AND e.deleted_at IS NULL
			`, examID, schoolID).Scan(
				&e.ID, &e.Title, &e.Description, &e.Subject, &e.ExamType,
				&e.GradeLevel, &e.Phase, &e.AcademicYearID,
				&e.DurationMinutes, &e.TotalQuestions, &e.TotalScore, &e.PassingScore,
				&e.AutoGradeMC, &e.ManualGradeEssay,
				&e.Status, &e.IsActive,
				&e.CreatedAt, &e.UpdatedAt, &e.PublishedAt,
				&e.Questions, &e.ScoringConfig,
				&e.AcademicYearName, &e.Semester,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Ambil targets
			targetRows, err := database.DB.Query(`
				SELECT 
					et.id, et.class_group_id, et.class_sub_group_id,
					COALESCE(cg.name, '') AS class_group_name,
					COALESCE(csg.name, '') AS class_sub_group_name,
					COALESCE(cg.level, '') AS class_level
				FROM exam_targets et
				LEFT JOIN class_groups cg ON et.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
				WHERE et.exam_id = $1
			`, examID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer targetRows.Close()

			type TargetItem struct {
				ID                 string  `json:"id"`
				ClassGroupID       *string `json:"class_group_id"`
				ClassSubGroupID    *string `json:"class_sub_group_id"`
				ClassGroupName     string  `json:"class_group_name"`
				ClassSubGroupName  string  `json:"class_sub_group_name"`
				ClassLevel         string  `json:"class_level"`
			}

			var targets []TargetItem
			for targetRows.Next() {
				var t TargetItem
				var cgID, csgID sql.NullString
				if err := targetRows.Scan(&t.ID, &cgID, &csgID, &t.ClassGroupName, &t.ClassSubGroupName, &t.ClassLevel); err == nil {
					if cgID.Valid { t.ClassGroupID = &cgID.String }
					if csgID.Valid { t.ClassSubGroupID = &csgID.String }
					targets = append(targets, t)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"exam": gin.H{
					"id":                 e.ID,
					"title":              e.Title,
					"description":        e.Description.String,
					"subject":            e.Subject,
					"exam_type":          e.ExamType,
					"grade_level":        e.GradeLevel.String,
					"phase":              e.Phase.String,
					"academic_year_id":   e.AcademicYearID.String,
					"academic_year_name": e.AcademicYearName.String,
					"semester":           e.Semester.String,
					"duration_minutes":   e.DurationMinutes,
					"total_questions":    e.TotalQuestions,
					"total_score":        e.TotalScore,
					"passing_score":      e.PassingScore.Float64,
					"auto_grade_mc":      e.AutoGradeMC,
					"manual_grade_essay": e.ManualGradeEssay,
					"status":             e.Status,
					"is_active":          e.IsActive,
					"created_at":         e.CreatedAt,
					"updated_at":         e.UpdatedAt,
					"published_at":       e.PublishedAt.Time,
					"questions":          json.RawMessage(e.Questions),
					"scoring_config":     json.RawMessage(e.ScoringConfig),
					"targets":            targets,
				},
			})
		})

		// -------------------- CREATE --------------------
		// POST /api/school-admin/exams
		api.POST("/school-admin/exams", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateSchoolExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// Validasi: harus ada minimal 1 target
			if len(req.Targets) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal pilih 1 kelas/sub kelas target"})
				return
			}

			// Hitung total score dari semua questions
			var totalScore float64
			for _, q := range req.Questions {
				totalScore += q.Score
			}
			if totalScore == 0 {
				totalScore = 100 // fallback
			}

			examType := req.ExamType
			if examType == "" { examType = "regular" }

			scoringConfig := req.ScoringConfig
			if len(scoringConfig) == 0 {
				scoringConfig = json.RawMessage(`{"multiple_choice":1,"essay":5,"essay_rubric":"manual"}`)
			}

			questionsJSON, _ := json.Marshal(req.Questions)

			// Mulai transaksi
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Insert exam header
			var examID string
			err = tx.QueryRow(`
				INSERT INTO school_exams (
					school_id, academic_year_id, created_by,
					title, description, subject, exam_type, grade_level, phase,
					duration_minutes, total_questions, total_score, passing_score,
					scoring_config, questions, status
				) VALUES (
					$1, NULLIF($2,'')::uuid, $3,
					$4, NULLIF($5,''), $6, $7, NULLIF($8,''), NULLIF($9,''),
					$10, $11, $12, $13,
					$14::jsonb, $15::jsonb, 'draft'
				) RETURNING id
			`, schoolID, req.AcademicYearID, adminID,
				req.Title, req.Description, req.Subject, examType, req.GradeLevel, req.Phase,
				req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
				scoringConfig, questionsJSON,
			).Scan(&examID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan ujian: " + err.Error()})
				return
			}

			// 2. Insert targets (validasi tiap sub kelas milik sekolah ini)
			for _, t := range req.Targets {
				// Kalau ada sub_group_id, validasi dulu
				if t.ClassSubGroupID != "" {
					var validSchoolID string
					err := tx.QueryRow(`
						SELECT school_id FROM class_sub_groups WHERE id = $1
					`, t.ClassSubGroupID).Scan(&validSchoolID)
					if err != nil || validSchoolID != schoolID {
						c.JSON(http.StatusBadRequest, gin.H{
							"error": fmt.Sprintf("Sub kelas %s tidak valid / bukan milik sekolah Anda", t.ClassSubGroupID),
						})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO exam_targets (exam_id, class_group_id, class_sub_group_id)
					VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid)
					ON CONFLICT (exam_id, class_sub_group_id) DO NOTHING
				`, examID, t.ClassGroupID, t.ClassSubGroupID)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan target: " + err.Error()})
					return
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message": "Ujian berhasil dibuat (draft)",
				"id":      examID,
			})
		})

		// -------------------- UPDATE --------------------
		// PUT /api/school-admin/exams/:id
		// Hanya bisa update kalau status = 'draft'
		api.PUT("/school-admin/exams/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Param("id")

			// Cek status dulu
			var currentStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, examID, schoolID).Scan(&currentStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}

			if currentStatus != "draft" {
				c.JSON(http.StatusConflict, gin.H{
					"error": "Hanya ujian berstatus draft yang bisa diubah. Arsipkan atau buat baru.",
				})
				return
			}

			var req CreateSchoolExamRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			var totalScore float64
			for _, q := range req.Questions {
				totalScore += q.Score
			}
			if totalScore == 0 { totalScore = 100 }

			examType := req.ExamType
			if examType == "" { examType = "regular" }

			scoringConfig := req.ScoringConfig
			if len(scoringConfig) == 0 {
				scoringConfig = json.RawMessage(`{"multiple_choice":1,"essay":5,"essay_rubric":"manual"}`)
			}

			questionsJSON, _ := json.Marshal(req.Questions)

			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// 1. Update header
			_, err = tx.Exec(`
				UPDATE school_exams SET
					academic_year_id = NULLIF($1,'')::uuid,
					title = $2,
					description = NULLIF($3,''),
					subject = $4,
					exam_type = $5,
					grade_level = NULLIF($6,''),
					phase = NULLIF($7,''),
					duration_minutes = $8,
					total_questions = $9,
					total_score = $10,
					passing_score = $11,
					scoring_config = $12::jsonb,
					questions = $13::jsonb,
					updated_at = NOW()
				WHERE id = $14 AND school_id = $15
			`, req.AcademicYearID, req.Title, req.Description, req.Subject, examType,
				req.GradeLevel, req.Phase,
				req.DurationMinutes, len(req.Questions), totalScore, req.PassingScore,
				scoringConfig, questionsJSON,
				examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update ujian: " + err.Error()})
				return
			}

			// 2. Hapus targets lama, insert ulang
			_, _ = tx.Exec(`DELETE FROM exam_targets WHERE exam_id = $1`, examID)

			for _, t := range req.Targets {
				if t.ClassSubGroupID != "" {
					var validSchoolID string
					err := tx.QueryRow(`
						SELECT school_id FROM class_sub_groups WHERE id = $1
					`, t.ClassSubGroupID).Scan(&validSchoolID)
					if err != nil || validSchoolID != schoolID {
						c.JSON(http.StatusBadRequest, gin.H{
							"error": fmt.Sprintf("Sub kelas %s tidak valid", t.ClassSubGroupID),
						})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO exam_targets (exam_id, class_group_id, class_sub_group_id)
					VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid)
					ON CONFLICT (exam_id, class_sub_group_id) DO NOTHING
				`, examID, t.ClassGroupID, t.ClassSubGroupID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update target: " + err.Error()})
					return
				}
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil diperbarui"})
		})

		// -------------------- PUBLISH --------------------
		// PATCH /api/school-admin/exams/:id/publish
		api.PATCH("/school-admin/exams/:id/publish", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Param("id")

			// Validasi: minimal ada 1 soal & 1 target
			var totalQ int
			var targetCount int
			err = database.DB.QueryRow(`
				SELECT total_questions FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, examID, schoolID).Scan(&totalQ)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}

			_ = database.DB.QueryRow(`SELECT COUNT(*) FROM exam_targets WHERE exam_id = $1`, examID).Scan(&targetCount)

			if totalQ == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak bisa publish: ujian belum punya soal"})
				return
			}
			if targetCount == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak bisa publish: belum ada target kelas"})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET status = 'published', published_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND status = 'draft'
			`, examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Ujian sudah dipublish atau status tidak valid"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil dipublikasikan"})
		})

		// -------------------- ARCHIVE --------------------
		// PATCH /api/school-admin/exams/:id/archive
		api.PATCH("/school-admin/exams/:id/archive", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET status = 'archived', updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND status IN ('draft', 'published')
			`, c.Param("id"), schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan atau tidak bisa diarsipkan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil diarsipkan"})
		})

		// -------------------- DELETE (soft) --------------------
		// DELETE /api/school-admin/exams/:id
		api.DELETE("/school-admin/exams/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			result, err := database.DB.Exec(`
				UPDATE school_exams 
				SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, c.Param("id"), schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Ujian berhasil dihapus"})
		})

		// -------------------- HELPER: DAFTAR KELAS UNTUK DROPDOWN --------------------
		// GET /api/school-admin/exams/target-options
		// Return semua class_group + sub_group milik sekolah (untuk dropdown saat buat ujian)
		api.GET("/school-admin/exams/target-options", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					cg.id AS class_group_id,
					cg.name AS class_group_name,
					cg.level,
					COALESCE(cg.class_type, 'Umum') AS class_type,
					say.id AS academic_year_id,
					COALESCE(say.name, '-') AS academic_year_name,
					COALESCE(say.semester, '-') AS semester,
					COALESCE(
						(SELECT json_agg(json_build_object(
							'id', csg.id,
							'name', csg.name,
							'code', csg.code,
							'capacity', csg.capacity
						))
						FROM class_sub_groups csg
						WHERE csg.class_group_id = cg.id AND csg.is_active = TRUE),
						'[]'::json
					) AS sub_classes
				FROM class_groups cg
				LEFT JOIN school_academic_years say ON cg.academic_year_id = say.id
				WHERE cg.school_id = $1
				ORDER BY cg.level ASC, cg.name ASC
			`, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type ClassOption struct {
				ClassGroupID     string          `json:"class_group_id"`
				ClassGroupName   string          `json:"class_group_name"`
				Level            string          `json:"level"`
				ClassType        string          `json:"class_type"`
				AcademicYearID   string          `json:"academic_year_id"`
				AcademicYearName string          `json:"academic_year_name"`
				Semester         string          `json:"semester"`
				SubClasses       json.RawMessage `json:"sub_classes"`
			}

			var list []ClassOption
			for rows.Next() {
				var co ClassOption
				var level, classType, ayID, ayName, semester sql.NullString
				var subClassesJSON []byte

				err := rows.Scan(
					&co.ClassGroupID, &co.ClassGroupName, &level, &classType,
					&ayID, &ayName, &semester, &subClassesJSON,
				)
				if err != nil {
					continue
				}
				co.Level = level.String
				co.ClassType = classType.String
				co.AcademicYearID = ayID.String
				co.AcademicYearName = ayName.String
				co.Semester = semester.String
				co.SubClasses = json.RawMessage(subClassesJSON)

				list = append(list, co)
			}

			c.JSON(http.StatusOK, gin.H{
				"total": len(list),
				"classes": list,
			})
		})

		// GET /api/school-admin/exam-schedules?filter=upcoming&search=xxx
		api.GET("/school-admin/exam-schedules", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			filter := c.Query("filter")   // all | today | week | upcoming | past
			search := c.Query("search")

			query := `
				SELECT 
					es.id, es.exam_id,
					e.title AS exam_title, e.subject, es.duration_minutes,
					es.duration_mode,
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.access_code, es.status,
					cg.name AS class_group_name,
					csg.name AS class_sub_group_name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score, es.highest_score, es.lowest_score
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.school_id = $1 AND es.deleted_at IS NULL
			`
			args := []any{schoolID}
			argIdx := 2

			switch filter {
			case "today":
				query += " AND es.schedule_date = CURRENT_DATE"
			case "week":
				query += " AND es.schedule_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '7 days'"
			case "upcoming":
				query += " AND es.schedule_date >= CURRENT_DATE"
			case "past":
				query += " AND es.schedule_date < CURRENT_DATE"
			}

			if search != "" {
				query += fmt.Sprintf(
					" AND (e.title ILIKE $%d OR es.room ILIKE $%d OR es.supervisor_name ILIKE $%d)",
					argIdx, argIdx, argIdx,
				)
				args = append(args, "%"+search+"%")
				argIdx++
			}

			query += " ORDER BY es.schedule_date ASC, es.start_time ASC"

			rows, err := database.DB.Query(query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat jadwal: " + err.Error()})
				return
			}
			defer rows.Close()

			type ScheduleItem struct {
				ID                 string          `json:"id"`
				ExamID             string          `json:"exam_id"`
				ExamTitle          string          `json:"exam_title"`
				Subject            string          `json:"subject"`
				DurationMinutes    int             `json:"duration_minutes"`
				DurationMode       string          `json:"duration_mode"`
				ScheduleDate       string          `json:"schedule_date"`
				StartTime          string          `json:"start_time"`
				EndTime            string          `json:"end_time"`
				Room               *string         `json:"room"`
				SupervisorName     *string         `json:"supervisor_name"`
				AccessCode         *string         `json:"access_code"`
				Status             string          `json:"status"`
				ClassGroupName     *string         `json:"class_group_name"`
				ClassSubGroupName  *string         `json:"class_sub_group_name"`
				TotalStudents      int             `json:"total_students"`
				TotalSubmitted     int             `json:"total_submitted"`
				TotalGraded        int             `json:"total_graded"`
				AverageScore       *float64        `json:"average_score"`
				HighestScore       *float64        `json:"highest_score"`
				LowestScore        *float64        `json:"lowest_score"`
			}

			var schedules []ScheduleItem
			for rows.Next() {
				var s ScheduleItem
				var scheduleDate time.Time
				var startTime, endTime time.Time
				var room, supervisorName, accessCode sql.NullString
				var classGroupName, classSubGroupName sql.NullString
				var avgScore, highScore, lowScore sql.NullFloat64

				err := rows.Scan(
					&s.ID, &s.ExamID,
					&s.ExamTitle, &s.Subject, &s.DurationMinutes,
					&s.DurationMode,  
					&scheduleDate, &startTime, &endTime,
					&room, &supervisorName, &accessCode, &s.Status,
					&classGroupName, &classSubGroupName,
					&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
					&avgScore, &highScore, &lowScore,
				)
				if err != nil {
					fmt.Printf("[GET SCHEDULES] Scan error: %v\n", err)
					continue
				}

				s.ScheduleDate = scheduleDate.Format("2006-01-02")
				s.StartTime = startTime.Format("15:04")
				s.EndTime = endTime.Format("15:04")

				if room.Valid { s.Room = &room.String }
				if supervisorName.Valid { s.SupervisorName = &supervisorName.String }
				if accessCode.Valid { s.AccessCode = &accessCode.String }
				if classGroupName.Valid { s.ClassGroupName = &classGroupName.String }
				if classSubGroupName.Valid { s.ClassSubGroupName = &classSubGroupName.String }
				if avgScore.Valid { s.AverageScore = &avgScore.Float64 }
				if highScore.Valid { s.HighestScore = &highScore.Float64 }
				if lowScore.Valid { s.LowestScore = &lowScore.Float64 }

				schedules = append(schedules, s)
			}

			if schedules == nil {
				schedules = []ScheduleItem{}
			}

			c.JSON(http.StatusOK, gin.H{
				"total":     len(schedules),
				"schedules": schedules,
			})
		})

		// GET /api/school-admin/exam-schedules/available-targets?exam_id=xxx
		api.GET("/school-admin/exam-schedules/available-targets", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			examID := c.Query("exam_id")
			if examID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "exam_id wajib diisi"})
				return
			}

			// Validasi exam milik sekolah ini & published
			var examExists bool
			err = database.DB.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM school_exams 
					WHERE id = $1 AND school_id = $2 
					AND deleted_at IS NULL
					AND status = 'published'
				)
			`, examID, schoolID).Scan(&examExists)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal validasi ujian: " + err.Error()})
				return
			}
			if !examExists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan atau belum dipublish"})
				return
			}

			rows, err := database.DB.Query(`
				SELECT 
					et.class_group_id,
					COALESCE(cg.name, '') AS class_group_name,
					et.class_sub_group_id,
					COALESCE(csg.name, '') AS class_sub_group_name,
					COALESCE(cg.level, '') AS class_level,
					(
						SELECT COUNT(*) FROM students s 
						WHERE s.class_sub_group_id = et.class_sub_group_id 
						AND s.is_active = TRUE
					) AS student_count,
					(
						SELECT COUNT(*) FROM exam_schedules es
						WHERE es.exam_id = et.exam_id
						AND es.class_sub_group_id = et.class_sub_group_id
						AND es.status IN ('scheduled', 'ongoing')
						AND es.deleted_at IS NULL
					) AS active_schedule_count
				FROM exam_targets et
				LEFT JOIN class_groups cg ON et.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON et.class_sub_group_id = csg.id
				WHERE et.exam_id = $1 
				AND et.class_sub_group_id IS NOT NULL
				AND EXISTS (
					SELECT 1 FROM class_groups 
					WHERE id = et.class_group_id AND school_id = $2
				)
				ORDER BY cg.level ASC, cg.name ASC, csg.name ASC
			`, examID, schoolID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat target: " + err.Error()})
				return
			}
			defer rows.Close()

			type TargetItem struct {
				ClassGroupID          string `json:"class_group_id"`
				ClassGroupName        string `json:"class_group_name"`
				ClassSubGroupID       string `json:"class_sub_group_id"`
				ClassSubGroupName     string `json:"class_sub_group_name"`
				ClassLevel            string `json:"class_level"`
				StudentCount          int    `json:"student_count"`
				ActiveScheduleCount   int    `json:"active_schedule_count"`
				HasActiveSchedule     bool   `json:"has_active_schedule"`
			}

			targets := []TargetItem{}
			for rows.Next() {
				var t TargetItem
				var classGroupID, classSubGroupID sql.NullString

				err := rows.Scan(
					&classGroupID,
					&t.ClassGroupName,
					&classSubGroupID,
					&t.ClassSubGroupName,
					&t.ClassLevel,
					&t.StudentCount,
					&t.ActiveScheduleCount,
				)
				if err != nil {
					fmt.Printf("[AVAILABLE TARGETS] Scan error: %v\n", err)
					continue
				}

				t.ClassGroupID = classGroupID.String
				t.ClassSubGroupID = classSubGroupID.String
				t.HasActiveSchedule = t.ActiveScheduleCount > 0

				targets = append(targets, t)
			}

			if err := rows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterasi rows: " + err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"exam_id": examID,
				"total":   len(targets),
				"targets": targets,
			})
		})

		api.POST("/school-admin/exam-schedules", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			var req CreateScheduleRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// ==========================================
			// Validasi duration_mode
			// ==========================================
			durationMode := req.DurationMode
			if durationMode == "" {
				durationMode = "strict"
			}
			if durationMode != "strict" && durationMode != "flexible" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "duration_mode harus 'strict' atau 'flexible'",
				})
				return
			}

			if durationMode == "strict" {
				startT, err1 := parseTimeString(req.StartTime)
				endT, err2 := parseTimeString(req.EndTime)
				if err1 != nil || err2 != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Format jam tidak valid"})
					return
				}
				diff := int(endT.Sub(startT).Minutes())
				if diff <= 0 {
					diff += 24 * 60
				}
				if diff <= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Jam selesai harus lebih besar dari jam mulai"})
					return
				}
				req.DurationMinutes = diff
			}

			// ==========================================
			// Validasi exam published & milik sekolah
			// ==========================================
			var examStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM school_exams 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, req.ExamID, schoolID).Scan(&examStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ujian tidak ditemukan"})
				return
			}
			if examStatus != "published" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya ujian published yang bisa dijadwalkan"})
				return
			}

			// ==========================================
			// Normalize access code
			// ==========================================
			accessCode := strings.ToUpper(strings.TrimSpace(req.AccessCode))
			if accessCode == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Kode akses wajib diisi"})
				return
			}

			// ==========================================
			// Cek duplikat access_code — HILANGKAN filter deleted_at
			// dan hanya cek untuk (access_code, class_sub_group_id) yang akan dipakai.
			// ==========================================
			for _, subGroupID := range req.TargetSubGroupIDs {
				var existingID string
				err := database.DB.QueryRow(`
					SELECT id FROM exam_schedules 
					WHERE access_code = $1 
					AND class_sub_group_id = $2
					AND deleted_at IS NULL
					LIMIT 1
				`, accessCode, subGroupID).Scan(&existingID)

				if err == nil && existingID != "" {
					c.JSON(http.StatusConflict, gin.H{
						"error": fmt.Sprintf(
							"Kode akses '%s' sudah dipakai untuk sub kelas ini. Generate ulang atau pakai kode lain.",
							accessCode,
						),
					})
					return
				}
			}

			// ==========================================
			// Tx + Insert
			// ==========================================
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			createdIDs := []string{}

			for _, subGroupID := range req.TargetSubGroupIDs {
				// Validasi sub group milik sekolah
				var validSchoolID string
				var classGroupID string
				err := tx.QueryRow(`
					SELECT school_id, class_group_id FROM class_sub_groups WHERE id = $1
				`, subGroupID).Scan(&validSchoolID, &classGroupID)
				if err != nil || validSchoolID != schoolID {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak valid: " + subGroupID})
					return
				}

				// Count students
				var totalStudents int
				_ = tx.QueryRow(`
					SELECT COUNT(*) FROM students 
					WHERE class_sub_group_id = $1 AND is_active = TRUE
				`, subGroupID).Scan(&totalStudents)

				// Lock check — SELECT FOR UPDATE biar anti race condition
				var dummy string
				_ = tx.QueryRow(`
					SELECT id FROM exam_schedules
					WHERE access_code = $1 AND class_sub_group_id = $2 AND deleted_at IS NULL
					FOR UPDATE
				`, accessCode, subGroupID).Scan(&dummy)

				var scheduleID string
				err = tx.QueryRow(`
					INSERT INTO exam_schedules (
						school_id, exam_id,
						class_group_id, class_sub_group_id,
						schedule_date, start_time, end_time, duration_minutes,
						duration_mode,
						room, supervisor_name, session_notes,
						access_code, require_login,
						status, total_students,
						created_by
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7, $8,
						$9,
						NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
						$13, $14, 'scheduled', $15, $16
					) RETURNING id
				`, schoolID, req.ExamID,
					classGroupID, subGroupID,
					req.ScheduleDate, req.StartTime, req.EndTime, req.DurationMinutes,
					durationMode,
					req.Room, req.SupervisorName, req.SessionNotes,
					accessCode, req.RequireLogin,
					totalStudents, adminID).Scan(&scheduleID)

				if err != nil {
					// Deteksi unique violation
					if strings.Contains(err.Error(), "23505") ||
					strings.Contains(err.Error(), "duplicate key") {
						c.JSON(http.StatusConflict, gin.H{
							"error": fmt.Sprintf(
								"Kode akses '%s' sudah dipakai untuk sub kelas ini. Generate ulang.",
								accessCode,
							),
						})
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan jadwal: " + err.Error()})
					return
				}
				createdIDs = append(createdIDs, scheduleID)
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":       fmt.Sprintf("%d jadwal berhasil dibuat", len(createdIDs)),
				"schedule_ids":  createdIDs,
				"access_code":   accessCode,
				"duration_mode": durationMode,
			})
		})

		api.GET("/school-admin/exam-schedules/:id/monitor", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule info
			var s struct {
				ID                string
				ExamID            string
				ExamTitle         string
				Subject           string
				DurationMinutes   int
				DurationMode      string  
				ScheduleDate      time.Time
				StartTime         time.Time
				EndTime           time.Time
				Room              sql.NullString
				SupervisorName    sql.NullString
				AccessCode        sql.NullString
				Status            string
				ClassGroupName    sql.NullString
				ClassSubGroupName sql.NullString
				TotalStudents     int
				TotalSubmitted    int
				TotalGraded       int
				AverageScore      sql.NullFloat64
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id,
					e.title, e.subject, es.duration_minutes,
					es.duration_mode,   
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.access_code, es.status,
					cg.name, csg.name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID,
				&s.ExamTitle, &s.Subject, &s.DurationMinutes,
				&s.DurationMode,      
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName, &s.AccessCode, &s.Status,
				&s.ClassGroupName, &s.ClassSubGroupName,
				&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
				&s.AverageScore,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Ambil semua siswa target + LEFT JOIN submissions
			rows, err := database.DB.Query(`
				SELECT 
					s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
					COALESCE(csg.name, ''),
					sub.id, sub.status,
					sub.started_at, sub.submitted_at, sub.last_activity_at,
					sub.time_spent_seconds,
					sub.total_score, sub.percentage, sub.is_passed,
					COALESCE(sub.is_flagged, false),
					COALESCE(sub.tab_switch_count, 0),
					sub.ip_address,
					els.id AS session_id,
					els.session_token,
					COALESCE(els.is_blocked, false) AS is_blocked,
					els.blocked_reason,
					els.blocked_at
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN exam_submissions sub 
					ON sub.student_id = s.id AND sub.schedule_id = $1
				LEFT JOIN exam_live_sessions els
					ON els.student_id = s.id 
					AND els.schedule_id = $1
					AND els.submitted_at IS NULL
				WHERE s.school_id = $2 
				AND s.class_sub_group_id = (SELECT class_sub_group_id FROM exam_schedules WHERE id = $1)
				AND s.is_active = TRUE
				ORDER BY s.full_name ASC
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type StudentRow struct {
				StudentID         string     `json:"student_id"`
				StudentName       string     `json:"student_name"`
				NISN              string     `json:"nisn"`
				StudentNumber     string     `json:"student_number"`
				ClassSubGroupName string     `json:"class_sub_group_name"`
				SubmissionID      *string    `json:"submission_id"`
				Status            string     `json:"status"`
				StartedAt         *time.Time `json:"started_at"`
				SubmittedAt       *time.Time `json:"submitted_at"`
				LastActivityAt    *time.Time `json:"last_activity_at"`
				TimeSpentSeconds  *int       `json:"time_spent_seconds"`
				TotalScore        *float64   `json:"total_score"`
				Percentage        *float64   `json:"percentage"`
				IsPassed          *bool      `json:"is_passed"`
				IsFlagged         bool       `json:"is_flagged"`
				TabSwitchCount    int        `json:"tab_switch_count"`
				IPAddress         *string    `json:"ip_address"`

				// Realtime control
				SessionID      *string    `json:"session_id"`
				SessionToken   *string    `json:"session_token"`
				IsBlocked      bool       `json:"is_blocked"`
				BlockedReason  *string    `json:"blocked_reason"`
				BlockedAt      *time.Time `json:"blocked_at"`
			}

			students := []StudentRow{}
			var inProgress, submitted, graded, notStarted, flagged, blocked int
			var totalScoreSum float64
			var scoreCount int

			for rows.Next() {
				var r StudentRow
				var subID sql.NullString
				var status sql.NullString
				var startedAt, submittedAt, lastActivityAt sql.NullTime
				var timeSpent sql.NullInt64
				var totalScore, percentage sql.NullFloat64
				var isPassed sql.NullBool
				var ipAddress sql.NullString

				// New fields
				var sessionID sql.NullString
				var sessionToken sql.NullString
				var isBlocked sql.NullBool
				var blockedReason sql.NullString
				var blockedAt sql.NullTime

				err := rows.Scan(
					&r.StudentID, &r.StudentName, &r.NISN, &r.StudentNumber,
					&r.ClassSubGroupName,
					&subID, &status,
					&startedAt, &submittedAt, &lastActivityAt,
					&timeSpent,
					&totalScore, &percentage, &isPassed,
					&r.IsFlagged, &r.TabSwitchCount,
					&ipAddress,
					&sessionID,
					&sessionToken,
					&isBlocked,
					&blockedReason,
					&blockedAt,
				)
				if err != nil {
					fmt.Printf("[MONITOR] Scan error: %v\n", err)  // ← tambah log
					continue
				}

				// ==========================================
				// ASSIGN SESSION FIELDS DULU
				// ==========================================
				if sessionID.Valid { r.SessionID = &sessionID.String }
				if sessionToken.Valid { r.SessionToken = &sessionToken.String }
				r.IsBlocked = isBlocked.Bool
				if blockedReason.Valid { r.BlockedReason = &blockedReason.String }
				if blockedAt.Valid { r.BlockedAt = &blockedAt.Time }

				// ==========================================
				// TENTUKAN STATUS (URUTAN PENTING!)
				// ==========================================
				if subID.Valid {
					// Sudah submit → pakai status dari submission
					r.SubmissionID = &subID.String
					r.Status = status.String
				} else if sessionID.Valid {
					// Ada session aktif tapi belum submit → SEDANG MENGERJAKAN
					r.Status = "in_progress"
				} else {
					// Tidak ada submission, tidak ada session → belum mulai
					r.Status = "not_started"
				}

				// ==========================================
				// ASSIGN FIELD LAINNYA
				// ==========================================
				if startedAt.Valid { r.StartedAt = &startedAt.Time }
				if submittedAt.Valid { r.SubmittedAt = &submittedAt.Time }
				if lastActivityAt.Valid { r.LastActivityAt = &lastActivityAt.Time }
				if timeSpent.Valid {
					t := int(timeSpent.Int64)
					r.TimeSpentSeconds = &t
				}
				if totalScore.Valid { r.TotalScore = &totalScore.Float64 }
				if percentage.Valid { r.Percentage = &percentage.Float64 }
				if isPassed.Valid { r.IsPassed = &isPassed.Bool }
				if ipAddress.Valid { r.IPAddress = &ipAddress.String }

				// ==========================================
				// AGGREGATE STATS
				// ==========================================
				switch r.Status {
				case "in_progress":
					inProgress++
				case "submitted":
					submitted++
				case "graded", "graded_with_pending":
					graded++
				case "not_started":
					notStarted++
				}
				if r.IsFlagged {
					flagged++
				}
				if r.IsBlocked {
					blocked++
				}
				if r.Percentage != nil {
					totalScoreSum += *r.Percentage
					scoreCount++
				}

				students = append(students, r)
			}

			var avgScore *float64
			if scoreCount > 0 {
				avg := totalScoreSum / float64(scoreCount)
				avgScore = &avg
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":                   s.ID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"subject":              s.Subject,
					"duration_minutes":     s.DurationMinutes,
					"duration_mode":        s.DurationMode,  
					"schedule_date":        s.ScheduleDate.Format("2006-01-02"),
					"start_time":           s.StartTime.Format("15:04"),
					"end_time":             s.EndTime.Format("15:04"),
					"room":                 s.Room.String,
					"supervisor_name":      s.SupervisorName.String,
					"access_code":          s.AccessCode.String,
					"status":               s.Status,
					"class_group_name":     s.ClassGroupName.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"total_students":       s.TotalStudents,
					"total_submitted":      s.TotalSubmitted,
					"total_graded":         s.TotalGraded,
					"average_score":        s.AverageScore.Float64,
				},
				"students": students,
				"stats": gin.H{
					"total_students": len(students),
					"in_progress":    inProgress,
					"submitted":      submitted,
					"graded":         graded,
					"not_started":    notStarted,
					"flagged":        flagged,
					"blocked":        blocked,
					"average_score":  avgScore,
				},
			})
		})

		api.PATCH("/school-admin/exam-schedules/:id/close", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			var currentStatus string
			err = database.DB.QueryRow(`
				SELECT status FROM exam_schedules 
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, scheduleID, schoolID).Scan(&currentStatus)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			if currentStatus == "completed" || currentStatus == "cancelled" {
				c.JSON(http.StatusConflict, gin.H{"error": "Sesi sudah tidak aktif"})
				return
			}

			// 1. Auto-submit semua submission yang masih in_progress
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			result, err := tx.Exec(`
				UPDATE exam_submissions 
				SET status = 'submitted', 
					submitted_at = NOW(),
					last_activity_at = NOW()
				WHERE schedule_id = $1 AND status = 'in_progress'
			`, scheduleID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			autoSubmitted, _ := result.RowsAffected()

			// 2. Set schedule status = completed
			_, err = tx.Exec(`
				UPDATE exam_schedules 
				SET status = 'completed', updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Auto-unblock semua siswa yang masih diblokir
			_, _ = tx.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = false,
					unblocked_at = NOW(),
					updated_at = NOW()
				WHERE schedule_id = $1 AND is_blocked = true
			`, scheduleID)
			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":         "Sesi berhasil ditutup",
				"auto_submitted":  autoSubmitted,
			})
		})

		api.GET("/school-admin/exam-schedules/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule + exam info
			var s struct {
				ID                string
				ExamID            string
				ExamTitle         string
				ExamDescription   sql.NullString
				Subject           string
				ExamType          string
				GradeLevel        sql.NullString
				Phase             sql.NullString
				TotalQuestions    int
				TotalScore        float64
				PassingScore      sql.NullFloat64
				DurationMinutes   int
				DurationMode 	  string
				ScheduleDate      time.Time
				StartTime         time.Time
				EndTime           time.Time

				Room              sql.NullString
				SupervisorName    sql.NullString
				SessionNotes      sql.NullString

				AccessCode        sql.NullString
				RequireLogin      bool

				Status            string

				ClassGroupID      sql.NullString
				ClassGroupName    sql.NullString
				ClassSubGroupID   sql.NullString
				ClassSubGroupName sql.NullString

				TotalStudents     int
				TotalSubmitted    int
				TotalGraded       int
				AverageScore      sql.NullFloat64
				HighestScore      sql.NullFloat64
				LowestScore       sql.NullFloat64

				CreatedAt         time.Time
				UpdatedAt         time.Time
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id,
					e.title, e.description, e.subject, e.exam_type,
					e.grade_level, e.phase,
					e.total_questions, e.total_score, e.passing_score,
					es.duration_minutes,
					es.duration_mode,      
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name, es.session_notes,
					es.access_code, es.require_login,
					es.status,
					es.class_group_id, cg.name,
					es.class_sub_group_id, csg.name,
					es.total_students, es.total_submitted, es.total_graded,
					es.average_score, es.highest_score, es.lowest_score,
					es.created_at, es.updated_at
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_groups cg ON es.class_group_id = cg.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID,
				&s.ExamTitle, &s.ExamDescription, &s.Subject, &s.ExamType,
				&s.GradeLevel, &s.Phase,
				&s.TotalQuestions, &s.TotalScore, &s.PassingScore,
				&s.DurationMinutes,
				&s.DurationMode,  
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName, &s.SessionNotes,
				&s.AccessCode, &s.RequireLogin,
				&s.Status,
				&s.ClassGroupID, &s.ClassGroupName,
				&s.ClassSubGroupID, &s.ClassSubGroupName,
				&s.TotalStudents, &s.TotalSubmitted, &s.TotalGraded,
				&s.AverageScore, &s.HighestScore, &s.LowestScore,
				&s.CreatedAt, &s.UpdatedAt,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Ambil daftar peserta (semua siswa di class_sub_group ini)
			type StudentParticipant struct {
				StudentID      string   `json:"student_id"`
				FullName       string   `json:"full_name"`
				NISN           string   `json:"nisn"`
				StudentNumber  string   `json:"student_number"`
				SubmissionID   *string  `json:"submission_id"`
				Status         string   `json:"status"`
				TotalScore     *float64 `json:"total_score"`
				Percentage     *float64 `json:"percentage"`
				IsPassed       *bool    `json:"is_passed"`
			}

			students := []StudentParticipant{}

			if s.ClassSubGroupID.Valid {
				rows, err := database.DB.Query(`
					SELECT 
						s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
						sub.id, sub.status,
						sub.total_score, sub.percentage, sub.is_passed
					FROM students s
					LEFT JOIN exam_submissions sub 
						ON sub.student_id = s.id AND sub.schedule_id = $1
					WHERE s.class_sub_group_id = $2 
					AND s.school_id = $3
					AND s.is_active = TRUE
					ORDER BY s.full_name ASC
				`, scheduleID, s.ClassSubGroupID.String, schoolID)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				defer rows.Close()

				for rows.Next() {
					var p StudentParticipant
					var subID, subStatus sql.NullString
					var totalScore, percentage sql.NullFloat64
					var isPassed sql.NullBool

					err := rows.Scan(
						&p.StudentID, &p.FullName, &p.NISN, &p.StudentNumber,
						&subID, &subStatus,
						&totalScore, &percentage, &isPassed,
					)
					if err != nil {
						continue
					}

					if subID.Valid {
						p.SubmissionID = &subID.String
					}
					if subStatus.Valid {
						p.Status = subStatus.String
					} else {
						p.Status = "not_started"
					}
					if totalScore.Valid { p.TotalScore = &totalScore.Float64 }
					if percentage.Valid { p.Percentage = &percentage.Float64 }
					if isPassed.Valid { p.IsPassed = &isPassed.Bool }

					students = append(students, p)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":                   s.ID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"exam_description":     s.ExamDescription.String,
					"subject":              s.Subject,
					"exam_type":            s.ExamType,
					"grade_level":          s.GradeLevel.String,
					"phase":                s.Phase.String,
					"total_questions":      s.TotalQuestions,
					"total_score":          s.TotalScore,
					"passing_score":        s.PassingScore.Float64,
					"duration_minutes":     s.DurationMinutes,
					"schedule_date":        s.ScheduleDate.Format("2006-01-02"),
					"start_time":           s.StartTime.Format("15:04"),
					"end_time":             s.EndTime.Format("15:04"),
					"room":                 s.Room.String,
					"supervisor_name":      s.SupervisorName.String,
					"session_notes":        s.SessionNotes.String,
					"access_code":          s.AccessCode.String,
					"require_login":        s.RequireLogin,
					"status":               s.Status,
					"class_group_id":       s.ClassGroupID.String,
					"class_group_name":     s.ClassGroupName.String,
					"class_sub_group_id":   s.ClassSubGroupID.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"total_students":       s.TotalStudents,
					"total_submitted":      s.TotalSubmitted,
					"total_graded":         s.TotalGraded,
					"average_score":        s.AverageScore.Float64,
					"highest_score":        s.HighestScore.Float64,
					"lowest_score":         s.LowestScore.Float64,
					"created_at":           s.CreatedAt,
					"updated_at":           s.UpdatedAt,
				},
				"students": students,
			})
		})

		api.PATCH("/school-admin/exam-schedules/:id/cancel", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// Ambil body opsional: { reason }
			var req struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&req)

			// 1. Ambil status & info jadwal
			var currentStatus string
			var examTitle, classSubGroupName string
			err = database.DB.QueryRow(`
				SELECT 
					es.status,
					e.title,
					COALESCE(csg.name, '')
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				LEFT JOIN class_sub_groups csg ON es.class_sub_group_id = csg.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(&currentStatus, &examTitle, &classSubGroupName)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 2. Cek status dasar
			if currentStatus == "cancelled" {
				c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah dibatalkan sebelumnya"})
				return
			}
			if currentStatus == "completed" {
				c.JSON(http.StatusConflict, gin.H{"error": "Jadwal sudah selesai, tidak bisa dibatalkan"})
				return
			}

			// 3. Cek apakah ada siswa yang sudah mulai mengerjakan
			//    (session aktif tanpa submitted_at, ATAU sudah ada submission)
			var activeSessions int
			var totalSubmissions int

			_ = database.DB.QueryRow(`
				SELECT COUNT(*) 
				FROM exam_live_sessions
				WHERE schedule_id = $1 AND submitted_at IS NULL
			`, scheduleID).Scan(&activeSessions)

			_ = database.DB.QueryRow(`
				SELECT COUNT(*) 
				FROM exam_submissions
				WHERE schedule_id = $1
			`, scheduleID).Scan(&totalSubmissions)

			// 4. Validasi: tidak bisa dibatalkan kalau sudah ada aktivitas siswa
			if activeSessions > 0 || totalSubmissions > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf(
						"Tidak bisa dibatalkan. Sudah ada %d siswa yang memulai dan %d siswa yang submit. Gunakan 'Tutup Sesi' di halaman Monitor untuk menghentikan.",
						activeSessions, totalSubmissions,
					),
					"active_sessions":   activeSessions,
					"total_submissions": totalSubmissions,
				})
				return
			}

			// 5. Cek status jadwal
			if currentStatus == "ongoing" {
				c.JSON(http.StatusConflict, gin.H{
					"error": "Jadwal sudah berlangsung. Gunakan 'Tutup Sesi' di halaman Monitor untuk menghentikan.",
				})
				return
			}

			// 6. Aman dibatalkan → lakukan update
			tx, err := database.DB.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
				return
			}
			defer tx.Rollback()

			// Update status jadwal
			result, err := tx.Exec(`
				UPDATE exam_schedules 
				SET status = 'cancelled', updated_at = NOW()
				WHERE id = $1 AND school_id = $2
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membatalkan jadwal: " + err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			// Hapus semua live session yang belum submit (kalau ada sisa)
			// — seharusnya kosong, tapi ini safety net
			_, _ = tx.Exec(`
				DELETE FROM exam_live_sessions
				WHERE schedule_id = $1 AND submitted_at IS NULL
			`, scheduleID)

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"})
				return
			}

			// 7. Log aktivitas (opsional)
			adminID, _ := getAdminIDFromUser(c)
			fmt.Printf("[CANCEL SCHEDULE] Admin %s membatalkan jadwal %s (%s) — %s\n",
				adminID, scheduleID, examTitle, classSubGroupName)

			c.JSON(http.StatusOK, gin.H{
				"message":     "Jadwal berhasil dibatalkan",
				"schedule_id": scheduleID,
				"reason":      req.Reason,
			})
		})

		api.DELETE("/school-admin/exam-schedules/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			result, err := database.DB.Exec(`
				UPDATE exam_schedules 
				SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND school_id = $2 AND deleted_at IS NULL
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Jadwal berhasil dihapus"})
		})

		api.GET("/school-admin/exam-submissions/:id", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			submissionID := c.Param("id")

			var s struct {
				ID                string
				ScheduleID        string
				ExamID            string
				ExamTitle         string
				Subject           string
				StudentID         string
				StudentName       string
				StudentNISN       sql.NullString
				StudentNumber     sql.NullString
				ClassSubGroupName sql.NullString
				Status            string
				StartedAt         sql.NullTime
				SubmittedAt       sql.NullTime
				LastActivityAt    sql.NullTime
				TimeSpentSeconds  sql.NullInt64
				TotalScore        sql.NullFloat64
				MaxScore          sql.NullFloat64
				Percentage        sql.NullFloat64
				IsPassed          sql.NullBool
				TabSwitchCount    int
				IsFlagged         bool
				IPAddress         sql.NullString
				UserAgent         sql.NullString
				Answers           []byte
				PassingScore      sql.NullFloat64
			}

			err = database.DB.QueryRow(`
				SELECT 
					sub.id, sub.schedule_id, sub.exam_id,
					e.title, e.subject,
					sub.student_id, sub.student_name,
					sub.student_nisn, sub.student_number,
					csg.name,
					sub.status,
					sub.started_at, sub.submitted_at, sub.last_activity_at,
					sub.time_spent_seconds,
					sub.total_score, sub.max_score, sub.percentage, sub.is_passed,
					sub.tab_switch_count, sub.is_flagged,
					sub.ip_address, sub.user_agent,
					sub.answers,
					COALESCE(e.passing_score, 70)
				FROM exam_submissions sub
				JOIN school_exams e ON sub.exam_id = e.id
				LEFT JOIN students stu ON stu.id = sub.student_id
				LEFT JOIN class_sub_groups csg ON stu.class_sub_group_id = csg.id
				WHERE sub.id = $1 AND sub.school_id = $2
			`, submissionID, schoolID).Scan(
				&s.ID, &s.ScheduleID, &s.ExamID,
				&s.ExamTitle, &s.Subject,
				&s.StudentID, &s.StudentName,
				&s.StudentNISN, &s.StudentNumber,
				&s.ClassSubGroupName,
				&s.Status,
				&s.StartedAt, &s.SubmittedAt, &s.LastActivityAt,
				&s.TimeSpentSeconds,
				&s.TotalScore, &s.MaxScore, &s.Percentage, &s.IsPassed,
				&s.TabSwitchCount, &s.IsFlagged,
				&s.IPAddress, &s.UserAgent,
				&s.Answers,
				&s.PassingScore,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Submission tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			var answers []any
			_ = json.Unmarshal(s.Answers, &answers)

			c.JSON(http.StatusOK, gin.H{
				"submission": gin.H{
					"id":                   s.ID,
					"schedule_id":          s.ScheduleID,
					"exam_id":              s.ExamID,
					"exam_title":           s.ExamTitle,
					"subject":              s.Subject,
					"student_id":           s.StudentID,
					"student_name":         s.StudentName,
					"student_nisn":         s.StudentNISN.String,
					"student_number":       s.StudentNumber.String,
					"class_sub_group_name": s.ClassSubGroupName.String,
					"status":               s.Status,
					"started_at":           s.StartedAt.Time,
					"submitted_at":         s.SubmittedAt.Time,
					"last_activity_at":     s.LastActivityAt.Time,
					"time_spent_seconds":   s.TimeSpentSeconds.Int64,
					"total_score":          s.TotalScore.Float64,
					"max_score":            s.MaxScore.Float64,
					"percentage":           s.Percentage.Float64,
					"is_passed":            s.IsPassed.Bool,
					"passing_score":        s.PassingScore.Float64,
					"tab_switch_count":     s.TabSwitchCount,
					"is_flagged":           s.IsFlagged,
					"ip_address":           s.IPAddress.String,
					"user_agent":           s.UserAgent.String,
					"answers":              answers,
					"questions":            []any{}, // optional: include question snapshots
				},
			})
		})

		api.PATCH("/school-admin/exam-submissions/:id/grade-essay", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			submissionID := c.Param("id")

			var req GradeEssayRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Ambil submission
			var scheduleID, examID, studentID string
			var answersJSON []byte
			var passingScore float64

			err = database.DB.QueryRow(`
				SELECT sub.schedule_id, sub.exam_id, sub.student_id, sub.answers,
					COALESCE(e.passing_score, 70)
				FROM exam_submissions sub
				JOIN school_exams e ON sub.exam_id = e.id
				WHERE sub.id = $1 AND sub.school_id = $2
			`, submissionID, schoolID).Scan(
				&scheduleID, &examID, &studentID, &answersJSON, &passingScore,
			)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Submission tidak ditemukan"})
				return
			}

			// 2. Parse answers, update skor essay
			var answers []map[string]any
			if err := json.Unmarshal(answersJSON, &answers); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Format answers error"})
				return
			}

			found := false
			var newTotalScore, newMaxScore float64

			for i, a := range answers {
				qID, _ := a["question_id"].(string)
				maxScore, _ := a["max_score"].(float64)
				newMaxScore += maxScore

				if qID == req.QuestionID {
					answers[i]["score"] = req.Score
					answers[i]["feedback"] = req.Feedback
					answers[i]["graded_at"] = time.Now().Format(time.RFC3339)
					found = true
					newTotalScore += req.Score
				} else {
					if sc, ok := a["score"].(float64); ok {
						newTotalScore += sc
					}
				}
			}

			if !found {
				c.JSON(http.StatusNotFound, gin.H{"error": "Soal tidak ditemukan di submission"})
				return
			}

			// 3. Cek apakah semua essay sudah dinilai
			hasPending := false
			for _, a := range answers {
				if a["type"] == "essay" {
					if _, ok := a["score"]; !ok || a["score"] == nil {
						hasPending = true
						break
					}
				}
			}

			// 4. Hitung ulang persentase
			percentage := 0.0
			if newMaxScore > 0 {
				percentage = (newTotalScore / newMaxScore) * 100
			}

			newStatus := "graded"
			if hasPending {
				newStatus = "graded_with_pending"
			}

			// 5. Update submission
			newAnswersJSON, _ := json.Marshal(answers)
			_, err = database.DB.Exec(`
				UPDATE exam_submissions SET
					answers = $1::jsonb,
					total_score = $2,
					percentage = $3,
					is_passed = $4,
					status = $5,
					updated_at = NOW(),
					fully_graded_at = CASE WHEN $6 = 'graded' THEN NOW() ELSE fully_graded_at END
				WHERE id = $7
			`, newAnswersJSON, newTotalScore, percentage, percentage >= passingScore,
				newStatus, newStatus, submissionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 6. Update schedule stats
			_, _ = database.DB.Exec(`
				UPDATE exam_schedules SET
					total_graded = (SELECT COUNT(*) FROM exam_submissions WHERE schedule_id = $1 AND status = 'graded'),
					average_score = (SELECT AVG(percentage) FROM exam_submissions WHERE schedule_id = $1),
					updated_at = NOW()
				WHERE id = $1
			`, scheduleID)

			c.JSON(http.StatusOK, gin.H{
				"message":       "Penilaian berhasil disimpan",
				"total_score":   newTotalScore,
				"percentage":    percentage,
				"is_passed":     percentage >= passingScore,
				"status":        newStatus,
				"has_pending":   hasPending,
			})
		})

		api.GET("/school-admin/exam-schedules/:id/grades", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			scheduleID := c.Param("id")

			// 1. Ambil schedule info
			var s struct {
				ID              string
				ExamID          string
				ExamTitle       string
				Subject         string
				ScheduleDate    time.Time
				StartTime       time.Time
				EndTime         time.Time
				Room            sql.NullString
				SupervisorName  sql.NullString
				PassingScore    sql.NullFloat64
				ClassSubGroupID sql.NullString
				TotalStudents   int
			}

			err = database.DB.QueryRow(`
				SELECT 
					es.id, es.exam_id, e.title, e.subject,
					es.schedule_date, es.start_time, es.end_time,
					es.room, es.supervisor_name,
					COALESCE(e.passing_score, 70),
					es.class_sub_group_id,
					es.total_students
				FROM exam_schedules es
				JOIN school_exams e ON es.exam_id = e.id
				WHERE es.id = $1 AND es.school_id = $2 AND es.deleted_at IS NULL
			`, scheduleID, schoolID).Scan(
				&s.ID, &s.ExamID, &s.ExamTitle, &s.Subject,
				&s.ScheduleDate, &s.StartTime, &s.EndTime,
				&s.Room, &s.SupervisorName,
				&s.PassingScore,
				&s.ClassSubGroupID,
				&s.TotalStudents,
			)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal tidak ditemukan"})
				return
			}

			// 2. Query semua siswa target + LEFT JOIN submissions
			rows, err := database.DB.Query(`
				SELECT 
					s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
					COALESCE(csg.name, ''),
					sub.id, sub.status,
					sub.total_score, sub.max_score, sub.percentage, sub.is_passed,
					sub.submitted_at, sub.time_spent_seconds,
					COALESCE(sub.tab_switch_count, 0),
					COALESCE(sub.is_flagged, false),
					sub.answers
				FROM students s
				LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
				LEFT JOIN exam_submissions sub 
					ON sub.student_id = s.id AND sub.schedule_id = $1
				WHERE s.school_id = $2 
				AND s.class_sub_group_id = (SELECT class_sub_group_id FROM exam_schedules WHERE id = $1)
				AND s.is_active = TRUE
				ORDER BY s.full_name ASC
			`, scheduleID, schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			type StudentRow struct {
				StudentID         string   `json:"student_id"`
				StudentName       string   `json:"student_name"`
				StudentNISN       string   `json:"student_nisn"`
				StudentNumber     string   `json:"student_number"`
				ClassSubGroupName string   `json:"class_sub_group_name"`
				SubmissionID      *string  `json:"submission_id"`
				Status            string   `json:"status"`
				TotalScore        *float64 `json:"total_score"`
				MaxScore          *float64 `json:"max_score"`
				Percentage        *float64 `json:"percentage"`
				IsPassed          *bool    `json:"is_passed"`
				SubmittedAt       *time.Time `json:"submitted_at"`
				TimeSpentSeconds  *int     `json:"time_spent_seconds"`
				TabSwitchCount    int      `json:"tab_switch_count"`
				IsFlagged         bool     `json:"is_flagged"`
				PendingEssayCount int      `json:"pending_essay_count"`
			}

			students := []StudentRow{}
			var submitted, graded, pending, notStarted, passed, failed, flagged int
			var totalPct float64
			var pctCount int
			var highest, lowest *float64

			for rows.Next() {
				var r StudentRow
				var subID sql.NullString
				var status sql.NullString
				var totalScore, maxScore, percentage sql.NullFloat64
				var isPassed sql.NullBool
				var submittedAt sql.NullTime
				var timeSpent sql.NullInt64
				var answersJSON []byte

				err := rows.Scan(
					&r.StudentID, &r.StudentName, &r.StudentNISN, &r.StudentNumber,
					&r.ClassSubGroupName,
					&subID, &status,
					&totalScore, &maxScore, &percentage, &isPassed,
					&submittedAt, &timeSpent,
					&r.TabSwitchCount, &r.IsFlagged,
					&answersJSON,
				)
				if err != nil {
					continue
				}

				if subID.Valid { r.SubmissionID = &subID.String }
				if status.Valid { r.Status = status.String } else { r.Status = "not_started" }
				if totalScore.Valid { r.TotalScore = &totalScore.Float64 }
				if maxScore.Valid { r.MaxScore = &maxScore.Float64 }
				if percentage.Valid {
					r.Percentage = &percentage.Float64
					totalPct += percentage.Float64
					pctCount++
					if highest == nil || percentage.Float64 > *highest { v := percentage.Float64; highest = &v }
					if lowest == nil || percentage.Float64 < *lowest { v := percentage.Float64; lowest = &v }
				}
				if isPassed.Valid { r.IsPassed = &isPassed.Bool }
				if submittedAt.Valid { r.SubmittedAt = &submittedAt.Time }
				if timeSpent.Valid { t := int(timeSpent.Int64); r.TimeSpentSeconds = &t }

				// Count pending essays
				if len(answersJSON) > 0 {
					var answers []map[string]any
					if json.Unmarshal(answersJSON, &answers) == nil {
						for _, a := range answers {
							if a["type"] == "essay" {
								if _, ok := a["score"]; !ok || a["score"] == nil {
									r.PendingEssayCount++
								}
							}
						}
					}
				}

				// Aggregate
				switch r.Status {
				case "in_progress": notStarted++ // in_progress tidak masuk "submitted"
				case "submitted":
					submitted++
				case "graded", "graded_with_pending":
					submitted++
					graded++
					if r.Status == "graded_with_pending" { pending++ }
				case "not_started":
					notStarted++
				}
				if r.IsPassed != nil && *r.IsPassed { passed++ }
				if r.IsPassed != nil && !*r.IsPassed { failed++ }
				if r.IsFlagged { flagged++ }

				students = append(students, r)
			}

			var avg *float64
			if pctCount > 0 {
				a := totalPct / float64(pctCount)
				avg = &a
			}

			c.JSON(http.StatusOK, gin.H{
				"schedule": gin.H{
					"id":               s.ID,
					"exam_id":          s.ExamID,
					"exam_title":       s.ExamTitle,
					"subject":          s.Subject,
					"schedule_date":    s.ScheduleDate.Format("2006-01-02"),
					"start_time":       s.StartTime.Format("15:04"),
					"end_time":         s.EndTime.Format("15:04"),
					"room":             s.Room.String,
					"supervisor_name":  s.SupervisorName.String,
					"passing_score":    s.PassingScore.Float64,
					"total_students":   s.TotalStudents,
				},
				"students": students,
				"stats": gin.H{
					"total":        len(students),
					"submitted":    submitted,
					"graded":       graded,
					"pending":      pending,
					"not_started":  notStarted,
					"passed":       passed,
					"failed":       failed,
					"flagged":      flagged,
					"average":      avg,
					"highest":      highest,
					"lowest":       lowest,
				},
			})
		})

		// ==========================================
		// BLOCK Student — admin sekolah blokir siswa dari ujian
		// POST /api/school-admin/exam-live-sessions/:sessionId/block
		// ==========================================
		api.POST("/school-admin/exam-live-sessions/:sessionId/block", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			var req BlockStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Ambil session info + validasi milik sekolah ini
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah dalam status diblokir"})
				return
			}

			// 2. Update DB
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = true,
					blocked_at = NOW(),
					blocked_by = $1,
					blocked_reason = $2,
					unblocked_at = NULL,
					unblocked_by = NULL,
					last_activity_at = NOW()
				WHERE id = $3
			`, adminID, req.Reason, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal blokir: " + err.Error()})
				return
			}

			// 3. Broadcast via Supabase Realtime (async — jangan block response)
			go func() {
				event := map[string]any{
					"type":          "block",
					"student_id":    studentID,
					"session_token": sessionToken,
					"reason":        req.Reason,
					"blocked_by":    adminID,
					"blocked_at":    time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[BLOCK] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Siswa %s berhasil diblokir", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
				"reason":       req.Reason,
			})
		})

		// ==========================================
		// UNBLOCK Student — cabut blokir
		// POST /api/school-admin/exam-live-sessions/:sessionId/unblock
		// ==========================================
		api.POST("/school-admin/exam-live-sessions/:sessionId/unblock", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			// 1. Validasi
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if !isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa tidak sedang diblokir"})
				return
			}

			// 2. Update DB
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					is_blocked = false,
					unblocked_at = NOW(),
					unblocked_by = $1,
					last_activity_at = NOW()
				WHERE id = $2
			`, adminID, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal unblock: " + err.Error()})
				return
			}

			// 3. Broadcast unblock
			go func() {
				event := map[string]any{
					"type":          "unblock",
					"student_id":    studentID,
					"session_token": sessionToken,
					"unblocked_by":  adminID,
					"unblocked_at":  time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[UNBLOCK] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Blokir untuk %s berhasil dicabut", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
			})
		})

		// ==========================================
		// WARN Student — kirim peringatan realtime
		// POST /api/school-admin/exam-live-sessions/:sessionId/warn
		// ==========================================
		api.POST("/school-admin/exam-live-sessions/:sessionId/warn", func(c *gin.Context) {
			schoolID, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			adminID, _ := getAdminIDFromUser(c)

			sessionID := c.Param("sessionId")

			var req WarnStudentRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
				return
			}

			// 1. Validasi
			var studentID, studentName, scheduleID, sessionToken string
			var isBlocked bool
			var submittedAt sql.NullTime

			err = database.DB.QueryRow(`
				SELECT 
					els.student_id, els.student_name_snapshot,
					els.schedule_id, els.session_token,
					els.is_blocked, els.submitted_at
				FROM exam_live_sessions els
				JOIN exam_schedules es ON es.id = els.schedule_id
				WHERE els.id = $1 AND es.school_id = $2
			`, sessionID, schoolID).Scan(
				&studentID, &studentName, &scheduleID, &sessionToken,
				&isBlocked, &submittedAt,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Sesi siswa tidak ditemukan"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if submittedAt.Valid {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sudah selesai mengerjakan"})
				return
			}

			if isBlocked {
				c.JSON(http.StatusConflict, gin.H{"error": "Siswa sedang diblokir, tidak bisa dikirim peringatan"})
				return
			}

			// 2. Update DB — track last warning
			_, err = database.DB.Exec(`
				UPDATE exam_live_sessions SET
					last_warning_at = NOW(),
					last_warning_message = $1,
					last_activity_at = NOW()
				WHERE id = $2
			`, req.Message, sessionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan peringatan: " + err.Error()})
				return
			}

			// 3. Broadcast warning
			go func() {
				event := map[string]any{
					"type":          "warning",
					"student_id":    studentID,
					"session_token": sessionToken,
					"message":       req.Message,
					"sent_by":       adminID,
					"sent_at":       time.Now().Format(time.RFC3339),
				}
				if bcErr := sendExamControlBroadcast(scheduleID, event); bcErr != nil {
					fmt.Printf("[WARN] broadcast error: %v\n", bcErr)
				}
			}()

			c.JSON(http.StatusOK, gin.H{
				"message":      fmt.Sprintf("Peringatan terkirim ke %s", studentName),
				"student_id":   studentID,
				"student_name": studentName,
				"schedule_id":  scheduleID,
			})
		})

		api.GET("/school-admin/exam-schedules/check-access-code", func(c *gin.Context) {
			_, err := getSchoolIDFromUser(c)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			code := strings.ToUpper(strings.TrimSpace(c.Query("code")))
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "code wajib diisi"})
				return
			}

			var count int
			err = database.DB.QueryRow(`
				SELECT COUNT(*) FROM exam_schedules 
				WHERE access_code = $1 AND deleted_at IS NULL
			`, code).Scan(&count)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"access_code": code,
				"available":   count == 0,
				"used_count":  count,
			})
		})

		//Udah terpisah dari route utama
		api.GET("/teacher/personal-students", teacher.ListPersonalStudents)
		api.GET("/teacher/personal-students/:id", teacher.GetPersonalStudent)
		api.POST("/teacher/personal-students", teacher.CreatePersonalStudent)
		api.PUT("/teacher/personal-students/:id", teacher.UpdatePersonalStudent)
		api.DELETE("/teacher/personal-students/:id", teacher.DeletePersonalStudent)

		api.GET("/teacher/personal-exams", teacher.ListPersonalExams)
		api.GET("/teacher/personal-exams/:id", teacher.GetPersonalExam)
		api.POST("/teacher/personal-exams", teacher.CreatePersonalExam)
		api.PUT("/teacher/personal-exams/:id", teacher.UpdatePersonalExam)
		api.POST("/teacher/personal-exams/:id/calculate-cost", teacher.CalculatePersonalExamCost)
		api.PATCH("/teacher/personal-exams/:id/publish", teacher.PublishPersonalExam)
		api.PATCH("/teacher/personal-exams/:id/archive", teacher.ArchivePersonalExam)
		api.DELETE("/teacher/personal-exams/:id", teacher.DeletePersonalExam)
		
		api.GET("/teacher/personal-exams/:id/participants", teacher.ListPersonalExamParticipants)
		api.POST("/teacher/personal-exams/:id/participants", teacher.AssignPersonalExamParticipants)
		api.DELETE("/teacher/personal-exams/:id/participants/:participantId", teacher.RemovePersonalExamParticipant)
		api.POST("/teacher/personal-exams/:id/generate-access-codes", teacher.GenerateAccessCodes)
		api.POST("/teacher/personal-exams/:id/participants/:participantId/regenerate-code", teacher.RegenerateParticipantCode)
		api.GET("/teacher/personal-exams/:id/qr-sheet", teacher.GetPersonalExamQRSheet)

		api.GET("/teacher/token-balance", teacher.GetTokenBalance)
		api.GET("/teacher/token-transactions", teacher.ListTokenTransactions)
		api.GET("/teacher/token-pricing", teacher.ListTokenPricing)

		api.POST("/teacher/ai/generate-questions", teacher.GenerateQuestionsWithCharge)
		api.GET("/teacher/ai/generate-cost", teacher.GetAIGenerateCost)

		

	}
}