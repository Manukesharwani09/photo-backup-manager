DROP INDEX IF EXISTS account_one_primary_per_user;
DROP INDEX IF EXISTS account_email_global;
DROP INDEX IF EXISTS account_google_sub_global;
ALTER TABLE account DROP COLUMN IF EXISTS google_sub;
