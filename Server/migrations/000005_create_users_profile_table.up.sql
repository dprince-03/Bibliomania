CREATE TABLE IF NOT EXISTS users_profile (
    id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id          BIGINT UNSIGNED NOT NULL UNIQUE,
    phone_number     VARCHAR(20),
    bio              TEXT,
    profile_picture  VARCHAR(255),

    -- Activity tracking
    last_online_at   DATETIME,
    last_read_book_id BIGINT UNSIGNED DEFAULT NULL,
    total_books_read  INT UNSIGNED NOT NULL DEFAULT 0,
    total_pages_read  INT UNSIGNED NOT NULL DEFAULT 0,

    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (last_read_book_id) REFERENCES books(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;