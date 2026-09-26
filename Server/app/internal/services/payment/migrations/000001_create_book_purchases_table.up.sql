CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- One-time reader→author purchases ("buy once, read for life"). A `paid`
-- row is a permanent entitlement, independent of any subscription — see
-- Server/app/docs/plan.md, Step 27. Library licences, reader
-- subscriptions and Stripe Connect payout splits are Step 27's remaining
-- scope, not built here.
CREATE TABLE IF NOT EXISTS book_purchases (
    id                    BIGSERIAL    PRIMARY KEY,
    user_id               BIGINT       NOT NULL,
    user_email            VARCHAR(255) NOT NULL,
    book_id               BIGINT       NOT NULL,
    -- Snapshots at purchase time: what was bought, for how much.
    book_title            VARCHAR(255) NOT NULL,
    amount_cents          BIGINT       NOT NULL CHECK (amount_cents >= 0),
    currency              CHAR(3)      NOT NULL,
    status                TEXT         NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'paid', 'failed', 'refunded')),
    stripe_session_id     VARCHAR(255) UNIQUE,
    stripe_payment_intent VARCHAR(255),
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_purchases_user ON book_purchases (user_id, created_at DESC);

-- A book can only be owned once per user.
CREATE UNIQUE INDEX IF NOT EXISTS uq_purchases_paid_per_user_book
    ON book_purchases (user_id, book_id) WHERE status = 'paid';

CREATE TRIGGER trg_book_purchases_updated_at BEFORE UPDATE ON book_purchases
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Stripe webhook dedupe: Stripe retries deliveries, and its event IDs
-- ("evt_...") aren't UUIDs, so they get their own table rather than
-- processed_events.
CREATE TABLE IF NOT EXISTS stripe_events (
    id          VARCHAR(255) PRIMARY KEY,
    type        VARCHAR(100) NOT NULL,
    received_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
