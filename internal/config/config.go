package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort                      string
	Env                          string
	DBHost                       string
	DBPort                       string
	DBUser                       string
	DBPassword                   string
	DBName                       string
	DBSSLMode                    string
	ReservationExpirationMinutes int
	CleanupIntervalSeconds       int
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppPort:                      getEnv("APP_PORT", "8080"),
		Env:                          getEnv("ENV", "development"),
		DBHost:                       getEnv("DB_HOST", "localhost"),
		DBPort:                       getEnv("DB_PORT", "5432"),
		DBUser:                       getEnv("DB_USER", "postgres"),
		DBPassword:                   getEnv("DB_PASSWORD", "postgres"),
		DBName:                       getEnv("DB_NAME", "flash_sale"),
		DBSSLMode:                    getEnv("DB_SSLMODE", "disable"),
		ReservationExpirationMinutes: getEnvAsInt("RESERVATION_EXPIRATION_MINUTES", 5),
		CleanupIntervalSeconds:       getEnvAsInt("CLEANUP_INTERVAL_SECONDS", 10),
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBUser, c.DBPassword, c.DBName, c.DBPort, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	strVal := getEnv(key, "")
	if strVal == "" {
		return fallback
	}
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return fallback
	}
	return val
}
