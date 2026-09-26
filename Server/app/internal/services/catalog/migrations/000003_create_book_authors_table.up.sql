-- Junction table: one book can have many authors, one author can have many books
CREATE TABLE IF NOT EXISTS book_authors (
    book_id   BIGINT NOT NULL REFERENCES books(id)   ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    role      TEXT   NOT NULL DEFAULT 'primary'
              CHECK (role IN ('primary', 'co-author', 'editor', 'illustrator')),
    PRIMARY KEY (book_id, author_id)
);

CREATE INDEX IF NOT EXISTS idx_ba_author_id ON book_authors (author_id);
