package config

import (
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	Env            string
	ServiceName    string
	RedisAddr      string
	S3Bucket       string
	AWSRegion      string
	KafkaBrokers   []string
	IdempotencyTTL time.Duration

	// S3Endpoint/S3AccessKey/S3SecretKey are only set for local
	// docker-compose (MinIO) — empty in production.
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:           getenv("PORT", "8080"),
		Env:            getenv("APP_ENV", "development"),
		ServiceName:    getenv("SERVICE_NAME", "fiapx-processing-worker"),
		RedisAddr:      os.Getenv("REDIS_ADDR"),
		S3Bucket:       os.Getenv("S3_BUCKET"),
		AWSRegion:      getenv("AWS_REGION", "us-east-1"),
		KafkaBrokers:   splitCSV(getenv("KAFKA_BROKERS", "")),
		IdempotencyTTL: 24 * time.Hour,
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:    os.Getenv("S3_SECRET_KEY"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
