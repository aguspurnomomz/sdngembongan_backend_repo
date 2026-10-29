package routes

import (
    "database/sql"
   // "encoding/json"
    "fmt"
    "net/http"

    "gembongan_dev_backend/database"

    "github.com/gin-gonic/gin"
)

type PermissionSet struct {
    Code         string  `json:"code"`
    Label        string  `json:"label"`
    Module       string  `json:"module"`
    ScopeType    string  `json:"scope_type"`     
    ClassIDs     []string `json:"class_ids,omitempty"` 
}

type PositionInfo struct {
    RoleID             string   `json:"role_id"`
    Code               string   `json:"code"`
    Label              string   `json:"label"`
    ScopeType          string   `json:"scope_type"`
    ClassSubGroupID    *string  `json:"class_sub_group_id,omitempty"`
    ClassSubGroupName  *string  `json:"class_sub_group_name,omitempty"`
    Permissions        []string `json:"permissions"`     
}

func CheckRoleHandler(c *gin.Context) {
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

    // 1. Ambil email
    var userEmail string
    _ = database.DB.QueryRow(
        `SELECT COALESCE(email, '') FROM auth.users WHERE id = $1`,
        userIDStr,
    ).Scan(&userEmail)

    // CEK SCHOOL ADMIN
    var userType string
    var schoolActive bool
    var schoolID sql.NullString
    err := database.DB.QueryRow(`
        SELECT 
            sa.user_type,
            COALESCE(s.is_active, false) AS school_active,
            sa.school_id
        FROM school_admins sa
        LEFT JOIN schools s ON sa.school_id = s.id
        WHERE (sa.id = $1 OR (sa.email <> '' AND sa.email = $2)) 
        AND sa.is_active = TRUE
    `, userIDStr, userEmail).Scan(&userType, &schoolActive, &schoolID)

    if err == nil {
        // Upsert profile sebagai school_admin (existing behavior)
        _, _ = database.DB.Exec(`
            INSERT INTO profiles (
                id, email_sekolah, token_balance, is_active,
                role, teacher_type, school_id, registration_source, updated_at
            )
            VALUES ($1, $2, 0, TRUE, 'school_admin', 'b2b', $3, 'created_by_superadmin', NOW())
            ON CONFLICT (id) DO UPDATE SET
                role = 'school_admin',
                teacher_type = 'b2b',
                school_id = COALESCE(EXCLUDED.school_id, profiles.school_id),
                registration_source = 'created_by_superadmin',
                updated_at = NOW()
        `, userIDStr, userEmail, schoolID.String)

        // School admin punya SEMUA permission 'school.*' dan 'teacher.read' dll
        perms := loadSchoolAdminPermissions()

        c.JSON(http.StatusOK, gin.H{
            "role":            userType,
            "school_inactive": !schoolActive,
            "school_id":       schoolID.String,
            "permissions":     perms,
            "positions":       []PositionInfo{},  // admin bukan posisi guru
        })
        return
    }

    // CEK GURU
    var existingProfile bool
    _ = database.DB.QueryRow(
        `SELECT EXISTS(SELECT 1 FROM profiles WHERE id = $1)`,
        userIDStr,
    ).Scan(&existingProfile)

    if !existingProfile {
        _, _ = database.DB.Exec(`
            INSERT INTO profiles (id, email_sekolah, token_balance, is_active, teacher_type, registration_source, updated_at)
            VALUES ($1, $2, 2, TRUE, 'b2c', 'self_register', NOW())
            ON CONFLICT (id) DO NOTHING
        `, userIDStr, userEmail)
    }

    var teacherType string
    _ = database.DB.QueryRow(
        `SELECT COALESCE(teacher_type, 'b2c') FROM profiles WHERE id = $1`,
        userIDStr,
    ).Scan(&teacherType)

    // Memberships
    type MembershipInfo struct {
        SchoolID     string `json:"school_id"`
        SchoolName   string `json:"school_name"`
        IsActive     bool   `json:"is_active"`
    }
    var memberships []MembershipInfo
    rows, err := database.DB.Query(`
        SELECT sm.school_id, s.school_name, sm.is_active
        FROM school_members sm
        JOIN schools s ON sm.school_id = s.id
        WHERE sm.user_id = $1 AND sm.left_at IS NULL
    `, userIDStr)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var m MembershipInfo
            if err := rows.Scan(&m.SchoolID, &m.SchoolName, &m.IsActive); err == nil {
                memberships = append(memberships, m)
            }
        }
    }

    // AMBIL SEMUA POSITION + PERMISSION
    positions, allPermissions := loadTeacherPositions(userIDStr)

    // Permission dasar teacher (personal exam, ebook, token) tidak tergantung sekolah
    basePerms := loadTeacherBasePermissions()
    allPermissions = mergeStringSlices(allPermissions, basePerms)

    c.JSON(http.StatusOK, gin.H{
        "role":            "teacher",
        "teacher_type":    teacherType,
        "memberships":     memberships,
        "school_inactive": false,
        "permissions":     allPermissions,     
        "positions":       positions,          
    })
}


