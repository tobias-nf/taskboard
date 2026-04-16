package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/events"
	"github.com/nearintents/taskboard-api/internal/middleware"
	"github.com/nearintents/taskboard-api/internal/models"
	"github.com/nearintents/taskboard-api/internal/storage"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) *models.Agent {
	agent := middleware.GetAgent(r.Context())
	if agent == nil || agent.Type != "admin" {
		writeError(w, http.StatusForbidden, "forbidden", "Admin access required")
		return nil
	}
	return agent
}

// hasElevatedAccess returns true for admin and service agents.
// Service agents get full task access (like admin) but NOT agent management or audit privileges.
func hasElevatedAccess(agent *models.Agent) bool {
	return agent.Type == "admin" || agent.Type == "service"
}

func checkTaskVisibility(store *db.Store, w http.ResponseWriter, r *http.Request) (*models.Task, db.VisibilityResult, bool) {
	agent := middleware.GetAgent(r.Context())
	id := chi.URLParam(r, "id")
	task, err := store.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Task not found")
		return nil, db.VisibilityResult{}, false
	}
	// Service and admin agents can see all tasks including private
	if hasElevatedAccess(agent) {
		return task, db.VisibilityResult{CanView: true, CanComment: true}, true
	}
	vis, err := store.CanAgentSeeTask(r.Context(), agent.ID, task)
	if err != nil || !vis.CanView {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this task")
		return nil, vis, false
	}
	return task, vis, true
}

func configuredAdminManagedMessage(action string) string {
	return fmt.Sprintf("Configured admin is managed via %s and cannot %s via API", db.ConfiguredAdminAPIKeyEnv, action)
}

// --- Agents ---

func CreateAgentByAdmin(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		var req struct {
			ID    string  `json:"id"`
			Type  string  `json:"type"`
			Email *string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
			return
		}
		if req.ID == "" || req.Type == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "id and type are required")
			return
		}
		if req.Type != "user" && req.Type != "admin" && req.Type != "service" {
			writeError(w, http.StatusBadRequest, "invalid_type", "Type must be 'user', 'admin', or 'service'")
			return
		}

		key, hash, prefix, err := db.GenerateAPIKey(req.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "key_gen_failed", "Failed to generate API key")
			return
		}

		agent := &models.Agent{
			ID:    req.ID,
			Type:  req.Type,
			Email: req.Email,
		}

		if err := store.CreateAgent(r.Context(), agent, hash, prefix, true, &admin.ID); err != nil {
			writeError(w, http.StatusConflict, "agent_exists", "Agent with this ID already exists")
			return
		}

		store.LogAudit(r.Context(), "agent_created", admin.ID, strPtr("agent"), &req.ID, nil)

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":      req.ID,
			"api_key": key,
			"active":  true,
		})
	}
}

func AdminRotateKey(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		id := chi.URLParam(r, "id")
		if id == db.ConfiguredAdminID {
			writeError(w, http.StatusForbidden, "env_managed", configuredAdminManagedMessage("rotate its key"))
			return
		}
		if _, err := store.GetAgentByID(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}
		key, err := store.RotateAPIKey(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "rotate_failed", "Failed to rotate key")
			return
		}
		store.LogAudit(r.Context(), "key_rotated", admin.ID, strPtr("agent"), &id, nil)
		writeJSON(w, http.StatusOK, map[string]string{"api_key": key})
	}
}

func GetMe(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		writeJSON(w, http.StatusOK, agent)
	}
}

func UpdateMe(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		if agent.ID == db.ConfiguredAdminID {
			writeError(w, http.StatusForbidden, "env_managed", configuredAdminManagedMessage("be updated"))
			return
		}
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
			return
		}
		if err := store.UpdateAgentProfile(r.Context(), agent.ID, fields); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		updated, _ := store.GetAgentByID(r.Context(), agent.ID)
		writeJSON(w, http.StatusOK, updated)
	}
}

func RotateKey(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		if agent.ID == db.ConfiguredAdminID {
			writeError(w, http.StatusForbidden, "env_managed", configuredAdminManagedMessage("rotate its key"))
			return
		}
		key, err := store.RotateAPIKey(r.Context(), agent.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "rotate_failed", "Failed to rotate key")
			return
		}
		store.LogAudit(r.Context(), "key_rotated", agent.ID, strPtr("agent"), &agent.ID, nil)
		writeJSON(w, http.StatusOK, map[string]string{"api_key": key})
	}
}

