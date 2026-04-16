package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nearintents/taskboard-api/internal/auth"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/models"
)

type contextKey string

const AgentContextKey contextKey = "agent"

func GetAgent(ctx context.Context) *models.Agent {
	if agent, ok := ctx.Value(AgentContextKey).(*models.Agent); ok {
		return agent
	}
	return nil
}

// BearerAuth accepts both API keys (hive_sk_*) and JWT session tokens.
func BearerAuth(store *db.Store, authHandler *auth.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing_auth", "Authorization header required")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				writeError(w, http.StatusUnauthorized, "invalid_auth", "Bearer token required")
				return
			}

			var agent *models.Agent
			var err error

			if strings.HasPrefix(token, "hive_sk_") {
				// API key authentication
				agent, err = store.AuthenticateByAPIKey(r.Context(), token)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "invalid_key", "Invalid API key")
					return
				}
			} else {
				// JWT session token
				claims, claimsErr := authHandler.ValidateSessionToken(token)
				if claimsErr != nil {
					writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired session token")
					return
				}
				agent, err = store.GetAgentByID(r.Context(), claims.AgentID)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "unknown_agent", "Agent not found")
					return
				}
			}

			if !agent.Active {
				writeError(w, http.StatusForbidden, "inactive", "Agent is not active")
				return
			}

			// X-Act-As: allow authenticated agents to impersonate another agent.
			// The caller must be active; the target must exist and be active.
			if actAs := r.Header.Get("X-Act-As"); actAs != "" && actAs != agent.ID {
				target, targetErr := store.GetAgentByID(r.Context(), actAs)
				if targetErr != nil {
					writeError(w, http.StatusBadRequest, "invalid_act_as", "Target agent not found")
					return
				}
				if !target.Active {
					writeError(w, http.StatusForbidden, "inactive_target", "Target agent is not active")
					return
				}
				agent = target
			}

			go store.UpdateLastSeen(context.Background(), agent.ID)

			ctx := context.WithValue(r.Context(), AgentContextKey, agent)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
