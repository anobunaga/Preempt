package config

import (
	"os"
	"strconv"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Stream   string
}

func GetRedisConfig() RedisConfig {
	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if parsed, err := strconv.Atoi(dbStr); err == nil {
			db = parsed
		}
	}

	return RedisConfig{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
		Stream:   getEnv("REDIS_STREAM", "weather_metrics"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
