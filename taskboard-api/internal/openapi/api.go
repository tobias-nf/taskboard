package openapi

import (
	stdctx "context"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/auth"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/events"
	"github.com/nearintents/taskboard-api/internal/handlers"
	"github.com/nearintents/taskboard-api/internal/middleware"
	"github.com/nearintents/taskboard-api/internal/models"
	"github.com/nearintents/taskboard-api/internal/storage"
)

const bearerAuthSchemeName = "bearerAuth"

type statusResponse struct {
	Status string `json:"status"`
}

type agentListEnvelope struct {
	Agents []models.Agent `json:"agents"`
	Total  int            `json:"total"`
}

type agentResponse struct {
	Body *models.Agent
}

type assignableAgentsResponse struct {
	Body agentListEnvelope
}

type statusOnlyResponse struct {
	Body statusResponse
}

type apiKeyResponse struct {
	Body struct {
		APIKey string `json:"api_key"`
	}
}

type createAgentInput struct{}

type createAgentResponse struct {
	Body struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
		Active bool   `json:"active"`
	}
}

type patchBodyInput struct{}

type agentPathInput struct {
	ID string `path:"id"`
}

type agentStateResponse struct {
	Body struct {
		ID         string `json:"id"`
		Active     bool   `json:"active"`
		ApprovedBy string `json:"approved_by,omitempty"`
	}
}

type agentsListInput struct {
	Limit  int    `query:"limit"`
	Offset int    `query:"offset"`
	Search string `query:"search"`
	Type   string `query:"type"`
	Active string `query:"active"`
}

type agentsPageResponse struct {
	Body struct {
		Agents []models.Agent `json:"agents"`
		Total  int            `json:"total"`
		Limit  int            `json:"limit"`
		Offset int            `json:"offset"`
	}
}

type taskPathInput struct {
	ID string `path:"id"`
}

type taskListInput struct {
	Status   string `query:"status"`
	Priority string `query:"priority"`
	Sort     string `query:"sort"`
	Tag      string `query:"tag"`
	Limit    int    `query:"limit"`
	Offset   int    `query:"offset"`
}

type taskCreateInput struct{}

type taskResponse struct {
	Body *models.Task
}

type taskListResponse struct {
	Body struct {
		Tasks  []models.Task `json:"tasks"`
		Total  int           `json:"total"`
		Limit  int           `json:"limit"`
		Offset int           `json:"offset"`
	}
}

type taskUpdateInput struct {
	ID string `path:"id"`
}

type taskCancelResponse struct {
	Body struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
}

type taskActivityResponse struct {
	Body *models.TaskActivity
}

type taskActivityListResponse struct {
	Body struct {
		Activity []models.TaskActivity `json:"activity"`
		Total    int                   `json:"total"`
	}
}

type taskActivityInput struct {
	ID string `path:"id"`
}

type taskReferenceResponse struct {
	Body *models.TaskReference
}

type taskReferencesResponse struct {
	Body struct {
		References []models.TaskReference `json:"references"`
		Total      int                    `json:"total"`
	}
}

type taskReferenceInput struct {
	ID string `path:"id"`
}

type taskReferencePathInput struct {
	ID    string `path:"id"`
	RefID int64  `path:"refId"`
}

type taskAttachmentResponse struct {
	Body *models.TaskAttachment
}

type taskAttachmentsResponse struct {
	Body struct {
		Attachments []models.TaskAttachment `json:"attachments"`
		Total       int                     `json:"total"`
	}
}

type attachmentPathInput struct {
	ID int64 `path:"id"`
}

type attachmentUploadInput struct {
	ID string `path:"id"`
}

type auditListInput struct {
	Limit int `query:"limit"`
}

type auditListResponse struct {
	Body struct {
		Entries []models.AdminAudit `json:"entries"`
		Total   int                 `json:"total"`
	}
}

type tagResponse struct {
	Body *models.Tag
}

