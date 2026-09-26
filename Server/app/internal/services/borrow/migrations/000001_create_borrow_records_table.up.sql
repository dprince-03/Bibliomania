CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- No foreign keys to users/books: those live in other services' databases.
-- user_id is trusted because it comes from a verified JWT; book_id was
-- validated (and a copy reserved) against catalog-service at write time.
CREATE TABLE IF NOT EXISTS borrow_records (
    id              BIGSERIAL    PRIMARY KEY,
    user_id         BIGINT       NOT NULL,
    -- Snapshots taken at borrow time — historical record, not live data
    -- (like an order keeping the product name it was bought under).
    user_email      VARCHAR(255) NOT NULL,
    book_id         BIGINT       NOT NULL,
    book_title      VARCHAR(255) NOT NULL,
    -- The key this borrow's copy is held under in catalog-service
    -- (ReserveCopy/ReleaseCopy).
    reservation_key VARCHAR(200) NOT NULL UNIQUE,
    -- Client-supplied Idempotency-Key (or a generated one): a retried
    -- POST /borrows with the same key returns this record instead of
    -- borrowing twice.
    idempotency_key VARCHAR(200) NOT NULL,
    borrowed_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    due_at          TIMESTAMPTZ  NOT NULL,
    returned_at     TIMESTAMPTZ,
    status          TEXT         NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'returned', 'overdue')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_borrows_user_id ON borrow_records (user_id);
CREATE INDEX IF NOT EXISTS idx_borrows_book_id ON borrow_records (book_id);
CREATE INDEX IF NOT EXISTS idx_borrows_status_due ON borrow_records (status, due_at);

-- One open (active or overdue) borrow per user per book, enforced by the
-- database — the HasActiveBorrow check is just the friendly error for the
-- common case; this closes the race between two concurrent requests.
CREATE UNIQUE INDEX IF NOT EXISTS uq_borrows_open_per_user_book
    ON borrow_records (user_id, book_id) WHERE status IN ('active', 'overdue');

CREATE TRIGGER trg_borrow_records_updated_at BEFORE UPDATE ON borrow_records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
