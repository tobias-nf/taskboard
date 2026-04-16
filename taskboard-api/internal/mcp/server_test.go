package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/models"
)

type stubExecutor struct {
	result any
	err    error
}

func (s stubExecutor) Call(_ context.Context, _ *models.Agent, _ string, _ map[string]any) (any, error) {
	return s.result, s.err
}

func TestInitializeReturnsToolCapability(t *testing.T) {
	server := NewServer(nil, stubExecutor{})
	resp := server.handleJSONRPC(context.Background(), &models.Agent{ID: "test-agent"}, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26"}`),
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected initialize response, got %#v", resp)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	caps := result["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok {
		t.Fatalf("expected tools capability in %#v", caps)
	}
}

func TestToolsListIncludesAllDomains(t *testing.T) {
	server := NewServer(nil, stubExecutor{})
	resp := server.handleJSONRPC(context.Background(), &models.Agent{ID: "test-agent"}, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tools/list response, got %#v", resp)
	}

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]tool)

	// Verify representative tools from each domain are present
	required := []string{"task_create", "task_list_visible", "task_cancel", "agent_whoami", "agent_list", "tag_list", "tag_create", "admin_audit"}
	toolSet := map[string]bool{}
	for _, tool := range tools {
		toolSet[tool.Name] = true
	}
	for _, name := range required {
		if !toolSet[name] {
			t.Errorf("expected tool %q in tools list (got %d tools total)", name, len(tools))
		}
	}

	if len(tools) < 30 {
		t.Errorf("expected at least 30 tools, got %d", len(tools))
	}
}

func TestTaskUpdateToolIncludesVisibilityField(t *testing.T) {
	server := NewServer(nil, stubExecutor{})
	resp := server.handleJSONRPC(context.Background(), &models.Agent{ID: "test-agent"}, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tools/list response, got %#v", resp)
	}

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]tool)
	for _, tool := range tools {
		if tool.Name != "task_update" {
			continue
		}
		properties := tool.InputSchema["properties"].(map[string]any)
		if _, ok := properties["visibility"]; !ok {
			t.Fatalf("expected task_update tool to expose visibility, got %#v", properties)
		}
		return
	}

	t.Fatalf("task_update not found in tools list")
}

func TestUnknownToolReturnsProtocolError(t *testing.T) {
	server := NewServer(nil, stubExecutor{err: errors.New("should not be used")})
	resp := server.handleJSONRPC(context.Background(), &models.Agent{ID: "test-agent"}, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nope","arguments":{}}`),
	})

	if resp == nil || resp.Error == nil {
		t.Fatalf("expected protocol error, got %#v", resp)
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected invalid params error, got %#v", resp.Error)
	}
}

func TestToolExecutionErrorReturnsToolErrorResult(t *testing.T) {
	server := NewServer(nil, stubExecutor{err: errors.New("boom")})
	resp := server.handleJSONRPC(context.Background(), &models.Agent{ID: "test-agent"}, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"task_get","arguments":{"task_id":"T-1"}}`),
	})

	if resp == nil || resp.Error != nil {
		t.Fatalf("expected tool error result, got %#v", resp)
	}

	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected tool error result, got %#v", result)
	}
}

func TestCatalogAndDocsRoutes(t *testing.T) {
	server := NewServer(nil, stubExecutor{})
	router := chi.NewRouter()
	server.Mount(router)

	req := httptest.NewRequest(http.MethodGet, "/catalog.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected catalog route to return 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"task_create"`) {
		t.Fatalf("expected catalog to include task_create, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected docs route to return 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/mcp/catalog.json") {
		t.Fatalf("expected docs page to reference catalog endpoint")
	}
}
