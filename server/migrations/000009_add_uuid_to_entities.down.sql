ALTER TABLE password_reset_tokens DROP COLUMN IF EXISTS uuid;
ALTER TABLE billing_cleanup_jobs  DROP COLUMN IF EXISTS uuid;
ALTER TABLE org_members           DROP COLUMN IF EXISTS uuid;
ALTER TABLE organizations         DROP COLUMN IF EXISTS uuid;
ALTER TABLE users                 DROP COLUMN IF EXISTS uuid;
