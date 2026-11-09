package exam

import (
	"database/sql"
	"fmt"
	"net/http"
	"gembongan_dev_backend/database"
	"time"

	"github.com/gin-gonic/gin"
)

// ==========================================
// STRUCT REQUEST
// ==========================================
type CreateExamRoomRequest struct {
	RoomName     string `json:"room_name" binding:"required"`
	LocationNote string `json:"location_note"`
	Capacity     int    `json:"capacity" binding:"required,min=1"`
}

type UpdateExamRoomRequest struct {
	RoomName     string `json:"room_name"`
	LocationNote string `json:"location_note"`
	Capacity     int    `json:"capacity"`
	IsActive     *bool  `json:"is_active"`
}

type AssignStudentsRequest struct {
	StudentIDs []string `json:"student_ids" binding:"required,min=1"`
	RoomID     string   `json:"room_id" binding:"required"`
}

// LIST RUANGAN (GET /school-admin/exam-rooms)
func ListExamRooms(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c) // pastikan helper ini ada di package Anda
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, room_name, COALESCE(location_note, ''), capacity, is_active, created_at
		FROM exam_rooms
		WHERE school_id = $1
		ORDER BY is_active DESC, room_name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat ruangan: " + err.Error()})
		return
	}
	defer rows.Close()

	type RoomItem struct {
		ID           string    `json:"id"`
		RoomName     string    `json:"room_name"`
		LocationNote string    `json:"location_note"`
		Capacity     int       `json:"capacity"`
		IsActive     bool      `json:"is_active"`
		CreatedAt    time.Time `json:"created_at"`
	}

	rooms := []RoomItem{}
	for rows.Next() {
		var r RoomItem
		if err := rows.Scan(&r.ID, &r.RoomName, &r.LocationNote, &r.Capacity, &r.IsActive, &r.CreatedAt); err == nil {
			rooms = append(rooms, r)
		}
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms, "total": len(rooms)})
}

// CREATE RUANGAN (POST /school-admin/exam-rooms)
func CreateExamRoom(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req CreateExamRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var roomID string
	err = database.DB.QueryRow(`
		INSERT INTO exam_rooms (school_id, room_name, location_note, capacity)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		RETURNING id
	`, schoolID, req.RoomName, req.LocationNote, req.Capacity).Scan(&roomID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan ruangan: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ruangan berhasil dibuat",
		"id":      roomID,
	})
}

// UPDATE RUANGAN (PUT /school-admin/exam-rooms/:id)
func UpdateExamRoom(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	roomID := c.Param("id")
	var req UpdateExamRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	var isActive interface{} = nil
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	result, err := database.DB.Exec(`
		UPDATE exam_rooms SET
			room_name = COALESCE(NULLIF($1, ''), room_name),
			location_note = COALESCE(NULLIF($2, ''), location_note),
			capacity = COALESCE(NULLIF($3, 0), capacity),
			is_active = COALESCE($4, is_active),
			updated_at = NOW()
		WHERE id = $5 AND school_id = $6
	`, req.RoomName, req.LocationNote, req.Capacity, isActive, roomID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update: " + err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ruangan tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ruangan berhasil diperbarui"})
}

