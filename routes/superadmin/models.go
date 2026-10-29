package superadmin

type LocationData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type AdminLoginRequest struct {
	Email    string        `json:"email" binding:"required,email"`
	Password string        `json:"password" binding:"required"`
	Location *LocationData `json:"location"`
}

type ChangePasswordRequest struct {
	OldPassword string        `json:"old_password" binding:"required"`
	NewPassword string        `json:"new_password" binding:"required,min=6"`
	Location    *LocationData `json:"location"`
}

type CreateSchoolRequest struct {
	SchoolName string `json:"school_name" binding:"required"`
	Npsn       string `json:"npsn" binding:"required"`
	Address    string `json:"address"`
	Jenjang    string `json:"jenjang" binding:"required"`
}

type CreateSchoolAdminRequest struct {
	SchoolID string `json:"school_id" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Nip      string `json:"nip"`
}

type CreateSchoolRoleRequest struct {
	Code        string `json:"code" binding:"required,min=2,max=50"`
	Label       string `json:"label" binding:"required,min=2,max=100"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	IsActive    *bool  `json:"is_active"`
}

type UpdateSchoolRoleRequest struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	SortOrder   *int   `json:"sort_order"`
	IsActive    *bool  `json:"is_active"`
}

type UpdateRolePermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids"`
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