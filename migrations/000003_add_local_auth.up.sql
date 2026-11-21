ALTER TABLE users
  ALTER COLUMN github_id DROP NOT NULL,
  ADD COLUMN email VARCHAR(255),
  ADD COLUMN password_hash VARCHAR(255);

CREATE UNIQUE INDEX idx_users_email
  ON users(email) WHERE email IS NOT NULL AND github_id IS NULL;

CREATE INDEX idx_users_email_lookup
  ON users(email) WHERE email IS NOT NULL;
