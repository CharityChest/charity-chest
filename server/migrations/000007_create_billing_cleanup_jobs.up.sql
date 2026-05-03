CREATE TABLE IF NOT EXISTS billing_cleanup_jobs (
    id                        SERIAL PRIMARY KEY,
    org_id                    INTEGER      NOT NULL,
    reason                    VARCHAR(64)  NOT NULL,
    stripe_subscription_id    VARCHAR(255),
    stripe_payment_intent_id  VARCHAR(255),
    subscription_cancelled_at TIMESTAMPTZ,
    payment_refunded_at       TIMESTAMPTZ,
    last_error                TEXT,
    attempt_count             INTEGER      NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_cleanup_jobs_org_id ON billing_cleanup_jobs (org_id);
