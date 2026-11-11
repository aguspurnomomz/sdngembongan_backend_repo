package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)


type Config struct {
	Port                   string
	EnvMode                string
	AppVersion             string
	SentryDSN              string
	SentryTracesSampleRate float64
	SupabaseDBURL          string
	SupabaseJWKSURL        string
	SupabaseJWTIssuer      string
	AllowedOrigins         []string
}


func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnvDefault("PORT", "8080"),
		EnvMode:           getEnvDefault("ENV_MODE", "development"),
		AppVersion:        os.Getenv("APP_VERSION"),
		SentryDSN:         os.Getenv("SENTRY_DSN"),
		SupabaseDBURL:     os.Getenv("SUPABASE_DB_URL"),
		SupabaseJWKSURL:   os.Getenv("SUPABASE_JWKS_URL"),
		SupabaseJWTIssuer: os.Getenv("SUPABASE_JWT_ISSUER"),
		AllowedOrigins:    splitAndTrim(getEnvDefault("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000,https://portalpendidikan.sdngembongan.id")),
	}

	rateStr := os.Getenv("SENTRY_TRACES_SAMPLE_RATE")
	if rateStr != "" {
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			return nil, fmt.Errorf("SENTRY_TRACES_SAMPLE_RATE tidak valid: %w", err)
		}
		cfg.SentryTracesSampleRate = rate
	} else if cfg.EnvMode == "production" {
		cfg.SentryTracesSampleRate = 0.1
	} else {
		cfg.SentryTracesSampleRate = 1.0
	}

	if cfg.SupabaseDBURL == "" {
		return nil, fmt.Errorf("SUPABASE_DB_URL wajib diisi")
	}
	if cfg.SupabaseJWKSURL == "" {
		return nil, fmt.Errorf("SUPABASE_JWKS_URL wajib diisi")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS tidak boleh kosong")
	}

	return cfg, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}