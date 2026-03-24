package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BzzoiroToken   string
	BzzoiroBaseURL string
	DatabasePath   string
	Port           string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	return &Config{
		BzzoiroToken:   getEnv("BZZOIRO_API_TOKEN", ""),
		BzzoiroBaseURL: getEnv("BZZOIRO_BASE_URL", "https://sports.bzzoiro.com"),
		DatabasePath:   getEnv("DATABASE_PATH", "./prediplay_fresh.db"),
		Port:           getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
