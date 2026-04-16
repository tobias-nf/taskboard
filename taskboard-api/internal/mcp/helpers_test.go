package mcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestTaskboardToolsCatalog(t *testing.T) {
	tools := taskboardTools()

	// Verify all domains are represented and no duplicates
	nameSet := map[string]bool{}
	for _, tool := range tools {
		if nameSet[tool.Name] {
			t.Fatalf("duplicate tool name: %s", tool.Name)
		}
		nameSet[tool.Name] = true
	}

	// Check representative tools from each domain
	for _, name := range []string{
		"task_create", "task_list_visible", "task_cancel",
		"agent_whoami", "agent_list", "agent_create",
		"tag_list", "tag_create", "task_add_tag",
		"task_add_owed_to", "task_add_mention",
		"admin_audit",
	} {
		if !nameSet[name] {
			t.Errorf("expected tool %q in catalog", name)
		}
	}

	if len(tools) < 30 {
		t.Fatalf("expected at least 30 tools, got %d", len(tools))
	}

	// Verify task_create still requires title
	for _, tool := range tools {
		if tool.Name == "task_create" {
			required, ok := tool.InputSchema["required"].([]string)
			if !ok || len(required) != 1 || required[0] != "title" {
				t.Fatalf("expected task_create to require title, got %#v", tool.InputSchema["required"])
			}
		}
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantTok string
		wantOK  bool
	}{
		{name: "missing", header: "", wantOK: false},
		{name: "not bearer", header: "Token abc", wantOK: false},
		{name: "bearer token", header: "Bearer abc123", wantTok: "abc123", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTok, gotOK := bearerToken(tt.header)
			if gotTok != tt.wantTok || gotOK != tt.wantOK {
				t.Fatalf("bearerToken(%q) = (%q, %t), want (%q, %t)", tt.header, gotTok, gotOK, tt.wantTok, tt.wantOK)
			}
		})
	}
}

func TestMessageEndpointURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://taskboard.local/mcp/sse", nil)
	req.Host = "taskboard.local"

	if got := messageEndpointURL(req, "sess-1"); got != "http://taskboard.local/mcp/messages/sess-1" {
		t.Fatalf("unexpected default endpoint URL: %s", got)
	}

	req.Header.Set("X-Forwarded-Proto", "https")
	if got := messageEndpointURL(req, "sess-1"); got != "https://taskboard.local/mcp/messages/sess-1" {
		t.Fatalf("unexpected forwarded endpoint URL: %s", got)
	}

	req.TLS = &tls.ConnectionState{}
	req.Header.Del("X-Forwarded-Proto")
	if got := messageEndpointURL(req, "sess-1"); got != "https://taskboard.local/mcp/messages/sess-1" {
		t.Fatalf("unexpected TLS endpoint URL: %s", got)
	}
}

func TestQueryArgsAndStringHelpers(t *testing.T) {
	args := map[string]any{
		"status":   "pending,accepted",
		"priority": 2,
		"limit":    float64(15),
		"ignored":  "",
	}

	values := queryArgs(args, "status", "priority", "limit", "ignored")
	if got := values.Get("status"); got != "pending,accepted" {
		t.Fatalf("expected status query arg, got %q", got)
	}
	if got := values.Get("priority"); got != "2" {
		t.Fatalf("expected integer query arg to stringify, got %q", got)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected float query arg to stringify, got %q", got)
	}
	if _, ok := values["ignored"]; ok {
		t.Fatal("expected empty string query arg to be omitted")
	}

	if got, err := requireString(map[string]any{"task_id": "T-1"}, "task_id"); err != nil || got != "T-1" {
		t.Fatalf("expected required string to be returned, got %q err=%v", got, err)
	}
	if _, err := requireString(map[string]any{}, "task_id"); err == nil {
		t.Fatal("expected missing string argument to fail")
	}
	if got, ok := stringArg(map[string]any{"content": "hello"}, "content"); !ok || got != "hello" {
		t.Fatalf("expected stringArg to return content, got %q %t", got, ok)
	}
}

func TestFilterActivitySince(t *testing.T) {
	cutoff := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	old := cutoff.Add(-time.Minute).Format(time.RFC3339)
	keep := cutoff.Add(time.Minute).Format(time.RFC3339)

	payload := map[string]any{
		"activity": []any{
			map[string]any{"id": 1, "created_at": old},
			map[string]any{"id": 2, "created_at": keep},
			map[string]any{"id": 3, "created_at": "not-a-timestamp"},
			map[string]any{"id": 4},
		},
		"total": 4,
	}

	got, err := filterActivitySince(payload, cutoff.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("filterActivitySince returned error: %v", err)
	}

	filtered := got.(map[string]any)
	if filtered["total"].(int) != 3 {
		t.Fatalf("expected filtered total 3, got %#v", filtered["total"])
	}

	activity := filtered["activity"].([]any)
	if len(activity) != 3 {
		t.Fatalf("expected 3 activity entries, got %d", len(activity))
	}

	gotIDs := []string{}
	for _, item := range activity {
		gotIDs = append(gotIDs, fmt.Sprintf("%v", item.(map[string]any)["id"]))
	}
	if !slices.Equal(gotIDs, []string{"2", "3", "4"}) {
		t.Fatalf("unexpected filtered activity ids: %v", gotIDs)
	}
}

func TestFilterActivitySinceRejectsBadTimestamp(t *testing.T) {
	_, err := filterActivitySince(map[string]any{"activity": []any{}}, "bad-time")
	if err == nil {
		t.Fatal("expected invalid since timestamp to fail")
	}
	if !strings.Contains(err.Error(), "invalid since timestamp") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNestedChiContextMustBeResetForInnerRouter(t *testing.T) {
	inner := chi.NewRouter()
	inner.Get("/tasks/me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	outer := chi.NewRouter()
	outer.Post("/messages/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		req := httptest.NewRequest(http.MethodGet, "/tasks/me", nil).WithContext(r.Context())
		rec := httptest.NewRecorder()
		inner.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected inherited chi route context to break inner routing, got %d", rec.Code)
		}

		resetCtx := context.WithValue(context.WithoutCancel(r.Context()), chi.RouteCtxKey, chi.NewRouteContext())
		req = httptest.NewRequest(http.MethodGet, "/tasks/me", nil).WithContext(resetCtx)
		rec = httptest.NewRecorder()
		inner.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected reset chi route context to restore routing, got %d", rec.Code)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/messages/sess-1", nil)
	rec := httptest.NewRecorder()
	outer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected outer handler to complete, got %d", rec.Code)
	}
}
