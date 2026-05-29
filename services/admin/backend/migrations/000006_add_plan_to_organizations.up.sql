ALTER TABLE organizations
    ADD COLUMN plan                   VARCHAR(50)  NOT NULL DEFAULT 'free',
    ADD COLUMN stripe_customer_id     VARCHAR(255),
    ADD COLUMN stripe_subscription_id VARCHAR(255),
    ADD CONSTRAINT chk_organizations_plan
        CHECK (plan IN ('free', 'pro', 'enterprise'));

-- One Stripe customer per organisation (NULL = no Stripe account yet).
CREATE UNIQUE INDEX uq_organizations_stripe_customer_id
    ON organizations(stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;

-- One active subscription per organisation; NULL when not on a paid plan.
CREATE UNIQUE INDEX uq_organizations_stripe_subscription_id
    ON organizations(stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;
