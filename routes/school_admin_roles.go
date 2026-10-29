package routes

import (
    "database/sql"
    "fmt"
    "net/http"

    "gembongan_dev_backend/database"

    "github.com/gin-gonic/gin"
)

// Request body untuk assign role
type AssignRoleRequest struct {
    RoleCode           string `json:"role_code" binding:"required"`   
    ClassSubGroupID    string `json:"class_sub_group_id"`            
    Notes              string `json:"notes"`
}

// POST /api/school-admin/teachers/:teacherId/roles
// Assign role ke guru (bisa multi-role)
func AssignRoleToTeacher(c *gin.Context) {
    schoolID, err := getSchoolIDFromUser(c)
    if err != nil {
        c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
        return
    }
    adminID, _ := getAdminIDFromUser(c)

    teacherID := c.Param("id") 

    var req AssignRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
        return
    }

    // 1. Cari teacher -> user_id & school_members.id
    var userID, memberID string
    err = database.DB.QueryRow(`
        SELECT t.user_id, sm.id
        FROM teachers t
        JOIN school_members sm ON sm.user_id = t.user_id AND sm.school_id = t.school_id
        WHERE t.id = $1 AND t.school_id = $2 AND t.is_active = TRUE
    `, teacherID, schoolID).Scan(&userID, &memberID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan di sekolah ini"})
        return
    }

    // 2. Cari role_id & scope_type
    var roleID, scopeType string
    err = database.DB.QueryRow(`
        SELECT id, scope_type FROM school_roles 
        WHERE code = $1 AND is_active = TRUE
    `, req.RoleCode).Scan(&roleID, &scopeType)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Peran tidak ditemukan atau tidak aktif"})
        return
    }

    // 3. Validasi scope
    var classSubGroupID interface{} = nil
    if scopeType == "class" {
        if req.ClassSubGroupID == "" {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("Peran '%s' membutuhkan class_sub_group_id", req.RoleCode),
            })
            return
        }
        // Validasi sub kelas milik sekolah ini
        var validSchool string
        err = database.DB.QueryRow(`
            SELECT school_id FROM class_sub_groups WHERE id = $1
        `, req.ClassSubGroupID).Scan(&validSchool)
        if err != nil || validSchool != schoolID {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Sub kelas tidak valid"})
            return
        }
        classSubGroupID = req.ClassSubGroupID
    }

    // 4. Upsert ke school_member_roles
    _, err = database.DB.Exec(`
        INSERT INTO school_member_roles 
            (member_id, role_id, class_sub_group_id, is_active, assigned_by, notes)
        VALUES ($1, $2, $3, TRUE, $4, NULLIF($5, ''))
        ON CONFLICT (member_id, role_id, class_sub_group_id) DO UPDATE SET
            is_active = TRUE,
            ended_at = NULL,
            assigned_by = EXCLUDED.assigned_by,
            notes = EXCLUDED.notes,
            assigned_at = NOW()
    `, memberID, roleID, classSubGroupID, adminID, req.Notes)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal assign peran: " + err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "message": "Peran berhasil di-assign",
        "teacher_id": teacherID,
        "role_code": req.RoleCode,
        "class_sub_group_id": req.ClassSubGroupID,
    })
}

// DELETE /api/school-admin/teachers/:teacherId/roles/:roleId
// Hapus role dari guru (soft delete dengan ended_at)
func RemoveRoleFromTeacher(c *gin.Context) {
    schoolID, err := getSchoolIDFromUser(c)
    if err != nil {
        c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
        return
    }

    teacherID := c.Param("id") 
    roleID := c.Param("roleId")

    var memberID string
    err = database.DB.QueryRow(`
        SELECT sm.id FROM teachers t
        JOIN school_members sm ON sm.user_id = t.user_id AND sm.school_id = t.school_id
        WHERE t.id = $1 AND t.school_id = $2
    `, teacherID, schoolID).Scan(&memberID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Guru tidak ditemukan"})
        return
    }

    result, err := database.DB.Exec(`
        UPDATE school_member_roles
        SET is_active = FALSE, ended_at = NOW()
        WHERE member_id = $1 AND role_id = $2 AND is_active = TRUE
    `, memberID, roleID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    affected, _ := result.RowsAffected()
    if affected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Relasi peran tidak ditemukan"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Peran berhasil dicabut"})
}

// GET /api/school-admin/teachers/:teacherId/roles
// Lihat semua peran guru
func GetTeacherRoles(c *gin.Context) {
    schoolID, err := getSchoolIDFromUser(c)
    if err != nil {
        c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
        return
    }
    teacherID := c.Param("id") 

    rows, err := database.DB.Query(`
        SELECT 
            smr.id, sr.id AS role_id, sr.code, sr.label, sr.scope_type,
            smr.class_sub_group_id, COALESCE(csg.name, ''),
            smr.is_active, smr.assigned_at, COALESCE(smr.notes, '')
        FROM teachers t
        JOIN school_members sm ON sm.user_id = t.user_id AND sm.school_id = t.school_id
        JOIN school_member_roles smr ON smr.member_id = sm.id
        JOIN school_roles sr ON smr.role_id = sr.id
        LEFT JOIN class_sub_groups csg ON smr.class_sub_group_id = csg.id
        WHERE t.id = $1 AND t.school_id = $2
        ORDER BY sr.sort_order, sr.label
    `, teacherID, schoolID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    type RoleItem struct {
        ID                 string     `json:"id"`
        RoleID             string     `json:"role_id"`
        Code               string     `json:"code"`
        Label              string     `json:"label"`
        ScopeType          string     `json:"scope_type"`
        ClassSubGroupID    *string    `json:"class_sub_group_id"`
        ClassSubGroupName  string     `json:"class_sub_group_name"`
        IsActive           bool       `json:"is_active"`
        AssignedAt         string     `json:"assigned_at"`
        Notes              string     `json:"notes"`
    }

    items := []RoleItem{}
    for rows.Next() {
        var it RoleItem
        var csgID sql.NullString
        var csgName sql.NullString
        var assignedAt sql.NullTime
        if err := rows.Scan(
            &it.ID, &it.RoleID, &it.Code, &it.Label, &it.ScopeType,
            &csgID, &csgName, &it.IsActive, &assignedAt, &it.Notes,
        ); err == nil {
            if csgID.Valid { it.ClassSubGroupID = &csgID.String }
            it.ClassSubGroupName = csgName.String
            if assignedAt.Valid { it.AssignedAt = assignedAt.Time.Format("2006-01-02 15:04:05") }
            items = append(items, it)
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "teacher_id": teacherID,
        "total":      len(items),
        "roles":      items,
    })
}