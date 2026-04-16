package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nearintents/taskboard-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	pool *pgxpool.Pool
}

const (
	ConfiguredAdminID        = "hive-admin"
	ConfiguredAdminAPIKeyEnv = "TASKBOARD_ADMIN_API_KEY"
)

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// --- API Key helpers ---

func GenerateAPIKey(agentID string) (key string, hash string, prefix string, err error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}
	return GenerateAPIKeyFromSecret(agentID, hex.EncodeToString(bytes))
}

func GenerateAPIKeyFromSecret(agentID, secret string) (key string, hash string, prefix string, err error) {
	if secret == "" {
		return "", "", "", fmt.Errorf("api key secret cannot be empty")
	}
	key = fmt.Sprintf("hive_sk_%s_%s", agentID, secret)
	prefix = fmt.Sprintf("hive_sk_%s_", agentID)

	// bcrypt has 72 byte limit — hash just the secret portion
	hashed, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", err
	}
	return key, string(hashed), prefix, nil
}

func ParseAPIKey(key string) (agentID, secret, prefix string, err error) {
	parts := strings.SplitN(key, "_", 4)
	if len(parts) < 4 || parts[0] != "hive" || parts[1] != "sk" {
		return "", "", "", fmt.Errorf("invalid key format")
	}
	agentID = parts[2]
	secret = parts[3]
	prefix = fmt.Sprintf("hive_sk_%s_", agentID)
	return agentID, secret, prefix, nil
}

func (s *Store) AuthenticateByAPIKey(ctx context.Context, key string) (*models.Agent, error) {
	agentID, secret, _, err := ParseAPIKey(key)
	if err != nil {
		return nil, err
	}

	agent, err := s.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(agent.APIKeyHash), []byte(secret)); err != nil {
		return nil, fmt.Errorf("invalid key")
	}

	return agent, nil
}

// --- Agents ---

