DROP INDEX IF EXISTS idx_users_email_lookup;
DROP INDEX IF EXISTS idx_users_email;

ALTER TABLE users
  DROP COLUMN password_hash,
  DROP COLUMN email,
  ALTER COLUMN github_id SET NOT NULL;
