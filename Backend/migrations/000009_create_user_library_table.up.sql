CREATE TABLE IF NOT EXISTS user_library (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT UNSIGNED NOT NULL,
    book_id    BIGINT UNSIGNED NOT NULL,
    status     ENUM(
                'wishlist',     -- wants to read someday
                'to_read',      -- actively plans to read next
                'reading',      -- currently reading
                'completed',    -- finished
                'dropped'       -- started but gave up
               ) NOT NULL DEFAULT 'to_read',
    added_at   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT fk_ul_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_ul_book FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
    UNIQUE KEY uq_user_book_library (user_id, book_id),   -- one entry per book per user
    INDEX idx_ul_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;