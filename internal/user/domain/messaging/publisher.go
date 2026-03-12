package messaging

import "context"

type IEventPublisher interface {
	Publish(ctx context.Context, event DomainEvent) error
}

type DomainEvent interface {
	EventName() string // e.g. "user.registered"
	EventKey() string  // Kafka partition key, typically UserUUID
}
