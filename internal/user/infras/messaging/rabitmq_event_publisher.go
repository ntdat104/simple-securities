package messaging

import (
	"context"

	"simple-securities/internal/user/domain/messaging"
)

type RabitMqEventPublisher struct{}

func NewRabitMqEventPublisher() messaging.IEventPublisher {
	return &RabitMqEventPublisher{}
}

func (p *RabitMqEventPublisher) Publish(_ context.Context, _ messaging.DomainEvent) error {
	return nil
}