func (s *Store) GetAgentByID(ctx context.Context, id string) (*models.Agent, error) {
	var a models.Agent
	var keyHash, keyPrefix *string
	err := s.pool.QueryRow(ctx,
		`SELECT id, type, api_key_hash, api_key_prefix, google_sub,
		        email, slack_id, preferred_tool, active, approved_by, last_seen_at,
		        created_at, updated_at
		 FROM agents WHERE id = $1`, id,
	).Scan(&a.ID, &a.Type, &keyHash, &keyPrefix, &a.GoogleSub,
		&a.Email, &a.SlackID, &a.PreferredTool, &a.Active, &a.ApprovedBy,
		&a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if keyHash != nil {
		a.APIKeyHash = *keyHash
	}
	if keyPrefix != nil {
		a.APIKeyPrefix = *keyPrefix
	}
	return &a, nil
}

// FindOrCreateAgentByGoogle looks up an agent by Google subject ID.
// If none exists, creates a new personal agent from the Google profile.
func (s *Store) FindOrCreateAgentByGoogle(ctx context.Context, googleSub, email, name string) (*models.Agent, bool, error) {
	// Try to find existing agent by google_sub
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM agents WHERE google_sub = $1`, googleSub).Scan(&id)
	if err == nil {
		agent, err := s.GetAgentByID(ctx, id)
		return agent, false, err
	}

	// Derive agent ID from email prefix (e.g. "tobias.holenstein@near.foundation" → "tobias.holenstein")
	agentID := email
	if idx := strings.Index(email, "@"); idx > 0 {
		agentID = email[:idx]
	}
	// Replace characters that aren't valid in agent IDs
	agentID = strings.ReplaceAll(agentID, "+", "-")

	// Check if an agent with this ID already exists (e.g. created via env var) — link it
	existing, err := s.GetAgentByID(ctx, agentID)
	if err == nil {
		_, err = s.pool.Exec(ctx,
			`UPDATE agents SET google_sub = $2, email = $3, updated_at = NOW() WHERE id = $1`,
			existing.ID, googleSub, email)
		if err != nil {
			return nil, false, fmt.Errorf("linking google to existing agent: %w", err)
		}
		existing.GoogleSub = &googleSub
		existing.Email = &email
		return existing, false, nil
	}

	// Create new user agent
	_, err = s.pool.Exec(ctx,
		`INSERT INTO agents (id, type, email, google_sub, active, approved_by)
		 VALUES ($1, 'user', $2, $3, true, $4)`,
		agentID, email, googleSub, ConfiguredAdminID,
	)
	if err != nil {
		return nil, false, fmt.Errorf("creating agent from google: %w", err)
	}

	agent, err := s.GetAgentByID(ctx, agentID)
	return agent, true, err
}

func (s *Store) CreateAgent(ctx context.Context, a *models.Agent, keyHash, keyPrefix string, active bool, approvedBy *string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO agents (id, type, api_key_hash, api_key_prefix,
		                     email, slack_id, preferred_tool, active, approved_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.Type, keyHash, keyPrefix,
		a.Email, a.SlackID, a.PreferredTool, active, approvedBy,
	)
	return err
}

// EnsureConfiguredAdmin syncs the fixed admin account from the configured API key.
func (s *Store) EnsureConfiguredAdmin(ctx context.Context, apiKey string) error {
	agentID, secret, _, err := ParseAPIKey(apiKey)
	if err != nil {
		return fmt.Errorf("parsing configured admin API key: %w", err)
	}
	if agentID != ConfiguredAdminID {
		return fmt.Errorf("configured admin API key must use agent ID %q", ConfiguredAdminID)
	}
	_, hash, prefix, err := GenerateAPIKeyFromSecret(agentID, secret)
	if err != nil {
		return fmt.Errorf("hashing configured admin API key: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO agents (id, type, api_key_hash, api_key_prefix, active)
		 VALUES ($1, 'admin', $2, $3, true)
		 ON CONFLICT (id) DO UPDATE SET
		     type = 'admin',
		     api_key_hash = EXCLUDED.api_key_hash,
		     api_key_prefix = EXCLUDED.api_key_prefix,
		     active = true,
		     approved_by = NULL,
		     email = NULL,
		     slack_id = NULL,
		     preferred_tool = NULL,
		     updated_at = NOW()`,
		ConfiguredAdminID, hash, prefix,
	)
	if err != nil {
		return fmt.Errorf("upserting configured admin agent: %w", err)
	}
	return nil
}

// AgentConfig describes an agent to ensure exists at startup.
type AgentConfig struct {
	APIKey string
	Type   string // "user" or "service"
	Email  string
}

