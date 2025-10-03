package kafka

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Manager struct {
	producer  *Producer
	consumers map[string]*Consumer
	logger    *zap.Logger
	mu        sync.RWMutex
	brokers   []string

	// Control flags
	producerEnabled bool
	consumerEnabled bool
	controlMu       sync.RWMutex
}

func NewManager(brokers []string, logger *zap.Logger) *Manager {
	return &Manager{
		producer:        NewProducer(brokers, logger),
		consumers:       make(map[string]*Consumer),
		logger:          logger,
		brokers:         brokers,
		producerEnabled: true, // Producer enabled by default
		consumerEnabled: true, // Consumer enabled by default
	}
}

func NewManagerWithConfig(brokers []string, logger *zap.Logger, producerEnabled, consumerEnabled bool) *Manager {
	return &Manager{
		producer:        NewProducer(brokers, logger),
		consumers:       make(map[string]*Consumer),
		logger:          logger,
		brokers:         brokers,
		producerEnabled: producerEnabled,
		consumerEnabled: consumerEnabled,
	}
}

func (m *Manager) SendMessage(ctx context.Context, topic string, key string, partition int, event Event) error {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()

	if !m.producerEnabled {
		return fmt.Errorf("kafka producer is disabled")
	}

	return m.producer.SendMessage(ctx, topic, key, partition, event) // partition -1 = let Kafka decide
}

func (m *Manager) AddConsumer(topic, groupID string, handler EventHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.consumers[topic]; exists {
		return fmt.Errorf("consumer for topic %s already exists", topic)
	}

	consumer := NewConsumer(
		m.brokers,
		topic,
		groupID,
		handler,
		m.logger,
	)

	m.consumers[topic] = consumer
	return nil
}

func (m *Manager) StartConsumer(ctx context.Context, topic string) error {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()

	if !m.consumerEnabled {
		return fmt.Errorf("kafka consumer is disabled")
	}

	m.mu.RLock()
	consumer, exists := m.consumers[topic]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("consumer for topic %s not found", topic)
	}

	return consumer.Start(ctx)
}

func (m *Manager) StartAllConsumers(ctx context.Context) {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()

	if !m.consumerEnabled {
		m.logger.Warn("kafka consumer is disabled, skipping start all consumers")
		return
	}

	for topic, consumer := range m.consumers {
		go func(t string, c *Consumer) {
			if err := c.Start(ctx); err != nil {
				m.logger.Error("consumer stopped with error", zap.String("topic", t), zap.Error(err))
			}
		}(topic, consumer)
	}
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all consumers
	for topic, consumer := range m.consumers {
		if err := consumer.Close(); err != nil {
			m.logger.Error("failed to close consumer", zap.String("topic", topic), zap.Error(err))
		}
	}

	// Close producer
	return m.producer.Close()
}

func (m *Manager) EnableProducer() {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.producerEnabled = true
	m.logger.Info("kafka producer enabled")
}

func (m *Manager) DisableProducer() {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.producerEnabled = false
	m.logger.Info("kafka producer disabled")
}

func (m *Manager) IsProducerEnabled() bool {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()
	return m.producerEnabled
}

func (m *Manager) EnableConsumer() {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.consumerEnabled = true
	m.logger.Info("kafka consumer enabled")
}

func (m *Manager) DisableConsumer() {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.consumerEnabled = false
	m.logger.Info("kafka consumer disabled")
}

func (m *Manager) IsConsumerEnabled() bool {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()
	return m.consumerEnabled
}

func (m *Manager) GetStatus() map[string]interface{} {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()

	m.mu.RLock()
	consumerCount := len(m.consumers)
	m.mu.RUnlock()

	return map[string]interface{}{
		"producer_enabled": m.producerEnabled,
		"consumer_enabled": m.consumerEnabled,
		"consumer_count":   consumerCount,
		"brokers":          m.brokers,
	}
}
