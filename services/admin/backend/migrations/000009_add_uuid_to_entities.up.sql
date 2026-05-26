-- Adds a public, non-enumerable UUID v4 identifier to every domain entity.
-- The integer primary keys remain the internal/foreign-key columns; the API
-- surface (URL params, JSON bodies) switches to UUID.
-- pgcrypto provides gen_random_uuid() on PostgreSQL < 13; on PG 13+ it is a
-- core built-in but creating the extension is a harmless no-op.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE users
    ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();

ALTER TABLE organizations
    ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();

ALTER TABLE org_members
    ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();

ALTER TABLE billing_cleanup_jobs
    ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();

ALTER TABLE password_reset_tokens
    ADD COLUMN uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();
