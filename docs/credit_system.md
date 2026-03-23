# Credit Wallet System — Tài liệu thiết kế

> Ngày tạo: 2026-03-23
> Service: `internal/credit`
> Microservice liên quan: `personal` (gọi unlock stock), `core/credit` (trừ credit)

---

## Mục lục

1. [Tổng quan](#1-tổng-quan)
2. [Cấu trúc file](#2-cấu-trúc-file)
3. [Database Schema](#3-database-schema)
4. [Proto / gRPC API](#4-proto--grpc-api)
5. [Domain Layer](#5-domain-layer)
6. [Application Layer](#6-application-layer)
7. [Infras Layer](#7-infras-layer)
8. [Handler Layer](#8-handler-layer)
9. [DI / Wire Providers](#9-di--wire-providers)
10. [Flow: Unlock Stock — 2-Phase Sync](#10-flow-unlock-stock--2-phase-sync)
11. [Flow: Unlock Stock — Kafka Saga](#11-flow-unlock-stock--kafka-saga)
12. [Sync vs Async — phân tích từng request](#12-sync-vs-async--phân-tích-từng-request)
13. [Các pattern đã áp dụng](#13-các-pattern-đã-áp-dụng)
14. [So sánh 2-Phase Sync vs Kafka Saga](#14-so-sánh-2-phase-sync-vs-kafka-saga)

---

## 1. Tổng quan

Mỗi user có nhiều **ví credit** (wallet) phân loại theo `wallet_type`:

| Type | Nguồn | Có expire? |
|---|---|---|
| `MEMBERSHIP` | Credit tặng từ membership | Có |
| `COURSE` | Credit tặng từ mua khoá học | Có |
| `PURCHASED` | Credit mua trực tiếp | **Không** |
| `GIVEAWAY` | Credit give away | Có |

**Chiến lược tiêu credit**: khi unlock stock, hệ thống tự động dùng credit theo thứ tự **expire sớm nhất trước** (FIFO by expiry). `PURCHASED` luôn dùng cuối cùng.

**Flow cơ bản**:
1. Personal Service gọi `ReserveCredits` → Credit Service soft-reserve credit
2. Personal insert unlock record vào DB
3. Personal gọi `CommitReservation` → Credit Service trừ credit thực sự
4. Nếu bước 2 hoặc 3 fail → gọi `RollbackReservation` để giải phóng reserved credit

---

## 2. Cấu trúc file

```
proto/credit/v1/
  credit_service.proto

migrations/mysql/
  000003_init.creditdb.up.sql
  000003_init.creditdb.down.sql

internal/credit/
  domain/
    enum/
      wallet_type.go           # MEMBERSHIP | COURSE | PURCHASED | GIVEAWAY
      tx_type.go               # TOP_UP | RESERVE | COMMIT | ROLLBACK | EXPIRE
      reservation_status.go    # PENDING | COMMITTED | ROLLED_BACK
    model/
      credit_wallet.go         # Entity + domain methods (SoftReserve, CommitReserved, ReleaseReserved)
      credit_transaction.go    # Immutable ledger entry
      credit_reservation.go    # Soft reservation + Commit/Rollback methods
    repo/
      credit_wallet_repo.go    # ICreditWalletRepo interface
      credit_transaction_repo.go
      credit_reservation_repo.go
      credit_reservation_item_repo.go
    messaging/
      event_publisher.go       # IEventPublisher + tất cả domain events

  application/
    constant/
      errors.go                # App errors + Redis key constants
    dto/
      credit_dto.go            # Request/Response DTOs cho tất cả use cases
    service/
      interfaces.go            # DistributedLock, IdempotencyStore interfaces
      top_up_svc.go
      reserve_credits_svc.go
      commit_reservation_svc.go
      rollback_reservation_svc.go
      get_credit_balance_svc.go

  infras/
    repo/
      credit_wallet_repo.go    # Concrete sqlx implementation
      credit_transaction_repo.go
      credit_reservation_repo.go
      credit_reservation_item_repo.go
    lock/
      redis_distributed_lock.go
    idem/
      redis_idempotency_store.go
    messaging/
      kafka_event_publisher.go
      saga/
        stock_unlocked_consumer.go       # Credit consumes StockUnlocked → auto commit
        credit_commit_failed_consumer.go # Personal consumes → compensate

  handler/
    grpc/
      credit_grpc_handler.go
      grpc_helper.go

  di/
    providers.go               # Wire providers cho tất cả dependencies
```

---

## 3. Database Schema

File: `migrations/mysql/000003_init.creditdb.up.sql`

```sql
START TRANSACTION;

-- ============================================================
-- credit_wallets: one row per (user, wallet_type) bucket.
-- PURCHASED type has NULL expire_at (no expiry).
-- ============================================================
CREATE TABLE IF NOT EXISTS credit_wallets (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid        CHAR(36)     NOT NULL UNIQUE,
    user_id     BIGINT UNSIGNED NOT NULL,
    wallet_type VARCHAR(30)  NOT NULL,            -- MEMBERSHIP | COURSE | PURCHASED | GIVEAWAY
    balance     BIGINT       NOT NULL DEFAULT 0,  -- total topped-up, never goes below 0
    reserved    BIGINT       NOT NULL DEFAULT 0,  -- soft-reserved, committed deductions reduce balance
    expire_at   DATETIME     NULL,                -- NULL = no expiry (PURCHASED)
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_cw_user_id (user_id),
    INDEX idx_cw_user_type (user_id, wallet_type),
    INDEX idx_cw_expire (expire_at),
    CONSTRAINT chk_cw_balance  CHECK (balance  >= 0),
    CONSTRAINT chk_cw_reserved CHECK (reserved >= 0),
    CONSTRAINT chk_cw_reserved_lte CHECK (reserved <= balance)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- credit_transactions: immutable ledger of every credit movement.
-- ============================================================
CREATE TABLE IF NOT EXISTS credit_transactions (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid            CHAR(36)        NOT NULL UNIQUE,
    user_id         BIGINT UNSIGNED NOT NULL,
    wallet_id       BIGINT UNSIGNED NOT NULL,
    wallet_type     VARCHAR(30)     NOT NULL,
    tx_type         VARCHAR(30)     NOT NULL,  -- TOP_UP | RESERVE | COMMIT | ROLLBACK | EXPIRE
    amount          BIGINT          NOT NULL,  -- always positive; direction from tx_type
    balance_before  BIGINT          NOT NULL,
    balance_after   BIGINT          NOT NULL,
    reservation_id  CHAR(36)        NULL,      -- links RESERVE / COMMIT / ROLLBACK rows
    reference_id    VARCHAR(255)    NULL,
    idempotency_key CHAR(36)        NULL,
    note            VARCHAR(500)    NULL,
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_ct_user_id (user_id),
    INDEX idx_ct_wallet_id (wallet_id),
    INDEX idx_ct_reservation (reservation_id),
    INDEX idx_ct_idempotency (idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- credit_reservations: tracks soft-reservations (pending unlock).
-- ============================================================
CREATE TABLE IF NOT EXISTS credit_reservations (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid            CHAR(36)        NOT NULL UNIQUE,
    user_id         BIGINT UNSIGNED NOT NULL,
    total_amount    BIGINT          NOT NULL,
    status          VARCHAR(20)     NOT NULL DEFAULT 'PENDING',
    idempotency_key CHAR(36)        NOT NULL UNIQUE,  -- idempotency tại reservation level
    stock_symbol    VARCHAR(20)     NULL,
    request_id      VARCHAR(255)    NULL,
    committed_at    DATETIME        NULL,
    rolled_back_at  DATETIME        NULL,
    expires_at      DATETIME        NOT NULL,          -- reservation TTL (10 phút)
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_cr_user_id (user_id),
    INDEX idx_cr_status (status),
    INDEX idx_cr_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- credit_reservation_items: per-wallet breakdown của một reservation.
-- ============================================================
CREATE TABLE IF NOT EXISTS credit_reservation_items (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    reservation_id BIGINT UNSIGNED NOT NULL,
    wallet_id      BIGINT UNSIGNED NOT NULL,
    wallet_type    VARCHAR(30)     NOT NULL,
    amount         BIGINT          NOT NULL,

    INDEX idx_cri_reservation (reservation_id),
    CONSTRAINT fk_cri_reservation FOREIGN KEY (reservation_id)
        REFERENCES credit_reservations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- idempotency_keys: cross-service dedup store cho TopUp calls.
-- ============================================================
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    idem_key        CHAR(36)        NOT NULL UNIQUE,
    service         VARCHAR(50)     NOT NULL,
    response_status VARCHAR(20)     NOT NULL DEFAULT 'PROCESSING',
    response_body   TEXT            NULL,
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME        NOT NULL,

    INDEX idx_ik_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

COMMIT;
```

**Giải thích 5 tables**:

| Table | Mục đích |
|---|---|
| `credit_wallets` | Một row per `(user_id, wallet_type)`. `expire_at NULL` = PURCHASED |
| `credit_transactions` | **Immutable ledger** — mọi movement đều có record, không bao giờ delete/update |
| `credit_reservations` | Soft reservation per unlock request, TTL 10 phút |
| `credit_reservation_items` | Breakdown wallet nào bị reserve bao nhiêu credit |
| `idempotency_keys` | Lưu response để dedup TopUp call (optional — Redis là primary) |

---

## 4. Proto / gRPC API

File: `proto/credit/v1/credit_service.proto`

```protobuf
syntax = "proto3";
package credit.v1;
option go_package = "simple-securities/gen/credit/v1";

import "google/api/annotations.proto";

enum WalletType {
  WALLET_TYPE_UNSPECIFIED   = 0;
  WALLET_TYPE_MEMBERSHIP    = 1; // credit tặng từ membership — có expire
  WALLET_TYPE_COURSE        = 2; // credit tặng từ mua khoá học — có expire
  WALLET_TYPE_PURCHASED     = 3; // credit mua — KHÔNG có expire
  WALLET_TYPE_GIVEAWAY      = 4; // credit give away — có expire
}

enum ReservationStatus {
  RESERVATION_STATUS_UNSPECIFIED = 0;
  RESERVATION_STATUS_PENDING     = 1;
  RESERVATION_STATUS_COMMITTED   = 2;
  RESERVATION_STATUS_ROLLED_BACK = 3;
}

service CreditService {
  // Internal — called by admin/membership/course service to top-up credits
  rpc TopUp(TopUpRequest) returns (TopUpResponse) {
    option (google.api.http) = { post: "/api/v1/credits/top-up" body: "*" };
  }

  // Called by personal service: soft-reserve credits atomically
  rpc ReserveCredits(ReserveCreditsRequest) returns (ReserveCreditsResponse) {
    option (google.api.http) = { post: "/api/v1/credits/reserve" body: "*" };
  }

  // Called by personal service after successful stock unlock insert
  rpc CommitReservation(CommitReservationRequest) returns (CommitReservationResponse) {
    option (google.api.http) = { post: "/api/v1/credits/commit" body: "*" };
  }

  // Called by personal service on failure to rollback reservation
  rpc RollbackReservation(RollbackReservationRequest) returns (RollbackReservationResponse) {
    option (google.api.http) = { post: "/api/v1/credits/rollback" body: "*" };
  }

  // Read balance — called by personal/frontend
  rpc GetCreditBalance(GetCreditBalanceRequest) returns (GetCreditBalanceResponse) {
    option (google.api.http) = { get: "/api/v1/credits/balance" };
  }
}
```

---

## 5. Domain Layer

### 5.1 Enums

**`domain/enum/wallet_type.go`**
```go
package enum

type WalletType string

const (
    WalletTypeMembership WalletType = "MEMBERSHIP" // có expire
    WalletTypeCourse     WalletType = "COURSE"     // có expire
    WalletTypePurchased  WalletType = "PURCHASED"  // không expire
    WalletTypeGiveaway   WalletType = "GIVEAWAY"   // có expire
)

func (w WalletType) HasExpiry() bool {
    return w != WalletTypePurchased
}
```

**`domain/enum/tx_type.go`**
```go
type TxType string

const (
    TxTypeTopUp    TxType = "TOP_UP"
    TxTypeReserve  TxType = "RESERVE"
    TxTypeCommit   TxType = "COMMIT"
    TxTypeRollback TxType = "ROLLBACK"
    TxTypeExpire   TxType = "EXPIRE"
)
```

**`domain/enum/reservation_status.go`**
```go
type ReservationStatus string

const (
    ReservationPending    ReservationStatus = "PENDING"
    ReservationCommitted  ReservationStatus = "COMMITTED"
    ReservationRolledBack ReservationStatus = "ROLLED_BACK"
)
```

### 5.2 Models

**`domain/model/credit_wallet.go`**
```go
type CreditWallet struct {
    ID         uint64          `db:"id"`
    Uuid       string          `db:"uuid"`
    UserID     uint64          `db:"user_id"`
    WalletType enum.WalletType `db:"wallet_type"`
    Balance    int64           `db:"balance"`   // total; never negative
    Reserved   int64           `db:"reserved"`  // soft-locked amount
    ExpireAt   *time.Time      `db:"expire_at"` // nil = no expiry (PURCHASED)
    CreatedAt  time.Time       `db:"created_at"`
    UpdatedAt  time.Time       `db:"updated_at"`
}

func (w CreditWallet) Available() int64 { return w.Balance - w.Reserved }
func (w CreditWallet) IsExpired() bool  { ... }

// Domain methods
func (w *CreditWallet) AddBalance(amount int64)       { ... } // TopUp
func (w *CreditWallet) SoftReserve(amount int64) bool { ... } // Reserve — returns false nếu không đủ
func (w *CreditWallet) CommitReserved(amount int64)   { ... } // balance -= amount, reserved -= amount
func (w *CreditWallet) ReleaseReserved(amount int64)  { ... } // reserved -= amount (balance intact)
```

**`domain/model/credit_transaction.go`**
```go
// Immutable ledger — không bao giờ update, chỉ insert
type CreditTransaction struct {
    ID             uint64          `db:"id"`
    Uuid           string          `db:"uuid"`
    UserID         uint64          `db:"user_id"`
    WalletID       uint64          `db:"wallet_id"`
    WalletType     enum.WalletType `db:"wallet_type"`
    TxType         enum.TxType     `db:"tx_type"`
    Amount         int64           `db:"amount"`         // always positive
    BalanceBefore  int64           `db:"balance_before"`
    BalanceAfter   int64           `db:"balance_after"`
    ReservationID  *string         `db:"reservation_id"` // nullable — links RESERVE/COMMIT/ROLLBACK
    ReferenceID    *string         `db:"reference_id"`
    IdempotencyKey *string         `db:"idempotency_key"`
    Note           *string         `db:"note"`
    CreatedAt      time.Time       `db:"created_at"`
}
```

**`domain/model/credit_reservation.go`**
```go
type CreditReservation struct {
    ID             uint64                 `db:"id"`
    Uuid           string                 `db:"uuid"`
    UserID         uint64                 `db:"user_id"`
    TotalAmount    int64                  `db:"total_amount"`
    Status         enum.ReservationStatus `db:"status"`
    IdempotencyKey string                 `db:"idempotency_key"` // UNIQUE — DB-level idempotency
    StockSymbol    *string                `db:"stock_symbol"`
    RequestID      *string                `db:"request_id"`
    CommittedAt    *time.Time             `db:"committed_at"`
    RolledBackAt   *time.Time             `db:"rolled_back_at"`
    ExpiresAt      time.Time              `db:"expires_at"`      // TTL 10 phút
    CreatedAt      time.Time              `db:"created_at"`
    UpdatedAt      time.Time              `db:"updated_at"`
}

func (r *CreditReservation) Commit()   { r.Status = enum.ReservationCommitted; ... }
func (r *CreditReservation) Rollback() { r.Status = enum.ReservationRolledBack; ... }
```

### 5.3 Repository Interfaces

**`domain/repo/credit_wallet_repo.go`**
```go
type ICreditWalletRepo interface {
    // Sorted: expire_at ASC NULLS LAST — earliest expire first, PURCHASED last
    FindByUserID(ctx context.Context, userID uint64) ([]*model.CreditWallet, error)
    FindByUserIDAndType(ctx context.Context, userID uint64, walletType enum.WalletType) (*model.CreditWallet, error)
    FindByID(ctx context.Context, id uint64) (*model.CreditWallet, error)
    Save(ctx context.Context, tx *sqlx.Tx, wallet *model.CreditWallet) (*model.CreditWallet, error)
    SaveAll(ctx context.Context, tx *sqlx.Tx, wallets []*model.CreditWallet) ([]*model.CreditWallet, error)
}
```

**`domain/repo/credit_transaction_repo.go`**
```go
type ICreditTransactionRepo interface {
    Save(ctx context.Context, tx *sqlx.Tx, txn *model.CreditTransaction) (*model.CreditTransaction, error)
    SaveAll(ctx context.Context, tx *sqlx.Tx, txns []*model.CreditTransaction) ([]*model.CreditTransaction, error)
}
```

**`domain/repo/credit_reservation_repo.go`**
```go
type ICreditReservationRepo interface {
    FindByUuid(ctx context.Context, uuid string) (*model.CreditReservation, error)
    FindByIdempotencyKey(ctx context.Context, key string) (*model.CreditReservation, error)
    Save(ctx context.Context, tx *sqlx.Tx, r *model.CreditReservation) (*model.CreditReservation, error)
    // Dùng cho sweep job — tự động rollback reservation hết hạn
    FindExpiredPending(ctx context.Context, limit int) ([]*model.CreditReservation, error)
}
```

### 5.4 Domain Events

**`domain/messaging/event_publisher.go`**
```go
type IEventPublisher interface {
    Publish(ctx context.Context, event any) error
}

// Events published by Credit Service → topic: credit.events
type CreditToppedUp struct { UserID, WalletType, Amount, NewBalance, ReferenceID, IdempotencyKey, Timestamp }
type CreditReserved  struct { ReservationID, UserID, TotalAmount, StockSymbol, RequestID, Timestamp }
type CreditCommitted struct { ReservationID, UserID, TotalDeducted, Timestamp }
type CreditRolledBack struct { ReservationID, UserID, Reason, Timestamp }

// Saga events
// Published by Personal Service → topic: stock.unlocked
type StockUnlocked struct {
    ReservationID  string // Credit cần để commit
    UserID         uint64
    StockSymbol    string
    UnlockRecordID string // Personal's DB record ID — dùng cho compensation khi cần rollback
    IdempotencyKey string
    Timestamp      int64
}

// Published by Credit Service khi auto-commit fail → topic: credit.commit.failed
type CreditCommitFailed struct {
    ReservationID  string
    UserID         uint64
    UnlockRecordID string // Personal dùng để xoá unlock record (compensate)
    Reason         string
    Timestamp      int64
}
```

---

## 6. Application Layer

### 6.1 Constants

**`application/constant/errors.go`**
```go
var (
    ErrInsufficientCredits   = errors.New(ErrorTypeBusiness, "insufficient credits")
    ErrWalletNotFound        = errors.New(ErrorTypeNotFound, "credit wallet not found")
    ErrReservationNotFound   = errors.New(ErrorTypeNotFound, "reservation not found")
    ErrReservationExpired    = errors.New(ErrorTypeBusiness, "reservation has expired")
    ErrReservationNotPending = errors.New(ErrorTypeBusiness, "reservation is not in PENDING status")
    ErrDuplicateIdempotency  = errors.New(ErrorTypeBusiness, "duplicate request: idempotency key already used")
    ErrInvalidAmount         = errors.New(ErrorTypeBusiness, "amount must be greater than zero")
    ErrExpireAtRequired      = errors.New(ErrorTypeBusiness, "expire_at is required for this wallet type")
)

const (
    LockKeyUserCredit    = "lock:credit:user:%d"        // distributed lock per user
    LockKeyReservation   = "lock:credit:reservation:%s" // lock per reservation uuid
    IdempotencyKeyPrefix = "idem:credit:%s"
    ReservationTTL       = 10 * 60                      // 10 phút (seconds)
)
```

### 6.2 Interfaces

**`application/service/interfaces.go`**
```go
// DistributedLock — abstraction over Redis SET NX PX
type DistributedLock interface {
    Acquire(ctx context.Context, key string, ttl time.Duration) (unlock func(), err error)
}

// IdempotencyStore — cache completed responses để enable idempotent retries
type IdempotencyStore interface {
    Get(ctx context.Context, key string) (any, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
}
```

### 6.3 Use Cases

#### TopUpSvc (`top_up_svc.go`)

```go
func (s *topUpSvc) Execute(ctx, req) (*TopUpResp, error) {
    // 1. Validate amount > 0, expire_at required nếu wallet có expiry
    // 2. Check Redis idempotency cache (fast-path)
    // 3. Acquire Redis distributed lock (per user)
    // 4. BEGIN TX
    //    - FindByUserIDAndType → tạo mới nếu chưa có
    //    - wallet.AddBalance(amount)
    //    - Save wallet
    //    - Insert CreditTransaction (TOP_UP)
    // 5. COMMIT TX
    // 6. Cache response vào Redis (24h)
    // 7. Publish CreditToppedUp → Kafka (non-fatal)
}
```

#### ReserveCreditsSvc (`reserve_credits_svc.go`)

```go
func (s *reserveCreditsSvc) Execute(ctx, req) (*ReserveCreditsResp, error) {
    // 1. Validate amount > 0
    // 2. Check Redis idempotency cache (fast-path)
    // 3. Check DB idempotency: FindByIdempotencyKey → trả về existing nếu có
    // 4. Acquire Redis distributed lock (per user)
    // 5. BEGIN TX
    //    - FindByUserID → wallets sorted by expire_at ASC (earliest first)
    //    - Greedy allocation: duyệt từng wallet, SoftReserve(take)
    //    - Nếu remaining > 0 sau khi duyệt hết → ErrInsufficientCredits
    //    - SaveAll updated wallets
    //    - Insert CreditReservation
    //    - Insert CreditReservationItems (per-wallet breakdown)
    //    - Insert CreditTransactions (RESERVE per wallet)
    // 6. COMMIT TX
    // 7. Cache response vào Redis (30 phút)
    // 8. Publish CreditReserved → Kafka (non-fatal)
}
```

#### CommitReservationSvc (`commit_reservation_svc.go`)

```go
func (s *commitReservationSvc) Execute(ctx, req) (*CommitReservationResp, error) {
    // 1. Check Redis idempotency cache (fast-path)
    // 2. FindByUuid → verify status == PENDING && not expired
    // 3. Acquire Redis distributed lock (per reservation)
    // 4. FindItemsByReservationID
    // 5. BEGIN TX
    //    - Với mỗi item: FindByID → wallet.CommitReserved(amount) → Save
    //    - Insert CreditTransactions (COMMIT per wallet)
    //    - reservation.Commit() → Save reservation
    // 6. COMMIT TX
    // 7. Cache response vào Redis (24h)
    // 8. Publish CreditCommitted → Kafka (non-fatal)
}
```

#### RollbackReservationSvc (`rollback_reservation_svc.go`)

```go
func (s *rollbackReservationSvc) Execute(ctx, req) (*RollbackReservationResp, error) {
    // 1. FindByUuid
    // 2. Idempotent: status == ROLLED_BACK → return success ngay
    // 3. Verify status == PENDING
    // 4. Acquire Redis distributed lock (per reservation)
    // 5. FindItemsByReservationID
    // 6. BEGIN TX
    //    - Với mỗi item: FindByID → wallet.ReleaseReserved(amount) → Save
    //    - Insert CreditTransactions (ROLLBACK per wallet)
    //    - reservation.Rollback() → Save reservation
    // 7. COMMIT TX
    // 8. Publish CreditRolledBack → Kafka (non-fatal)
}
```

#### GetCreditBalanceSvc (`get_credit_balance_svc.go`)

```go
func (s *getCreditBalanceSvc) Execute(ctx, req) (*GetCreditBalanceResp, error) {
    // 1. FindByUserID → wallets sorted expire_at ASC
    // 2. Filter out expired wallets
    // 3. Sum Available() cho totalAvailable
    // 4. Map sang WalletBalanceDto
}
```

---

## 7. Infras Layer

### 7.1 Redis Distributed Lock

**`infras/lock/redis_distributed_lock.go`**
```go
// Acquire: SET key token NX PX ttl — atomic
// token là UUID unique per acquire → tránh foreign release
func (l *RedisDistributedLock) Acquire(ctx, key, ttl) (func(), error) {
    token := uuid.NewGoogleUUID()
    ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
    // ...
    unlock := func() {
        // Compare-and-Delete via Lua — chỉ delete nếu vẫn là owner
        script := redis.NewScript(`
            if redis.call("GET", KEYS[1]) == ARGV[1] then
                return redis.call("DEL", KEYS[1])
            else
                return 0
            end
        `)
        script.Run(ctx, l.client, []string{key}, token)
    }
    return unlock, nil
}
```

**Lock scope:**
- `lock:credit:user:{userID}` — dùng cho TopUp và ReserveCredits (tránh race condition trên balance)
- `lock:credit:reservation:{uuid}` — dùng cho Commit và Rollback (tránh double-commit)

### 7.2 Redis Idempotency Store

**`infras/idem/redis_idempotency_store.go`**
```go
// Prefix: "idem:credit:"
// Serialize response sang JSON để cache
func (s *RedisIdempotencyStore) Get(ctx, key) (any, error)
func (s *RedisIdempotencyStore) Set(ctx, key, value, ttl) error
```

**TTL theo operation:**
- TopUp: 24h
- ReserveCredits: 30 phút (bằng reservation TTL)
- CommitReservation: 24h

### 7.3 Kafka Event Publisher

**`infras/messaging/kafka_event_publisher.go`**
```go
const topicCredit = "credit.events"

func (p *kafkaEventPublisher) Publish(ctx, event any) error {
    data, _ := json.Marshal(event)
    return p.producer.SendMessage(ctx, topicCredit, "", 0, nil, kafka.Event{
        Meta: kafka.Meta{ServiceName: "credit", Timestamp: time.Now().UnixMilli()},
        Data: json.RawMessage(data),
    })
}
```

### 7.4 Saga Consumers

**`infras/messaging/saga/stock_unlocked_consumer.go`** (Credit Service side)
```go
// Consume topic: stock.unlocked
func (c *StockUnlockedConsumer) Handle(ctx, key, raw []byte) error {
    // parse payload
    _, err = c.commitSvc.Execute(ctx, &CommitReservationReq{
        ReservationID:  payload.ReservationID,
        IdempotencyKey: payload.IdempotencyKey,
    })
    if err != nil {
        // Commit fail → publish CreditCommitFailed → Personal compensate
        c.publisher.Publish(ctx, CreditCommitFailed{
            ReservationID:  payload.ReservationID,
            UnlockRecordID: payload.UnlockRecordID,
            Reason:         err.Error(),
        })
        // Rollback reservation → giải phóng reserved credit
        c.rollbackSvc.Execute(ctx, &RollbackReservationReq{...})
        return nil // không retry — đã compensate rồi
    }
    return nil
}
```

**`infras/messaging/saga/credit_commit_failed_consumer.go`** (Personal Service side)
```go
// Consume topic: credit.commit.failed
func (c *CreditCommitFailedConsumer) Handle(ctx, key, raw []byte) error {
    // parse payload
    if err := c.unlockRepo.DeleteByID(ctx, payload.UnlockRecordID); err != nil {
        return err // Kafka retry — at-least-once delivery
    }
    // Publish CreditRolledBack cho audit
    c.publisher.Publish(ctx, CreditRolledBack{...})
    return nil
}

// Interface Personal Service phải implement
type UnlockRecordCompensator interface {
    DeleteByID(ctx context.Context, unlockRecordID string) error
}
```

---

## 8. Handler Layer

**`handler/grpc/credit_grpc_handler.go`**
```go
type CreditGrpcHandler struct {
    creditv1.UnimplementedCreditServiceServer
    topUpSvc      service.TopUpSvc
    reserveSvc    service.ReserveCreditsSvc
    commitSvc     service.CommitReservationSvc
    rollbackSvc   service.RollbackReservationSvc
    getBalanceSvc service.GetCreditBalanceSvc
}

// Mỗi method: parse proto request → gọi service.Execute → map response sang proto
func (h *CreditGrpcHandler) TopUp(ctx, req)              → TopUpResponse
func (h *CreditGrpcHandler) ReserveCredits(ctx, req)     → ReserveCreditsResponse
func (h *CreditGrpcHandler) CommitReservation(ctx, req)  → CommitReservationResponse
func (h *CreditGrpcHandler) RollbackReservation(ctx, req)→ RollbackReservationResponse
func (h *CreditGrpcHandler) GetCreditBalance(ctx, req)   → GetCreditBalanceResponse
```

**`handler/grpc/grpc_helper.go`**
```go
// Error mapping: AppError.Type → gRPC status code
// ErrorTypeNotFound    → codes.NotFound
// ErrorTypeBusiness    → codes.InvalidArgument
// ErrorTypeUnauthorized→ codes.Unauthenticated
// default              → codes.Internal

// WalletType mapping: domain enum → proto enum
func toProtoWalletType(wt enum.WalletType) creditv1.WalletType
```

---

## 9. DI / Wire Providers

**`di/providers.go`**
```go
// Repos
func NewCreditWalletRepo(db *sqlx.DB) repo.ICreditWalletRepo
func NewCreditTransactionRepo(db *sqlx.DB) repo.ICreditTransactionRepo
func NewCreditReservationRepo(db *sqlx.DB) repo.ICreditReservationRepo

// Infrastructure
func NewDistributedLock(client *redis.Client) service.DistributedLock
func NewIdempotencyStore(client *redis.Client) service.IdempotencyStore
func NewKafkaEventPublisher(producer *kafka.Producer) messaging.IEventPublisher

// Services
func NewTopUpSvc(walletRepo, txRepo, txManager, lock, idemStore, publisher) service.TopUpSvc
func NewReserveCreditsSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, idemStore, publisher) service.ReserveCreditsSvc
func NewCommitReservationSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, idemStore, publisher) service.CommitReservationSvc
func NewRollbackReservationSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, publisher) service.RollbackReservationSvc
func NewGetCreditBalanceSvc(walletRepo) service.GetCreditBalanceSvc
```

---

## 10. Flow: Unlock Stock — 2-Phase Sync

**Tất cả request trong flow này đều là SYNC (blocking).**

```
User          Personal Service           Credit Service (Core)
 │                  │                           │
 │── unlock req ───►│                           │
 │                  │                           │
 │                  │──[SYNC] ReserveCredits ──►│
 │                  │    (idempotency_key,       │ 1. Check Redis cache (fast-path)
 │                  │     amount, stock_symbol)  │ 2. Check DB (FindByIdempotencyKey)
 │                  │                           │ 3. Acquire lock: lock:credit:user:{id}
 │                  │                           │ 4. BEGIN TX
 │                  │                           │    Load wallets (expire-first sort)
 │                  │                           │    Greedy SoftReserve per wallet
 │                  │                           │    Insert reservation + items + txns
 │                  │                           │ 5. COMMIT TX
 │                  │                           │ 6. Cache → Redis
 │                  │                           │ 7. Publish CreditReserved → Kafka [ASYNC]
 │                  │◄── {reservation_id,        │
 │                  │     deductions} ───────────│
 │                  │                           │
 │                  │ [insert unlock record DB]  │
 │                  │                           │
 │                  │──[SYNC] CommitReservation ►│
 │                  │    (reservation_id,        │ 1. Check Redis cache (idempotency)
 │                  │     idempotency_key)        │ 2. FindByUuid → verify PENDING + not expired
 │                  │                           │ 3. Acquire lock: lock:credit:reservation:{uuid}
 │                  │                           │ 4. BEGIN TX
 │                  │                           │    Per item: CommitReserved → balance -= amount
 │                  │                           │    Insert CreditTransactions (COMMIT)
 │                  │                           │    Update reservation → COMMITTED
 │                  │                           │ 5. COMMIT TX
 │                  │                           │ 6. Cache → Redis
 │                  │                           │ 7. Publish CreditCommitted → Kafka [ASYNC]
 │                  │◄── {success, total_deducted}│
 │◄── 200 OK ───────│                           │
 │                  │                           │
 │   [on any failure]                           │
 │                  │──[SYNC] RollbackReservation►│
 │                  │    (reservation_id, reason) │ 1. FindByUuid
 │                  │                           │ 2. Idempotent: ROLLED_BACK → return OK
 │                  │                           │ 3. Acquire lock per reservation
 │                  │                           │ 4. BEGIN TX
 │                  │                           │    Per item: ReleaseReserved (balance intact!)
 │                  │                           │    Insert CreditTransactions (ROLLBACK)
 │                  │                           │    Update reservation → ROLLED_BACK
 │                  │                           │ 5. COMMIT TX
 │                  │                           │ 6. Publish CreditRolledBack → Kafka [ASYNC]
 │                  │◄── {success} ─────────────│
 │◄── 4xx error ────│                           │
```

---

## 11. Flow: Unlock Stock — Kafka Saga

**Chỉ `ReserveCredits` là SYNC. Commit chuyển sang ASYNC qua Kafka.**

```
User        Personal Service              Kafka                 Credit Service
 │               │                         │                        │
 │── unlock ────►│                         │                        │
 │               │──[SYNC] ReserveCredits ────────────────────────►│
 │               │◄── {reservation_id} ───────────────────────────│
 │               │                         │                        │
 │               │ [insert unlock record DB]│                        │
 │               │                         │                        │
 │               │──publish ──────────────►│ topic: stock.unlocked  │
 │               │  StockUnlocked           │                        │
 │◄── 200 OK ────│  {reservation_id,        │──consume─────────────►│
 │               │   unlock_record_id,      │              CommitReservation()
 │               │   idempotency_key}       │                        │
 │               │                         │         [happy path: done]
 │               │                         │                        │
 │               │         [commit fail path]                       │
 │               │                         │◄── CreditCommitFailed──│
 │               │                         │    topic: credit.commit.failed
 │               │◄──consume───────────────│    {reservation_id,    │
 │               │                         │     unlock_record_id,  │
 │               │ [delete unlock record]   │     reason}            │
 │               │ [publish CreditRolledBack]│                       │
```

**Kafka topics:**

| Topic | Publisher | Consumer | Mục đích |
|---|---|---|---|
| `stock.unlocked` | Personal Service | Credit Service | Trigger auto-commit |
| `credit.commit.failed` | Credit Service | Personal Service | Trigger compensation |
| `credit.events` | Credit Service | Analytics, Notification | Audit trail |

---

## 12. Sync vs Async — phân tích từng request

### SYNC (blocking — caller đợi response)

| Request | Lý do Sync |
|---|---|
| `ReserveCredits` | Personal cần `reservation_id` để tiếp tục. Không có reservation_id = không làm gì được |
| `CommitReservation` (2-Phase) | Personal cần biết commit thành công trước khi trả 200 cho user |
| `RollbackReservation` (2-Phase) | Personal cần confirm rollback để biết có retry được không |
| `GetCreditBalance` | Read query — user đang đợi hiển thị |
| `TopUp` | Admin/system cần confirm credit đã nạp thành công |

### ASYNC (fire-and-forget qua Kafka)

| Event | Publisher | Consumer | Lý do Async |
|---|---|---|---|
| `CreditToppedUp` | Credit Svc | Notification, Analytics | User không cần đợi email |
| `CreditReserved` | Credit Svc | Audit log | Chỉ để record |
| `CreditCommitted` | Credit Svc | Notification | User đã có response rồi |
| `CreditRolledBack` | Credit Svc | Audit, Alert | Xảy ra sau khi đã trả lỗi |
| `StockUnlocked` | Personal Svc | Credit Svc (Saga) | Trigger commit sau khi Personal done |
| `CreditCommitFailed` | Credit Svc | Personal Svc (Saga) | Trigger compensating transaction |

---

## 13. Các pattern đã áp dụng

### Redis Distributed Lock

```
Acquire: SET key token NX PX ttl   (atomic — không cần Lua)
Release: Lua compare-and-delete    (chỉ xoá nếu vẫn là owner)

Lock scope:
  lock:credit:user:{userID}          → TopUp, ReserveCredits
  lock:credit:reservation:{uuid}     → CommitReservation, RollbackReservation
```

**Tại sao cần lock per user ở Reserve?**
Nếu 2 request đồng thời unlock cùng user, cả 2 đều thấy `available = 100` và cùng reserve 80 → tổng reserve 160 > 100 → balance âm. Lock ngăn điều này.

### Idempotency (2 tầng)

```
Tầng 1 — Redis cache:
  Key: "idem:credit:{idempotency_key}"
  Value: JSON-serialized response
  TTL: 24h (TopUp, Commit), 30min (Reserve)
  → Fast-path: hit cache → trả ngay, không hit DB

Tầng 2 — DB unique constraint:
  credit_reservations.idempotency_key UNIQUE
  → Nếu Redis miss (restart/eviction), DB chặn double-insert
  → Consumer retry: idempotency_key đã tồn tại → trả existing record
```

### Soft Reservation

```
credit_wallets (per wallet):
  balance  = tổng đã nạp vào (chỉ tăng khi TopUp)
  reserved = tổng đang bị giữ (soft lock)
  available = balance - reserved  ← dùng để check đủ không

Khi Reserve:  reserved += amount           (balance KHÔNG đổi)
Khi Commit:   balance  -= amount           (trừ thật)
              reserved -= amount           (xoá lock)
Khi Rollback: reserved -= amount           (xoá lock, balance KHÔNG đổi)
```

### Wallet Deduction Strategy (Expire-First)

```sql
SELECT * FROM credit_wallets
WHERE user_id = ? AND (expire_at IS NULL OR expire_at > NOW())
ORDER BY
    CASE WHEN expire_at IS NULL THEN 1 ELSE 0 END ASC,  -- PURCHASED xuống cuối
    expire_at ASC                                         -- expire sớm dùng trước
```

Greedy allocation: duyệt từng wallet theo thứ tự trên, lấy tối đa `min(available, remaining)` từ mỗi wallet cho đến khi đủ `amount`.

### Reservation TTL + Sweep Job

```
ExpiresAt = now + 10 phút
Background job: FindExpiredPending(limit=100) → tự động Rollback
→ Giải phóng reserved credit cho các reservation không bao giờ được commit/rollback
```

### Immutable Ledger

`credit_transactions` không bao giờ UPDATE hay DELETE. Mỗi operation tạo một row mới:
- TopUp → 1 row (TOP_UP)
- Reserve 50 credit từ 2 wallets → 2 rows (RESERVE), linked bởi `reservation_id`
- Commit → 2 rows (COMMIT), cùng `reservation_id`
- Rollback → 2 rows (ROLLBACK), cùng `reservation_id`

---

## 14. So sánh 2-Phase Sync vs Kafka Saga

| | **2-Phase Sync** | **Kafka Saga** |
|---|---|---|
| **User latency** | Reserve + Insert + Commit (3 hops gRPC) | Reserve + Insert (2 hops) — Commit async |
| **Consistency** | **Strong** — user biết ngay commit thành công | **Eventual** — user thấy 200 OK, commit xảy ra async |
| **Failure handling** | Rollback ngay trong cùng request | Compensating transaction qua Kafka consumer |
| **Debug** | Dễ — 1 request trace end-to-end | Khó hơn — phải trace qua nhiều Kafka topics |
| **Service coupling** | Personal phụ thuộc Credit gRPC cho Commit | Personal chỉ publish event, không biết Credit |
| **Retry logic** | Caller tự retry toàn bộ flow | Kafka at-least-once, consumer tự retry |
| **Idempotency yêu cầu** | Trên CommitReservation | Trên cả consumer lẫn compensator |
| **Khi nào dùng** | Team nhỏ, latency chấp nhận, ưu tiên simplicity | Credit Service chậm, muốn tách coupling hoàn toàn |

**Lưu ý quan trọng về Saga:**
- `ReserveCredits` **vẫn là SYNC** trong cả 2 pattern — Personal không thể continue mà không có `reservation_id`
- `UnlockRecordID` phải được Personal truyền vào `StockUnlocked` event — Credit giữ nguyên và truyền lại trong `CreditCommitFailed` để Personal biết chính xác record nào cần xoá
- Consumer trả `nil` với parse errors (không retry message corrupt), trả `error` với DB/network errors (Kafka retry)
- `CommitReservation` dùng `idempotency_key` từ `StockUnlocked` event → nếu Kafka deliver duplicate message, commit chỉ chạy 1 lần

---

*File này được generate tự động từ source code tại `internal/credit/` — cập nhật lại nếu có thay đổi lớn về architecture.*
