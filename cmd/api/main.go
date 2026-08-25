package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	events "github.com/noggrj/fiapx-events"
	eventskafka "github.com/noggrj/fiapx-events/transport/kafka"

	"github.com/noggrj/fiapx-processing-worker/internal/processing/ffmpeg"
	"github.com/noggrj/fiapx-processing-worker/internal/processing/usecase"
	"github.com/noggrj/fiapx-processing-worker/internal/saga"

	"github.com/noggrj/fiapx-processing-worker/internal/platform/cache"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/config"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/health"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/idempotency"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/logging"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/messaging"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/metrics"
	"github.com/noggrj/fiapx-processing-worker/internal/platform/storage"
)

var version = "dev"

const consumerGroup = "processing-worker.video-uploaded"

func main() {
	// CI smoke test runs `./server --version` against a container with no
	// Redis/Kafka/S3 reachable — must exit immediately instead of falling
	// through to ListenAndServe, which blocks forever.
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	cfg := config.Load()
	log := logging.New(cfg.ServiceName)
	slog.SetDefault(log)

	log.Info("starting processing-worker",
		slog.String("version", version), slog.String("env", cfg.Env), slog.String("port", cfg.Port))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Redis (idempotency store) ────────────────────────────────
	redisClient, err := cache.NewClient(cfg.RedisAddr)
	if err != nil {
		log.Warn("redis unavailable at startup — consumer will not start until reachable", slog.Any("error", err))
	} else {
		log.Info("redis connected")
		defer func() { _ = redisClient.Close() }()
	}

	// ── S3 ──────────────────────────────────────────────────────
	s3Store, err := storage.NewS3(ctx, cfg.S3Bucket, cfg.AWSRegion)
	if err != nil {
		log.Warn("s3 unavailable at startup — processing will fail until reachable", slog.Any("error", err))
	} else {
		log.Info("s3 client ready", slog.String("bucket", cfg.S3Bucket))
	}

	// ── Kafka ───────────────────────────────────────────────────
	var pub *saga.Publisher
	if len(cfg.KafkaBrokers) > 0 {
		w := messaging.NewWriter(cfg.KafkaBrokers)
		defer func() { _ = w.Close() }()

		innerPub, err := eventskafka.NewPublisher(w)
		if err != nil {
			log.Error("failed to init kafka publisher", slog.Any("error", err))
		} else if p, err := saga.NewPublisher(innerPub); err == nil {
			pub = p
		}
	} else {
		log.Warn("KAFKA_BROKERS unset — consumer disabled")
	}

	// ── Use case ────────────────────────────────────────────────
	var processUC *usecase.ProcessVideoUseCase
	if s3Store != nil && pub != nil {
		processUC = usecase.NewProcessVideo(s3Store, s3Store, ffmpeg.NewRunner(), pub, log)
	}

	// ── Saga consumer (video.uploaded) ───────────────────────────
	if len(cfg.KafkaBrokers) > 0 && redisClient != nil && processUC != nil {
		idemStore := idempotency.NewRedisStore(redisClient, cfg.IdempotencyTTL)
		reader := messaging.NewReader(cfg.KafkaBrokers, consumerGroup, string(events.TopicVideoUploaded))
		dlqWriter := messaging.NewWriter(cfg.KafkaBrokers)
		consumer := saga.NewConsumer(reader, dlqWriter, consumerGroup, idemStore, processUC)
		go func() {
			defer func() { _ = reader.Close(); _ = dlqWriter.Close() }()
			if err := consumer.Start(ctx, log); err != nil && err != context.Canceled {
				log.Error("saga consumer exited", slog.Any("error", err))
			}
		}()
	}

	// ── Health probes ──────────────────────────────────────────
	probes := map[string]health.Probe{
		"self": func() health.Check { return health.Check{Status: "healthy"} },
		"redis": func() health.Check {
			if err := cache.Healthy(ctx, redisClient); err != nil {
				return health.Check{Status: "unhealthy", Detail: err.Error()}
			}
			return health.Check{Status: "healthy"}
		},
		"s3": func() health.Check {
			if s3Store == nil {
				return health.Check{Status: "unhealthy", Detail: "client not initialized"}
			}
			if err := s3Store.Healthy(ctx); err != nil {
				return health.Check{Status: "unhealthy", Detail: err.Error()}
			}
			return health.Check{Status: "healthy"}
		},
	}
	hh := health.New(version, probes)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer, metrics.Middleware)
	r.Get("/health", hh.Live)
	r.Get("/ready", hh.Ready)
	r.Handle("/metrics", metrics.Handler())

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	idle := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		shutdownCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
		}
		close(idle)
	}()
	log.Info("server listening", slog.String("port", cfg.Port))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
	<-idle
	log.Info("shutdown complete")
}