// EnsureConfiguredAgent upserts an env-managed agent.
func (s *Store) EnsureConfiguredAgent(ctx context.Context, cfg AgentConfig) error {
	agentID, secret, _, err := ParseAPIKey(cfg.APIKey)
	if err != nil {
		return fmt.Errorf("parsing agent API key: %w", err)
	}
	_, hash, prefix, err := GenerateAPIKeyFromSecret(agentID, secret)
	if err != nil {
		return fmt.Errorf("hashing agent API key: %w", err)
	}

	var email *string
	if cfg.Email != "" {
		email = &cfg.Email
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO agents (id, type, api_key_hash, api_key_prefix, email, active, approved_by)
		 VALUES ($1, $2, $3, $4, $5, true, $6)
		 ON CONFLICT (id) DO UPDATE SET
		     type = EXCLUDED.type,
		     api_key_hash = EXCLUDED.api_key_hash,
		     api_key_prefix = EXCLUDED.api_key_prefix,
		     email = EXCLUDED.email,
		     active = true,
		     approved_by = EXCLUDED.approved_by,
		     updated_at = NOW()`,
		agentID, cfg.Type, hash, prefix, email, ConfiguredAdminID,
	)
	if err != nil {
		return fmt.Errorf("upserting configured agent %s: %w", agentID, err)
	}
	return nil
}

func (s *Store) UpdateLastSeen(ctx context.Context, id string) {
	s.pool.Exec(ctx, `UPDATE agents SET last_seen_at = NOW() WHERE id = $1`, id)
}

func (s *Store) ListAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, type, email, slack_id,
		        preferred_tool, active, approved_by, last_seen_at, created_at, updated_at
		 FROM agents ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Email, &a.SlackID,
			&a.PreferredTool, &a.Active, &a.ApprovedBy,
			&a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

type AgentListParams struct {
	Search string
	Type   string
	Active *bool
	Limit  int
	Offset int
}

func (s *Store) ListAgentsPaginated(ctx context.Context, params AgentListParams) ([]models.Agent, int, error) {
	limit := params.Limit
	offset := params.Offset
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := "WHERE 1=1"
	args := []any{}
	idx := 1
	if params.Type != "" {
		where += fmt.Sprintf(" AND type = $%d", idx)
		args = append(args, params.Type)
		idx++
	}
	if params.Active != nil {
		where += fmt.Sprintf(" AND active = $%d", idx)
		args = append(args, *params.Active)
		idx++
	}
	if params.Search != "" {
		pattern := "%" + strings.ToLower(params.Search) + "%"
		where += fmt.Sprintf(` AND (
			LOWER(id) LIKE $%d OR
			LOWER(COALESCE(email, '')) LIKE $%d
		)`, idx, idx)
		args = append(args, pattern)
		idx++
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agents `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx,
		`SELECT id, type, email, slack_id,
		        preferred_tool, active, approved_by, last_seen_at, created_at, updated_at
		 FROM agents `+where+` ORDER BY created_at LIMIT $`+fmt.Sprintf("%d", idx)+` OFFSET $`+fmt.Sprintf("%d", idx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Email, &a.SlackID,
			&a.PreferredTool, &a.Active, &a.ApprovedBy,
			&a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		agents = append(agents, a)
	}
	return agents, total, nil
}

func (s *Store) ApproveAgent(ctx context.Context, id, approvedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE agents SET active = true, approved_by = $2, updated_at = NOW() WHERE id = $1`,
		id, approvedBy)
	return err
}

func (s *Store) SuspendAgent(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents SET active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) RotateAPIKey(ctx context.Context, id string) (string, error) {
	key, hash, prefix, err := GenerateAPIKey(id)
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE agents SET api_key_hash = $2, api_key_prefix = $3, updated_at = NOW() WHERE id = $1`,
		id, hash, prefix)
	if err != nil {
		return "", err
	}
	return key, nil
}

// --- Tasks ---

