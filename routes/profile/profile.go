package profile

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"gembongan_dev_backend/database"

	"github.com/gin-gonic/gin"
)

type UpdateProfileRequest struct {
	NamaGuru               string `json:"nama_guru"`
	NipGuru                string `json:"nip_guru"`
	NamaSekolah            string `json:"nama_sekolah"`
	JenisKelamin           string `json:"jenis_kelamin"`
	MataPelajaran          string `json:"mata_pelajaran"`
	Fase                   string `json:"fase"`
	Kelas                  string `json:"kelas"`
	Semester               string `json:"semester"`
	TahunPelajaran         string `json:"tahun_pelajaran"`
	NamaKepalaSekolah      string `json:"nama_kepala_sekolah"`
	NipKepalaSekolah       string `json:"nip_kepala_sekolah"`
	KotaKabupaten          string `json:"kota_kabupaten"`
	TanggalPenandatanganan string `json:"tanggal_penandatanganan"`
	AlamatSekolah          string `json:"alamat_sekolah"`
	KecamatanKabupaten     string `json:"kecamatan_kabupaten"`
	KodePos                string `json:"kode_pos"`
	TeleponSekolah         string `json:"telepon_sekolah"`
	EmailSekolah           string `json:"email_sekolah"`
	Npsn                   string `json:"npsn"`
	WebsiteSekolah         string `json:"website_sekolah"`
}

func GetProfile(c *gin.Context) {
	// fmt.Println("========================================")
	// fmt.Println(">>> [1] Handler /profile MULAI")

	userID, exists := c.Get("user_id")
	if !exists {
		fmt.Println(">>> [1b] ERROR: user_id tidak ada di context!")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	fmt.Printf(">>> [2] userID=%v (type=%T)\n", userID, userID)

	fmt.Println(">>> [3] Akan UPDATE last_login...")
	res, err := database.DB.Exec(`UPDATE profiles SET last_login = NOW() WHERE id = $1`, userID)
	if err != nil {
		fmt.Printf(">>> [3b] UPDATE last_login ERROR: %v\n", err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Printf(">>> [3c] UPDATE last_login OK, rows affected=%d\n", rowsAffected)
	}

	var p UpdateProfileRequest
	var tokenBalance int

	queryFixed := `SELECT 
		COALESCE(nama_guru, ''), COALESCE(nip_guru, ''), COALESCE(nama_sekolah, ''), 
		COALESCE(mata_pelajaran, ''), COALESCE(fase, ''), COALESCE(kelas, ''), 
		COALESCE(jenis_kelamin, ''),
		COALESCE(semester, ''), COALESCE(tahun_pelajaran, ''), COALESCE(nama_kepala_sekolah, ''), 
		COALESCE(nip_kepala_sekolah, ''), COALESCE(kota_kabupaten, ''), COALESCE(tanggal_penandatanganan, ''), 
		COALESCE(alamat_sekolah, ''), COALESCE(kecamatan_kabupaten, ''), COALESCE(kode_pos, ''), 
		COALESCE(telepon_sekolah, ''), COALESCE(email_sekolah, ''), COALESCE(npsn, ''), 
		COALESCE(website_sekolah, ''), token_balance 
		FROM profiles WHERE id = $1`

	dollarCount := strings.Count(queryFixed, "$")
	fmt.Printf(">>> [4] Query SELECT siap. Jumlah placeholder $N = %d\n", dollarCount)
	fmt.Printf(">>> [4b] Query full:\n%s\n", queryFixed)

	args := []any{userID}
	fmt.Printf(">>> [5] Jumlah argumen yang dikirim = %d\n", len(args))
	for i, a := range args {
		fmt.Printf(">>> [5.%d] arg[%d] = %v (type=%T)\n", i, i, a, a)
	}

	if dollarCount != len(args) {
		fmt.Printf(">>> [5b] MISMATCH! Query butuh %d param, tapi dikirim %d param\n", dollarCount, len(args))
	}

	fmt.Println(">>> [6] Akan eksekusi QueryRow...")
	err = database.DB.QueryRow(queryFixed, args...).Scan(
		&p.NamaGuru, &p.NipGuru, &p.NamaSekolah, &p.MataPelajaran, &p.Fase, &p.Kelas,
		&p.JenisKelamin,
		&p.Semester, &p.TahunPelajaran, &p.NamaKepalaSekolah, &p.NipKepalaSekolah,
		&p.KotaKabupaten, &p.TanggalPenandatanganan, &p.AlamatSekolah, &p.KecamatanKabupaten,
		&p.KodePos, &p.TeleponSekolah, &p.EmailSekolah, &p.Npsn, &p.WebsiteSekolah, &tokenBalance,
	)

	if err != nil {
		fmt.Printf(">>> [6b] QueryRow ERROR: %v\n", err)
		fmt.Printf(">>> [6c] Error type: %T\n", err)

		if err == sql.ErrNoRows {
			fmt.Println(">>> [6d] Tidak ada row di profiles untuk userID ini")
			c.JSON(http.StatusNotFound, gin.H{"error": "Profil tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// fmt.Printf(">>> [7] QueryRow SUKSES! token_balance=%d, nama_guru=%s\n", tokenBalance, p.NamaGuru)
	// fmt.Println(">>> [8] Handler /profile SELESAI")
	// fmt.Println("========================================")

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"profile":       p,
		"token_balance": tokenBalance,
	})
}

func UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	query := `UPDATE profiles SET 
		nama_guru = $1, nip_guru = $2, jenis_kelamin = $3, nama_sekolah = $4, 
		mata_pelajaran = $5, fase = $6, kelas = $7, semester = $8, tahun_pelajaran = $9, 
		nama_kepala_sekolah = $10, nip_kepala_sekolah = $11, kota_kabupaten = $12, 
		tanggal_penandatanganan = $13, alamat_sekolah = $14, kecamatan_kabupaten = $15, 
		kode_pos = $16, telepon_sekolah = $17, email_sekolah = $18, npsn = $19, 
		website_sekolah = $20, updated_at = NOW() 
		WHERE id = $21`

	_, err := database.DB.Exec(query,
		req.NamaGuru, req.NipGuru, req.JenisKelamin, req.NamaSekolah,
		req.MataPelajaran, req.Fase, req.Kelas, req.Semester, req.TahunPelajaran,
		req.NamaKepalaSekolah, req.NipKepalaSekolah, req.KotaKabupaten,
		req.TanggalPenandatanganan, req.AlamatSekolah, req.KecamatanKabupaten,
		req.KodePos, req.TeleponSekolah, req.EmailSekolah, req.Npsn,
		req.WebsiteSekolah, userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui profil: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Identitas perangkat berhasil diperbarui"})
}