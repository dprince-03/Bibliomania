CREATE TABLE IF NOT EXISTS books (
    id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    title            VARCHAR(255)    NOT NULL,
    isbn             VARCHAR(255)    NOT NULL,
    genre            VARCHAR(255)    NOT NULL,
    description      TEXT,
    cover_image      VARCHAR(500),
    published_year   YEAR,
    total_copies     INT             NOT NULL DEFAULT 1,
    available_copies INT             NOT NULL DEFAULT 1,

    -- E-Library fields
    file_path        VARCHAR(500),
    file_size_bytes  BIGINT,
    file_format      ENUM('pdf', 'epub'),
    is_digital       BOOLEAN         NOT NULL DEFAULT FALSE,

    created_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_books_isbn (isbn),
    INDEX idx_books_genre (genre),
    FULLTEXT idx_books_search (title, description)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;