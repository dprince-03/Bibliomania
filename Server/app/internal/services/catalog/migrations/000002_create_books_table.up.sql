CREATE TABLE IF NOT EXISTS books (
    id               BIGSERIAL    PRIMARY KEY,
    title            VARCHAR(255) NOT NULL,
    isbn             VARCHAR(255) NOT NULL UNIQUE,
    genre            VARCHAR(255) NOT NULL,
    description      TEXT,
    cover_image      VARCHAR(500),
    published_year   INT,
    total_copies     INT          NOT NULL DEFAULT 1,
    -- The CHECK is a second line of defence behind ReserveCopy's
    -- `available_copies > 0` guard: the count can never go negative.
    available_copies INT          NOT NULL DEFAULT 1 CHECK (available_copies >= 0),

    -- E-Library fields
    file_path        VARCHAR(500),
    file_size_bytes  BIGINT,
    file_format      TEXT         CHECK (file_format IN ('pdf', 'epub')),
    is_digital       BOOLEAN      NOT NULL DEFAULT FALSE,

    -- Sale price for payment-service's one-time purchase. NULL = not for
    -- sale. Added with the microservices split so payment-service has
    -- something to charge; pricing policy itself is Step 27's to design.
    price_cents      BIGINT       CHECK (price_cents >= 0),
    currency         CHAR(3),

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),

    -- Replaces MySQL's FULLTEXT(title, description). 'simple' config (no
    -- stemming/stop words) since titles span languages. Title outweighs
    -- description in ranking.
    search_vector    tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(description, '')), 'B')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_books_genre      ON books (genre);
CREATE INDEX IF NOT EXISTS idx_books_created_at ON books (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_books_search     ON books USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS idx_books_title_trgm ON books USING GIN (title gin_trgm_ops);

CREATE TRIGGER trg_books_updated_at BEFORE UPDATE ON books
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
