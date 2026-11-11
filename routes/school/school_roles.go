package school

import (
	"net/http"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/helpers"

	"github.com/gin-gonic/gin"
)

// ============================================================
// RoleOption — item response untuk list school roles
// ============================================================

type RoleOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// ============================================================
// GET /api/school-admin/school-roles
// — List semua role sekolah aktif (untuk dropdown assign role)
// ============================================================

func ListSchoolRoles(c *gin.Context) {
	// Validasi apakah user adalah admin sekolah aktif
	_, err := helpers.GetSchoolIDFromUser(c)
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
}