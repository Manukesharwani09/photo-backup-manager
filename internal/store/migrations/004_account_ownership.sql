-- Google's stable account id ("sub" from userinfo). Emails can change; sub never does.
ALTER TABLE account ADD COLUMN IF NOT EXISTS google_sub TEXT;

-- A Google account belongs to exactly one owner (app_user), in any role: once linked as primary or
-- overflow for one owner, it can't be linked by anyone else.
CREATE UNIQUE INDEX IF NOT EXISTS account_google_sub_global ON account (google_sub) WHERE google_sub IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS account_email_global ON account (lower(email));

-- Each owner has at most one primary account (also checked in Go, enforced here).
CREATE UNIQUE INDEX IF NOT EXISTS account_one_primary_per_user ON account (user_id) WHERE role = 'primary';
