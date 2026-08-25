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
func NewReader(brokers []string, groupID, topic string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
}
