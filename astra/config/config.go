package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	// AstraRoot contains Astra-owned state. Keeping it outside connected
	// repositories prevents session history, attachments, and generated
	// artifacts from accumulating in every project directory.
	AstraRoot      string
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

	homeDir, _ := os.UserHomeDir()
	dataRoot := getEnv("ASTRA_DATA_DIR", "")
	if dataRoot == "" {
		if homeDir != "" {
			dataRoot = filepath.Join(homeDir, ".astra")
		} else {
			dataRoot = ".astra"
		}
	}
	dataRoot, _ = filepath.Abs(dataRoot)

	mindPalaceRoot := getEnv("ASTRA_MIND_PALACE_DIR", "")
	if mindPalaceRoot == "" {
		// User memory is identity-scoped, not project-scoped. Keep it outside
		// the connected repository so it remains available across projects and
		// sessions. An explicit environment value still wins for deployments.
		if homeDir != "" {
			mindPalaceRoot = filepath.Join(dataRoot, "mind-palace")
		} else {
			mindPalaceRoot = filepath.Join(".astra", "mind-palace")
		}
	}

	return Config{
		AstraRoot:      dataRoot,
		MindPalaceRoot: mindPalaceRoot,
	}
}

// ProjectDataRoot returns the private, stable Astra directory for a connected
// project. The project path is hashed so the directory name is portable and
// does not leak a full local path into a listing.
func ProjectDataRoot(projectRoot string) string {
	cfg := LoadConfig()
	digest := sha256.Sum256([]byte(filepath.Clean(projectRoot)))
	projectID := "project_" + hex.EncodeToString(digest[:])[:16]
	return filepath.Join(cfg.AstraRoot, "projects", projectID)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}
