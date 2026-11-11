package routes

import (
	"net/http"
	
	"gembongan_dev_backend/internal/middleware"
	"gembongan_dev_backend/routes/exam"
	"gembongan_dev_backend/routes/payment"
	"gembongan_dev_backend/routes/public"
	"gembongan_dev_backend/routes/auth"

	"gembongan_dev_backend/routes/questionbank"
	"gembongan_dev_backend/routes/school"
	"gembongan_dev_backend/routes/teacher"
	"gembongan_dev_backend/routes/teachingjournal"
	"gembongan_dev_backend/routes/attendance"
	"gembongan_dev_backend/routes/profile"

	"gembongan_dev_backend/routes/ebook"
	"gembongan_dev_backend/routes/upload"
	"gembongan_dev_backend/routes/membership"

	"github.com/gin-gonic/gin"
	
	"gembongan_dev_backend/routes/superadmin"
)




func SetupRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "Server berjalan dengan baik"})
	})

	r.GET("/debug-sentry", func(c *gin.Context) {
		panic("Test Sentry Error dari Backend Golang!")
	})

	r.POST("/api/payment/webhook", payment.Webhook)
	r.POST("/api/superadmin/superadmin-login", superadmin.Login)


	adminApi := r.Group("/api/superadmin")
	adminApi.Use(middleware.AdminAuthMiddleware())
	{
		adminApi.GET("/stats", superadmin.Stats)
		adminApi.GET("/logs", superadmin.ListLogs)
		adminApi.POST("/change-password", superadmin.ChangePassword)
		// TODO Struct users baru 27 Oct 2026
		adminApi.GET("/registered-users", superadmin.ListRegisteredUsers)
		adminApi.PATCH("/users/:id/status", superadmin.UpdateUserStatus)
		// TODO Struct ebooks 27 Oct 2026
		adminApi.GET("/ebooks", superadmin.ListEbooks)
		adminApi.POST("/ebooks", superadmin.CreateEbook)
		adminApi.DELETE("/ebooks/:id", superadmin.DeleteEbook)
		// TODO Struct schools baru 27 Oct 2026
		adminApi.GET("/schools", superadmin.ListSchools)
		adminApi.POST("/schools", superadmin.CreateSchool)
		adminApi.PATCH("/schools/:id/status", superadmin.UpdateSchoolStatus)
		adminApi.GET("/school-admins", superadmin.ListSchoolAdmins)
		adminApi.POST("/school-admins", superadmin.CreateSchoolAdmin)
		// TODO Struct schools role baru 27 Oct 2026
		adminApi.GET("/school-roles", superadmin.ListSchoolRoles)
		adminApi.POST("/school-roles", superadmin.CreateSchoolRole)
		adminApi.PUT("/school-roles/:id", superadmin.UpdateSchoolRole)
		adminApi.DELETE("/school-roles/:id", superadmin.DeleteSchoolRole)
		// TODO Struct school role permissions 27 Oct 2026
		adminApi.GET("/permissions", superadmin.ListPermissions)
		adminApi.GET("/school-roles/:id/permissions", superadmin.GetRolePermissions)
		adminApi.PUT("/school-roles/:id/permissions", superadmin.UpdateRolePermissions)
	}
	// Endpoint yang diakses siswa
	apiPub := r.Group("/api")
	{
		apiPub.GET("/public/personal-exam/access/:code", public.GetPersonalExamByCode)
		apiPub.POST("/public/personal-exam/start", public.StartPersonalExam)
		apiPub.POST("/public/personal-exam/submit", public.SubmitPersonalExam)
		apiPub.GET("/public/personal-exam/result/:sessionToken", public.GetPersonalExamResult)
		// School Exam (Siswa)
		apiPub.GET("/exam/session-questions", public.GetExamSessionQuestions)
		apiPub.POST("/exam/submit", public.SubmitExam)
		apiPub.GET("/exam/check-student", public.CheckStudentExamStatus)
		apiPub.GET("/exam/:sessionToken/status", public.GetExamSessionStatus)
		apiPub.POST("/exam/start", public.StartExam)
		apiPub.POST("/exam/:sessionToken/submit", public.SubmitLiveExam)
	}

	// Route Protected 
	// Hanya guru yang login yang bisa mengakses data miliknya sendiri
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/auth/check-role", CheckRoleHandler)
		// SCHOOL ADMIN MANAGE TEACHER ROLES
		api.POST("/school-admin/teachers/:id/roles", AssignRoleToTeacher)
		api.DELETE("/school-admin/teachers/:id/roles/:roleId", RemoveRoleFromTeacher)
		api.GET("/school-admin/teachers/:id/roles", GetTeacherRoles)
		// TEACHING JOURNAL — guru
		api.GET("/teacher/journals/schedule", teachingjournal.GetTeacherSchedule)
		api.GET("/teacher/journals", teachingjournal.ListTeacherJournals)
		api.GET("/teacher/journals/stats", teachingjournal.GetTeacherJournalStats)
		api.GET("/teacher/journals/:id", teachingjournal.GetTeachingJournal)
		api.POST("/teacher/journals", teachingjournal.CreateTeachingJournal)
		api.PUT("/teacher/journals/:id", teachingjournal.UpdateTeachingJournal)
		api.DELETE("/teacher/journals/:id", teachingjournal.DeleteTeachingJournal)
		api.POST("/teacher/journals/:id/submit", teachingjournal.SubmitTeachingJournal)
		// TEACHING JOURNAL — admin sekolah
		api.GET("/school-admin/journals", teachingjournal.ListSchoolJournals)
		api.GET("/school-admin/journals/rekap", teachingjournal.GetSchoolJournalRecap)
		api.PATCH("/school-admin/journals/:id/verify", teachingjournal.VerifyTeachingJournal)
		// ambil dari tabel auth supabasenya
		//TODO 11 Nov 2026 notify changed pass
		api.POST("/auth/notify-password-changed", auth.NotifyPasswordChanged)
		// TODO Midtrans
		api.POST("/payment/create-snap", payment.CreateSnap)
		//Todo update 11 Nov 2026 Profile
		api.GET("/profile", profile.GetProfile)
		api.PUT("/profile", profile.UpdateProfile)
		//Todo update 11 Nov 2026 question banks OLD
		api.POST("/question-banks", questionbank.CreateQuestionBank)
		api.GET("/my-question-banks", questionbank.ListMyQuestionBanks)
		//Todo update 11 Nov 2026 ebook
		api.GET("/ebooks-list", ebook.ListEbooks)
		api.POST("/ebooks-list/history", ebook.RecordHistory)
		// Cloudflare R2 Object Storage — generate presigned URL 11 Nov Edited
		api.GET("/upload/presigned-url", upload.PresignedURL)
	
		api.GET("/school-admin/academic-years", school.GetAcademicYears)
		api.POST("/school-admin/academic-years", school.CreateAcademicYear)
		api.PATCH("/school-admin/academic-years/:id/activate", school.ActivateAcademicYear)
		api.GET("/school-admin/profile", school.GetSchoolProfile)
		api.PUT("/school-admin/profile", school.UpdateSchoolProfile)
		api.GET("/school-admin/classes", school.GetClasses)
		api.POST("/school-admin/classes", school.CreateClass)
		api.DELETE("/school-admin/classes/:id", school.DeleteClass)
		api.GET("/school-admin/classes/:classId/sub-classes", school.GetSubClassesByClass)
		api.GET("/school-admin/sub-classes", school.GetAllSubClasses)
		api.POST("/school-admin/sub-classes", school.CreateSubClass)
		api.PUT("/school-admin/sub-classes/:id", school.UpdateSubClass)
		api.DELETE("/school-admin/sub-classes/:id", school.DeleteSubClass)
		api.GET("/school-admin/students", school.GetStudents)
		api.POST("/school-admin/students", school.CreateStudent)
		api.PUT("/school-admin/students/:id", school.UpdateStudent)
		api.DELETE("/school-admin/students/:id", school.DeleteStudent)
		api.GET("/school-admin/students/stats", school.GetStudentStats)
		api.GET("/school-admin/students/gender-stats", school.GetStudentGenderStats)
		api.GET("/school-admin/calendar", school.GetCalendarEvents)
		api.POST("/school-admin/calendar", school.CreateCalendarEvent)
		api.PUT("/school-admin/calendar/:id", school.UpdateCalendarEvent)
		api.DELETE("/school-admin/calendar/:id", school.DeleteCalendarEvent)
		api.GET("/school-admin/dashboard-stats", school.GetDashboardStats)

		api.GET("/school-admin/attendance/shifts", attendance.GetShifts)
		api.POST("/school-admin/attendance/shifts", attendance.CreateShift)
		api.PUT("/school-admin/attendance/shifts/:id", attendance.UpdateShift)
		api.DELETE("/school-admin/attendance/shifts/:id", attendance.DeleteShift)
		// List assignments
		api.GET("/school-admin/attendance/assignments", attendance.GetClassAssignments)
		api.POST("/school-admin/attendance/assignments", attendance.CreateClassAssignment)
		api.DELETE("/school-admin/attendance/assignments/:id", attendance.DeleteClassAssignment)
		// List QR codes siswa
		api.GET("/school-admin/students/qr-list", attendance.GetStudentQRList)
		api.POST("/school-admin/students/:id/generate-qr", attendance.GenerateSingleQR)
		api.POST("/school-admin/students/generate-qr-bulk", attendance.GenerateBulkQR)
		api.POST("/school-admin/students/:id/regenerate-qr", attendance.RegenerateQR)

		api.GET("/school-admin/attendance/sessions/active-today", attendance.GetActiveTodaySession)
		api.GET("/school-admin/attendance/sessions", attendance.GetAttendanceSessions)
		api.GET("/school-admin/attendance/sessions/today", attendance.GetTodaySessions)
		api.GET("/school-admin/attendance/sessions/:id", attendance.GetSessionDetail)
		api.POST("/school-admin/attendance/sessions", attendance.CreateAttendanceSession)
		api.POST("/school-admin/attendance/sessions/:id/open", attendance.OpenSession)
		api.POST("/school-admin/attendance/sessions/:id/close", attendance.CloseSession)
		api.DELETE("/school-admin/attendance/sessions/:id", attendance.DeleteSession)
		api.GET("/school-admin/attendance/sessions/:id/records", attendance.GetSessionRecords)
		api.POST("/school-admin/attendance/scan", attendance.ScanQR)
		api.POST("/school-admin/attendance/scan-rfid", HandleScanRFID)
		// RFID CRUD
		api.GET("/school-admin/rfid-cards", attendance.HandleListRFIDCards)
		api.POST("/school-admin/rfid-cards", attendance.HandleBulkRegisterRFID)
		api.POST("/school-admin/rfid-cards/bulk-register", attendance.HandleBulkRegisterRFID)
		api.DELETE("/school-admin/rfid-cards/:id", attendance.HandleDeleteRFID)
		api.POST("/school-admin/rfid-cards/:id/revoke", attendance.HandleRevokeRFID)
		// DEVICES / KIOSK
		api.GET("/school-admin/attendance/devices", attendance.HandleListDevices)
		api.POST("/school-admin/attendance/devices", attendance.HandleCreateDevice)
		api.PATCH("/school-admin/attendance/devices/:id", attendance.HandleUpdateDevice)
		api.DELETE("/school-admin/attendance/devices/:id", attendance.HandleDeleteDevice)
		// Public endpoint untuk kiosk (device-code auth, tidak butuh login admin)
		apiPub.GET("/kiosk/:code/active-session", attendance.KioskGetActiveSession)
		apiPub.POST("/kiosk/:code/scan-rfid", attendance.KioskScanRFID)

		api.POST("/teacher/join-school", membership.JoinSchool)
		api.POST("/teacher/leave-school", membership.LeaveSchool)
		api.GET("/teacher/my-schools", membership.ListMySchools)
		api.GET("/teacher/available-schools", membership.ListAvailableSchools)
	
		api.GET("/school-admin/teachers", teacher.ListSchoolTeachers)
		api.POST("/school-admin/teachers/invite", teacher.InviteTeacher)
		api.GET("/school-admin/school-roles", school.ListSchoolRoles)
		api.POST("/school-admin/teachers", teacher.CreateTeacher)
		api.PUT("/school-admin/teachers/:id", teacher.UpdateTeacher)
		api.POST("/school-admin/teachers/assign", teacher.AssignTeacher)
		api.DELETE("/school-admin/teachers/:id", teacher.RemoveTeacher)
		api.GET("/school-admin/subjects", school.ListSubjects)
		api.POST("/school-admin/subjects", school.CreateSubject)
		api.PUT("/school-admin/subjects/:id", school.UpdateSubject)
		api.DELETE("/school-admin/subjects/:id", school.DeleteSubject)
		api.GET("/school-admin/subjects/:id", school.GetSubjectDetail)
		api.POST("/school-admin/teachers/:id/subjects", school.AssignSubjectsToTeacher)
		api.GET("/school-admin/teachers/:id/subjects", school.GetTeacherSubjects)
		api.DELETE("/school-admin/teachers/:id/subjects/:subjectId", school.RemoveTeacherSubject)
		api.GET("/school-admin/timetables", school.ListTimetables)
		api.POST("/school-admin/timetables", school.CreateTimetable)
		api.GET("/school-admin/timetables/:id", school.GetTimetableDetail)
		api.DELETE("/school-admin/timetables/:id", school.DeleteTimetable)
		api.PATCH("/school-admin/timetables/:id/activate", school.ActivateTimetable)
		api.POST("/school-admin/timetables/:id/entries", school.CreateTimetableEntry)
		api.DELETE("/school-admin/timetables/:id/entries/:entryId", school.DeleteTimetableEntry)
		api.POST("/school-admin/timetables/:id/validate", school.ValidateTimetable)
		// ===== Time Slots =========
		api.GET("/school-admin/time-slots", school.ListTimeSlots)
		api.POST("/school-admin/time-slots", school.CreateTimeSlot)
		api.PUT("/school-admin/time-slots/:id", school.UpdateTimeSlot)
		api.DELETE("/school-admin/time-slots/:id", school.DeleteTimeSlot)
		// ===== Time Slots Overrides ====
		api.GET("/school-admin/time-slot-overrides", school.ListTimeSlotOverrides)
		api.POST("/school-admin/time-slot-overrides", school.UpsertTimeSlotOverride)
		api.PUT("/school-admin/time-slot-overrides/:id", school.UpdateTimeSlotOverride)
		api.DELETE("/school-admin/time-slot-overrides/:id", school.DeleteTimeSlotOverride)
		// ===== Exams School =====
		api.GET("/school-admin/exams", exam.ListExamsDraft)
		api.GET("/school-admin/exams/:id", exam.ListExams)
		api.POST("/school-admin/exams", exam.PostExams)
		api.PUT("/school-admin/exams/:id", exam.UpdateExams)
		api.PATCH("/school-admin/exams/:id/publish", exam.PublishExams)
		api.PATCH("/school-admin/exams/:id/archive", exam.ArchiveExams)
		api.DELETE("/school-admin/exams/:id", exam.ArchiveExams)
		api.GET("/school-admin/exams/target-options", exam.ExamTargetClass)
		api.GET("/school-admin/exam-schedules", exam.ExamSchedulesList)
		api.GET("/school-admin/exam-schedules/available-targets", exam.ExamSchedulesTarget)
		api.POST("/school-admin/exam-schedules", exam.ExamSchedulesCreate)
		api.GET("/school-admin/exam-schedules/:id/monitor", exam.ExamSchedulesMonitoring)
		api.PATCH("/school-admin/exam-schedules/:id/close", exam.ExamCloseSessions)
		api.GET("/school-admin/exam-schedules/:id", exam.ExamSchedulesDetail)
		api.PATCH("/school-admin/exam-schedules/:id/cancel", exam.ExamSchedulesCancel)
		api.DELETE("/school-admin/exam-schedules/:id", exam.ExamSchedulesDelete)
		api.GET("/school-admin/exam-submissions/:id", exam.ExamSubmissionDetail)
		api.PATCH("/school-admin/exam-submissions/:id/grade-essay", exam.ExamGradeEssay)
		api.GET("/school-admin/exam-schedules/:id/grades", exam.ExamGradeRecap)
		api.GET("/school-admin/exam-schedules/check-access-code", exam.ExamSchedulesAccesCode)
		// BLOCK Student — admin sekolah blokir siswa dari ujian
		api.POST("/school-admin/exam-live-sessions/:sessionId/block", exam.StudentExamBlock)
		api.POST("/school-admin/exam-live-sessions/:sessionId/unblock", exam.StudentExamOpenBlock)
		api.POST("/school-admin/exam-live-sessions/:sessionId/warn", exam.StudentExamWarnRealtime)
		// ===== exam room=====
		api.GET("/school-admin/exam-rooms", exam.ListExamRooms)
		api.POST("/school-admin/exam-rooms", exam.CreateExamRoom)
		api.PUT("/school-admin/exam-rooms/:id", exam.UpdateExamRoom)
		api.DELETE("/school-admin/exam-rooms/:id", exam.DeleteExamRoom)
		api.GET("/school-admin/exam-schedules/:id/students-placement", exam.GetStudentsPlacementData)
		api.POST("/school-admin/exam-schedules/:id/assign-room", exam.AssignStudentsToRoom)
		api.POST("/school-admin/exam-schedules/:id/auto-assign-rooms", exam.AutoAssignStudentsToRooms)
		api.DELETE("/school-admin/exam-schedules/:id/rooms/:roomId/students/:studentId", exam.RemoveStudentFromRoom)
		// ===== exam =====
		api.GET("/school-admin/exam-schedules/:id/cards", exam.GetExamCardsData)
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