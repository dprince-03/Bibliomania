-- Public, non-guessable identifier for borrow records (UUIDv7: random but
-- time-ordered, so the unique index stays append-friendly). The numeric id
-- stays the internal key; API responses carry public_id and routes accept
-- either — see Server/app/docs/API.md → "IDs". Existing rows get a v4 UUID
-- (Postgres 16 has no built-in v7); new rows get v7 from the service.
ALTER TABLE borrow_records ADD COLUMN public_id UUID;
UPDATE borrow_records SET public_id = gen_random_uuid() WHERE public_id IS NULL;
ALTER TABLE borrow_records ALTER COLUMN public_id SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_borrow_records_public_id ON borrow_records (public_id);