// DELETE RUANGAN (DELETE /school-admin/exam-rooms/:id)
func DeleteExamRoom(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	roomID := c.Param("id")

	// Cek apakah sedang dipakai di jadwal ujian yang belum selesai
	var activeUsage int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM exam_room_assignments era
		JOIN exam_schedules es ON era.schedule_id = es.id
		WHERE era.room_id = $1 AND es.status IN ('scheduled', 'ongoing')
	`, roomID).Scan(&activeUsage)

	if activeUsage > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Tidak bisa hapus: ruangan sedang dipakai di %d jadwal ujian aktif", activeUsage),
		})
		return
	}

	result, err := database.DB.Exec(`DELETE FROM exam_rooms WHERE id = $1 AND school_id = $2`, roomID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ruangan tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ruangan berhasil dihapus"})
}

// GET DATA PLOTTING (GET /school-admin/exam-schedules/:scheduleId/students-placement)
// Mengembalikan: daftar siswa UNASSIGNED + daftar siswa yang SUDAH masuk ruangan
func GetStudentsPlacementData(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	scheduleID := c.Param("id")

	// 1. Ambil info schedule (untuk tahu target class-nya)
	var classGroupID, classSubGroupID sql.NullString
	err = database.DB.QueryRow(`
		SELECT class_group_id, class_sub_group_id
		FROM exam_schedules
		WHERE id = $1 AND school_id = $2
	`, scheduleID, schoolID).Scan(&classGroupID, &classSubGroupID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal ujian tidak ditemukan"})
		return
	}

	// 2. Ambil semua siswa target (belum di-assign)
	query := `
		SELECT s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
			COALESCE(csg.name, ''), COALESCE(cg.name, '')
		FROM students s
		LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
		LEFT JOIN class_groups cg ON s.class_group_id = cg.id
		WHERE s.school_id = $1 AND s.is_active = TRUE
		AND NOT EXISTS (
			SELECT 1 FROM exam_room_assignments era
			WHERE era.student_id = s.id AND era.schedule_id = $2
		)
	`
	args := []any{schoolID, scheduleID}
	argIdx := 3

	if classSubGroupID.Valid {
		query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx)
		args = append(args, classSubGroupID.String)
		argIdx++
	} else if classGroupID.Valid {
		query += fmt.Sprintf(" AND s.class_group_id = $%d", argIdx)
		args = append(args, classGroupID.String)
		argIdx++
	}
	query += " ORDER BY s.full_name ASC"

	unassignedRows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat siswa: " + err.Error()})
		return
	}
	defer unassignedRows.Close()

	type StudentItem struct {
		ID            string `json:"id"`
		FullName      string `json:"full_name"`
		NISN          string `json:"nisn"`
		StudentNumber string `json:"student_number"`
		SubClassName  string `json:"sub_class_name"`
		ClassName     string `json:"class_name"`
	}

	unassigned := []StudentItem{}
	for unassignedRows.Next() {
		var s StudentItem
		if err := unassignedRows.Scan(&s.ID, &s.FullName, &s.NISN, &s.StudentNumber, &s.SubClassName, &s.ClassName); err == nil {
			unassigned = append(unassigned, s)
		}
	}

	// 3. Ambil siswa yang sudah di-assign beserta info ruangan
	assignedRows, err := database.DB.Query(`
		SELECT era.id, era.student_id, s.full_name, COALESCE(s.nisn, ''),
			COALESCE(era.seat_number, ''),
			er.id AS room_id, er.room_name
		FROM exam_room_assignments era
		JOIN students s ON era.student_id = s.id
		JOIN exam_rooms er ON era.room_id = er.id
		WHERE era.schedule_id = $1 AND er.school_id = $2
		ORDER BY er.room_name ASC, era.seat_number ASC
	`, scheduleID, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat siswa ter-plot: " + err.Error()})
		return
	}
	defer assignedRows.Close()

	type AssignedItem struct {
		AssignmentID string `json:"assignment_id"`
		StudentID    string `json:"student_id"`
		FullName     string `json:"full_name"`
		NISN         string `json:"nisn"`
		SeatNumber   string `json:"seat_number"`
		RoomID       string `json:"room_id"`
		RoomName     string `json:"room_name"`
	}

	assigned := []AssignedItem{}
	for assignedRows.Next() {
		var a AssignedItem
		if err := assignedRows.Scan(&a.AssignmentID, &a.StudentID, &a.FullName, &a.NISN, &a.SeatNumber, &a.RoomID, &a.RoomName); err == nil {
			assigned = append(assigned, a)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"unassigned_students": unassigned,
		"assigned_students":   assigned,
	})
}

// ASSIGN SISWA KE RUANGAN (POST /school-admin/exam-schedules/:scheduleId/assign-room)
func AssignStudentsToRoom(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	scheduleID := c.Param("scheduleId")

	var req AssignStudentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	// Validasi ruangan milik sekolah
	var roomCapacity int
	err = database.DB.QueryRow(`
		SELECT capacity FROM exam_rooms WHERE id = $1 AND school_id = $2 AND is_active = TRUE
	`, req.RoomID, schoolID).Scan(&roomCapacity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ruangan tidak valid atau tidak aktif"})
		return
	}

	// Cek kapasitas saat ini
	var currentCount int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM exam_room_assignments WHERE room_id = $1 AND schedule_id = $2
	`, req.RoomID, scheduleID).Scan(&currentCount)

	if currentCount+len(req.StudentIDs) > roomCapacity {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Kapasitas ruangan tidak cukup. Tersisa %d kursi, mencoba assign %d siswa.",
				roomCapacity-currentCount, len(req.StudentIDs)),
		})
		return
	}

	// Ambil nomor kursi terakhir untuk auto-increment
	var lastSeat string
	_ = database.DB.QueryRow(`
		SELECT COALESCE(seat_number, '')
		FROM exam_room_assignments
		WHERE room_id = $1 AND schedule_id = $2 AND seat_number IS NOT NULL
		ORDER BY seat_number DESC LIMIT 1
	`, req.RoomID, scheduleID).Scan(&lastSeat)

	nextNumber := currentCount + 1

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mulai transaksi"})
		return
	}
	defer tx.Rollback()

	inserted := 0
	for _, sid := range req.StudentIDs {
		seatNum := fmt.Sprintf("A-%02d", nextNumber)
		_, err := tx.Exec(`
			INSERT INTO exam_room_assignments (schedule_id, room_id, student_id, seat_number)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (schedule_id, student_id) DO NOTHING
		`, scheduleID, req.RoomID, sid, seatNum)
		if err == nil {
			inserted++
			nextNumber++
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("%d siswa berhasil ditempatkan", inserted),
		"inserted": inserted,
	})
}

