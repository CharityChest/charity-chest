ALTER TABLE organizations
    DROP COLUMN plan,
    DROP COLUMN stripe_customer_id,
    DROP COLUMN stripe_subscription_id;
