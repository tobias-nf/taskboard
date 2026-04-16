package models

import "time"

type Agent struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	APIKeyHash    string     `json:"-"`
	APIKeyPrefix  string     `json:"-"`
	GoogleSub     *string    `json:"-"`
	Email         *string    `json:"email,omitempty"`
	SlackID       *string    `json:"slack_id,omitempty"`
	PreferredTool *string    `json:"preferred_tool,omitempty"`
	Active        bool       `json:"active"`
	ApprovedBy    *string    `json:"approved_by,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Task struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Description   *string    `json:"description,omitempty"`
	CreatedBy     string     `json:"created_by"`
	AssignedTo    *string    `json:"assigned_to,omitempty"`
	Visibility    string     `json:"visibility"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	Deadline      *time.Time `json:"deadline,omitempty"`
	ParentID      *string    `json:"parent_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     *string   `json:"color,omitempty"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskOwedTo struct {
	TaskID    string    `json:"task_id"`
	AgentID   string    `json:"agent_id"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskMention struct {
	TaskID    string    `json:"task_id"`
	AgentID   string    `json:"agent_id"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskActivity struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	Type      string    `json:"type"`
	Actor     string    `json:"actor"`
	ActorType string    `json:"actor_type"`
	Summary   *string   `json:"summary,omitempty"`
	Data      any       `json:"data,omitempty"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskReference struct {
	ID         int64     `json:"id"`
	TaskID     string    `json:"task_id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	ExternalID *string   `json:"external_id,omitempty"`
	URL        *string   `json:"url,omitempty"`
	Title      string    `json:"title"`
	Metadata   any       `json:"metadata,omitempty"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

type TaskAttachment struct {
	ID         int64     `json:"id"`
	TaskID     string    `json:"task_id"`
	Filename   string    `json:"filename"`
	MimeType   *string   `json:"mime_type,omitempty"`
	SizeBytes  *int64    `json:"size_bytes,omitempty"`
	SHA256     *string   `json:"sha256,omitempty"`
	StorageKey string    `json:"storage_key"`
	StorageURL *string   `json:"url,omitempty"`
	Label      *string   `json:"label,omitempty"`
	UploadedBy string    `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

type AdminAudit struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	Actor      string    `json:"actor"`
	TargetType *string   `json:"target_type,omitempty"`
	TargetID   *string   `json:"target_id,omitempty"`
	Details    any       `json:"details,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
