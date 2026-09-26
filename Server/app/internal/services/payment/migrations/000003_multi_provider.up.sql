-- Paystack alongside Stripe: purchases record which provider took them,
-- and the Stripe-specific columns become provider-neutral. A new
-- migration rather than an edit to 000001, so databases that already ran
-- 000001 move forward cleanly.
ALTER TABLE book_purchases
    ADD COLUMN provider TEXT NOT NULL DEFAULT 'stripe' CHECK (provider IN ('stripe', 'paystack'));
ALTER TABLE book_purchases ALTER COLUMN provider DROP DEFAULT;

-- Stripe: the Checkout Session ID. Paystack: our transaction reference.
ALTER TABLE book_purchases RENAME COLUMN stripe_session_id TO provider_reference;
-- Stripe: the PaymentIntent ID. Paystack: the transaction ID.
ALTER TABLE book_purchases RENAME COLUMN stripe_payment_intent TO provider_payment_id;

ALTER TABLE book_purchases DROP CONSTRAINT book_purchases_stripe_session_id_key;
ALTER TABLE book_purchases
    ADD CONSTRAINT uq_purchases_provider_reference UNIQUE (provider, provider_reference);

-- Webhook dedupe for every provider (Paystack events carry no event ID of
-- their own — see paystack.go for the key used).
ALTER TABLE stripe_events RENAME TO webhook_events;
ALTER TABLE webhook_events ADD COLUMN provider TEXT NOT NULL DEFAULT 'stripe';
ALTER TABLE webhook_events ALTER COLUMN provider DROP DEFAULT;
ALTER TABLE webhook_events DROP CONSTRAINT stripe_events_pkey;
ALTER TABLE webhook_events ADD PRIMARY KEY (provider, id);
