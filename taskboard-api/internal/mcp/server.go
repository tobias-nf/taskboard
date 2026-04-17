package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nearintents/taskboard-api/internal/models"
)

const serverVersion = "0.1.0"

type Authenticator func(context.Context, string) (*models.Agent, error)

type ToolExecutor interface {
	Call(context.Context, *models.Agent, string, map[string]any) (any, error)
}

type Server struct {
	authenticator Authenticator
	executor      ToolExecutor
	tools         []tool

	mu       sync.RWMutex
	sessions map[string]*session
}

type session struct {
	id       string
	agent    *models.Agent
	messages chan json.RawMessage
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func NewServer(authenticator Authenticator, executor ToolExecutor) *Server {
	return &Server{
		authenticator: authenticator,
		executor:      executor,
		tools:         taskboardTools(),
		sessions:      make(map[string]*session),
	}
}

func (s *Server) Mount(r chi.Router) {
	r.Get("/catalog.json", s.handleCatalog)
	r.Get("/docs", s.handleDocs)
	r.Get("/sse", s.handleSSE)
	r.Post("/sse", s.handleStreamableHTTP) // Streamable HTTP transport
	r.Post("/messages/{sessionID}", s.handleMessage)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"server": map[string]any{
			"name":    "taskboard",
			"version": serverVersion,
		},
		"transport": map[string]any{
			"sse":      "/mcp/sse",
			"messages": "/mcp/messages/{sessionID}",
		},
		"tools": s.tools,
	})
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Taskboard MCP Docs</title>
    <style>
      :root { color-scheme: light; --bg: #f5f5ef; --panel: #fffdf7; --ink: #181513; --muted: #6f655d; --line: #ddd2c3; --accent: #0f766e; --chip: #efe5d8; }
      body { margin: 0; font-family: ui-sans-serif, system-ui, sans-serif; background: linear-gradient(180deg, #f3efe7 0%, #f8f6f1 100%); color: var(--ink); }
      main { max-width: 980px; margin: 0 auto; padding: 40px 20px 72px; }
      h1, h2, h3, p { margin-top: 0; }
      .hero { background: var(--panel); border: 1px solid var(--line); border-radius: 20px; padding: 28px; box-shadow: 0 20px 40px rgba(24, 21, 19, 0.06); }
      .meta { display: flex; gap: 12px; flex-wrap: wrap; margin-top: 16px; }
      .chip { background: var(--chip); color: var(--ink); border-radius: 999px; padding: 6px 10px; font-size: 13px; }
      .tools { margin-top: 24px; display: grid; gap: 16px; }
      .tool { background: var(--panel); border: 1px solid var(--line); border-radius: 16px; padding: 18px; }
      pre { overflow: auto; background: #161311; color: #f6f0e8; padding: 14px; border-radius: 12px; font-size: 13px; }
      code { font-family: ui-monospace, SFMono-Regular, monospace; }
      .muted { color: var(--muted); }
      a { color: var(--accent); }
    </style>
  </head>
  <body>
    <main>
      <section class="hero">
        <h1>Taskboard MCP Docs</h1>
        <p class="muted">Live documentation for the embedded Taskboard MCP server, powered by the same tool catalog returned by <code>tools/list</code>.</p>
        <div class="meta">
          <span class="chip">Catalog: <a href="/mcp/catalog.json">/mcp/catalog.json</a></span>
          <span class="chip">SSE: <code>/mcp/sse</code></span>
          <span class="chip">Messages: <code>/mcp/messages/{sessionID}</code></span>
        </div>
      </section>
      <section class="tools" id="tools"></section>
    </main>
    <script>
      async function load() {
        const res = await fetch('/mcp/catalog.json');
        const data = await res.json();
        const tools = document.getElementById('tools');
        for (const tool of data.tools) {
          const card = document.createElement('section');
          card.className = 'tool';
          card.innerHTML =
            '<h2>' + tool.name + '</h2>' +
            '<p>' + (tool.description || '') + '</p>' +
            '<h3>Input Schema</h3>' +
            '<pre><code>' + JSON.stringify(tool.inputSchema || {}, null, 2) + '</code></pre>';
          tools.appendChild(card);
        }
      }
      load().catch((err) => {
        document.getElementById('tools').innerHTML = '<p>Failed to load catalog: ' + err.message + '</p>';
      });
    </script>
  </body>
</html>`)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if s.authenticator == nil {
		http.Error(w, "authentication is not configured", http.StatusInternalServerError)
		return
	}

	apiKey, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	agent, err := s.authenticator(r.Context(), apiKey)
	if err != nil {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}
	if !agent.Active {
		http.Error(w, "Agent is not active", http.StatusForbidden)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sess := &session{
		id:       randomID(),
		agent:    agent,
		messages: make(chan json.RawMessage, 32),
	}
	s.storeSession(sess)
	defer s.deleteSession(sess.id)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpointURL(r, sess.id))
	flusher.Flush()

	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case msg, ok := <-sess.messages:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// handleStreamableHTTP implements the MCP Streamable HTTP transport.
// Accepts POST with JSON-RPC body, authenticates via Bearer token,
// and returns the response as JSON. No SSE session needed.
func (s *Server) handleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	if s.authenticator == nil {
		http.Error(w, "authentication is not configured", http.StatusInternalServerError)
		return
	}

	apiKey, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	agent, err := s.authenticator(r.Context(), apiKey)
	if err != nil {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}
	if !agent.Active {
		http.Error(w, "Agent is not active", http.StatusForbidden)
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON-RPC payload", http.StatusBadRequest)
		return
	}

	resp := s.handleJSONRPC(r.Context(), agent, req)
	if resp == nil {
		// Notifications (no ID) don't get a response
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	sessID := chi.URLParam(r, "sessionID")
	sess, ok := s.getSession(sessID)
	if !ok {
		http.Error(w, "Unknown MCP session", http.StatusNotFound)
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON-RPC payload", http.StatusBadRequest)
		return
	}

	resp := s.handleJSONRPC(r.Context(), sess.agent, req)
	if resp != nil {
		payload, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		select {
		case sess.messages <- payload:
		case <-r.Context().Done():
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleJSONRPC(ctx context.Context, agent *models.Agent, req jsonRPCRequest) *jsonRPCResponse {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, -32600, "Invalid JSON-RPC version")
	}

	switch req.Method {
	case "initialize":
		var params initializeParams
		_ = json.Unmarshal(req.Params, &params)
		version := params.ProtocolVersion
		if version == "" {
			version = "2025-03-26"
		}
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": version,
				"capabilities": map[string]any{
					"tools": map[string]any{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]any{
					"name":    "taskboard",
					"version": serverVersion,
				},
				"instructions": "Taskboard tools are available over the embedded MCP endpoint.",
			},
		}
	case "notifications/initialized":
		return nil
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": s.tools,
			},
		}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, -32602, "Invalid tool call arguments")
		}
		if params.Name == "" {
			return s.errorResponse(req.ID, -32602, "Tool name is required")
		}
		if !s.hasTool(params.Name) {
			return s.errorResponse(req.ID, -32602, fmt.Sprintf("unknown tool: %s", params.Name))
		}
		result, err := s.executor.Call(ctx, agent, params.Name, params.Arguments)
		if err != nil {
			if errors.Is(err, ErrUnknownTool) {
				return s.errorResponse(req.ID, -32602, err.Error())
			}
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": err.Error()},
					},
					"isError": true,
				},
			}
		}
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": stringify(result)},
				},
				"isError": false,
			},
		}
	default:
		return s.errorResponse(req.ID, -32601, "Method not found")
	}
}

func (s *Server) errorResponse(id json.RawMessage, code int, message string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Server) storeSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.id] = sess
}

func (s *Server) getSession(id string) (*session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *Server) deleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		delete(s.sessions, id)
		close(sess.messages)
	}
}

func (s *Server) hasTool(name string) bool {
	for _, tool := range s.tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func bearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if token == header || token == "" {
		return "", false
	}
	return token, true
}

func messageEndpointURL(r *http.Request, sessionID string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return fmt.Sprintf("%s://%s/mcp/messages/%s", scheme, r.Host, sessionID)
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("failed to read random bytes for session id: %v", err)
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func stringify(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}
