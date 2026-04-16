-- Remove name, title, and description from agents.
-- Agents are identified by ID; email provides the human-readable label.
ALTER TABLE agents DROP COLUMN IF EXISTS name;
ALTER TABLE agents DROP COLUMN IF EXISTS title;
ALTER TABLE agents DROP COLUMN IF EXISTS description;
