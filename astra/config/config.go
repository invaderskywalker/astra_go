package config

import (
	"os"
)

type Config struct {
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBName     string
	JWTSecret  string
}

func LoadConfig() Config {
	// if err := godotenv.Load(); err != nil {
	// 	log.Println("No .env file found, using system environment variables")
	// }

	return Config{
		DBUser:     getEnv("DB_USER", ""),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBHost:     getEnv("DB_HOST", ""),
		DBPort:     getEnv("DB_PORT", ""),
		DBName:     getEnv("DB_NAME", ""),
		JWTSecret:  getEnv("JWT_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	// fmt.Println("get env ", key, value)
	if value != "" {
		return value
	}
	return fallback
}
