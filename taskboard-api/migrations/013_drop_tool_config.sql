-- Remove the unused tool_config column from agents
ALTER TABLE agents DROP COLUMN IF EXISTS tool_config;
