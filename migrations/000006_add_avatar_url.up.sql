-- ZITADEL profile pictures and GitHub avatars are the same user-facing
-- concept: one avatar column serves both. github_avatar_url is intentionally
-- kept for now (dropping legacy columns is a separate, irreversible change),
-- so existing values are copied over once.
ALTER TABLE users ADD COLUMN avatar_url TEXT;

UPDATE users
SET avatar_url = github_avatar_url
WHERE avatar_url IS NULL
  AND github_avatar_url IS NOT NULL
  AND github_avatar_url <> '';
