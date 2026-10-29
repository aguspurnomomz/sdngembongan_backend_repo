package main

import (
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gembongan_dev_backend/database"
	"gembongan_dev_backend/routes"

	sentrygin "github.com/getsentry/sentry-go/gin"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Peringatan: File .env tidak ditemukan, menggunakan environment system")
	}

	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		// Fallback langsung ke DSN dari Sentry Anda jika belum ada di .env
		dsn = "https://e365332f65f5668dd790f18ca57790ab@o4511336547221504.ingest.us.sentry.io/4511977382281216"
	}

	err = sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: os.Getenv("ENV_MODE"),
		Debug:       true,
	})
	if err != nil {
		log.Printf("Sentry initialization failed: %v\n", err)
	}
	defer sentry.Flush(2 * time.Second)

	database.ConnectDB()

	r := gin.Default()

	// Pasang Sentry Gin Middleware resmi di awal router
	r.Use(sentrygin.New(sentrygin.Options{
		Repanic: true,
	}))

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",                 
			"http://localhost:3000",                 
			"https://portalpendidikan.sdngembongan.id", 
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, 
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	routes.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server berjalan di port %s...", port)
	r.Run(":" + port)
}