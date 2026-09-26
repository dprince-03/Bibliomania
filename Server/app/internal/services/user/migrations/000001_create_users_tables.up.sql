CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- user-service's copy of each account. The ID is auth-service's (no
-- sequence here): rows arrive via auth.user_registered. is_active is the
-- one identity column written here (admin action) and replicated back to
-- auth via user.status_changed.
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT       PRIMARY KEY,
    first_name VARCHAR(225) NOT NULL,
    last_name  VARCHAR(225) NOT NULL,
    email      VARCHAR(225) NOT NULL UNIQUE,
    role       TEXT         NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'librarian', 'member')),
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS users_profile (
    id                BIGSERIAL    PRIMARY KEY,
    user_id           BIGINT       NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    phone_number      VARCHAR(20),
    bio               TEXT,
    profile_picture   VARCHAR(255),

    -- Activity tracking. last_read_book_id has no FK (the book lives in
    -- catalog-service); it and last_online_at are updated from
    -- reading.progress_updated.
    last_online_at    TIMESTAMPTZ,
    last_read_book_id BIGINT,
    -- Eventually-consistent cached copies of reading-service's data,
    -- bumped from reading.book_completed.
    total_books_read  INT          NOT NULL DEFAULT 0,
    total_pages_read  BIGINT       NOT NULL DEFAULT 0,

    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_profile_updated_at BEFORE UPDATE ON users_profile
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS user_library (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Validated against catalog-service when added; no FK across services.
    book_id    BIGINT      NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'to_read'
               CHECK (status IN ('wishlist', 'to_read', 'reading', 'completed', 'dropped')),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, book_id)
);

CREATE INDEX IF NOT EXISTS idx_ul_user_status ON user_library (user_id, status);

CREATE TRIGGER trg_user_library_updated_at BEFORE UPDATE ON user_library
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
