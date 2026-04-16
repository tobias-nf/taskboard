package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/events"
	"github.com/nearintents/taskboard-api/internal/handlers"
	"github.com/nearintents/taskboard-api/internal/middleware"
	"github.com/nearintents/taskboard-api/internal/models"
	"github.com/nearintents/taskboard-api/internal/storage"
)

var ErrUnknownTool = errors.New("unknown tool")

type Executor struct {
	router http.Handler
}

func NewExecutor(store *db.Store, broker *events.Broker, s3 *storage.S3Client) *Executor {
	r := chi.NewRouter()

	// Tasks
	r.Post("/tasks", handlers.CreateTask(store, broker))
	r.Get("/tasks/me", handlers.GetMyTasks(store))
	r.Get("/tasks/me/created", handlers.GetMyCreatedTasks(store))
	r.Get("/tasks/me/owed", handlers.GetTasksOwedToMe(store))
	r.Get("/tasks/visible", handlers.GetVisibleTasks(store))
	r.Get("/tasks/{id}", handlers.GetTask(store))
	r.Patch("/tasks/{id}", handlers.UpdateTask(store, broker))
	r.Delete("/tasks/{id}", handlers.CancelTask(store, broker))
	r.Post("/tasks/{id}/activity", handlers.AddActivity(store, broker))
	r.Get("/tasks/{id}/activity", handlers.GetActivity(store))
	r.Post("/tasks/{id}/references", handlers.AddReference(store))
	r.Get("/tasks/{id}/references", handlers.GetReferences(store))
	r.Delete("/tasks/{id}/references/{refId}", handlers.RemoveReference(store))
	r.Get("/tasks/{id}/attachments", handlers.ListAttachments(store))
	r.Delete("/attachments/{id}", handlers.DeleteAttachment(store, s3))

	// Tags on tasks
	r.Get("/tasks/{id}/tags", handlers.GetTaskTags(store))
	r.Post("/tasks/{id}/tags", handlers.AddTaskTag(store))
	r.Delete("/tasks/{id}/tags/{tagId}", handlers.RemoveTaskTag(store))

	// Owed-to
	r.Get("/tasks/{id}/owed-to", handlers.GetOwedTo(store))
	r.Post("/tasks/{id}/owed-to", handlers.AddOwedTo(store))
	r.Delete("/tasks/{id}/owed-to/{agentId}", handlers.RemoveOwedTo(store))

	// Mentions
	r.Get("/tasks/{id}/mentions", handlers.GetMentions(store))
	r.Post("/tasks/{id}/mentions", handlers.AddMention(store))
	r.Delete("/tasks/{id}/mentions/{agentId}", handlers.RemoveMention(store))

	// Tags (global)
	r.Get("/tags", handlers.ListTags(store))
	r.Post("/tags", handlers.CreateTag(store))
	r.Delete("/tags/{id}", handlers.DeleteTag(store))

	// Agents
	r.Get("/agents/me", handlers.GetMe(store))
	r.Patch("/agents/me", handlers.UpdateMe(store))
	r.Post("/agents/me/rotate-key", handlers.RotateKey(store))
	r.Get("/agents/me/assignable", handlers.GetAssignable(store))
	r.Get("/agents", handlers.ListAgents(store))
	r.Post("/agents", handlers.CreateAgentByAdmin(store))
	r.Get("/agents/{id}", handlers.GetAgent(store))
	r.Patch("/agents/{id}", handlers.UpdateAgent(store))
	r.Post("/agents/{id}/approve", handlers.ApproveAgent(store))
	r.Post("/agents/{id}/suspend", handlers.SuspendAgent(store))
	r.Post("/agents/{id}/reactivate", handlers.ReactivateAgent(store))

	// Admin
	r.Get("/admin/audit", handlers.GetAuditLog(store))

	return &Executor{router: r}
}

