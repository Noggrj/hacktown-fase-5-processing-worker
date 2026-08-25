// Package saga wires Kafka transport for the Processing Worker:
//   - Publisher emits video.processed and video.failed
//   - Consumer subscribes to video.uploaded
package saga

import (
	"context"
	"fmt"
	"time"

	events "github.com/noggrj/fiapx-events"
	"github.com/noggrj/fiapx-events/payloads"
	eventskafka "github.com/noggrj/fiapx-events/transport/kafka"
)

const sourceName = "processing-worker"

// Publisher implements usecase.Publisher.
type Publisher struct {
	inner *eventskafka.Publisher
}

func NewPublisher(inner *eventskafka.Publisher) (*Publisher, error) {
	if inner == nil {
		return nil, fmt.Errorf("nil kafka publisher")
	}
	return &Publisher{inner: inner}, nil
}

func (p *Publisher) PublishVideoProcessed(ctx context.Context, traceparent, videoID, s3ZipKey string, frameCount int) error {
	payload := payloads.VideoProcessed{
		VideoID:     videoID,
		S3ZipKey:    s3ZipKey,
		FrameCount:  frameCount,
		ProcessedAt: time.Now().UTC(),
	}
	env, err := events.NewEnvelope(events.TopicVideoProcessed, sourceName, videoID, traceparent, payload)
	if err != nil {
		return err
	}
	return p.inner.Publish(ctx, env)
}

func (p *Publisher) PublishVideoFailed(ctx context.Context, traceparent, videoID, userID, userEmail, reason string) error {
	payload := payloads.VideoFailed{
		VideoID:   videoID,
		UserID:    userID,
		UserEmail: userEmail,
		Reason:    reason,
		FailedAt:  time.Now().UTC(),
	}
	env, err := events.NewEnvelope(events.TopicVideoFailed, sourceName, videoID, traceparent, payload)
	if err != nil {
		return err
	}
	return p.inner.Publish(ctx, env)
}
