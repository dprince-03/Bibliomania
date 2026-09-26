-- One row per ReserveCopy idempotency key (borrow Saga). Makes ReserveCopy
-- and ReleaseCopy idempotent: a retried reserve with the same key returns
-- the original outcome instead of taking a second copy, and a release only
-- gives a copy back if this key still holds one.
CREATE TABLE IF NOT EXISTS copy_reservations (
    idempotency_key VARCHAR(200) PRIMARY KEY,
    book_id         BIGINT       NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    status          TEXT         NOT NULL CHECK (status IN ('reserved', 'released')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_copy_reservations_book ON copy_reservations (book_id, status);

CREATE TRIGGER trg_copy_reservations_updated_at BEFORE UPDATE ON copy_reservations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
