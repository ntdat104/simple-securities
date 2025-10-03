package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type EventHandler func(ctx context.Context, key, event []byte) error

type Consumer struct {
	reader  *kafka.Reader
	logger  *zap.Logger
	handler EventHandler
}

func NewConsumer(brokers []string, topic, groupID string, handler EventHandler, logger *zap.Logger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	return &Consumer{
		reader:  reader,
		logger:  logger,
		handler: handler,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("🧩 connect kafka consumer",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping kafka consumer")
			return c.reader.Close()
		default:
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.logger.Error("failed to read message", zap.Error(err))
				continue
			}

			c.logger.Debug("📥 kafka recieves",
				zap.String("topic", m.Topic),
				zap.Int("partition", m.Partition),
				zap.Int64("offset", m.Offset),
				zap.ByteString("key", m.Key),
				zap.String("event", string(m.Value)),
			)

			if err := c.handler(ctx, m.Key, m.Value); err != nil {
				c.logger.Error("failed to handle message", zap.Error(err))
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func (c *Consumer) ConsumeMessages(ctx context.Context, handler func(key, event []byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				return fmt.Errorf("failed to read message: %w", err)
			}

			if err := handler(m.Key, m.Value); err != nil {
				c.logger.Error("failed to process message", zap.Error(err))
			}
		}
	}
}