func GetAssignable(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agents, err := store.GetAssignableAgents(r.Context())
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"agents": agents, "total": len(agents)})
	}
}

func ListAgents(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		limit, offset := parsePagination(r)
		q := r.URL.Query()
		params := db.AgentListParams{
			Search: strings.TrimSpace(q.Get("search")),
			Type:   strings.TrimSpace(q.Get("type")),
			Limit:  limit,
			Offset: offset,
		}
		if agent.Type == "admin" {
			if active := q.Get("active"); active != "" {
				if parsed, err := strconv.ParseBool(active); err == nil {
					params.Active = &parsed
				}
			}
		} else {
			active := true
			params.Active = &active
		}
		agents, total, err := store.ListAgentsPaginated(r.Context(), params)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"agents": agents, "total": total, "limit": limit, "offset": offset})
	}
}

func GetAgent(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		agent, err := store.GetAgentByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}
		writeJSON(w, http.StatusOK, agent)
	}
}

func UpdateAgent(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		id := chi.URLParam(r, "id")
		if id == db.ConfiguredAdminID {
			writeError(w, http.StatusForbidden, "env_managed", configuredAdminManagedMessage("be updated"))
			return
		}
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
			return
		}
		if err := store.UpdateAgentAdmin(r.Context(), id, fields); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		updated, _ := store.GetAgentByID(r.Context(), id)
		writeJSON(w, http.StatusOK, updated)
	}
}

func ApproveAgent(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		id := chi.URLParam(r, "id")
		if err := store.ApproveAgent(r.Context(), id, admin.ID); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		store.LogAudit(r.Context(), "agent_approved", admin.ID, strPtr("agent"), &id, nil)
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": true, "approved_by": admin.ID})
	}
}

func SuspendAgent(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		id := chi.URLParam(r, "id")
		if id == db.ConfiguredAdminID {
			writeError(w, http.StatusForbidden, "env_managed", configuredAdminManagedMessage("be suspended"))
			return
		}
		store.SuspendAgent(r.Context(), id)
		store.LogAudit(r.Context(), "agent_suspended", admin.ID, strPtr("agent"), &id, nil)
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": false})
	}
}

func ReactivateAgent(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		id := chi.URLParam(r, "id")
		store.ApproveAgent(r.Context(), id, admin.ID)
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": true})
	}
}

// --- Tasks ---

func CreateTask(store *db.Store, broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		var req struct {
			models.Task
			OwedTo   []string `json:"owed_to"`
			Mentions []string `json:"mentions"`
			Tags     []string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		t := req.Task
		t.CreatedBy = agent.ID
		t.Title = stripHTMLTags(t.Title)
		if t.Description != nil {
			cleaned := stripHTMLTags(*t.Description)
			t.Description = &cleaned
		}
		if t.AssignedTo == nil {
			t.AssignedTo = &agent.ID
		}
		if t.Priority == "" {
			t.Priority = "standard"
		}

		// Subtasks inherit visibility from parent
		if t.ParentID != nil {
			parentTask, err := store.GetTask(r.Context(), *t.ParentID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_parent", "Parent task not found")
				return
			}
			// Verify caller can see the parent
			if !hasElevatedAccess(agent) {
				vis, err := store.CanAgentSeeTask(r.Context(), agent.ID, parentTask)
				if err != nil || !vis.CanView {
					writeError(w, http.StatusForbidden, "forbidden", "You do not have access to the parent task")
					return
				}
			}
			t.Visibility = parentTask.Visibility
		} else {
			if t.Visibility == "" {
				t.Visibility = "public"
			}
			if t.Visibility != "public" && t.Visibility != "private" {
				writeError(w, http.StatusBadRequest, "invalid_visibility", "visibility must be 'public' or 'private'")
				return
			}
		}

		// Validate assignee exists and is active
		if t.AssignedTo != nil && *t.AssignedTo != agent.ID {
			assignee, err := store.GetAgentByID(r.Context(), *t.AssignedTo)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_assignee", "Assignee not found")
				return
			}
			if !assignee.Active {
				writeError(w, http.StatusBadRequest, "invalid_assignee", "Assignee is not active")
				return
			}
		}

		// Allow creating tasks with 'draft' status for approval workflows
		if t.Status != "" && t.Status != "pending" && t.Status != "draft" {
			writeError(w, http.StatusBadRequest, "invalid_status", "tasks can only be created with status 'pending' or 'draft'")
			return
		}

		id, err := store.CreateTask(r.Context(), &t)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		t.ID = id
		if t.Status == "" {
			t.Status = "pending"
		}

		// Add owed_to entries
		for _, agentID := range req.OwedTo {
			store.AddOwedTo(r.Context(), id, agentID, agent.ID)
		}

		// Add mentions
		for _, agentID := range req.Mentions {
			store.AddMention(r.Context(), id, agentID, agent.ID)
		}

		// Add tags (create-on-use)
		for _, tagName := range req.Tags {
			tag, err := store.GetTagByName(r.Context(), tagName)
			if err != nil {
				tag = &models.Tag{Name: tagName, CreatedBy: agent.ID}
				if err := store.CreateTag(r.Context(), tag); err != nil {
					tag, _ = store.GetTagByName(r.Context(), tagName)
				}
			}
			if tag != nil {
				store.AddTaskTag(r.Context(), id, tag.ID, agent.ID)
			}
		}

		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: id, Type: "created", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Task created"),
		})

		emitTaskEvent(broker, store, &t, "task.created", agent.ID, map[string]string{"title": t.Title})
		writeJSON(w, http.StatusCreated, t)
	}
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			offset = n
		}
	}
	return
}

