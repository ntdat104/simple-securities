package messaging

import (
	"context"
	"encoding/json"

	"simple-securities/common/constants"
	"simple-securities/config"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/messaging"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/kafka"

	"go.uber.org/zap"
)

type KafkaEventPublisher struct {
	manager *kafka.Manager
	logger  *zap.Logger
}

func NewKafkaEventPublisher(manager *kafka.Manager, logger *zap.Logger) messaging.IEventPublisher {
	return &KafkaEventPublisher{
		manager: manager,
		logger:  logger,
	}
}

func (p *KafkaEventPublisher) Publish(ctx context.Context, event messaging.DomainEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	requestId := util.GetValueFromCtx(ctx, constants.RequestId)
	now := datetime.Now()
	kafkaEvent := kafka.Event{
		Meta: kafka.Meta{
			ServiceName: config.GlobalConfig.App.Name,
			RequestID:   requestId,
			Code:        200,
			Message:     "Success",
			Timestamp:   now.Unix(),
			Datetime:    datetime.ConvertTimeToString(now, datetime.YYYY_MM_DD_HH_MM_SS),
		},
		Data: json.RawMessage(data),
	}

	return p.manager.NewSendMessage().
		Topic(event.EventName()).
		Key(event.EventKey()).
		Partition(1).
		Headers(map[string]string{
			constants.RequestId: requestId,
			constants.Timestamp: conv.ConvertInt64ToString(kafkaEvent.Meta.Timestamp),
			constants.Datetime:  kafkaEvent.Meta.Datetime,
		}).
		Event(kafkaEvent).
		Do(ctx)
}
