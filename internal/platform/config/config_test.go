package config_test

import (
	"testing"

	"github.com/noggrj/hacktown-fase-5-processing-worker/internal/platform/config"
)

func TestLoad_DefaultsWhenEnvUnset(t *testing.T) {
	for _, k := range []string{"PORT", "APP_ENV", "SERVICE_NAME", "REDIS_ADDR", "S3_BUCKET", "AWS_REGION", "KAFKA_BROKERS"} {
		t.Setenv(k, "")
	}
	cfg := config.Load()
	if cfg.Port != "8080" || cfg.Env != "development" || cfg.ServiceName != "fiapx-processing-worker" || cfg.AWSRegion != "us-east-1" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.IdempotencyTTL <= 0 {
		t.Fatal("expected a positive default idempotency TTL")
	}
}

func TestLoad_ParsesKafkaBrokersCSV(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "broker-1:9092,broker-2:9092")
	cfg := config.Load()
	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaBrokers[0] != "broker-1:9092" || cfg.KafkaBrokers[1] != "broker-2:9092" {
		t.Fatalf("unexpected brokers: %v", cfg.KafkaBrokers)
	}
}
