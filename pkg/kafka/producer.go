package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"simple-securities/common/constants"
	"simple-securities/pkg/datetime"

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
		Async:        false,            // wait for ack
		BatchTimeout: 0,                // send immediately || time.Second * 2
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
	headers map[string]string,
	event Event,
) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 2. Convert map to kafka.Header slice
	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	msg := kafka.Message{
		Headers: kafkaHeaders,
		Topic:   topic,
		Value:   eventBytes,
		Time:    datetime.Now(),
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

	p.logger.Info("🔔 Kafka sends",
		zap.String(constants.Topic, topic),
		zap.String(constants.Key, key),
		zap.Int(constants.Partition, msg.Partition),
		zap.Any(constants.Header, headers),
		zap.Any(constants.Event, event),
	)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