func parseTaskListParams(r *http.Request) db.TaskListParams {
	q := r.URL.Query()
	params := db.TaskListParams{
		Sort: q.Get("sort"),
		Tag:  q.Get("tag"),
	}
	if s := q.Get("status"); s != "" {
		params.Status = strings.Split(s, ",")
	}
	if p := q.Get("priority"); p != "" {
		params.Priority = strings.Split(p, ",")
	}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			params.Limit = n
		}
	}
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			params.Offset = n
		}
	}
	return params
}

func GetTasksOwedToMe(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		params := parseTaskListParams(r)
		tasks, total, err := store.GetTasksOwedTo(r.Context(), agent.ID, params)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		limit := params.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "total": total, "limit": limit, "offset": params.Offset})
	}
}

func GetMyTasks(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		params := parseTaskListParams(r)
		tasks, total, err := store.GetTasksForAgent(r.Context(), agent.ID, params)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		limit := params.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "total": total, "limit": limit, "offset": params.Offset})
	}
}

func GetMyCreatedTasks(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		params := parseTaskListParams(r)
		tasks, total, err := store.GetTasksCreatedBy(r.Context(), agent.ID, params)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		limit := params.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "total": total, "limit": limit, "offset": params.Offset})
	}
}

func GetVisibleTasks(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		params := parseTaskListParams(r)
		var tasks []models.Task
		var total int
		var err error
		if hasElevatedAccess(agent) {
			tasks, total, err = store.GetAllTasks(r.Context(), params)
		} else {
			tasks, total, err = store.GetVisibleTasks(r.Context(), agent.ID, params)
		}
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		limit := params.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "total": total, "limit": limit, "offset": params.Offset})
	}
}

func GetTask(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, task)
	}
}

