package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	MindPalaceRoot string
}

// LoadConfig loads environment variables in this priority order:
// 1. Local .env (if exists)
// 2. Global ~/.astra/.astra.env (fallback)
func LoadConfig() Config {
	// Try to load local .env first
	if err := godotenv.Load(".env"); err != nil {
		// fallback: ~/.astra/.astra.env
		homeDir, err := os.UserHomeDir()
		if err == nil {
			globalEnvPath := filepath.Join(homeDir, ".astra", ".astra.env")
			if _, err := os.Stat(globalEnvPath); err == nil {
				_ = godotenv.Load(globalEnvPath)
			}
		}
	}

	mindPalaceRoot := getEnv("ASTRA_MIND_PALACE_DIR", "")
	if mindPalaceRoot == "" {
		// User memory is identity-scoped, not project-scoped. Keep it outside
		// the connected repository so it remains available across projects and
		// sessions. An explicit environment value still wins for deployments.
		homeDir, err := os.UserHomeDir()
		if err == nil {
			mindPalaceRoot = filepath.Join(homeDir, ".astra", "mind-palace")
		} else {
			mindPalaceRoot = filepath.Join(".astra", "mind-palace")
		}
	}

	return Config{
		MindPalaceRoot: mindPalaceRoot,
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}
