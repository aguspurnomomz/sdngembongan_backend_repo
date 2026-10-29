package helpers

import (
	"database/sql"
	"fmt"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type ScopeType string

const (
	ScopeSchool   ScopeType = "school"
	ScopePersonal ScopeType = "personal"
)

type RequestScope struct {
	Type          ScopeType
	SchoolID      string
	UserID        string
	UserIDRaw     string
	UserEmail     string
	TeacherType   string
	IsSchoolAdmin bool
}


func GetScopeFromUser(c *gin.Context) (*RequestScope, error) {
	return GetScope(c, false)
}


func GetPersonalScope(c *gin.Context) (*RequestScope, error) {
	return GetScope(c, true)
}

func GetScope(c *gin.Context, preferPersonal bool) (*RequestScope, error) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		return nil, fmt.Errorf("unauthorized: user_id tidak ditemukan di context")
	}

	var userIDStr string
	switch v := userIDRaw.(type) {
	case string:
		userIDStr = v
	case fmt.Stringer:
		userIDStr = v.String()
	default:
		userIDStr = fmt.Sprintf("%v", v)
	}

	if userIDStr == "" {
		return nil, fmt.Errorf("unauthorized: user_id kosong")
	}

	var userEmail string
	_ = database.DB.QueryRow(
		`SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
		userIDStr,
	).Scan(&userEmail)

	scope := &RequestScope{
		UserIDRaw: userIDStr,
		UserEmail: userEmail,
	}


	if preferPersonal {
		var teacherType sql.NullString
		err := database.DB.QueryRow(`
			SELECT COALESCE(teacher_type, 'b2c')
			FROM profiles
			WHERE id = $1
		`, userIDStr).Scan(&teacherType)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("profile tidak ditemukan untuk user ini")
			}
			return nil, fmt.Errorf("gagal resolve profile: %w", err)
		}


		var isSchoolAdminOnly bool
		_ = database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM school_admins 
				WHERE (id = $1 OR (email <> '' AND email = $2))
				  AND is_active = TRUE
			) AND NOT EXISTS(
				SELECT 1 FROM profiles WHERE id = $1
			)
		`, userIDStr, userEmail).Scan(&isSchoolAdminOnly)

		if isSchoolAdminOnly {
			return nil, fmt.Errorf("akses ditolak: akun admin sekolah tidak bisa membuat exam personal")
		}

		scope.Type = ScopePersonal
		scope.UserID = userIDStr
		scope.IsSchoolAdmin = false
		scope.TeacherType = teacherType.String
		return scope, nil
	}


	var schoolID sql.NullString
	var userType sql.NullString
	var schoolActive sql.NullBool

	err := database.DB.QueryRow(`
		SELECT 
			sa.school_id,
			sa.user_type,
			COALESCE(s.is_active, false) AS school_active
		FROM school_admins sa
		LEFT JOIN schools s ON sa.school_id = s.id
		WHERE (sa.id = $1 OR (sa.email <> '' AND sa.email = $2))
		  AND sa.is_active = TRUE
		LIMIT 1
	`, userIDStr, userEmail).Scan(&schoolID, &userType, &schoolActive)

	if err == nil && schoolID.Valid {
		if !schoolActive.Bool {
			return nil, fmt.Errorf("sekolah Anda sedang dinonaktifkan. Hubungi administrator untuk informasi lebih lanjut")
		}

		scope.Type = ScopeSchool
		scope.SchoolID = schoolID.String
		scope.IsSchoolAdmin = true
		scope.TeacherType = "b2b"
		return scope, nil
	}


	var teacherSchoolID sql.NullString
	var teacherType sql.NullString

	err = database.DB.QueryRow(`
		SELECT 
			sm.school_id,
			COALESCE(p.teacher_type, 'b2c') AS teacher_type
		FROM school_members sm
		JOIN schools s ON sm.school_id = s.id
		JOIN profiles p ON p.id = sm.user_id
		WHERE sm.user_id = $1
		  AND sm.is_active = TRUE
		  AND sm.left_at IS NULL
		  AND s.is_active = TRUE
		LIMIT 1
	`, userIDStr).Scan(&teacherSchoolID, &teacherType)

	if err == nil && teacherSchoolID.Valid {
		scope.Type = ScopeSchool
		scope.SchoolID = teacherSchoolID.String
		scope.IsSchoolAdmin = false
		scope.TeacherType = teacherType.String
		return scope, nil
	}


	err = database.DB.QueryRow(`
		SELECT COALESCE(teacher_type, 'b2c')
		FROM profiles
		WHERE id = $1
	`, userIDStr).Scan(&teacherType)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("profile tidak ditemukan untuk user ini")
		}
		return nil, fmt.Errorf("gagal resolve profile: %w", err)
	}

	scope.Type = ScopePersonal
	scope.UserID = userIDStr
	scope.IsSchoolAdmin = false
	scope.TeacherType = teacherType.String
	return scope, nil
}


func MustGetSchoolID(c *gin.Context) (string, error) {
	scope, err := GetScopeFromUser(c)
	if err != nil {
		return "", err
	}
	if scope.Type != ScopeSchool {
		return "", fmt.Errorf("akses ditolak: endpoint ini hanya untuk admin sekolah atau guru di sekolah")
	}
	return scope.SchoolID, nil
}


func MustGetPersonalScope(c *gin.Context) (*RequestScope, error) {
	scope, err := GetPersonalScope(c)
	if err != nil {
		return nil, err
	}
	if scope.Type != ScopePersonal {
		return nil, fmt.Errorf("akses ditolak: endpoint ini hanya untuk guru personal (B2C)")
	}
	return scope, nil
}