func UpdateTask(store *db.Store, broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		id := chi.URLParam(r, "id")
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		oldTask, err := store.GetTask(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Task not found")
			return
		}

		// Check visibility first (service/admin agents bypass)
		var vis db.VisibilityResult
		if hasElevatedAccess(agent) {
			vis = db.VisibilityResult{CanView: true, CanComment: true}
		} else {
			vis, err = store.CanAgentSeeTask(r.Context(), agent.ID, oldTask)
		}
		if err != nil || !vis.CanView {
			writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this task")
			return
		}

		// Only creator, assignee, or service/admin can update fields
		isOwner := oldTask.CreatedBy == agent.ID || (oldTask.AssignedTo != nil && *oldTask.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only creator, assignee, or service/admin can update this task")
			return
		}

		// Handle status change separately
		if status, ok := req["status"].(string); ok {
			isCreatorOrAdmin := oldTask.CreatedBy == agent.ID || hasElevatedAccess(agent)
			if err := db.ValidateStatusTransition(oldTask.Status, status, isCreatorOrAdmin); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_transition", err.Error())
				return
			}
			store.UpdateTaskStatus(r.Context(), id, status)
			store.AddActivity(r.Context(), &models.TaskActivity{
				TaskID: id, Type: "status_changed", Actor: agent.ID, ActorType: "agent",
				Summary:  strPtr("Status changed to " + status),
				OldValue: strPtr(oldTask.Status), NewValue: &status,
			})
			delete(req, "status")
		}

		// Handle priority change with activity
		if priority, ok := req["priority"].(string); ok && priority != oldTask.Priority {
			store.AddActivity(r.Context(), &models.TaskActivity{
				TaskID: id, Type: "priority_changed", Actor: agent.ID, ActorType: "agent",
				Summary:  strPtr("Priority changed to " + priority),
				OldValue: strPtr(oldTask.Priority), NewValue: &priority,
			})
		}

		// Handle assigned_to change
		if assignedTo, ok := req["assigned_to"].(string); ok {
			oldAssignee := ""
			if oldTask.AssignedTo != nil {
				oldAssignee = *oldTask.AssignedTo
			}
			if assignedTo != oldAssignee {
				// Validate assignee exists and is active
				assignee, err := store.GetAgentByID(r.Context(), assignedTo)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_assignee", "Assignee not found")
					return
				}
				if !assignee.Active {
					writeError(w, http.StatusBadRequest, "invalid_assignee", "Assignee is not active")
					return
				}
				store.AddActivity(r.Context(), &models.TaskActivity{
					TaskID: id, Type: "reassigned", Actor: agent.ID, ActorType: "agent",
					Summary:  strPtr("Reassigned to " + assignedTo),
					OldValue: strPtr(oldAssignee), NewValue: &assignedTo,
				})
			}
		}

		// Handle parent_id change (move task)
		if parentID, ok := req["parent_id"]; ok {
			newParent, _ := parentID.(string)
			oldParent := ""
			if oldTask.ParentID != nil {
				oldParent = *oldTask.ParentID
			}
			if newParent != oldParent {
				if newParent != "" {
					// Validate parent exists
					if _, err := store.GetTask(r.Context(), newParent); err != nil {
						writeError(w, http.StatusBadRequest, "invalid_parent", "Parent task not found")
						return
					}
					// Prevent circular reference
					if newParent == id {
						writeError(w, http.StatusBadRequest, "invalid_parent", "A task cannot be its own parent")
						return
					}
				}
				if newParent == "" {
					// Detach: set to nil so the DB gets NULL
					req["parent_id"] = nil
				}
				summary := "Moved under " + newParent
				if newParent == "" {
					summary = "Detached from parent"
				}
				store.AddActivity(r.Context(), &models.TaskActivity{
					TaskID: id, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
					Summary:  strPtr(summary),
					OldValue: strPtr(oldParent), NewValue: &newParent,
				})
			}
		}

		// Handle visibility change with activity
		if visibility, ok := req["visibility"].(string); ok && visibility != oldTask.Visibility {
			// Subtasks cannot change visibility independently
			if oldTask.ParentID != nil {
				writeError(w, http.StatusBadRequest, "inherited_visibility", "Subtask visibility is inherited from the parent task")
				return
			}
			if visibility != "public" && visibility != "private" {
				writeError(w, http.StatusBadRequest, "invalid_visibility", "visibility must be 'public' or 'private'")
				return
			}
			store.AddActivity(r.Context(), &models.TaskActivity{
				TaskID: id, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
				Summary:  strPtr("Visibility changed to " + visibility),
				OldValue: strPtr(oldTask.Visibility), NewValue: &visibility,
			})
			// Cascade to all children
			store.CascadeVisibility(r.Context(), id, visibility)
		}

		// Sanitize text fields
		for _, field := range []string{"title", "description"} {
			if v, ok := req[field].(string); ok {
				req[field] = stripHTMLTags(v)
			}
		}

		// Update remaining fields — re-fetch updated_at in case status/priority change already bumped it
		if len(req) > 0 {
			current, _ := store.GetTask(r.Context(), id)
			if current != nil {
				oldTask = current
			}
			if err := store.UpdateTaskFields(r.Context(), id, req, &oldTask.UpdatedAt); err != nil {
				if strings.Contains(err.Error(), "conflict") {
					writeError(w, http.StatusConflict, "conflict", "Task was modified by another request, please retry")
					return
				}
				log.Printf("db error: %v", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
				return
			}
		}

		task, _ := store.GetTask(r.Context(), id)
		emitTaskEvent(broker, store, task, "task.updated", agent.ID, req)
		writeJSON(w, http.StatusOK, task)
	}
}

