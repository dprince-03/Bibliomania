-- Copies the borrow Saga reserved but failed to release inline (catalog-
-- service unreachable at compensation time). A background worker retries
-- ReleaseCopy for each row until it succeeds, so a failed borrow never
-- permanently leaks a copy.
CREATE TABLE IF NOT EXISTS saga_compensations (
    reservation_key VARCHAR(200) PRIMARY KEY,
    book_id         BIGINT       NOT NULL,
    attempts        INT          NOT NULL DEFAULT 0,
    last_error      TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
