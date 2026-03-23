package messaging

import "context"

// IEventPublisher publishes domain events to the message bus (Kafka).
type IEventPublisher interface {
	Publish(ctx context.Context, event any) error
}

// --- Domain Events ---

type CreditToppedUp struct {
	UserID         uint64 `json:"user_id"`
	WalletType     string `json:"wallet_type"`
	Amount         int64  `json:"amount"`
	NewBalance     int64  `json:"new_balance"`
	ReferenceID    string `json:"reference_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Timestamp      int64  `json:"timestamp"`
}

type CreditReserved struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	TotalAmount   int64  `json:"total_amount"`
	StockSymbol   string `json:"stock_symbol"`
	RequestID     string `json:"request_id"`
	Timestamp     int64  `json:"timestamp"`
}

type CreditCommitted struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	TotalDeducted int64  `json:"total_deducted"`
	Timestamp     int64  `json:"timestamp"`
}

type CreditRolledBack struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	Reason        string `json:"reason"`
	Timestamp     int64  `json:"timestamp"`
}
