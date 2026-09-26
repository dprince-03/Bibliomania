-- Public, non-guessable identifier for purchases (see borrow's 000004).
ALTER TABLE book_purchases ADD COLUMN public_id UUID;
UPDATE book_purchases SET public_id = gen_random_uuid() WHERE public_id IS NULL;
ALTER TABLE book_purchases ALTER COLUMN public_id SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_book_purchases_public_id ON book_purchases (public_id);
