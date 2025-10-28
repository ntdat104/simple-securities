CREATE TABLE users
(
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid            VARCHAR(36) UNIQUE NOT NULL,
    email           VARCHAR(255) UNIQUE NOT NULL,
    hashed_password VARCHAR(255) NOT NULL,
    last_login_at   DATETIME DEFAULT NULL,
    status          VARCHAR(50) NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_by      BIGINT UNSIGNED NOT NULL,
    updated_by      BIGINT UNSIGNED NOT NULL,
    deleted_at      TIMESTAMP DEFAULT NULL
);

CREATE INDEX users_uuid_idx ON users (uuid);
CREATE INDEX users_email_idx ON users (email);
