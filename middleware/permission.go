package middleware

import (
	"database/sql"
    "fmt"
    "net/http"

    "teach_partner_dev/database"

    "github.com/gin-gonic/gin"
)

// RequirePermission: middleware yang memastikan user punya permission tertentu.
// Bisa dipakai oleh teacher maupun school_admin (karena kita cek permission union).
func RequirePermission(code string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }

        var uid string
        switch v := userID.(type) {
        case string:
            uid = v
        default:
            uid = fmt.Sprintf("%v", v)
        }

        // Cek apakah punya permission ini (via role mana pun)
        var has bool
        err := database.DB.QueryRow(`
            SELECT EXISTS(
                -- dari role school_member_roles
                SELECT 1 
                FROM school_member_roles smr
                JOIN school_members sm ON smr.member_id = sm.id
                JOIN school_role_permissions srp ON srp.role_id = smr.role_id
                JOIN permissions p ON srp.permission_id = p.id
                WHERE sm.user_id = $1
                  AND smr.is_active = TRUE
                  AND smr.ended_at IS NULL
                  AND sm.is_active = TRUE
                  AND sm.left_at IS NULL
                  AND p.code = $2
                
                UNION
                
                -- kalau school_admin: semua permission sekolah
                SELECT 1 FROM school_admins sa, permissions p
                WHERE (sa.id = $1 OR sa.email = (SELECT COALESCE(email,'') FROM auth.users WHERE id = $1))
                  AND sa.is_active = TRUE
                  AND p.code = $2
                  AND p.module NOT IN ('personal_exam', 'personal_student', 'homeroom')
            )
        `, uid, code).Scan(&has)

        if err != nil || !has {
            c.JSON(http.StatusForbidden, gin.H{
                "error": gin.H{
                    "code":    "FORBIDDEN",
                    "message": "Anda tidak memiliki izin untuk aksi ini",
                    "required_permission": code,
                },
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// RequirePosition: middleware yang memastikan user punya posisi tertentu
// (misal: wali_kelas). Scope data otomatis disimpan di context.
func RequirePosition(roleCode string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, _ := c.Get("user_id")
        var uid string
        switch v := userID.(type) {
        case string:
            uid = v
        default:
            uid = fmt.Sprintf("%v", v)
        }

        var classSubGroupID sql.NullString
        err := database.DB.QueryRow(`
            SELECT smr.class_sub_group_id
            FROM school_member_roles smr
            JOIN school_members sm ON smr.member_id = sm.id
            JOIN school_roles sr ON smr.role_id = sr.id
            WHERE sm.user_id = $1
              AND sr.code = $2
              AND smr.is_active = TRUE
              AND smr.ended_at IS NULL
              AND sm.is_active = TRUE
            LIMIT 1
        `, uid, roleCode).Scan(&classSubGroupID)

        if err != nil {
            c.JSON(http.StatusForbidden, gin.H{"error": "Anda bukan " + roleCode})
            c.Abort()
            return
        }

        // Simpan scope ke context agar handler tahu kelas mana yang boleh diakses
        if classSubGroupID.Valid {
            c.Set("position_class_sub_group_id", classSubGroupID.String)
        }
        c.Set("position_code", roleCode)
        c.Next()
    }
}