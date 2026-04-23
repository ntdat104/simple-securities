package dto

import "simple-securities/internal/user/domain/enum"

// --- TopUp ---

type TopUpReq struct {
	UserID         uint64          `json:"user_id"`
	WalletType     enum.WalletType `json:"wallet_type"`
	Amount         int64           `json:"amount"`
	ExpireAtMs     *int64          `json:"expire_at_ms"` // unix ms; nil = no expiry
	IdempotencyKey string          `json:"idempotency_key"`
	ReferenceID    string          `json:"reference_id"`
}

type TopUpResp struct {
	WalletID   string `json:"wallet_id"`
	NewBalance int64  `json:"new_balance"`
}

// --- ReserveCredits ---

type ReserveCreditsReq struct {
	UserID         uint64 `json:"user_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
	StockSymbol    string `json:"stock_symbol"`
	RequestID      string `json:"request_id"`
}

type WalletDeductionDto struct {
	WalletID   string          `json:"wallet_id"`
	WalletType enum.WalletType `json:"wallet_type"`
	Amount     int64           `json:"amount"`
}

type ReserveCreditsResp struct {
	ReservationID  string               `json:"reservation_id"`
	ReservedAmount int64                `json:"reserved_amount"`
	Deductions     []WalletDeductionDto `json:"deductions"`
}

// --- CommitReservation ---

type CommitReservationReq struct {
	ReservationID  string `json:"reservation_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type CommitReservationResp struct {
	Success       bool  `json:"success"`
	TotalDeducted int64 `json:"total_deducted"`
}

// --- RollbackReservation ---

type RollbackReservationReq struct {
	ReservationID string `json:"reservation_id"`
	Reason        string `json:"reason"`
}

type RollbackReservationResp struct {
	Success bool `json:"success"`
}

// --- GetCreditBalance ---

type GetCreditBalanceReq struct {
	UserID uint64 `json:"user_id"`
}

type WalletBalanceDto struct {
	WalletID   string          `json:"wallet_id"`
	WalletType enum.WalletType `json:"wallet_type"`
	Balance    int64           `json:"balance"`
	Reserved   int64           `json:"reserved"`
	Available  int64           `json:"available"`
	ExpireAtMs *int64          `json:"expire_at_ms,omitempty"`
}

type GetCreditBalanceResp struct {
	UserID         uint64             `json:"user_id"`
	TotalAvailable int64              `json:"total_available"`
	Wallets        []WalletBalanceDto `json:"wallets"`
}
