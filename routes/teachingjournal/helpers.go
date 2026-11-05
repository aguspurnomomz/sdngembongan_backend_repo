package teachingjournal

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"gembongan_dev_backend/database"
	"time"

	"github.com/gin-gonic/gin"
)

func nowJakarta() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return time.Now().In(loc)
}

func normalizeUserID(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

func writeInternalError(c *gin.Context, publicMsg string, err error) {
	if err != nil {
		log.Printf("[TEACHING JOURNAL] %s | detail: %v", publicMsg, err)
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{"code": "INTERNAL_ERROR", "message": publicMsg},
	})
}

func resolveTeacherID(userID, schoolID string) (teacherID, resolvedSchoolID string, err error) {
	query := `SELECT id, school_id FROM teachers WHERE user_id = $1 AND is_active = TRUE`
	args := []any{userID}

	if schoolID != "" {
		query += ` AND school_id = $2`
		args = append(args, schoolID)
	}
	query += ` LIMIT 1`

	err = database.DB.QueryRow(query, args...).Scan(&teacherID, &resolvedSchoolID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("Anda belum terdaftar sebagai guru aktif di sekolah ini")
		}
		return "", "", err
	}
	return teacherID, resolvedSchoolID, nil
}

// validateTeacherOwnsSlot memvalidasi bahwa user adalah guru pemilik slot jadwal
// pada tanggal tertentu dan mengembalikan snapshot konteks.
func validateTeacherOwnsSlot(userID, entryID, journalDate string) (*journalSlotContext, error) {
	// 1. Cari teacher_id
	teacherID, schoolID, err := resolveTeacherID(userID, "")
	if err != nil {
		return nil, err
	}

	ctx := &journalSlotContext{TeacherID: teacherID, SchoolID: schoolID}

	var (
		academicYearID, classGroupID, classSubGroupID sql.NullString
		subjectID, subjectName, className             sql.NullString
		startTime, endTime                            sql.NullString
	)
	err = database.DB.QueryRow(`
		SELECT
			te.id,
			te.timetable_id,
			te.day_of_week,
			te.period_start,
			te.period_end,
			COALESCE(te.subject_id::text,''),
			COALESCE(te.subject_name_snapshot,''),
			ct.academic_year_id,
			ct.class_group_id,
			ct.class_sub_group_id,
			COALESCE(csg.name,''),
			to_char(ts.start_time,'HH24:MI'),
			to_char(ts.end_time,'HH24:MI')
		FROM timetable_entries te
		JOIN class_timetables ct ON ct.id = te.timetable_id
		LEFT JOIN class_sub_groups csg ON csg.id = ct.class_sub_group_id
		LEFT JOIN school_time_slots ts
		       ON ts.school_id = ct.school_id AND ts.period_number = te.period_start
		WHERE te.id = $1
		  AND te.teacher_id = $2
		  AND ct.status = 'active'
	`, entryID, teacherID).Scan(
		&ctx.TimetableEntryID, &ctx.TimetableID, &ctx.DayOfWeek,
		&ctx.PeriodStart, &ctx.PeriodEnd,
		&subjectID, &subjectName,
		&academicYearID, &classGroupID, &classSubGroupID,
		&className, &startTime, &endTime,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Slot jadwal tidak ditemukan atau bukan milik Anda / jadwal tidak aktif")
		}
		return nil, err
	}

	ctx.SubjectID = subjectID.String
	ctx.SubjectName = subjectName.String
	ctx.ClassName = className.String
	ctx.AcademicYearID = academicYearID.String
	ctx.ClassGroupID = classGroupID.String
	ctx.ClassSubGroupID = classSubGroupID.String
	ctx.ScheduledStart = startTime.String
	ctx.ScheduledEnd = endTime.String


	t, err := time.Parse("2006-01-02", journalDate)
	if err != nil {
		return nil, fmt.Errorf("Format tanggal tidak valid (YYYY-MM-DD)")
	}
	isoDow := int(t.Weekday())
	if isoDow == 0 {
		isoDow = 7
	}
	if isoDow != ctx.DayOfWeek {
		return nil, fmt.Errorf("Tanggal tidak sesuai dengan hari slot jadwal (jadwal hari ke-%d)", ctx.DayOfWeek)
	}

	var effFrom, effTo sql.NullTime
	_ = database.DB.QueryRow(
		`SELECT effective_from, effective_to FROM class_timetables WHERE id = $1`,
		ctx.TimetableID,
	).Scan(&effFrom, &effTo)
	if effFrom.Valid && t.Before(effFrom.Time) {
		return nil, fmt.Errorf("Tanggal sebelum jadwal berlaku (%s)", effFrom.Time.Format("2006-01-02"))
	}
	if effTo.Valid && t.After(effTo.Time) {
		return nil, fmt.Errorf("Tanggal setelah jadwal berakhir (%s)", effTo.Time.Format("2006-01-02"))
	}

	var isHoliday bool
	_ = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM academic_calendar
			WHERE school_id = $1
			  AND calendar_date = $2::date
			  AND (day_type = 'holiday' OR is_attendance_required = FALSE)
		)
	`, schoolID, journalDate).Scan(&isHoliday)
	if isHoliday {
		return nil, fmt.Errorf("Tanggal tersebut adalah hari libur menurut kalender akademik")
	}

	return ctx, nil
}

func parseHHMM(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("15:04"), nil
		}
	}
	return "", fmt.Errorf("format waktu tidak valid (harus HH:MM): %s", s)
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}