// REMOVE SISWA DARI RUANGAN (DELETE /school-admin/exam-schedules/:scheduleId/rooms/:roomId/students/:studentId)
func RemoveStudentFromRoom(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	scheduleID := c.Param("id") 
	roomID := c.Param("roomId")
	studentID := c.Param("studentId")

	_, err = database.DB.Exec(`
		DELETE FROM exam_room_assignments
		WHERE schedule_id = $1 AND room_id = $2 AND student_id = $3
		AND room_id IN (SELECT id FROM exam_rooms WHERE school_id = $4)
	`, scheduleID, roomID, studentID, schoolID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Siswa dikeluarkan dari ruangan"})
}

// GET KARTU UJIAN (GET /school-admin/exam-schedules/:scheduleId/cards)
// Query param opsional: ?room_id=xxx  (filter per ruangan)
func GetExamCardsData(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	scheduleID := c.Param("id")
	roomFilter := c.Query("room_id") // opsional

	// 1. Info Exam + School
	var examTitle, subject, schoolName string
	var scheduleDate time.Time
	var startTime, endTime time.Time

	err = database.DB.QueryRow(`
		SELECT e.title, e.subject, s.school_name,
			es.schedule_date, es.start_time, es.end_time
		FROM exam_schedules es
		JOIN school_exams e ON es.exam_id = e.id
		JOIN schools s ON es.school_id = s.id
		WHERE es.id = $1 AND es.school_id = $2
	`, scheduleID, schoolID).Scan(
		&examTitle, &subject, &schoolName, &scheduleDate, &startTime, &endTime,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal ujian tidak ditemukan"})
		return
	}

	// 2. Query Kartu (join siswa + ruangan + assignment)
	query := `
		SELECT 
			s.id, s.full_name, COALESCE(s.nisn, ''), COALESCE(s.student_number, ''),
			COALESCE(csg.name, ''), COALESCE(cg.name, ''),
			er.room_name, COALESCE(era.seat_number, '')
		FROM exam_room_assignments era
		JOIN students s ON era.student_id = s.id
		JOIN exam_rooms er ON era.room_id = er.id
		LEFT JOIN class_sub_groups csg ON s.class_sub_group_id = csg.id
		LEFT JOIN class_groups cg ON s.class_group_id = cg.id
		WHERE era.schedule_id = $1 AND er.school_id = $2
	`
	args := []any{scheduleID, schoolID}

	if roomFilter != "" {
		query += " AND era.room_id = $3"
		args = append(args, roomFilter)
	}
	query += " ORDER BY er.room_name ASC, era.seat_number ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type CardItem struct {
		StudentID     string `json:"student_id"`
		FullName      string `json:"full_name"`
		NISN          string `json:"nisn"`
		StudentNumber string `json:"student_number"`
		SubClassName  string `json:"sub_class_name"`
		ClassName     string `json:"class_name"`
		RoomName      string `json:"room_name"`
		SeatNumber    string `json:"seat_number"`
	}

	cards := []CardItem{}
	for rows.Next() {
		var c CardItem
		if err := rows.Scan(&c.StudentID, &c.FullName, &c.NISN, &c.StudentNumber, &c.SubClassName, &c.ClassName, &c.RoomName, &c.SeatNumber); err == nil {
			cards = append(cards, c)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"school_name": schoolName,
		"exam_info": gin.H{
			"title":       examTitle,
			"subject":     subject,
			"date":        scheduleDate.Format("2006-01-02"),
			"start_time":  startTime.Format("15:04"),
			"end_time":    endTime.Format("15:04"),
		},
		"total_cards": len(cards),
		"cards":       cards,
	})
}