type tagListResponse struct {
	Body struct {
		Tags  []models.Tag `json:"tags"`
		Total int          `json:"total"`
	}
}

type tagPathInput struct {
	ID int64 `path:"id"`
}

type taskTagPathInput struct {
	ID    string `path:"id"`
	TagID int64  `path:"tagId"`
}

type taskAgentPathInput struct {
	ID      string `path:"id"`
	AgentID string `path:"agentId"`
}

type owedToListResponse struct {
	Body struct {
		OwedTo []models.TaskOwedTo `json:"owed_to"`
		Total  int                 `json:"total"`
	}
}

type mentionListResponse struct {
	Body struct {
		Mentions []models.TaskMention `json:"mentions"`
		Total    int                  `json:"total"`
	}
}

type updateMeRequestDoc struct {
	Email         *string `json:"email,omitempty"`
	SlackID       *string `json:"slack_id,omitempty"`
	PreferredTool *string `json:"preferred_tool,omitempty"`
}

type createAgentRequestDoc struct {
	ID    string  `json:"id"`
	Type  string  `json:"type"`
	Email *string `json:"email,omitempty"`
}

type updateAgentRequestDoc struct {
	Type          *string `json:"type,omitempty"`
	Email         *string `json:"email,omitempty"`
	SlackID       *string `json:"slack_id,omitempty"`
	PreferredTool *string `json:"preferred_tool,omitempty"`
	Active        *bool  `json:"active,omitempty"`
}

type createTaskRequestDoc struct {
	Title       string         `json:"title"`
	Description *string        `json:"description,omitempty"`
	AssignedTo *string        `json:"assigned_to,omitempty"`
	Visibility string         `json:"visibility,omitempty"`
	Priority   string         `json:"priority,omitempty"`
	Deadline   *string        `json:"deadline,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
	ParentID   *string        `json:"parent_id,omitempty"`
	OwedTo     []string       `json:"owed_to,omitempty"`
	Mentions   []string       `json:"mentions,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
}

type updateTaskRequestDoc struct {
	Title         *string        `json:"title,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Status        *string        `json:"status,omitempty"`
	Priority      *string        `json:"priority,omitempty"`
	Visibility    *string        `json:"visibility,omitempty"`
	Result        map[string]any `json:"result,omitempty"`
	AssignedTo    *string        `json:"assigned_to,omitempty"`
	ParentID      *string        `json:"parent_id,omitempty"`
}

type addTaskActivityRequestDoc struct {
	Type      string         `json:"type,omitempty"`
	ActorType string         `json:"actor_type,omitempty"`
	Summary   *string        `json:"summary,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	OldValue  *string        `json:"old_value,omitempty"`
	NewValue  *string        `json:"new_value,omitempty"`
}

