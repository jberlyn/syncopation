package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port   string
	DBPath string
}

func LoadConfig() *Config {
	// Load .env if it exists
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/joplin.sqlite3"
	}

	return &Config{
		Port:   port,
		DBPath: dbPath,
	}
}
