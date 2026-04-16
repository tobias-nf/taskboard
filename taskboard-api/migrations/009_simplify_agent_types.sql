-- Simplify agent types: service and personal → user, keep admin
-- Drop old constraint first so the UPDATE doesn't violate it
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_type_check;
UPDATE agents SET type = 'user' WHERE type IN ('service', 'personal');
ALTER TABLE agents ADD CONSTRAINT agents_type_check CHECK (type IN ('user', 'admin'));
