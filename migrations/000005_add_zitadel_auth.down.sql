DROP INDEX IF EXISTS idx_users_email;

CREATE UNIQUE INDEX idx_users_email
  ON users(email) WHERE email IS NOT NULL AND github_id IS NULL;

DROP INDEX IF EXISTS idx_users_zitadel_sub;

ALTER TABLE users
  DROP COLUMN auth_provider,
  DROP COLUMN zitadel_sub;
