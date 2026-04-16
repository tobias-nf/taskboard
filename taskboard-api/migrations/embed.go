package migrations

import _ "embed"

//go:embed 001_init.sql
var schema string

//go:embed 002_seed.sql
var seed string

//go:embed 003_workspaces.sql
var workspaces string

//go:embed 004_context_cleanup.sql
var contextCleanup string

//go:embed 005_visibility_model.sql
var visibilityModel string

//go:embed 006_body_and_subtasks.sql
var bodyAndSubtasks string

//go:embed 007_draft_status.sql
var draftStatus string

//go:embed 008_google_auth.sql
var googleAuth string

//go:embed 009_simplify_agent_types.sql
var simplifyAgentTypes string

//go:embed 010_drop_domains.sql
var dropDomains string

//go:embed 011_service_agent_type.sql
var serviceAgentType string

//go:embed 012_drop_accepted_at.sql
var dropAcceptedAt string

//go:embed 013_drop_tool_config.sql
var dropToolConfig string

//go:embed 014_drop_blocked_result_failure.sql
var dropBlockedResultFailure string

//go:embed 015_drop_agent_name_title_desc.sql
var dropAgentNameTitleDesc string

// All migrations in order. Each is idempotent and safe to run on every startup.
var All = []struct {
	Name string
	SQL  string
}{
	{"001_init", schema},
	{"002_seed", seed},
	{"003_workspaces", workspaces},
	{"004_context_cleanup", contextCleanup},
	{"005_visibility_model", visibilityModel},
	{"006_body_and_subtasks", bodyAndSubtasks},
	{"007_draft_status", draftStatus},
	{"008_google_auth", googleAuth},
	{"009_simplify_agent_types", simplifyAgentTypes},
	{"010_drop_domains", dropDomains},
	{"011_service_agent_type", serviceAgentType},
	{"012_drop_accepted_at", dropAcceptedAt},
	{"013_drop_tool_config", dropToolConfig},
	{"014_drop_blocked_result_failure", dropBlockedResultFailure},
	{"015_drop_agent_name_title_desc", dropAgentNameTitleDesc},
}
