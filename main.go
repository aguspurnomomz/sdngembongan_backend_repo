package main

import (
	"log"

	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"gembongan_dev_backend/database"
	"gembongan_dev_backend/internal/config"
	"gembongan_dev_backend/internal/middleware"
	"gembongan_dev_backend/internal/server"
	"gembongan_dev_backend/routes"

	sentrygin "github.com/getsentry/sentry-go/gin"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Peringatan: File .env tidak ditemukan, menggunakan environment system")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Konfigurasi tidak valid: %v", err)
	}

	if err := middleware.InitAuth(); err != nil {
		log.Fatalf("Gagal inisialisasi auth middleware: %v", err)
	}


	if err := initSentry(cfg); err != nil {
		log.Fatalf("Sentry initialization failed: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	database.ConnectDB(cfg.SupabaseDBURL)

	r := setupRouter(cfg)

	srv := server.New(cfg.Port, r)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func initSentry(cfg *config.Config) error {
	if cfg.SentryDSN == "" {
		log.Println("Peringatan: SENTRY_DSN kosong, Sentry dinonaktifkan")
		return nil
	}

	return sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      cfg.EnvMode,
		Release:          cfg.AppVersion,
		Debug:            cfg.EnvMode != "production",
		TracesSampleRate: cfg.SentryTracesSampleRate,
		AttachStacktrace: true,
	})
}

func setupRouter(cfg *config.Config) *gin.Engine {
	r := gin.New()

	r.Use(sentrygin.New(sentrygin.Options{
		Repanic:         true,
		WaitForDelivery: false,
	}))
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.Use(middleware.CORS(cfg.AllowedOrigins))

	routes.SetupRoutes(r)

	return r
}