// HELPER: Load permissions
// loadTeacherBasePermissions: permission yang selalu dimiliki guru (B2C features)
func loadTeacherBasePermissions() []string {
    return []string{
        "personal_exam.read", "personal_exam.create",
        "personal_exam.edit", "personal_exam.delete", "personal_exam.publish",
        "personal_student.read", "personal_student.create",
        "personal_student.edit", "personal_student.delete",
        "ebook.read", "ebook.download",
        "token.purchase", "token.read",
    }
}

// loadSchoolAdminPermissions: semua permission sekolah (kecuali homeroom yang khusus guru)
func loadSchoolAdminPermissions() []string {
    rows, err := database.DB.Query(`
        SELECT DISTINCT code FROM permissions
        WHERE module NOT IN ('personal_exam', 'personal_student', 'homeroom')
    `)
    if err != nil {
        return []string{}
    }
    defer rows.Close()

    var out []string
    for rows.Next() {
        var code string
        if err := rows.Scan(&code); err == nil {
            out = append(out, code)
        }
    }
    return out
}

// loadTeacherPositions: ambil jabatan aktif guru + permission per jabatan
func loadTeacherPositions(userID string) ([]PositionInfo, []string) {

    rows, err := database.DB.Query(`
        SELECT 
            smr.id,
            sr.id AS role_id,
            sr.code,
            sr.label,
            sr.scope_type,
            smr.class_sub_group_id,
            COALESCE(csg.name, '') AS class_sub_group_name
        FROM school_member_roles smr
        JOIN school_members sm ON smr.member_id = sm.id
        JOIN school_roles sr ON smr.role_id = sr.id
        LEFT JOIN class_sub_groups csg ON smr.class_sub_group_id = csg.id
        WHERE sm.user_id = $1
          AND smr.is_active = TRUE
          AND smr.ended_at IS NULL
          AND sm.is_active = TRUE
          AND sm.left_at IS NULL
    `, userID)
    if err != nil {
        return []PositionInfo{}, []string{}
    }
    defer rows.Close()

    type rawPos struct {
        ID                 string
        RoleID             string
        Code               string
        Label              string
        ScopeType          string
        ClassSubGroupID    sql.NullString
        ClassSubGroupName  sql.NullString
    }
    var raws []rawPos
    for rows.Next() {
        var r rawPos
        if err := rows.Scan(
            &r.ID, &r.RoleID, &r.Code, &r.Label, &r.ScopeType,
            &r.ClassSubGroupID, &r.ClassSubGroupName,
        ); err == nil {
            raws = append(raws, r)
        }
    }

    if len(raws) == 0 {
        return []PositionInfo{}, []string{}
    }

    // Query 2 Ambil permission per role_id
    rolePermsMap := map[string][]string{}
    roleIDs := make([]string, 0, len(raws))
    seen := map[string]bool{}
    for _, r := range raws {
        if !seen[r.RoleID] {
            roleIDs = append(roleIDs, r.RoleID)
            seen[r.RoleID] = true
        }
    }

    // Build query IN
    q := `SELECT srp.role_id, p.code 
          FROM school_role_permissions srp
          JOIN permissions p ON srp.permission_id = p.id
          WHERE srp.role_id = ANY($1::uuid[])`
    permRows, err := database.DB.Query(q, roleIDs)
    if err == nil {
        defer permRows.Close()
        for permRows.Next() {
            var rid, code string
            if err := permRows.Scan(&rid, &code); err == nil {
                rolePermsMap[rid] = append(rolePermsMap[rid], code)
            }
        }
    }

    // Build output
    allPermsSet := map[string]bool{}
    var positions []PositionInfo
    for _, r := range raws {
        perms := rolePermsMap[r.RoleID]
        for _, p := range perms {
            allPermsSet[p] = true
        }

        pos := PositionInfo{
            RoleID:      r.RoleID,
            Code:        r.Code,
            Label:       r.Label,
            ScopeType:   r.ScopeType,
            Permissions: perms,
        }
        if r.ClassSubGroupID.Valid {
            pos.ClassSubGroupID = &r.ClassSubGroupID.String
        }
        if r.ClassSubGroupName.Valid && r.ClassSubGroupName.String != "" {
            pos.ClassSubGroupName = &r.ClassSubGroupName.String
        }
        positions = append(positions, pos)
    }

    var allPerms []string
    for k := range allPermsSet {
        allPerms = append(allPerms, k)
    }
    return positions, allPerms
}

func mergeStringSlices(a, b []string) []string {
    set := map[string]bool{}
    for _, s := range a { set[s] = true }
    for _, s := range b { set[s] = true }
    out := make([]string, 0, len(set))
    for k := range set { out = append(out, k) }
    return out
}