package di

import (
	"simple-securities/internal/user/infras/messaging"

	"github.com/google/wire"
)

var (
	KafkaEventPublisherSet = wire.NewSet(messaging.NewKafkaEventPublisher)
	RabitMqEventPublisher  = wire.NewSet(messaging.NewRabitMqEventPublisher)
)