func CancelTask(store *db.Store, broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		id := chi.URLParam(r, "id")
		task, err := store.GetTask(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Task not found")
			return
		}
		if task.CreatedBy != agent.ID && agent.Type != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator or admin can cancel")
			return
		}
		isCreatorOrAdmin := true
		if err := db.ValidateStatusTransition(task.Status, "cancelled", isCreatorOrAdmin); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_transition", err.Error())
			return
		}
		store.UpdateTaskStatus(r.Context(), id, "cancelled")
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: id, Type: "status_changed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Task cancelled"),
		})
		emitTaskEvent(broker, store, task, "task.cancelled", agent.ID, nil)
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "cancelled"})
	}
}

// --- Tags ---

func ListTags(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := store.ListTags(r.Context())
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tags": tags, "total": len(tags)})
	}
}

func CreateTag(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		var req struct {
			Name  string  `json:"name"`
			Color *string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "name is required")
			return
		}
		tag := &models.Tag{Name: req.Name, Color: req.Color, CreatedBy: agent.ID}
		if err := store.CreateTag(r.Context(), tag); err != nil {
			writeError(w, http.StatusConflict, "tag_exists", "Tag with this name already exists")
			return
		}
		writeJSON(w, http.StatusCreated, tag)
	}
}

func DeleteTag(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid tag ID")
			return
		}
		if err := store.DeleteTag(r.Context(), id); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func GetTaskTags(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		taskID := chi.URLParam(r, "id")
		tags, err := store.GetTaskTags(r.Context(), taskID)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tags": tags, "total": len(tags)})
	}
}

func AddTaskTag(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can add tags")
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "name is required")
			return
		}

		// Create-on-use
		tag, err := store.GetTagByName(r.Context(), req.Name)
		if err != nil {
			tag = &models.Tag{Name: req.Name, CreatedBy: agent.ID}
			if err := store.CreateTag(r.Context(), tag); err != nil {
				tag, _ = store.GetTagByName(r.Context(), req.Name)
			}
		}
		if tag == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create tag")
			return
		}

		if err := store.AddTaskTag(r.Context(), taskID, tag.ID, agent.ID); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusCreated, tag)
	}
}

func RemoveTaskTag(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can remove tags")
			return
		}

		tagIDStr := chi.URLParam(r, "tagId")
		tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid tag ID")
			return
		}
		store.RemoveTaskTag(r.Context(), taskID, tagID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- Owed-to ---

func GetOwedTo(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		taskID := chi.URLParam(r, "id")
		entries, err := store.GetOwedTo(r.Context(), taskID)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"owed_to": entries, "total": len(entries)})
	}
}

func AddOwedTo(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can add owed_to")
			return
		}

		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentID == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "agent_id is required")
			return
		}

		if _, err := store.GetAgentByID(r.Context(), req.AgentID); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}

		if err := store.AddOwedTo(r.Context(), taskID, req.AgentID, agent.ID); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: taskID, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Added " + req.AgentID + " as stakeholder"),
		})
		writeJSON(w, http.StatusCreated, map[string]string{"task_id": taskID, "agent_id": req.AgentID})
	}
}

func RemoveOwedTo(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")
		agentID := chi.URLParam(r, "agentId")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can remove owed_to")
			return
		}

		store.RemoveOwedTo(r.Context(), taskID, agentID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- Mentions ---

func GetMentions(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		taskID := chi.URLParam(r, "id")
		entries, err := store.GetMentions(r.Context(), taskID)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mentions": entries, "total": len(entries)})
	}
}

func AddMention(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can add mentions")
			return
		}

		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentID == "" {
			writeError(w, http.StatusBadRequest, "missing_fields", "agent_id is required")
			return
		}

		if _, err := store.GetAgentByID(r.Context(), req.AgentID); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}

		if err := store.AddMention(r.Context(), taskID, req.AgentID, agent.ID); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: taskID, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Mentioned " + req.AgentID),
		})
		writeJSON(w, http.StatusCreated, map[string]string{"task_id": taskID, "agent_id": req.AgentID})
	}
}

