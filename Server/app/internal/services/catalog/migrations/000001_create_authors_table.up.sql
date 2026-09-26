-- Shared trigger: keep updated_at current on every UPDATE (Postgres has no
-- ON UPDATE CURRENT_TIMESTAMP like MySQL).
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigram matching for partial author-name search (`q` on GET /search).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS authors (
    id            BIGSERIAL    PRIMARY KEY,
    first_name    VARCHAR(255) NOT NULL,
    last_name     VARCHAR(255) NOT NULL,
    middle_name   VARCHAR(255),
    image         VARCHAR(512),
    date_of_birth DATE,
    biography     TEXT,
    phone         VARCHAR(255),
    email         VARCHAR(255),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_authors_name  ON authors (first_name, last_name);
CREATE INDEX IF NOT EXISTS idx_authors_email ON authors (email);
CREATE INDEX IF NOT EXISTS idx_authors_first_name_trgm ON authors USING GIN (first_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_authors_last_name_trgm  ON authors USING GIN (last_name gin_trgm_ops);

CREATE TRIGGER trg_authors_updated_at BEFORE UPDATE ON authors
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