type addTaskReferenceRequestDoc struct {
	Type       string         `json:"type,omitempty"`
	Source     string         `json:"source"`
	ExternalID *string        `json:"external_id,omitempty"`
	URL        *string        `json:"url,omitempty"`
	Title      string         `json:"title"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type createTagRequestDoc struct {
	Name  string  `json:"name"`
	Color *string `json:"color,omitempty"`
}

type addAgentRequestDoc struct {
	AgentID string `json:"agent_id"`
}

type addTagRequestDoc struct {
	Name string `json:"name"`
}

func Mount(r chi.Router, store *db.Store, broker *events.Broker, authLimiter *middleware.RateLimiter, s3 *storage.S3Client, authHandler *auth.Handler) huma.API {
	config := huma.DefaultConfig("Taskboard API", "0.2.0")
	config.OpenAPIPath = "/openapi"
	config.DocsPath = "/docs"
	config.DocsRenderer = huma.DocsRendererScalar
	config.OpenAPI.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		bearerAuthSchemeName: {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "API key or JWT",
			Description:  "Use `Authorization: Bearer <token>` with either an API key (hive_sk_*) or a JWT session token.",
		},
	}

	api := humachi.New(r, config)

	authenticated := huma.NewGroup(api, "/api/v1")
	authenticated.UseMiddleware(adaptMiddleware(middleware.BearerAuth(store, authHandler)))
	authenticated.UseMiddleware(adaptMiddleware(middleware.RateLimit(authLimiter)))

	registerAgentOperations(authenticated, store)
	registerTaskOperations(authenticated, store, broker, s3)
	registerTagOperations(authenticated, store)
	registerAdminOperations(authenticated, store)

	return api
}

func registerAgentOperations(api huma.API, store *db.Store) {
	registerLegacyOperation[struct{}, agentResponse](api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/agents/me",
		Summary:     "Get my profile",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetMe(store))

	registerLegacyOperation[patchBodyInput, agentResponse](api, huma.Operation{
		OperationID: "update-me",
		Method:      http.MethodPatch,
		Path:        "/agents/me",
		Summary:     "Update my profile",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[updateMeRequestDoc]()), "Fields to update on the current agent profile.", true),
	}, handlers.UpdateMe(store))

	registerLegacyOperation[struct{}, apiKeyResponse](api, huma.Operation{
		OperationID: "rotate-my-key",
		Method:      http.MethodPost,
		Path:        "/agents/me/rotate-key",
		Summary:     "Rotate my API key",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.RotateKey(store))

	registerLegacyOperation[struct{}, assignableAgentsResponse](api, huma.Operation{
		OperationID: "get-assignable-agents",
		Method:      http.MethodGet,
		Path:        "/agents/me/assignable",
		Summary:     "List agents I can assign to",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetAssignable(store))

	registerLegacyOperation[createAgentInput, createAgentResponse](api, huma.Operation{
		OperationID: "create-agent",
		Method:      http.MethodPost,
		Path:        "/agents",
		Summary:     "Create an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[createAgentRequestDoc]()), "Agent details. The API key is generated by the server.", true),
	}, handlers.CreateAgentByAdmin(store))

	registerLegacyOperation[agentsListInput, agentsPageResponse](api, huma.Operation{
		OperationID: "list-agents",
		Method:      http.MethodGet,
		Path:        "/agents",
		Summary:     "List agents",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.ListAgents(store))

	registerLegacyOperation[agentPathInput, agentResponse](api, huma.Operation{
		OperationID: "get-agent",
		Method:      http.MethodGet,
		Path:        "/agents/{id}",
		Summary:     "Get an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetAgent(store))

	registerLegacyOperation[struct {
		ID   string `path:"id"`
		Body map[string]any
	}, agentResponse](api, huma.Operation{
		OperationID: "update-agent",
		Method:      http.MethodPatch,
		Path:        "/agents/{id}",
		Summary:     "Update an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[updateAgentRequestDoc]()), "Partial admin update for an agent.", true),
	}, handlers.UpdateAgent(store))

	registerLegacyOperation[agentPathInput, agentStateResponse](api, huma.Operation{
		OperationID: "approve-agent",
		Method:      http.MethodPost,
		Path:        "/agents/{id}/approve",
		Summary:     "Approve an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.ApproveAgent(store))

	registerLegacyOperation[agentPathInput, agentStateResponse](api, huma.Operation{
		OperationID: "suspend-agent",
		Method:      http.MethodPost,
		Path:        "/agents/{id}/suspend",
		Summary:     "Suspend an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.SuspendAgent(store))

	registerLegacyOperation[agentPathInput, agentStateResponse](api, huma.Operation{
		OperationID: "reactivate-agent",
		Method:      http.MethodPost,
		Path:        "/agents/{id}/reactivate",
		Summary:     "Reactivate an agent",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.ReactivateAgent(store))

	registerLegacyOperation[agentPathInput, apiKeyResponse](api, huma.Operation{
		OperationID: "admin-rotate-agent-key",
		Method:      http.MethodPost,
		Path:        "/agents/{id}/rotate-key",
		Summary:     "Rotate an agent API key",
		Tags:        []string{"Agents"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.AdminRotateKey(store))
}

func registerTagOperations(api huma.API, store *db.Store) {
	registerLegacyOperation[struct{}, tagListResponse](api, huma.Operation{
		OperationID: "list-tags",
		Method:      http.MethodGet,
		Path:        "/tags",
		Summary:     "List all tags",
		Tags:        []string{"Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.ListTags(store))

	registerLegacyOperation[struct{}, tagResponse](api, huma.Operation{
		OperationID: "create-tag",
		Method:      http.MethodPost,
		Path:        "/tags",
		Summary:     "Create a tag",
		Tags:        []string{"Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[createTagRequestDoc]()), "Tag details.", true),
	}, handlers.CreateTag(store))

	registerLegacyOperation[tagPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "delete-tag",
		Method:      http.MethodDelete,
		Path:        "/tags/{id}",
		Summary:     "Delete a tag (admin only)",
		Tags:        []string{"Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.DeleteTag(store))
}

func registerTaskOperations(api huma.API, store *db.Store, broker *events.Broker, s3 *storage.S3Client) {
	registerLegacyOperation[taskCreateInput, taskResponse](api, huma.Operation{
		OperationID: "create-task",
		Method:      http.MethodPost,
		Path:        "/tasks",
		Summary:     "Create a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[createTaskRequestDoc]()), "Task fields accepted on create.", true),
	}, handlers.CreateTask(store, broker))

	registerLegacyOperation[taskListInput, taskListResponse](api, huma.Operation{
		OperationID: "list-my-tasks",
		Method:      http.MethodGet,
		Path:        "/tasks/me",
		Summary:     "List tasks assigned to me",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetMyTasks(store))

	registerLegacyOperation[taskListInput, taskListResponse](api, huma.Operation{
		OperationID: "list-created-tasks",
		Method:      http.MethodGet,
		Path:        "/tasks/me/created",
		Summary:     "List tasks created by me",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetMyCreatedTasks(store))

	registerLegacyOperation[taskListInput, taskListResponse](api, huma.Operation{
		OperationID: "list-owed-to-me",
		Method:      http.MethodGet,
		Path:        "/tasks/me/owed",
		Summary:     "List tasks owed to me (where I am a stakeholder)",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetTasksOwedToMe(store))

	registerLegacyOperation[taskListInput, taskListResponse](api, huma.Operation{
		OperationID: "list-visible-tasks",
		Method:      http.MethodGet,
		Path:        "/tasks/visible",
		Summary:     "List tasks visible to me",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetVisibleTasks(store))

	registerLegacyOperation[taskPathInput, taskResponse](api, huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}",
		Summary:     "Get a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetTask(store))

	registerLegacyOperation[taskUpdateInput, taskResponse](api, huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPatch,
		Path:        "/tasks/{id}",
		Summary:     "Update a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[updateTaskRequestDoc]()), "Partial task update. Fields are optional.", true),
	}, handlers.UpdateTask(store, broker))

	registerLegacyOperation[taskPathInput, taskCancelResponse](api, huma.Operation{
		OperationID: "cancel-task",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}",
		Summary:     "Cancel a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.CancelTask(store, broker))

	// Tags on tasks
	registerLegacyOperation[taskPathInput, tagListResponse](api, huma.Operation{
		OperationID: "get-task-tags",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/tags",
		Summary:     "Get task tags",
		Tags:        []string{"Tasks", "Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetTaskTags(store))

	registerLegacyOperation[taskPathInput, tagResponse](api, huma.Operation{
		OperationID: "add-task-tag",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/tags",
		Summary:     "Add a tag to a task",
		Tags:        []string{"Tasks", "Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[addTagRequestDoc]()), "Tag name to add. Created on use if not exists.", true),
	}, handlers.AddTaskTag(store))

	registerLegacyOperation[taskTagPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "remove-task-tag",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}/tags/{tagId}",
		Summary:     "Remove a tag from a task",
		Tags:        []string{"Tasks", "Tags"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.RemoveTaskTag(store))

	// Owed-to
	registerLegacyOperation[taskPathInput, owedToListResponse](api, huma.Operation{
		OperationID: "get-task-owed-to",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/owed-to",
		Summary:     "Get task stakeholders",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetOwedTo(store))

	registerLegacyOperation[taskPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "add-task-owed-to",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/owed-to",
		Summary:     "Add a stakeholder to a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[addAgentRequestDoc]()), "Agent to add as stakeholder.", true),
	}, handlers.AddOwedTo(store))

	registerLegacyOperation[taskAgentPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "remove-task-owed-to",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}/owed-to/{agentId}",
		Summary:     "Remove a stakeholder from a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.RemoveOwedTo(store))

	// Mentions
	registerLegacyOperation[taskPathInput, mentionListResponse](api, huma.Operation{
		OperationID: "get-task-mentions",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/mentions",
		Summary:     "Get task mentions",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetMentions(store))

	registerLegacyOperation[taskPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "add-task-mention",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/mentions",
		Summary:     "Mention an agent on a task (grants access to private tasks)",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[addAgentRequestDoc]()), "Agent to mention.", true),
	}, handlers.AddMention(store))

	registerLegacyOperation[taskAgentPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "remove-task-mention",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}/mentions/{agentId}",
		Summary:     "Remove a mention from a task",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.RemoveMention(store))

	// Activity
	registerLegacyOperation[taskActivityInput, taskActivityResponse](api, huma.Operation{
		OperationID: "add-task-activity",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/activity",
		Summary:     "Add task activity",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[addTaskActivityRequestDoc]()), "Activity item to append to the task timeline.", true),
	}, handlers.AddActivity(store, broker))

	registerLegacyOperation[taskPathInput, taskActivityListResponse](api, huma.Operation{
		OperationID: "get-task-activity",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/activity",
		Summary:     "Get task activity",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetActivity(store))

	// References
	registerLegacyOperation[taskReferenceInput, taskReferenceResponse](api, huma.Operation{
		OperationID: "add-task-reference",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/references",
		Summary:     "Add a task reference",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: jsonRequestBody(schemaForType(api, reflect.TypeFor[addTaskReferenceRequestDoc]()), "Reference to link to the task.", true),
	}, handlers.AddReference(store))

	registerLegacyOperation[taskPathInput, taskReferencesResponse](api, huma.Operation{
		OperationID: "get-task-references",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/references",
		Summary:     "Get task references",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetReferences(store))

	registerLegacyOperation[taskReferencePathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "remove-task-reference",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}/references/{refId}",
		Summary:     "Remove a task reference",
		Tags:        []string{"Tasks"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.RemoveReference(store))

	// Attachments
	registerLegacyOperation[attachmentUploadInput, taskAttachmentResponse](api, huma.Operation{
		OperationID: "upload-task-attachment",
		Method:      http.MethodPost,
		Path:        "/tasks/{id}/attachments",
		Summary:     "Upload a task attachment",
		Tags:        []string{"Tasks", "Attachments"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		RequestBody: attachmentUploadRequestBody(),
	}, handlers.UploadAttachment(store, s3))

	registerLegacyOperation[taskPathInput, taskAttachmentsResponse](api, huma.Operation{
		OperationID: "list-task-attachments",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}/attachments",
		Summary:     "List task attachments",
		Tags:        []string{"Tasks", "Attachments"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.ListAttachments(store))

	registerLegacyOperation[attachmentPathInput, struct{}](api, huma.Operation{
		OperationID: "download-attachment",
		Method:      http.MethodGet,
		Path:        "/attachments/{id}/download",
		Summary:     "Download an attachment",
		Tags:        []string{"Attachments"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		Responses:   attachmentDownloadResponses(),
	}, handlers.DownloadAttachment(store, s3))

	registerLegacyOperation[attachmentPathInput, statusOnlyResponse](api, huma.Operation{
		OperationID: "delete-attachment",
		Method:      http.MethodDelete,
		Path:        "/attachments/{id}",
		Summary:     "Delete an attachment",
		Tags:        []string{"Attachments"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.DeleteAttachment(store, s3))

	// Events
	registerLegacyOperation[struct{}, struct{}](api, huma.Operation{
		OperationID: "events-stream",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "Open the task event stream",
		Tags:        []string{"Events"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
		Responses:   eventStreamResponses(),
	}, handlers.EventStream(broker))
}

func registerAdminOperations(api huma.API, store *db.Store) {
	registerLegacyOperation[auditListInput, auditListResponse](api, huma.Operation{
		OperationID: "get-audit-log",
		Method:      http.MethodGet,
		Path:        "/admin/audit",
		Summary:     "Get the admin audit log",
		Tags:        []string{"Admin"},
		Security:    []map[string][]string{{bearerAuthSchemeName: {}}},
	}, handlers.GetAuditLog(store))
}

func registerLegacyOperation[Input any, Output any](api huma.API, op huma.Operation, handler http.HandlerFunc) {
	op.Middlewares = append(op.Middlewares, invokeLegacyHandler(handler))
	huma.Register(api, op, func(ctx stdctx.Context, input *Input) (*Output, error) {
		return nil, nil
	})
}

func schemaForType(api huma.API, t reflect.Type) *huma.Schema {
	return huma.SchemaFromType(api.OpenAPI().Components.Schemas, t)
}

func jsonRequestBody(schema *huma.Schema, description string, required bool) *huma.RequestBody {
	return &huma.RequestBody{
		Description: description,
		Required:    required,
		Content: map[string]*huma.MediaType{
			"application/json": {
				Schema: schema,
			},
		},
	}
}

func attachmentUploadRequestBody() *huma.RequestBody {
	return &huma.RequestBody{
		Description: "Multipart upload containing the file field and an optional label field.",
		Required:    true,
		Content: map[string]*huma.MediaType{
			"multipart/form-data": {
				Schema: &huma.Schema{
					Type: huma.TypeObject,
					Properties: map[string]*huma.Schema{
						"file": {
							Type:        huma.TypeString,
							Format:      "binary",
							Description: "File contents to upload.",
						},
						"label": {
							Type:        huma.TypeString,
							Description: "Optional attachment label stored alongside the file.",
						},
					},
					Required: []string{"file"},
				},
				Encoding: map[string]*huma.Encoding{
					"file": {
						ContentType: "application/octet-stream",
					},
				},
			},
		},
	}
}

func attachmentDownloadResponses() map[string]*huma.Response {
	return map[string]*huma.Response{
		"200": {
			Description: "Attachment file response.",
			Headers: map[string]*huma.Param{
				"Content-Disposition": {
					Description: "Attachment filename disposition header.",
					Schema:      &huma.Schema{Type: huma.TypeString},
				},
			},
			Content: map[string]*huma.MediaType{
				"application/octet-stream": {
					Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"},
				},
			},
		},
	}
}

func eventStreamResponses() map[string]*huma.Response {
	return map[string]*huma.Response{
		"200": {
			Description: "Server-sent event stream for task updates visible to the authenticated agent.",
			Content: map[string]*huma.MediaType{
				"text/event-stream": {
					Schema: &huma.Schema{
						Type:        huma.TypeString,
						Description: "SSE stream payload with heartbeat comments and event messages.",
					},
				},
			},
		},
	}
}

func invokeLegacyHandler(handler http.HandlerFunc) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		req, res := humachi.Unwrap(ctx)
		handler.ServeHTTP(res, req.WithContext(ctx.Context()))
	}
}

func adaptMiddleware(mw func(http.Handler) http.Handler) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		req, res := humachi.Unwrap(ctx)
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next(huma.WithContext(ctx, r.Context()))
		}))
		wrapped.ServeHTTP(res, req.WithContext(ctx.Context()))
	}
}
