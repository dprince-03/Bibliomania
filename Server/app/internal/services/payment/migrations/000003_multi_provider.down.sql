DELETE FROM webhook_events WHERE provider <> 'stripe';
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_pkey;
ALTER TABLE webhook_events DROP COLUMN provider;
ALTER TABLE webhook_events ADD PRIMARY KEY (id);
ALTER TABLE webhook_events RENAME TO stripe_events;
ALTER TABLE stripe_events RENAME CONSTRAINT webhook_events_pkey TO stripe_events_pkey;

DELETE FROM book_purchases WHERE provider <> 'stripe';
ALTER TABLE book_purchases DROP CONSTRAINT uq_purchases_provider_reference;
ALTER TABLE book_purchases RENAME COLUMN provider_payment_id TO stripe_payment_intent;
ALTER TABLE book_purchases RENAME COLUMN provider_reference TO stripe_session_id;
ALTER TABLE book_purchases ADD CONSTRAINT book_purchases_stripe_session_id_key UNIQUE (stripe_session_id);
ALTER TABLE book_purchases DROP COLUMN provider;
