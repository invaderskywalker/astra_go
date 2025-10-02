// astra/config/config.go (updated)
package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	JWT_SECRET  string
}

func LoadConfig() Config {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "secret" // Default for development; change in production
	}
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWT_SECRET:  secret,
	}
}
