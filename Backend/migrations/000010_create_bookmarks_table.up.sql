CREATE TABLE IF NOT EXISTS bookmarks (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT UNSIGNED NOT NULL,
    book_id     BIGINT UNSIGNED NOT NULL,
    page        INT UNSIGNED    NOT NULL,
    note        TEXT,                           -- optional annotation
    highlight   TEXT,                           -- selected text
    color       VARCHAR(20)     DEFAULT 'yellow', -- highlight color
    created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_bm_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_bm_book FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
    INDEX idx_bm_user_book (user_id, book_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;