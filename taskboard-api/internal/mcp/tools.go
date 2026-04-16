package mcp

func taskboardTools() []tool {
	return []tool{
		// ── Tasks ──────────────────────────────────────────────────
		{
			Name:        "task_create",
			Description: "Create a new task. Tasks are public by default. Set visibility to 'private' to restrict access. Subtasks inherit visibility from parent.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":      map[string]any{"type": "string", "description": "Task title"},
					"assign_to":  map[string]any{"type": "string", "description": "Agent ID to assign to"},
					"status":     map[string]any{"type": "string", "enum": []string{"pending", "draft"}, "default": "pending", "description": "Initial status. Use 'draft' for tasks requiring approval before distribution."},
					"priority":   map[string]any{"type": "string", "enum": []string{"low", "standard", "high", "urgent", "emergency"}, "default": "standard"},
					"description": map[string]any{"type": "string", "description": "Task description (markdown). Use headings to structure content."},
					"visibility": map[string]any{"type": "string", "enum": []string{"public", "private"}, "default": "public", "description": "Ignored for subtasks — inherited from parent"},
					"deadline":   map[string]any{"type": "string", "description": "ISO 8601 deadline"},
					"parent_id":  map[string]any{"type": "string", "description": "Parent task ID for subtasks (visibility inherited from parent)"},
					"owed_to":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Agent IDs who are stakeholders (the task result is owed to them)"},
					"mentions":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Agent IDs to mention (grants access to private tasks)"},
					"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tag names to apply (created on use)"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "task_get",
			Description: "Get full task details including body and status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID (e.g., T-2026-00042)"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_list_mine",
			Description: "List tasks assigned to this agent, optionally filtered by status or tag.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string", "description": "Filter by status (comma-separated: pending,in_progress,completed,cancelled,blocked,review,failed)"},
					"priority": map[string]any{"type": "string", "description": "Filter by priority (comma-separated)"},
					"tag":      map[string]any{"type": "string", "description": "Filter by tag name"},
					"sort":     map[string]any{"type": "string", "description": "Sort field (e.g., created_at, deadline, priority)"},
					"limit":    map[string]any{"type": "integer", "description": "Max results (default 50)", "default": 50},
					"offset":   map[string]any{"type": "integer", "description": "Offset for pagination", "default": 0},
				},
			},
		},
		{
			Name:        "task_list_created",
			Description: "List tasks created by this agent, optionally filtered by status or tag.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string", "description": "Filter by status (comma-separated)"},
					"priority": map[string]any{"type": "string", "description": "Filter by priority (comma-separated)"},
					"tag":      map[string]any{"type": "string", "description": "Filter by tag name"},
					"sort":     map[string]any{"type": "string", "description": "Sort field"},
					"limit":    map[string]any{"type": "integer", "description": "Max results (default 50)", "default": 50},
					"offset":   map[string]any{"type": "integer", "description": "Offset for pagination", "default": 0},
				},
			},
		},
		{
			Name:        "task_list_owed",
			Description: "List tasks owed to this agent (where this agent is a stakeholder).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string", "description": "Filter by status (comma-separated)"},
					"priority": map[string]any{"type": "string", "description": "Filter by priority (comma-separated)"},
					"tag":      map[string]any{"type": "string", "description": "Filter by tag name"},
					"sort":     map[string]any{"type": "string", "description": "Sort field"},
					"limit":    map[string]any{"type": "integer", "description": "Max results (default 50)", "default": 50},
					"offset":   map[string]any{"type": "integer", "description": "Offset for pagination", "default": 0},
				},
			},
		},
		{
			Name:        "task_list_visible",
			Description: "List all tasks visible to this agent. Public tasks are always visible. Private tasks are visible if you are the creator, assignee, stakeholder, or mentioned.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string", "description": "Filter by status (comma-separated)"},
					"priority": map[string]any{"type": "string", "description": "Filter by priority (comma-separated)"},
					"tag":      map[string]any{"type": "string", "description": "Filter by tag name"},
					"sort":     map[string]any{"type": "string", "description": "Sort field"},
					"limit":    map[string]any{"type": "integer", "description": "Max results (default 50)", "default": 50},
					"offset":   map[string]any{"type": "integer", "description": "Offset for pagination", "default": 0},
				},
			},
		},
		{
			Name:        "task_update",
			Description: "Update task fields such as status, priority, visibility, result, body, or reassignment. Subtask visibility cannot be changed independently.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":        map[string]any{"type": "string", "description": "Task ID"},
					"status":         map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "blocked", "cancelled"}},
					"priority":       map[string]any{"type": "string", "enum": []string{"low", "standard", "high", "urgent", "emergency"}},
					"visibility":     map[string]any{"type": "string", "enum": []string{"public", "private"}, "description": "Change task visibility (cascades to subtasks, cannot be set on subtasks)"},
					"title":          map[string]any{"type": "string", "description": "New task title"},
					"description":    map[string]any{"type": "string", "description": "Updated task description (markdown)"},
					"result":         map[string]any{"type": "object", "description": "Result JSON (for task completion)"},
					"assigned_to":    map[string]any{"type": "string", "description": "Reassign to this agent"},
					"parent_id":      map[string]any{"type": "string", "description": "Move task under a parent (set to empty string to detach)"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_cancel",
			Description: "Cancel a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID to cancel"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_comment",
			Description: "Add a comment to a task timeline. Use @agent-id to mention agents (auto-grants visibility on private tasks).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
					"content": map[string]any{"type": "string", "description": "Comment text (markdown supported)"},
				},
				"required": []string{"task_id", "content"},
			},
		},
		{
			Name:        "task_get_activity",
			Description: "Get the activity timeline for a task (comments, status changes, step completions).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
					"since":   map[string]any{"type": "string", "description": "ISO 8601 timestamp — only return activity after this time"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_add_reference",
			Description: "Add a cross-reference linking a task to an external system (email, Jira, Slack, etc.).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":     map[string]any{"type": "string", "description": "Task ID"},
					"ref_type":    map[string]any{"type": "string", "enum": []string{"origin", "related", "blocks", "depends_on", "output"}, "default": "origin"},
					"source":      map[string]any{"type": "string", "description": "Source system (e.g., gmail, jira, slack, github)"},
					"external_id": map[string]any{"type": "string", "description": "ID in the external system"},
					"title":       map[string]any{"type": "string", "description": "Human-readable label"},
					"url":         map[string]any{"type": "string", "description": "URL to the external resource"},
				},
				"required": []string{"task_id", "source", "external_id"},
			},
		},
		{
			Name:        "task_list_references",
			Description: "List all cross-references on a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_delete_reference",
			Description: "Remove a cross-reference from a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
					"ref_id":  map[string]any{"type": "integer", "description": "Reference ID to remove"},
				},
				"required": []string{"task_id", "ref_id"},
			},
		},
		{
			Name:        "task_list_attachments",
			Description: "List attachments on a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "task_delete_attachment",
			Description: "Delete an attachment by ID.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"attachment_id": map[string]any{"type": "integer", "description": "Attachment ID to delete"},
				},
				"required": []string{"attachment_id"},
			},
		},

		// ── Tags ───────────────────────────────────────────────────
		{
			Name:        "tag_list",
			Description: "List all tags in the system.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "tag_create",
			Description: "Create a new tag for organizing tasks.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string", "description": "Tag name"},
					"color": map[string]any{"type": "string", "description": "Optional color hex code"},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "task_add_tag",
			Description: "Add a tag to a task. Creates the tag if it does not exist.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
					"name":    map[string]any{"type": "string", "description": "Tag name"},
				},
				"required": []string{"task_id", "name"},
			},
		},
		{
			Name:        "task_remove_tag",
			Description: "Remove a tag from a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
					"tag_id":  map[string]any{"type": "integer", "description": "Tag ID to remove"},
				},
				"required": []string{"task_id", "tag_id"},
			},
		},
		{
			Name:        "task_get_tags",
			Description: "List tags on a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
				},
				"required": []string{"task_id"},
			},
		},

		// ── Owed-to (stakeholders) ─────────────────────────────────
		{
			Name:        "task_add_owed_to",
			Description: "Add a stakeholder to a task — the person or agent the task result is owed to. Grants visibility on private tasks.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":  map[string]any{"type": "string", "description": "Task ID"},
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to add as stakeholder"},
				},
				"required": []string{"task_id", "agent_id"},
			},
		},
		{
			Name:        "task_remove_owed_to",
			Description: "Remove a stakeholder from a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":  map[string]any{"type": "string", "description": "Task ID"},
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to remove"},
				},
				"required": []string{"task_id", "agent_id"},
			},
		},
		{
			Name:        "task_get_owed_to",
			Description: "List stakeholders of a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
				},
				"required": []string{"task_id"},
			},
		},

		// ── Mentions (access grants) ───────────────────────────────
		{
			Name:        "task_add_mention",
			Description: "Mention an agent on a task — grants them visibility on private tasks. On public tasks, useful for notification.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":  map[string]any{"type": "string", "description": "Task ID"},
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to mention"},
				},
				"required": []string{"task_id", "agent_id"},
			},
		},
		{
			Name:        "task_remove_mention",
			Description: "Remove a mention from a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":  map[string]any{"type": "string", "description": "Task ID"},
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to remove"},
				},
				"required": []string{"task_id", "agent_id"},
			},
		},
		{
			Name:        "task_get_mentions",
			Description: "List mentions on a task.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "Task ID"},
				},
				"required": []string{"task_id"},
			},
		},

		// ── Agents ─────────────────────────────────────────────────
		{
			Name:        "agent_whoami",
			Description: "Get the current agent's identity, type, and metadata.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "agent_assignable",
			Description: "List all active agents that tasks can be assigned to.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "agent_list",
			Description: "List all agents in the system. Supports search and filtering.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"search": map[string]any{"type": "string", "description": "Search by name or ID"},
					"type":   map[string]any{"type": "string", "description": "Filter by type (user, admin)"},
					"active": map[string]any{"type": "string", "description": "Filter by active status (true/false)"},
					"limit":  map[string]any{"type": "integer", "description": "Max results", "default": 50},
					"offset": map[string]any{"type": "integer", "description": "Offset for pagination", "default": 0},
				},
			},
		},
		{
			Name:        "agent_get",
			Description: "Get an agent's profile by ID.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{"type": "string", "description": "Agent ID"},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "agent_create",
			Description: "Create a new agent (admin only). Returns the generated API key.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":    map[string]any{"type": "string", "description": "Agent ID (slug)"},
					"type":  map[string]any{"type": "string", "enum": []string{"user", "admin", "service"}, "description": "Agent type"},
					"email": map[string]any{"type": "string", "description": "Email address"},
				},
				"required": []string{"id", "type"},
			},
		},
		{
			Name:        "agent_update_me",
			Description: "Update the current agent's own profile.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"email":          map[string]any{"type": "string", "description": "Email address"},
					"slack_id":       map[string]any{"type": "string", "description": "Slack user ID"},
					"preferred_tool": map[string]any{"type": "string", "description": "Preferred tool identifier"},
				},
			},
		},
		{
			Name:        "agent_update",
			Description: "Update another agent's profile (admin only).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id":       map[string]any{"type": "string", "description": "Agent ID to update"},
					"type":           map[string]any{"type": "string", "description": "Agent type"},
					"email":          map[string]any{"type": "string", "description": "Email address"},
					"slack_id":       map[string]any{"type": "string", "description": "Slack user ID"},
					"preferred_tool": map[string]any{"type": "string", "description": "Preferred tool identifier"},
					"active":         map[string]any{"type": "boolean", "description": "Active status"},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "agent_approve",
			Description: "Approve a pending agent (admin only).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to approve"},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "agent_suspend",
			Description: "Suspend an active agent (admin only).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to suspend"},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "agent_reactivate",
			Description: "Reactivate a suspended agent (admin only).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{"type": "string", "description": "Agent ID to reactivate"},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "agent_rotate_key",
			Description: "Rotate the current agent's API key. Returns the new key (the old key is immediately invalidated).",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},

		// ── Admin ──────────────────────────────────────────────────
		{
			Name:        "admin_audit",
			Description: "Get the admin audit log.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "Max entries to return", "default": 50},
				},
			},
		},
	}
}