func RemoveMention(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")
		agentID := chi.URLParam(r, "agentId")

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can remove mentions")
			return
		}

		store.RemoveMention(r.Context(), taskID, agentID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- Activity ---

func AddActivity(store *db.Store, broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, vis, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())

		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner && !vis.CanComment {
			writeError(w, http.StatusForbidden, "comment_denied", "You do not have comment permission on this task")
			return
		}

		id := chi.URLParam(r, "id")
		var a models.TaskActivity
		json.NewDecoder(r.Body).Decode(&a)
		a.TaskID = id
		a.Actor = agent.ID
		if a.ActorType == "" {
			a.ActorType = "agent"
		}
		if a.Summary != nil {
			cleaned := stripHTMLTags(*a.Summary)
			a.Summary = &cleaned
		}
		if err := store.AddActivity(r.Context(), &a); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}

		// Parse @mentions from comments and add to task_mentions
		if a.Type == "commented" && a.Summary != nil {
			for _, mentionID := range parseMentions(*a.Summary) {
				if _, err := store.GetAgentByID(r.Context(), mentionID); err == nil {
					store.AddMention(r.Context(), id, mentionID, agent.ID)
				}
			}
		}

		eventType := "task.updated"
		if a.Type == "commented" {
			eventType = "task.commented"
		}
		emitTaskEvent(broker, store, task, eventType, agent.ID, map[string]string{"type": a.Type, "summary": derefStr(a.Summary)})
		writeJSON(w, http.StatusCreated, a)
	}
}

func GetActivity(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		acts, err := store.GetActivity(r.Context(), id)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"activity": acts, "total": len(acts)})
	}
}

// --- References ---

func AddReference(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")
		var ref models.TaskReference
		json.NewDecoder(r.Body).Decode(&ref)
		ref.TaskID = taskID
		ref.CreatedBy = agent.ID
		if err := store.AddReference(r.Context(), &ref); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: taskID, Type: "reference_added", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Reference added: " + ref.Title),
		})
		writeJSON(w, http.StatusCreated, ref)
	}
}

func GetReferences(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		taskID := chi.URLParam(r, "id")
		refs, err := store.GetReferences(r.Context(), taskID)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"references": refs, "total": len(refs)})
	}
}

func RemoveReference(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, _, ok := checkTaskVisibility(store, w, r)
		if !ok {
			return
		}
		agent := middleware.GetAgent(r.Context())
		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can remove references")
			return
		}
		taskID := chi.URLParam(r, "id")
		refIDStr := chi.URLParam(r, "refId")
		refID, err := strconv.ParseInt(refIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid reference ID")
			return
		}
		if err := store.DeleteReference(r.Context(), refID); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: taskID, Type: "reference_removed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Reference removed"),
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- Attachments ---

func UploadAttachment(store *db.Store, s3 *storage.S3Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s3 == nil {
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "Object storage not configured")
			return
		}
		agent := middleware.GetAgent(r.Context())
		taskID := chi.URLParam(r, "id")

		r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "file_too_large", "File exceeds 50 MB limit")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing_file", "No file provided in 'file' field")
			return
		}
		defer file.Close()

		hasher := sha256.New()
		body := io.TeeReader(file, hasher)
		tmpBytes, err := io.ReadAll(body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read_error", "Failed to read file")
			return
		}
		hash := hex.EncodeToString(hasher.Sum(nil))
		size := int64(len(tmpBytes))
		contentType := http.DetectContentType(tmpBytes)
		safeFilename := sanitizeFilename(header.Filename)
		key := fmt.Sprintf("tasks/%s/%d_%s", taskID, time.Now().UnixMilli(), safeFilename)

		reader := io.NopCloser(io.Reader(bytes_reader(tmpBytes)))
		if err := s3.Upload(r.Context(), key, reader, size, contentType); err != nil {
			writeError(w, http.StatusInternalServerError, "upload_failed", "Failed to upload to storage")
			return
		}

		label := r.FormValue("label")
		var labelPtr *string
		if label != "" {
			labelPtr = &label
		}

		att := &models.TaskAttachment{
			TaskID:     taskID,
			Filename:   safeFilename,
			MimeType:   &contentType,
			SizeBytes:  &size,
			SHA256:     &hash,
			StorageKey: key,
			Label:      labelPtr,
			UploadedBy: agent.ID,
		}

		if err := store.CreateAttachment(r.Context(), att); err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}

		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: taskID, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Attachment uploaded: " + header.Filename),
		})

		writeJSON(w, http.StatusCreated, att)
	}
}