func (e *Executor) Call(ctx context.Context, agent *models.Agent, name string, arguments map[string]any) (any, error) {
	switch name {

	// ── Tasks ──────────────────────────────────────────────────────

	case "task_create":
		body := cloneMap(arguments)
		if assignTo, ok := body["assign_to"]; ok {
			body["assigned_to"] = assignTo
			delete(body, "assign_to")
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks", nil, body)

	case "task_get":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID), nil, nil)

	case "task_list_mine":
		return e.request(ctx, agent, http.MethodGet, "/tasks/me", queryArgs(arguments, "status", "priority", "tag", "sort", "limit", "offset"), nil)

	case "task_list_created":
		return e.request(ctx, agent, http.MethodGet, "/tasks/me/created", queryArgs(arguments, "status", "priority", "tag", "sort", "limit", "offset"), nil)

	case "task_list_owed":
		return e.request(ctx, agent, http.MethodGet, "/tasks/me/owed", queryArgs(arguments, "status", "priority", "tag", "sort", "limit", "offset"), nil)

	case "task_list_visible":
		return e.request(ctx, agent, http.MethodGet, "/tasks/visible", queryArgs(arguments, "status", "priority", "tag", "sort", "limit", "offset"), nil)

	case "task_update":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		body := cloneMap(arguments)
		delete(body, "task_id")
		return e.request(ctx, agent, http.MethodPatch, "/tasks/"+url.PathEscape(taskID), nil, body)

	case "task_cancel":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/tasks/"+url.PathEscape(taskID), nil, nil)

	case "task_comment":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		content, err := requireString(arguments, "content")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/activity", nil, map[string]any{
			"type":       "commented",
			"summary":    content,
			"actor_type": "agent",
		})

	case "task_get_activity":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		result, err := e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/activity", nil, nil)
		if err != nil {
			return nil, err
		}
		since, _ := stringArg(arguments, "since")
		if since == "" {
			return result, nil
		}
		return filterActivitySince(result, since)

	case "task_add_reference":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		source, err := requireString(arguments, "source")
		if err != nil {
			return nil, err
		}
		externalID, err := requireString(arguments, "external_id")
		if err != nil {
			return nil, err
		}
		refType, _ := stringArg(arguments, "ref_type")
		if refType == "" {
			refType = "origin"
		}
		title, _ := stringArg(arguments, "title")
		if title == "" {
			title = fmt.Sprintf("%s:%s", source, externalID)
		}
		body := map[string]any{
			"type":        refType,
			"source":      source,
			"external_id": externalID,
			"title":       title,
		}
		if rawURL, ok := stringArg(arguments, "url"); ok && rawURL != "" {
			body["url"] = rawURL
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/references", nil, body)

	case "task_list_references":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/references", nil, nil)

	case "task_delete_reference":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		refID, err := requireInt(arguments, "ref_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/tasks/"+url.PathEscape(taskID)+"/references/"+strconv.Itoa(refID), nil, nil)

	case "task_list_attachments":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/attachments", nil, nil)

	case "task_delete_attachment":
		attID, err := requireInt(arguments, "attachment_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/attachments/"+strconv.Itoa(attID), nil, nil)

	// ── Tags ──────────────────────────────────────────────────────

	case "tag_list":
		return e.request(ctx, agent, http.MethodGet, "/tags", nil, nil)

	case "tag_create":
		return e.request(ctx, agent, http.MethodPost, "/tags", nil, arguments)

	case "task_add_tag":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		name, err := requireString(arguments, "name")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/tags", nil, map[string]any{"name": name})

	case "task_remove_tag":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		tagID, err := requireInt(arguments, "tag_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/tasks/"+url.PathEscape(taskID)+"/tags/"+strconv.Itoa(tagID), nil, nil)

	case "task_get_tags":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/tags", nil, nil)

	// ── Owed-to ───────────────────────────────────────────────────

	case "task_add_owed_to":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/owed-to", nil, map[string]any{"agent_id": agentID})

	case "task_remove_owed_to":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/tasks/"+url.PathEscape(taskID)+"/owed-to/"+url.PathEscape(agentID), nil, nil)

	case "task_get_owed_to":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/owed-to", nil, nil)

	// ── Mentions ──────────────────────────────────────────────────

	case "task_add_mention":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/mentions", nil, map[string]any{"agent_id": agentID})

	case "task_remove_mention":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodDelete, "/tasks/"+url.PathEscape(taskID)+"/mentions/"+url.PathEscape(agentID), nil, nil)

	case "task_get_mentions":
		taskID, err := requireString(arguments, "task_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/mentions", nil, nil)

	// ── Agents ─────────────────────────────────────────────────────

	case "agent_whoami":
		return e.request(ctx, agent, http.MethodGet, "/agents/me", nil, nil)

	case "agent_assignable":
		return e.request(ctx, agent, http.MethodGet, "/agents/me/assignable", nil, nil)

	case "agent_list":
		return e.request(ctx, agent, http.MethodGet, "/agents", queryArgs(arguments, "search", "type", "active", "limit", "offset"), nil)

	case "agent_get":
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodGet, "/agents/"+url.PathEscape(agentID), nil, nil)

	case "agent_create":
		return e.request(ctx, agent, http.MethodPost, "/agents", nil, arguments)

	case "agent_update_me":
		return e.request(ctx, agent, http.MethodPatch, "/agents/me", nil, arguments)

	case "agent_update":
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		body := cloneMap(arguments)
		delete(body, "agent_id")
		return e.request(ctx, agent, http.MethodPatch, "/agents/"+url.PathEscape(agentID), nil, body)

	case "agent_approve":
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/approve", nil, nil)

	case "agent_suspend":
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/suspend", nil, nil)

	case "agent_reactivate":
		agentID, err := requireString(arguments, "agent_id")
		if err != nil {
			return nil, err
		}
		return e.request(ctx, agent, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/reactivate", nil, nil)

	case "agent_rotate_key":
		return e.request(ctx, agent, http.MethodPost, "/agents/me/rotate-key", nil, nil)

	// ── Admin ──────────────────────────────────────────────────────

	case "admin_audit":
		return e.request(ctx, agent, http.MethodGet, "/admin/audit", queryArgs(arguments, "limit"), nil)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
}

