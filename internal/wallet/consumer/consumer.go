// Package consumer handles the Kafka events the wallet reacts to: user registrations (airdrop),
// matching events (settlement, lock release) and futures -> spot transfers.
package consumer

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/internal/wallet/app"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/inbox"
	"simple-securities/pkg/kafka"
)

// Group is the consumer group (and inbox consumer name).
const Group = "wallet"

// Topics consumed by the wallet.
var Topics = []string{events.TopicUserEvents, events.TopicMatchingEvents, events.TopicFuturesTransfers}

// Handler processes batches in one transaction with inbox dedup.
type Handler struct {
	db  *gorm.DB
	svc *app.Service
	log *zap.Logger
}

// NewHandler returns the batch handler.
func NewHandler(db *gorm.DB, svc *app.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, svc: svc, log: log}
}

// New builds the group consumer.
func New(brokers []string, h *Handler, log *zap.Logger) (*kafka.GroupConsumer, error) {
	return kafka.NewGroupConsumer(kafka.GroupConsumerConfig{Brokers: brokers, Group: Group, Topics: Topics},
		h.Handle, log)
}

// Handle implements kafka.BatchHandler: claim every message, decode the relevant ones and apply
// them in order in a single transaction.
func (h *Handler) Handle(ctx context.Context, msgs []kafka.Message) error {
	applied := false
	err := database.WithTx(ctx, h.db, func(tx *gorm.DB) error {
		evs := make([]app.Event, 0, len(msgs))
		for _, m := range msgs {
			env := m.Envelope
			ok, err := inbox.Claim(ctx, tx, Group, env.ID, env.Type)
			if err != nil {
				return err
			}
			if !ok {
				continue // duplicate delivery
			}
			ev, relevant, err := decode(env)
			if err != nil {
				return fmt.Errorf("%s %s (offset %d): %w", env.Type, env.ID, m.Offset, err)
			}
			if relevant {
				evs = append(evs, ev)
			}
		}
		if len(evs) == 0 {
			return nil
		}
		applied = true
		return h.svc.ApplyEvents(ctx, tx, evs)
	})
	if err != nil {
		return err
	}
	if applied {
		h.svc.Notify()
	}
	return nil
}

// decode maps an envelope to an app.Event; relevant is false for events the wallet ignores.
func decode(env events.Envelope) (ev app.Event, relevant bool, err error) {
	switch env.Type {
	case events.TypeUserRegistered:
		var p events.UserRegistered
		if err := env.Decode(&p); err != nil {
			return ev, false, err
		}
		return app.Event{UserRegistered: &p}, true, nil
	case events.TypeTradeExecuted:
		var p events.TradeExecuted
		if err := env.Decode(&p); err != nil {
			return ev, false, err
		}
		return app.Event{Trade: &p}, true, nil
	case events.TypeOrderStatusChanged:
		var p events.OrderStatusChanged
		if err := env.Decode(&p); err != nil {
			return ev, false, err
		}
		if !events.IsTerminal(p.Status) {
			return ev, false, nil
		}
		return app.Event{OrderClosed: &p}, true, nil
	case events.TypeFuturesToSpot:
		var p events.Transfer
		if err := env.Decode(&p); err != nil {
			return ev, false, err
		}
		return app.Event{FromFutures: &p}, true, nil
	}
	return ev, false, nil
}