// AUTO ASSIGN SISWA KE SEMUA RUANGAN (POST /school-admin/exam-schedules/:scheduleId/auto-assign-rooms)
func AutoAssignStudentsToRooms(c *gin.Context) {
	schoolID, err := getSchoolIDFromUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	scheduleID := c.Param("id")

	// 1. Ambil semua ruangan aktif, urutkan nama
	type Room struct {
		ID       string
		Name     string
		Capacity int
	}
	roomRows, err := database.DB.Query(`
		SELECT id, room_name, capacity FROM exam_rooms
		WHERE school_id = $1 AND is_active = TRUE
		ORDER BY room_name ASC
	`, schoolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat ruangan: " + err.Error()})
		return
	}
	defer roomRows.Close()

	var rooms []Room
	for roomRows.Next() {
		var r Room
		if err := roomRows.Scan(&r.ID, &r.Name, &r.Capacity); err == nil {
			rooms = append(rooms, r)
		}
	}

	if len(rooms) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Belum ada ruangan aktif. Buat ruangan terlebih dahulu."})
		return
	}

	// 2. Ambil semua siswa target yang BELUM di-assign
	var classGroupID, classSubGroupID sql.NullString
	_ = database.DB.QueryRow(`
		SELECT class_group_id, class_sub_group_id FROM exam_schedules
		WHERE id = $1 AND school_id = $2
	`, scheduleID, schoolID).Scan(&classGroupID, &classSubGroupID)

	query := `
		SELECT s.id FROM students s
		WHERE s.school_id = $1 AND s.is_active = TRUE
		AND NOT EXISTS (
			SELECT 1 FROM exam_room_assignments era
			WHERE era.student_id = s.id AND era.schedule_id = $2
		)
	`
	args := []any{schoolID, scheduleID}
	argIdx := 3
	if classSubGroupID.Valid {
		query += fmt.Sprintf(" AND s.class_sub_group_id = $%d", argIdx)
		args = append(args, classSubGroupID.String)
		argIdx++
	} else if classGroupID.Valid {
		query += fmt.Sprintf(" AND s.class_group_id = $%d", argIdx)
		args = append(args, classGroupID.String)
		argIdx++
	}
	query += " ORDER BY s.full_name ASC"

	studentRows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat siswa: " + err.Error()})
		return
	}
	defer studentRows.Close()

	var studentIDs []string
	for studentRows.Next() {
		var id string
		if err := studentRows.Scan(&id); err == nil {
			studentIDs = append(studentIDs, id)
		}
	}

	// 3. Cek total kapasitas vs jumlah siswa
	totalCapacity := 0
	for _, r := range rooms {
		totalCapacity += r.Capacity
	}
	if len(studentIDs) > totalCapacity {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf(
				"Total kapasitas ruangan (%d) kurang dari jumlah siswa yang belum di-plot (%d)",
				totalCapacity, len(studentIDs),
			),
		})
		return
	}

	// 4. Distribusi siswa ke ruangan
	type Assignment struct {
		RoomID     string
		StudentID  string
		SeatNumber string
	}

	// Hitung kursi terisi per ruangan
	roomFilled := make(map[string]int)
	for _, r := range rooms {
		var count int
		_ = database.DB.QueryRow(`
			SELECT COUNT(*) FROM exam_room_assignments
			WHERE room_id = $1 AND schedule_id = $2
		`, r.ID, scheduleID).Scan(&count)
		roomFilled[r.ID] = count
	}

	var assignments []Assignment
	roomIdx := 0
	for _, sid := range studentIDs {
		// Cari ruangan yang masih punya slot
		for roomIdx < len(rooms) {
			r := rooms[roomIdx]
			if roomFilled[r.ID] < r.Capacity {
				roomFilled[r.ID]++
				seatNum := fmt.Sprintf("A-%02d", roomFilled[r.ID])
				assignments = append(assignments, Assignment{
					RoomID:     r.ID,
					StudentID:  sid,
					SeatNumber: seatNum,
				})
				break
			}
			roomIdx++
		}
	}

	// 5. Insert dalam transaksi
	tx, _ := database.DB.Begin()
	defer tx.Rollback()
	inserted := 0
	for _, a := range assignments {
		_, err := tx.Exec(`
			INSERT INTO exam_room_assignments (schedule_id, room_id, student_id, seat_number)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (schedule_id, student_id) DO NOTHING
		`, scheduleID, a.RoomID, a.StudentID, a.SeatNumber)
		if err == nil {
			inserted++
		}
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("%d siswa berhasil didistribusikan ke %d ruangan", inserted, len(rooms)),
		"inserted": inserted,
		"rooms_used": len(rooms),
	})
}