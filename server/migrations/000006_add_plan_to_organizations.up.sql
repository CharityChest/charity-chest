ALTER TABLE organizations
    ADD COLUMN plan                   VARCHAR(50)  NOT NULL DEFAULT 'free',
    ADD COLUMN stripe_customer_id     VARCHAR(255),
    ADD COLUMN stripe_subscription_id VARCHAR(255);
