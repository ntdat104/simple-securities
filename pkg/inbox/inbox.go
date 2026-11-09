// Package inbox implements idempotent consumption: a consumer claims an event id inside the same
// transaction that applies the event. A duplicate delivery (Kafka is at-least-once) fails the claim
// and is skipped, giving effectively-once processing.
package inbox

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Event is a processed event id per consumer (table inbox_events).
type Event struct {
	EventID    string    `gorm:"type:uuid;primaryKey"`
	Consumer   string    `gorm:"primaryKey"`
	EventType  string    `gorm:"not null"`
	ReceivedAt time.Time `gorm:"not null;default:now();index:idx_inbox_received"`
}

// TableName implements gorm.Tabler.
func (Event) TableName() string { return "inbox_events" }

// Claim records (eventID, consumer). It returns false when the event was already processed.
// It MUST be called with the transaction that applies the event's side effects.
func Claim(ctx context.Context, tx *gorm.DB, consumer, eventID, eventType string) (bool, error) {
	res := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&Event{EventID: eventID, Consumer: consumer, EventType: eventType, ReceivedAt: time.Now().UTC()})
	if res.Error != nil {
		return false, fmt.Errorf("inbox claim: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}
