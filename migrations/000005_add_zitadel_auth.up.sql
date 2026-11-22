-- ZITADEL is an external identity provider: the subject (sub) is the stable
-- identity, email is profile data that may also be owned by a local account.
ALTER TABLE users ADD COLUMN zitadel_sub TEXT;
ALTER TABLE users ADD COLUMN auth_provider VARCHAR(32);

CREATE UNIQUE INDEX idx_users_zitadel_sub
  ON users(zitadel_sub) WHERE zitadel_sub IS NOT NULL;

UPDATE users
SET auth_provider = CASE
  WHEN github_id IS NOT NULL THEN 'github'
  ELSE 'password'
END
WHERE auth_provider IS NULL;

-- Email uniqueness only applies between accounts that can log in with a
-- password. A ZITADEL user with the same verified email is a separate identity
-- and must not collide with the password account.
DROP INDEX IF EXISTS idx_users_email;

CREATE UNIQUE INDEX idx_users_email
  ON users(email) WHERE email IS NOT NULL AND password_hash IS NOT NULL;
