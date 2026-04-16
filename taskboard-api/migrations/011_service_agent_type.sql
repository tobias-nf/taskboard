-- Reintroduce service agent type for system integrations (Slack app, Fireflies bridge, etc.)
-- Service agents get elevated task access (read all, update all) but NOT admin privileges
-- (no agent management, no audit log, no tag deletion).
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_type_check;
ALTER TABLE agents ADD CONSTRAINT agents_type_check CHECK (type IN ('user', 'admin', 'service'));
