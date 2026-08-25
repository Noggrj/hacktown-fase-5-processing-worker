package saga

import (
	"context"
	"fmt"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	events "github.com/noggrj/fiapx-events"
	"github.com/noggrj/fiapx-events/idempotency"
	"github.com/noggrj/fiapx-events/payloads"
	eventskafka "github.com/noggrj/fiapx-events/transport/kafka"

	"github.com/noggrj/fiapx-processing-worker/internal/processing/usecase"
)

// Consumer subscribes to video.uploaded and hands each event to
// ProcessVideoUseCase, guarded by idempotency.
type Consumer struct {
	reader       *kafkago.Reader
	dlq          eventskafka.Writer
	consumerName string
	store        idempotency.Store
	process      *usecase.ProcessVideoUseCase
}

func NewConsumer(
	reader *kafkago.Reader,
	dlq eventskafka.Writer,
	consumerName string,
	store idempotency.Store,
	process *usecase.ProcessVideoUseCase,
) *Consumer {
	return &Consumer{reader: reader, dlq: dlq, consumerName: consumerName, store: store, process: process}
}

func (c *Consumer) Start(ctx context.Context, log *slog.Logger) error {
	inner, err := eventskafka.NewConsumer(c.reader, c.dlq)
	if err != nil {
		return fmt.Errorf("setup consumer: %w", err)
	}

	dispatch := func(ctx context.Context, env *events.Envelope, _ kafkago.Message) error {
		var p payloads.VideoUploaded
		if err := env.Decode(&p); err != nil {
			return err
		}
		log.Info("event received",
			slog.String("eventId", env.EventID),
			slog.String("videoId", p.VideoID))
		return c.process.Execute(ctx, p, env.Traceparent)
	}

	withIdem := idempotency.Middleware(c.store, c.consumerName, dispatch)
	log.Info("saga consumer ready", slog.String("consumer", c.consumerName))
	return inner.Consume(ctx, withIdem)
}
