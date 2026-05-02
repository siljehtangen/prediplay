package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	BzzoiroToken   string
	BzzoiroBaseURL string
	DatabasePath   string
	Port           string
	CORSOrigins    []string
}

// Load reads configuration from environment variables, loading a .env file first if present.
// It terminates the process if any required variables are missing.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := &Config{
		BzzoiroToken:   getEnv("BZZOIRO_API_TOKEN", ""),
		BzzoiroBaseURL: getEnv("BZZOIRO_BASE_URL", "https://sports.bzzoiro.com"),
		DatabasePath:   getEnv("DATABASE_PATH", "./prediplay_fresh.db"),
		Port:           getEnv("PORT", "8080"),
		CORSOrigins:    parseCORSOrigins(getEnv("CORS_ORIGINS", "http://localhost:4200,http://localhost:3000")),
	}

	if cfg.BzzoiroToken == "" {
		log.Fatal("BZZOIRO_API_TOKEN is required")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCORSOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if o := strings.TrimSpace(p); o != "" {
			out = append(out, o)
		}
	}
	return out
}
