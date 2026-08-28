-- Junction table: one book can have many authors, one author can have many books
CREATE TABLE IF NOT EXISTS book_authors (
    book_id   BIGINT UNSIGNED NOT NULL,
    author_id BIGINT UNSIGNED NOT NULL,
    role      ENUM('primary', 'co-author', 'editor', 'illustrator') NOT NULL DEFAULT 'primary',

    PRIMARY KEY (book_id, author_id),   -- composite PK prevents duplicates
    CONSTRAINT fk_ba_book   FOREIGN KEY (book_id)   REFERENCES books(id)   ON DELETE CASCADE,
    CONSTRAINT fk_ba_author FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE CASCADE,
    INDEX idx_ba_author_id (author_id),
    INDEX idx_ba_book_id   (book_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;