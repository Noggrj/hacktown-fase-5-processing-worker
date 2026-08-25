// Package messaging wraps segmentio/kafka-go writer/reader construction
// shared by the saga publisher and consumers.
package messaging

import (
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// NewWriter creates a Writer with no fixed Topic — every fiapx-events
// Publish call sets kafkago.Message.Topic per message, so one Writer is
// reused for every topic this service publishes to.
func NewWriter(brokers []string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.Hash{}, // key-based partitioning keeps a video's events in order
		RequiredAcks: kafkago.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
	}
}

// NewReader creates a Reader bound to a single topic within a consumer
// group — multiple replicas of the same service sharing groupID split
// the topic's partitions between them.
//
// StartOffset is explicit (FirstOffset) rather than left at the
// zero-value: a consumer group that has never committed an offset for
// this topic must start from the beginning, not from whatever happens to
// be latest at join time — otherwise a message published before this
// service's first-ever startup (or during a redeploy gap) would be
// silently skipped, which is exactly the "não deve perder uma
// requisição" requirement this system exists to satisfy.
func NewReader(brokers []string, groupID, topic string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       topic,
		StartOffset: kafkago.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
}
