-- Google OAuth: add google_sub for identity linking, make api_key optional for OAuth-only users
ALTER TABLE agents ADD COLUMN IF NOT EXISTS google_sub TEXT UNIQUE;
CREATE INDEX IF NOT EXISTS idx_agents_google_sub ON agents(google_sub);

-- Allow NULL api_key_hash for agents that authenticate only via Google OAuth
ALTER TABLE agents ALTER COLUMN api_key_hash DROP NOT NULL;
ALTER TABLE agents ALTER COLUMN api_key_prefix DROP NOT NULL;
