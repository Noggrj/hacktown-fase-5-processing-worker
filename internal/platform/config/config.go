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