func ListAttachments(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := checkTaskVisibility(store, w, r); !ok {
			return
		}
		taskID := chi.URLParam(r, "id")
		attachments, err := store.ListAttachments(r.Context(), taskID)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachments": attachments, "total": len(attachments)})
	}
}

func DownloadAttachment(store *db.Store, s3 *storage.S3Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s3 == nil {
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "Object storage not configured")
			return
		}
		agent := middleware.GetAgent(r.Context())
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid attachment ID")
			return
		}
		att, err := store.GetAttachment(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
			return
		}

		task, err := store.GetTask(r.Context(), att.TaskID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Task not found")
			return
		}
		if !hasElevatedAccess(agent) {
			vis, err := store.CanAgentSeeTask(r.Context(), agent.ID, task)
			if err != nil || !vis.CanView {
				writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this attachment")
				return
			}
		}

		reader, contentType, err := s3.Download(r.Context(), att.StorageKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "download_failed", "Failed to retrieve file")
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, att.Filename))
		io.Copy(w, reader)
	}
}

func DeleteAttachment(store *db.Store, s3 *storage.S3Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid attachment ID")
			return
		}
		att, err := store.GetAttachment(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
			return
		}

		task, err := store.GetTask(r.Context(), att.TaskID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Task not found")
			return
		}
		isOwner := task.CreatedBy == agent.ID || (task.AssignedTo != nil && *task.AssignedTo == agent.ID) || hasElevatedAccess(agent)
		if !isOwner {
			writeError(w, http.StatusForbidden, "forbidden", "Only task creator, assignee, or admin can delete attachments")
			return
		}

		att, err = store.DeleteAttachment(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to delete attachment")
			return
		}
		if s3 != nil {
			s3.Delete(r.Context(), att.StorageKey)
		}
		store.AddActivity(r.Context(), &models.TaskActivity{
			TaskID: att.TaskID, Type: "field_changed", Actor: agent.ID, ActorType: "agent",
			Summary: strPtr("Attachment deleted: " + att.Filename),
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

type bytesReaderWrapper struct {
	data []byte
	pos  int
}

func bytes_reader(data []byte) io.Reader {
	return &bytesReaderWrapper{data: data}
}

func (r *bytesReaderWrapper) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// --- SSE Events ---

func EventStream(broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := middleware.GetAgent(r.Context())

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming not supported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		corsOrigin := os.Getenv("CORS_ORIGIN")
		if corsOrigin == "" {
			corsOrigin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", corsOrigin)

		ch, unsub := broker.Subscribe(agent.ID)
		defer unsub()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, msg)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}
}

// emitTaskEvent sends an SSE event to relevant agents.
func emitTaskEvent(broker *events.Broker, store *db.Store, task *models.Task, eventType, actor string, data any) {
	if broker == nil {
		return
	}
	seen := map[string]bool{}
	targets := []string{}
	addTarget := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			targets = append(targets, id)
		}
	}
	if task.AssignedTo != nil {
		addTarget(*task.AssignedTo)
	}
	addTarget(task.CreatedBy)

	// Notify owed_to agents and mentioned agents
	if store != nil {
		owedTo, _ := store.GetOwedTo(context.Background(), task.ID)
		for _, e := range owedTo {
			addTarget(e.AgentID)
		}
		mentions, _ := store.GetMentions(context.Background(), task.ID)
		for _, e := range mentions {
			addTarget(e.AgentID)
		}
	}

	if len(targets) == 0 {
		return
	}
	broker.Publish(targets, events.Event{
		Type:   eventType,
		TaskID: task.ID,
		Actor:  actor,
		Data:   data,
	})
}

// --- Admin ---

func GetAuditLog(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}
		entries, err := store.GetAuditLog(r.Context(), limit)
		if err != nil {
			log.Printf("db error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
	}
}

// parseMentions extracts @agent-id patterns from text.
var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_-]+)`)

func parseMentions(text string) []string {
	matches := mentionRe.FindAllStringSubmatch(text, -1)
	seen := map[string]bool{}
	var ids []string
	for _, m := range matches {
		id := m[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// stripHTMLTags removes HTML tags from a string to prevent XSS.
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagPattern.ReplaceAllString(s, "")
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = unsafeChars.ReplaceAllString(name, "_")
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name
}

func strPtr(s string) *string { return &s }

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
