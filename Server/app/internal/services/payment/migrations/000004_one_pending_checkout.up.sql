-- One open checkout per user per book. A retried or concurrent checkout
-- for the same book reuses the pending purchase (and its checkout_url)
-- instead of opening a second payment the reader could complete twice.
ALTER TABLE book_purchases ADD COLUMN checkout_url TEXT;

-- Older duplicate pendings (possible before this migration) would block
-- the index: keep the newest, fail the rest.
UPDATE book_purchases p SET status = 'failed'
WHERE status = 'pending' AND EXISTS (
    SELECT 1 FROM book_purchases q
    WHERE q.user_id = p.user_id AND q.book_id = p.book_id
      AND q.status = 'pending' AND q.id > p.id
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchases_pending_per_user_book
    ON book_purchases (user_id, book_id) WHERE status = 'pending';
