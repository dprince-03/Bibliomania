CREATE TABLE IF NOT EXISTS reading_sessions (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED NOT NULL,
    book_id         BIGINT UNSIGNED NOT NULL,

    -- Progress tracking
    current_page    INT UNSIGNED    NOT NULL DEFAULT 0,
    total_pages     INT UNSIGNED    NOT NULL DEFAULT 0,
    progress_pct    DECIMAL(5,2)    NOT NULL DEFAULT 0.00,  -- e.g. 73.45%
    current_chapter VARCHAR(255),

    -- Session metadata
    started_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_read_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at    DATETIME,                               -- NULL until finished
    is_completed    BOOLEAN         NOT NULL DEFAULT FALSE,

    -- Offline sync
    -- client sends this; server uses it to resolve conflicts (last write wins)
    client_updated_at DATETIME,

    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT fk_rs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_rs_book FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
    UNIQUE KEY uq_user_book_session (user_id, book_id),    -- one active session per user per book
    INDEX idx_rs_user_id (user_id),
    INDEX idx_rs_last_read (last_read_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;