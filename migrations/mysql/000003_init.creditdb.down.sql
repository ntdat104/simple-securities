START TRANSACTION;

DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS credit_reservation_items;
DROP TABLE IF EXISTS credit_reservations;
DROP TABLE IF EXISTS credit_transactions;
DROP TABLE IF EXISTS credit_wallets;

COMMIT;
