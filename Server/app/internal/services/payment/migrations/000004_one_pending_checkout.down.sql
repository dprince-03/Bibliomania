DROP INDEX IF EXISTS uq_purchases_pending_per_user_book;
ALTER TABLE book_purchases DROP COLUMN IF EXISTS checkout_url;
