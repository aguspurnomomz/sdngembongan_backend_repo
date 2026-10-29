// routes/superadmin/roles.go
package superadmin

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

// GET /api/superadmin/school-roles
func ListSchoolRoles(c *gin.Context) {
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
}

// POST /api/superadmin/school-roles
func CreateSchoolRole(c *gin.Context) {
	var req CreateSchoolRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak valid: " + err.Error()})
		return
	}

	code := strings.ToLower(strings.TrimSpace(req.Code))
	code = strings.ReplaceAll(code, " ", "_")

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

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

	if adminID, ok := GetAdminIDFromContext(c); ok {
		LogSuperAdminActivity(adminID, "CREATE_SCHOOL_ROLE", c.ClientIP(),
			c.Request.UserAgent(), "",
			fmt.Sprintf("Menambahkan peran: %s (%s)", req.Label, code))
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Peran berhasil ditambahkan", "id": roleID, "code": code})
}

// PUT /api/superadmin/school-roles/:id
func UpdateSchoolRole(c *gin.Context) {
	roleID := c.Param("id")

	var req UpdateSchoolRoleRequest
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
}

// DELETE /api/superadmin/school-roles/:id
func DeleteSchoolRole(c *gin.Context) {
	roleID := c.Param("id")

	// Ambil info role dulu
	var code, label string
	err := database.DB.QueryRow(`SELECT code, label FROM school_roles WHERE id = $1`, roleID).Scan(&code, &label)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Peran tidak ditemukan"})
		return
	}

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

	if adminID, ok := GetAdminIDFromContext(c); ok {
		LogSuperAdminActivity(adminID, "DELETE_SCHOOL_ROLE", c.ClientIP(),
			c.Request.UserAgent(), "",
			fmt.Sprintf("Menghapus peran: %s (%s)", label, code))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Peran berhasil dihapus"})
}

// GET /api/superadmin/permissions
func ListPermissions(c *gin.Context) {
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
}

// GET /api/superadmin/school-roles/:id/permissions
func GetRolePermissions(c *gin.Context) {
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

	permissionIDs := []string{}
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err == nil {
			permissionIDs = append(permissionIDs, pid)
		}
	}

	c.JSON(http.StatusOK, gin.H{"permission_ids": permissionIDs})
}

// PUT /api/superadmin/school-roles/:id/permissions
func UpdateRolePermissions(c *gin.Context) {
	roleID := c.Param("id")

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

	var req UpdateRolePermissionsRequest
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

	_, _ = tx.Exec(`DELETE FROM school_role_permissions WHERE role_id = $1`, roleID)

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
}