func (s *Store) CreateTask(ctx context.Context, t *models.Task) (string, error) {
	status := t.Status
	if status == "" {
		status = "pending"
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tasks (title, description, created_by, assigned_to, visibility, status, priority, deadline, parent_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		t.Title, t.Description, t.CreatedBy, t.AssignedTo, t.Visibility, status, t.Priority, t.Deadline, t.ParentID,
	).Scan(&id)
	return id, err
}

func (s *Store) GetTask(ctx context.Context, id string) (*models.Task, error) {
	var t models.Task
	err := s.pool.QueryRow(ctx,
		`SELECT id, title, description, created_by, assigned_to, visibility, status, priority, deadline,
		        parent_id, created_at, started_at, completed_at, updated_at
		 FROM tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.CreatedBy, &t.AssignedTo, &t.Visibility, &t.Status, &t.Priority, &t.Deadline,
		&t.ParentID, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type TaskListParams struct {
	Status   []string
	Priority []string
	Tag      string
	Limit    int
	Offset   int
	Sort     string
}

// GetTasksForAgent returns tasks assigned to this agent.
// Draft tasks are excluded — they are not yet approved for the assignee.
func (s *Store) GetTasksForAgent(ctx context.Context, agentID string, params TaskListParams) ([]models.Task, int, error) {
	return s.queryTasksFilteredWhere(ctx, "WHERE assigned_to = $1 AND status != 'draft'", []any{agentID}, params)
}

func (s *Store) GetTasksCreatedBy(ctx context.Context, agentID string, params TaskListParams) ([]models.Task, int, error) {
	return s.queryTasksFilteredWhere(ctx, "WHERE created_by = $1", []any{agentID}, params)
}

// GetTasksOwedTo returns tasks where this agent is a stakeholder.
func (s *Store) GetTasksOwedTo(ctx context.Context, agentID string, params TaskListParams) ([]models.Task, int, error) {
	where := "WHERE EXISTS (SELECT 1 FROM task_owed_to WHERE task_id = tasks.id AND agent_id = $1)"
	return s.queryTasksFilteredWhere(ctx, where, []any{agentID}, params)
}

// GetVisibleTasks returns all tasks visible to the agent.
// A task is visible if:
// - it is public, OR
// - the agent has direct access (creator, assignee, owed_to, mentioned), OR
// - for subtasks: the agent has access to ANY ancestor (parent access flows down,
//   but subtasks can add additional people who cannot see the parent)
func (s *Store) GetVisibleTasks(ctx context.Context, agentID string, params TaskListParams) ([]models.Task, int, error) {
	where := `WHERE (
		visibility = 'public'
		OR created_by = $1
		OR assigned_to = $1
		OR EXISTS (SELECT 1 FROM task_owed_to WHERE task_id = tasks.id AND agent_id = $1)
		OR EXISTS (SELECT 1 FROM task_mentions WHERE task_id = tasks.id AND agent_id = $1)
		OR (parent_id IS NOT NULL AND EXISTS (
			WITH RECURSIVE ancestors AS (
				SELECT id, parent_id, visibility, created_by, assigned_to FROM tasks WHERE id = tasks.parent_id
				UNION ALL
				SELECT p.id, p.parent_id, p.visibility, p.created_by, p.assigned_to FROM tasks p JOIN ancestors a ON p.id = a.parent_id
			)
			SELECT 1 FROM ancestors WHERE (
				visibility = 'public'
				OR created_by = $1
				OR assigned_to = $1
				OR EXISTS (SELECT 1 FROM task_owed_to WHERE task_id = ancestors.id AND agent_id = $1)
				OR EXISTS (SELECT 1 FROM task_mentions WHERE task_id = ancestors.id AND agent_id = $1)
			)
		))
	)`
	args := []any{agentID}
	return s.queryTasksFilteredWhere(ctx, where, args, params)
}

// GetAllTasks returns tasks with no visibility filtering (for service/admin agents).
func (s *Store) GetAllTasks(ctx context.Context, params TaskListParams) ([]models.Task, int, error) {
	return s.queryTasksFilteredWhere(ctx, "WHERE TRUE", nil, params)
}

func (s *Store) queryTasks(ctx context.Context, where string, args ...any) ([]models.Task, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, title, description, created_by, assigned_to, visibility, status, priority, deadline,
		        parent_id, created_at, started_at, completed_at, updated_at
		 FROM tasks `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.CreatedBy, &t.AssignedTo, &t.Visibility, &t.Status, &t.Priority, &t.Deadline,
			&t.ParentID, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// Valid forward transitions.
var validTransitions = map[string][]string{
	"draft":       {"pending", "cancelled"},
	"pending":     {"in_progress", "completed", "blocked", "cancelled"},
	"in_progress": {"blocked", "review", "completed", "failed", "cancelled"},
	"blocked":     {"in_progress", "cancelled"},
	"review":      {"completed", "in_progress", "cancelled"},
}

func ValidateStatusTransition(currentStatus, newStatus string, isOwnerOrAdmin bool) error {
	if (currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled") && newStatus == "pending" {
		if !isOwnerOrAdmin {
			return fmt.Errorf("only task creator or admin can reopen a %s task", currentStatus)
		}
		return nil
	}

	allowed, ok := validTransitions[currentStatus]
	if !ok {
		return fmt.Errorf("cannot transition from terminal status %q", currentStatus)
	}
	for _, s := range allowed {
		if s == newStatus {
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", currentStatus, newStatus)
}

func (s *Store) UpdateTaskStatus(ctx context.Context, id, status string) error {
	now := time.Now()
	var extra string
	switch status {
	case "in_progress":
		extra = ", started_at = $3"
	case "completed", "failed":
		extra = ", completed_at = $3"
	case "pending":
		_, err := s.pool.Exec(ctx,
			`UPDATE tasks SET status = $2, updated_at = NOW(), started_at = NULL, completed_at = NULL WHERE id = $1`,
			id, status)
		return err
	default:
		_, err := s.pool.Exec(ctx, `UPDATE tasks SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
		return err
	}
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`UPDATE tasks SET status = $2, updated_at = NOW()%s WHERE id = $1`, extra), id, status, now)
	return err
}

// --- Visibility Check ---

type VisibilityResult struct {
	CanView    bool
	CanComment bool
}

// agentHasTaskAccess checks if an agent has direct access to a specific task
// (creator, assignee, owed_to, or mentioned). Does not check parent chain.
func (s *Store) agentHasTaskAccess(ctx context.Context, agentID string, task *models.Task) bool {
	if task.CreatedBy == agentID {
		return true
	}
	if task.AssignedTo != nil && *task.AssignedTo == agentID {
		return true
	}
	var exists bool
	s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM task_owed_to WHERE task_id = $1 AND agent_id = $2
			UNION ALL
			SELECT 1 FROM task_mentions WHERE task_id = $1 AND agent_id = $2
		)`, task.ID, agentID).Scan(&exists)
	return exists
}

func (s *Store) CanAgentSeeTask(ctx context.Context, agentID string, task *models.Task) (VisibilityResult, error) {
	// Public tasks (or subtasks of public parents) are visible to everyone
	if task.Visibility == "public" {
		return VisibilityResult{true, true}, nil
	}

	// Private task: check direct access on THIS task first
	if s.agentHasTaskAccess(ctx, agentID, task) {
		return VisibilityResult{true, true}, nil
	}

	// For subtasks: walk up the parent chain — access to ANY ancestor grants access
	if task.ParentID != nil {
		var hasAncestorAccess bool
		err := s.pool.QueryRow(ctx,
			`WITH RECURSIVE ancestors AS (
				SELECT id, parent_id, visibility, created_by, assigned_to FROM tasks WHERE id = $1
				UNION ALL
				SELECT p.id, p.parent_id, p.visibility, p.created_by, p.assigned_to FROM tasks p JOIN ancestors a ON p.id = a.parent_id
			)
			SELECT EXISTS(
				SELECT 1 FROM ancestors WHERE (
					visibility = 'public'
					OR created_by = $2
					OR assigned_to = $2
					OR EXISTS (SELECT 1 FROM task_owed_to WHERE task_id = ancestors.id AND agent_id = $2)
					OR EXISTS (SELECT 1 FROM task_mentions WHERE task_id = ancestors.id AND agent_id = $2)
				)
			)`, *task.ParentID, agentID).Scan(&hasAncestorAccess)
		if err == nil && hasAncestorAccess {
			return VisibilityResult{true, true}, nil
		}
	}

	return VisibilityResult{false, false}, nil
}

// GetRootTask follows the parent_id chain to find the root task.
func (s *Store) GetRootTask(ctx context.Context, taskID string) (*models.Task, error) {
	var t models.Task
	err := s.pool.QueryRow(ctx,
		`WITH RECURSIVE chain AS (
			SELECT id, title, description, created_by, assigned_to, visibility, status, priority, deadline,
			       parent_id, created_at, started_at, completed_at, updated_at
			FROM tasks WHERE id = $1
			UNION ALL
			SELECT p.id, p.title, p.description, p.created_by, p.assigned_to, p.visibility, p.status, p.priority, p.deadline,
			       p.parent_id, p.created_at, p.started_at, p.completed_at, p.updated_at
			FROM tasks p JOIN chain c ON p.id = c.parent_id
		)
		SELECT id, title, description, created_by, assigned_to, visibility, status, priority, deadline,
		       parent_id, created_at, started_at, completed_at, updated_at
		FROM chain WHERE parent_id IS NULL LIMIT 1`, taskID,
	).Scan(&t.ID, &t.Title, &t.Description, &t.CreatedBy, &t.AssignedTo, &t.Visibility, &t.Status, &t.Priority, &t.Deadline,
		&t.ParentID, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CascadeVisibility updates all child tasks to match the parent's visibility.
func (s *Store) CascadeVisibility(ctx context.Context, parentID, visibility string) error {
	_, err := s.pool.Exec(ctx,
		`WITH RECURSIVE children AS (
			SELECT id FROM tasks WHERE parent_id = $1
			UNION ALL
			SELECT t.id FROM tasks t JOIN children c ON t.parent_id = c.id
		)
		UPDATE tasks SET visibility = $2, updated_at = NOW() WHERE id IN (SELECT id FROM children)`,
		parentID, visibility)
	return err
}

// GetAssignableAgents returns all active agents.
func (s *Store) GetAssignableAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, type, email, slack_id,
		        preferred_tool, active, approved_by, last_seen_at, created_at, updated_at
		 FROM agents WHERE active = true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Email, &a.SlackID,
			&a.PreferredTool, &a.Active, &a.ApprovedBy,
			&a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

// queryTasksFilteredWhere runs a filtered/paginated task query with a custom WHERE clause.
func (s *Store) queryTasksFilteredWhere(ctx context.Context, where string, args []any, params TaskListParams) ([]models.Task, int, error) {
	idx := len(args) + 1

	if len(params.Status) > 0 {
		where += fmt.Sprintf(" AND status = ANY($%d)", idx)
		args = append(args, params.Status)
		idx++
	}
	if len(params.Priority) > 0 {
		where += fmt.Sprintf(" AND priority = ANY($%d)", idx)
		args = append(args, params.Priority)
		idx++
	}
	if params.Tag != "" {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM task_tags tt JOIN tags t ON t.id = tt.tag_id WHERE tt.task_id = tasks.id AND t.name = $%d)", idx)
		args = append(args, params.Tag)
		idx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tasks "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := "ORDER BY "
	switch params.Sort {
	case "created_at":
		orderBy += "created_at DESC"
	default:
		orderBy += "CASE priority WHEN 'emergency' THEN 0 WHEN 'urgent' THEN 1 WHEN 'standard' THEN 2 ELSE 3 END, deadline ASC NULLS LAST"
	}

	limit := params.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := where + " " + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	tasks, err := s.queryTasks(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// --- Activity ---

func (s *Store) AddActivity(ctx context.Context, a *models.TaskActivity) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO task_activity (task_id, type, actor, actor_type, summary, data, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.TaskID, a.Type, a.Actor, a.ActorType, a.Summary, a.Data, a.OldValue, a.NewValue)
	return err
}

func (s *Store) GetActivity(ctx context.Context, taskID string) ([]models.TaskActivity, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, task_id, type, actor, actor_type, summary, data, old_value, new_value, created_at
		 FROM task_activity WHERE task_id = $1 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acts []models.TaskActivity
	for rows.Next() {
		var a models.TaskActivity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Type, &a.Actor, &a.ActorType, &a.Summary, &a.Data, &a.OldValue, &a.NewValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	return acts, nil
}

// --- Agent Updates ---

func (s *Store) UpdateAgentProfile(ctx context.Context, id string, fields map[string]any) error {
	setClauses := []string{}
	args := []any{}
	i := 1
	allowed := map[string]bool{"email": true, "slack_id": true, "preferred_tool": true}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++
	args = append(args, id)
	query := fmt.Sprintf("UPDATE agents SET %s WHERE id = $%d", strings.Join(setClauses, ", "), i)
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

func (s *Store) UpdateAgentAdmin(ctx context.Context, id string, fields map[string]any) error {
	setClauses := []string{}
	args := []any{}
	i := 1
	allowed := map[string]bool{"email": true, "slack_id": true, "preferred_tool": true, "type": true}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++
	args = append(args, id)
	query := fmt.Sprintf("UPDATE agents SET %s WHERE id = $%d", strings.Join(setClauses, ", "), i)
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// --- Task Updates ---

func (s *Store) UpdateTaskFields(ctx context.Context, id string, fields map[string]any, expectedUpdatedAt *time.Time) error {
	setClauses := []string{}
	args := []any{}
	i := 1
	allowed := map[string]bool{"description": true, "priority": true, "deadline": true, "assigned_to": true, "title": true, "visibility": true, "parent_id": true}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++
	args = append(args, id)
	where := fmt.Sprintf("WHERE id = $%d", i)

	if expectedUpdatedAt != nil {
		i++
		args = append(args, *expectedUpdatedAt)
		where += fmt.Sprintf(" AND updated_at = $%d", i)
	}

	query := fmt.Sprintf("UPDATE tasks SET %s %s", strings.Join(setClauses, ", "), where)
	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 && expectedUpdatedAt != nil {
		return fmt.Errorf("conflict: task was modified by another request")
	}
	return err
}

// --- Tags ---

func (s *Store) CreateTag(ctx context.Context, tag *models.Tag) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO tags (name, color, created_by) VALUES ($1, $2, $3) RETURNING id, created_at`,
		tag.Name, tag.Color, tag.CreatedBy,
	).Scan(&tag.ID, &tag.CreatedAt)
}

func (s *Store) GetTagByName(ctx context.Context, name string) (*models.Tag, error) {
	var t models.Tag
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, color, created_by, created_at FROM tags WHERE name = $1`, name,
	).Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTags(ctx context.Context) ([]models.Tag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, color, created_by, created_at FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Store) DeleteTag(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	return err
}

func (s *Store) AddTaskTag(ctx context.Context, taskID string, tagID int64, addedBy string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO task_tags (task_id, tag_id, added_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		taskID, tagID, addedBy)
	return err
}

func (s *Store) RemoveTaskTag(ctx context.Context, taskID string, tagID int64) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM task_tags WHERE task_id = $1 AND tag_id = $2`, taskID, tagID)
	return err
}

func (s *Store) GetTaskTags(ctx context.Context, taskID string) ([]models.Tag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.name, t.color, t.created_by, t.created_at
		 FROM tags t JOIN task_tags tt ON t.id = tt.tag_id
		 WHERE tt.task_id = $1 ORDER BY t.name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// --- Owed-to ---

func (s *Store) AddOwedTo(ctx context.Context, taskID, agentID, addedBy string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO task_owed_to (task_id, agent_id, added_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		taskID, agentID, addedBy)
	return err
}

func (s *Store) RemoveOwedTo(ctx context.Context, taskID, agentID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM task_owed_to WHERE task_id = $1 AND agent_id = $2`, taskID, agentID)
	return err
}

func (s *Store) GetOwedTo(ctx context.Context, taskID string) ([]models.TaskOwedTo, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT task_id, agent_id, added_by, created_at FROM task_owed_to WHERE task_id = $1 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []models.TaskOwedTo
	for rows.Next() {
		var e models.TaskOwedTo
		if err := rows.Scan(&e.TaskID, &e.AgentID, &e.AddedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Mentions ---

func (s *Store) AddMention(ctx context.Context, taskID, agentID, addedBy string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO task_mentions (task_id, agent_id, added_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		taskID, agentID, addedBy)
	return err
}

func (s *Store) RemoveMention(ctx context.Context, taskID, agentID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM task_mentions WHERE task_id = $1 AND agent_id = $2`, taskID, agentID)
	return err
}

func (s *Store) GetMentions(ctx context.Context, taskID string) ([]models.TaskMention, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT task_id, agent_id, added_by, created_at FROM task_mentions WHERE task_id = $1 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []models.TaskMention
	for rows.Next() {
		var e models.TaskMention
		if err := rows.Scan(&e.TaskID, &e.AgentID, &e.AddedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- References ---

func (s *Store) AddReference(ctx context.Context, ref *models.TaskReference) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO task_references (task_id, type, source, external_id, url, title, metadata, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		ref.TaskID, ref.Type, ref.Source, ref.ExternalID, ref.URL, ref.Title, ref.Metadata, ref.CreatedBy,
	).Scan(&ref.ID)
}

func (s *Store) GetReferences(ctx context.Context, taskID string) ([]models.TaskReference, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, task_id, type, source, external_id, url, title, metadata, created_by, created_at
		 FROM task_references WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []models.TaskReference
	for rows.Next() {
		var r models.TaskReference
		if err := rows.Scan(&r.ID, &r.TaskID, &r.Type, &r.Source, &r.ExternalID, &r.URL, &r.Title, &r.Metadata, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, nil
}

func (s *Store) DeleteReference(ctx context.Context, refID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM task_references WHERE id = $1`, refID)
	return err
}

// --- Attachments ---

func (s *Store) CreateAttachment(ctx context.Context, a *models.TaskAttachment) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO task_attachments (task_id, filename, mime_type, size_bytes, sha256, storage_key, storage_url, label, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		a.TaskID, a.Filename, a.MimeType, a.SizeBytes, a.SHA256, a.StorageKey, a.StorageURL, a.Label, a.UploadedBy,
	).Scan(&a.ID)
}

func (s *Store) GetAttachment(ctx context.Context, id int64) (*models.TaskAttachment, error) {
	var a models.TaskAttachment
	err := s.pool.QueryRow(ctx,
		`SELECT id, task_id, filename, mime_type, size_bytes, sha256, storage_key, storage_url, label, uploaded_by, created_at
		 FROM task_attachments WHERE id = $1`, id,
	).Scan(&a.ID, &a.TaskID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.StorageURL, &a.Label, &a.UploadedBy, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListAttachments(ctx context.Context, taskID string) ([]models.TaskAttachment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, task_id, filename, mime_type, size_bytes, sha256, storage_key, storage_url, label, uploaded_by, created_at
		 FROM task_attachments WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attachments []models.TaskAttachment
	for rows.Next() {
		var a models.TaskAttachment
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.SHA256, &a.StorageKey, &a.StorageURL, &a.Label, &a.UploadedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, nil
}

func (s *Store) DeleteAttachment(ctx context.Context, id int64) (*models.TaskAttachment, error) {
	a, err := s.GetAttachment(ctx, id)
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM task_attachments WHERE id = $1`, id)
	return a, err
}

// --- Audit ---

func (s *Store) LogAudit(ctx context.Context, action, actor string, targetType, targetID *string, details any) {
	s.pool.Exec(ctx,
		`INSERT INTO admin_audit (action, actor, target_type, target_id, details)
		 VALUES ($1, $2, $3, $4, $5)`,
		action, actor, targetType, targetID, details)
}

func (s *Store) GetAuditLog(ctx context.Context, limit int) ([]models.AdminAudit, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, action, actor, target_type, target_id, details, created_at
		 FROM admin_audit ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []models.AdminAudit
	for rows.Next() {
		var e models.AdminAudit
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.TargetType, &e.TargetID, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
