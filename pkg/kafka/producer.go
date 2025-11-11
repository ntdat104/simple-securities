package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Producer struct {
	writer *kafka.Writer
	logger *zap.Logger
}

func NewProducer(brokers []string, logger *zap.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne, // wait for leader
		Async:        false, // wait for ack
		BatchTimeout: 0, // send immediately || time.Second * 2
	}

	return &Producer{
		writer: writer,
		logger: logger,
	}
}

// SendMessage sends an event to Kafka.
// - If key is provided → Kafka hashes key → same key = same partition.
// - If key is empty → Kafka balances messages across partitions.
// - If partition >= 0 → overrides Kafka partitioner.
func (p *Producer) SendMessage(
	ctx context.Context,
	topic string,
	key string,
	partition int, // set -1 to let Kafka decide
	event Event,
) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		Value: eventBytes,
		Time:  time.Now(),
	}

	// Add key if provided
	if key != "" {
		msg.Key = []byte(key)
	}

	// Force partition if explicitly given
	if partition >= 0 {
		msg.Partition = partition
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	p.logger.Info("📤 kafka sends",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.Int("partition", msg.Partition),
		zap.ByteString("event", eventBytes),
	)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
