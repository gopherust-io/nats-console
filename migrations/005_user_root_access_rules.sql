ALTER TABLE users ADD COLUMN IF NOT EXISTS is_root BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS access_rules JSONB;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_root ON users (is_root) WHERE is_root = true;
