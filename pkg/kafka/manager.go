package kafka

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Config struct {
	ServiceName string
	Version     string
	Port        string
	Env         string
}

type Manager struct {
	config    Config
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

func NewManager(cfg Config, brokers []string, logger *zap.Logger) *Manager {
	return &Manager{
		config:          cfg,
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

func (m *Manager) NewSendMessage() *SendMessage {
	return &SendMessage{
		m:         m,
		partition: -1,
	}
}

func (s *SendMessage) Topic(topic string) *SendMessage {
	s.topic = topic
	return s
}

func (s *SendMessage) Key(key string) *SendMessage {
	s.key = key
	return s
}

func (s *SendMessage) Partition(partition int) *SendMessage {
	s.partition = partition
	return s
}

func (s *SendMessage) Headers(headers map[string]string) *SendMessage {
	s.headers = headers
	return s
}

func (s *SendMessage) Event(event Event) *SendMessage {
	s.event = event
	return s
}

func (s *SendMessage) Do(ctx context.Context) error {
	s.m.controlMu.RLock()
	defer s.m.controlMu.RUnlock()

	if !s.m.producerEnabled {
		return fmt.Errorf("kafka producer is disabled")
	}

	headers := map[string]string{
		"service_name": s.m.config.ServiceName,
		"version":      s.m.config.Version,
		"port":         s.m.config.Port,
		"env":          s.m.config.Env,
	}
	for k, v := range s.headers {
		headers[k] = v
	}

	return s.m.producer.SendMessage(ctx, s.topic, s.key, s.partition, headers, s.event)
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
