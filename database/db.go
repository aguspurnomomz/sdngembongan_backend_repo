package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func ConnectDB() {
	dsn := os.Getenv("SUPABASE_DB_URL")
	if dsn == "" {
		log.Fatal("SUPABASE_DB_URL belum diatur di file .env")
	}
	

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Gagal membuka koneksi database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Gagal melakukan ping ke Supabase: %v", err)
	}

	fmt.Println("Berhasil terhubung ke database PostgreSQL Supabase!")
}