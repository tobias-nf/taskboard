package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/middleware"
)

func TestMountServesDocsAndOpenAPISpec(t *testing.T) {
	router := chi.NewRouter()
	Mount(router, nil, nil, middleware.NewRateLimiter(1, time.Minute), nil)

	docsReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsRec := httptest.NewRecorder()
	router.ServeHTTP(docsRec, docsReq)

	if docsRec.Code != http.StatusOK {
		t.Fatalf("expected /docs to return 200, got %d", docsRec.Code)
	}

	specReq := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	specRec := httptest.NewRecorder()
	router.ServeHTTP(specRec, specReq)

	if specRec.Code != http.StatusOK {
		t.Fatalf("expected /openapi.json to return 200, got %d", specRec.Code)
	}

	spec := specRec.Body.String()
	for _, want := range []string{
		`"/api/v1/agents/me"`,
		`"/api/v1/tags"`,
		`"/api/v1/tasks/{id}/owed-to"`,
		`"/api/v1/tasks/{id}/mentions"`,
		`"multipart/form-data"`,
		`"application/octet-stream"`,
		`"text/event-stream"`,
		`"bearerAuth"`,
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("expected OpenAPI spec to contain %q", want)
		}
	}
}

func TestRegisterLegacyOperationPreservesContextFromMiddleware(t *testing.T) {
	type contextKey string

	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("Test API", "1.0.0"))
	group := huma.NewGroup(api, "/api/v1")
	group.UseMiddleware(adaptMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), contextKey("agent-id"), "sandbox-alpha")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}))
	group.UseMiddleware(adaptMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Context().Value(contextKey("agent-id")); got != "sandbox-alpha" {
				t.Fatalf("expected prior middleware context to survive, got %v", got)
			}
			next.ServeHTTP(w, r)
		})
	}))

	registerLegacyOperation[struct{}, statusOnlyResponse](group, huma.Operation{
		OperationID: "context-check",
		Method:      http.MethodGet,
		Path:        "/context-check",
	}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": r.Context().Value(contextKey("agent-id")).(string),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/context-check", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "sandbox-alpha" {
		t.Fatalf("expected middleware context to reach handler, got %q", body["status"])
	}
}
