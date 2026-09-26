-- Transactional outbox + consumer dedupe (internal/events/outbox.go).
CREATE TABLE IF NOT EXISTS outbox_events (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    event_id     CHAR(36)        NOT NULL UNIQUE,
    event_type   VARCHAR(100)    NOT NULL,
    payload      JSON            NOT NULL,
    created_at   DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at DATETIME(6)     NULL,
    INDEX idx_outbox_unpublished (published_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS processed_events (
    event_id     CHAR(36)    NOT NULL PRIMARY KEY,
    processed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
