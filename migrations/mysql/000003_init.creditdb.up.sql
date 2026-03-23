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
    INDEX idx_cw_expire (expire_at),              -- for expire sweep job
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
    reference_id    VARCHAR(255)    NULL,      -- external reference (order_id, membership_id, …)
    idempotency_key CHAR(36)        NULL,      -- caller-provided key for dedup
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
    status          VARCHAR(20)     NOT NULL DEFAULT 'PENDING', -- PENDING | COMMITTED | ROLLED_BACK
    idempotency_key CHAR(36)        NOT NULL UNIQUE,           -- idempotency at reservation level
    stock_symbol    VARCHAR(20)     NULL,
    request_id      VARCHAR(255)    NULL,
    committed_at    DATETIME        NULL,
    rolled_back_at  DATETIME        NULL,
    expires_at      DATETIME        NOT NULL,                  -- reservation TTL (e.g. 10 min)
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_cr_user_id (user_id),
    INDEX idx_cr_status (status),
    INDEX idx_cr_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- credit_reservation_items: per-wallet breakdown of a reservation.
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
-- idempotency_keys: cross-service dedup store for TopUp calls.
-- ============================================================
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    idem_key        CHAR(36)        NOT NULL UNIQUE,
    service         VARCHAR(50)     NOT NULL,  -- e.g. 'credit.top_up'
    response_status VARCHAR(20)     NOT NULL DEFAULT 'PROCESSING', -- PROCESSING | SUCCESS | FAILED
    response_body   TEXT            NULL,      -- cached JSON response
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME        NOT NULL,

    INDEX idx_ik_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

COMMIT;
