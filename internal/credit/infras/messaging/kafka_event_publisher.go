package messaging

import (
	"context"
	"encoding/json"
	"time"

	"simple-securities/internal/credit/domain/messaging"
	"simple-securities/pkg/kafka"
)

const topicCredit = "credit.events"

type kafkaEventPublisher struct {
	producer *kafka.Producer
}

func NewKafkaEventPublisher(producer *kafka.Producer) messaging.IEventPublisher {
	return &kafkaEventPublisher{producer: producer}
}

func (p *kafkaEventPublisher) Publish(ctx context.Context, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.producer.SendMessage(ctx, topicCredit, "", 0, nil, kafka.Event{
		Meta: kafka.Meta{
			ServiceName: "credit",
			Timestamp:   time.Now().UnixMilli(),
		},
		Data: json.RawMessage(data),
	})
}
