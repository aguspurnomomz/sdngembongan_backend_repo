package middleware

import (
	"fmt"
	"os"
	"sync"

	"github.com/MicahParks/keyfunc/v3"
)

var (
	jwksOnce sync.Once
	jwks     keyfunc.Keyfunc
	jwksErr  error
)

func InitAuth() error {
	jwksOnce.Do(func() {
		jwksURL := os.Getenv("SUPABASE_JWKS_URL")
		if jwksURL == "" {
			jwksErr = fmt.Errorf("SUPABASE_JWKS_URL tidak dikonfigurasi")
			return
		}

		k, err := keyfunc.NewDefault([]string{jwksURL})
		if err != nil {
			jwksErr = fmt.Errorf("gagal inisialisasi JWKS: %w", err)
			return
		}
		jwks = k
	})
	return jwksErr
}

// mustJWKS mengembalikan instance JWKS atau panic jika belum diinisialisasi.
// Dipakai di dalam AuthMiddleware agar salah konfigurasi ketahuan saat startup, bukan saat request.
func mustJWKS() keyfunc.Keyfunc {
	if jwks == nil {
		panic("middleware: InitAuth() harus dipanggil sebelum AuthMiddleware digunakan")
	}
	return jwks
}