func (e *Executor) request(ctx context.Context, agent *models.Agent, method, path string, query url.Values, body any) (any, error) {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	reqCtx := context.WithValue(context.WithoutCancel(ctx), middleware.AgentContextKey, agent)
	reqCtx = context.WithValue(reqCtx, chi.RouteCtxKey, chi.NewRouteContext())
	req := httptest.NewRequest(method, path, reader).WithContext(reqCtx)
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	if rec.Code >= 400 {
		var errBody map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err == nil {
			if message, ok := errBody["message"].(string); ok && message != "" {
				return nil, fmt.Errorf("%s %s -> HTTP %d: %s", method, path, rec.Code, message)
			}
		}
		return nil, fmt.Errorf("%s %s -> HTTP %d: %s", method, path, rec.Code, rec.Body.String())
	}

	if rec.Body.Len() == 0 {
		return map[string]any{"status": "ok"}, nil
	}

	var payload any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func queryArgs(arguments map[string]any, keys ...string) url.Values {
	values := url.Values{}
	for _, key := range keys {
		switch v := arguments[key].(type) {
		case string:
			if v != "" {
				values.Set(key, v)
			}
		case float64:
			values.Set(key, strconv.Itoa(int(v)))
		case int:
			values.Set(key, strconv.Itoa(v))
		}
	}
	return values
}

func requireString(arguments map[string]any, key string) (string, error) {
	value, ok := stringArg(arguments, key)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func requireInt(arguments map[string]any, key string) (int, error) {
	value, ok := arguments[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func stringArg(arguments map[string]any, key string) (string, bool) {
	value, ok := arguments[key]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

func filterActivitySince(result any, since string) (any, error) {
	cutoff, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return nil, fmt.Errorf("invalid since timestamp: %w", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		return result, nil
	}

	rawActivities, ok := payload["activity"].([]any)
	if !ok {
		return result, nil
	}

	filtered := make([]any, 0, len(rawActivities))
	for _, item := range rawActivities {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		createdAt, _ := entry["created_at"].(string)
		if createdAt == "" {
			filtered = append(filtered, item)
			continue
		}
		ts, err := time.Parse(time.RFC3339, createdAt)
		if err != nil || !ts.Before(cutoff) {
			filtered = append(filtered, item)
		}
	}

	payload["activity"] = filtered
	payload["total"] = len(filtered)
	return payload, nil
}
