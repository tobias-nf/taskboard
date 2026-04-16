package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nearintents/taskboard-api/internal/auth"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/events"
	"github.com/nearintents/taskboard-api/internal/mcp"
	"github.com/nearintents/taskboard-api/internal/middleware"
	"github.com/nearintents/taskboard-api/internal/models"
	"github.com/nearintents/taskboard-api/internal/openapi"
	"github.com/nearintents/taskboard-api/internal/storage"
	"github.com/nearintents/taskboard-api/migrations"
)

func main() {
	port := getEnv("PORT", "4000")
	databaseURL := getEnv("DATABASE_URL", "postgres://hive:hive@localhost:5432/hive?sslmode=disable")

	// Connect to Postgres
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Run migrations (idempotent — safe on every startup)
	log.Println("Running migrations...")
	for _, m := range migrations.All {
		if _, err := pool.Exec(context.Background(), m.SQL); err != nil {
			log.Fatalf("Migration %s failed: %v", m.Name, err)
		}
	}
	log.Println("Migrations complete")

	store := db.NewStore(pool)
	adminAPIKey := getRequiredEnv(db.ConfiguredAdminAPIKeyEnv)
	if err := store.EnsureConfiguredAdmin(context.Background(), adminAPIKey); err != nil {
		log.Fatalf("Failed to ensure configured admin: %v", err)
	}
	log.Printf("Configured admin %s synced from %s", db.ConfiguredAdminID, db.ConfiguredAdminAPIKeyEnv)

	// Ensure env-managed agents (TASKBOARD_ENSURE_AGENT_1, _2, …)
	for i := 1; ; i++ {
		raw := os.Getenv(fmt.Sprintf("TASKBOARD_ENSURE_AGENT_%d", i))
		if raw == "" {
			break
		}
		cfg, err := parseAgentEnv(raw)
		if err != nil {
			log.Fatalf("TASKBOARD_ENSURE_AGENT_%d: %v", i, err)
		}
		if err := store.EnsureConfiguredAgent(context.Background(), cfg); err != nil {
			log.Fatalf("Failed to ensure agent from TASKBOARD_ENSURE_AGENT_%d: %v", i, err)
		}
		agentID, _, _, _ := db.ParseAPIKey(cfg.APIKey)
		log.Printf("Ensured agent %s (%s) from TASKBOARD_ENSURE_AGENT_%d", agentID, cfg.Type, i)
	}

	broker := events.NewBroker()

	// Rate limiters — relaxed in dev mode to avoid 429s during hot-reload
	rateLimit := 300
	if getBoolEnv("DEV_MODE", false) {
		rateLimit = 3000
	}
	authLimiter := middleware.NewRateLimiter(rateLimit, time.Minute)

	// Object storage (optional — gracefully degrades if not configured)
	var s3 *storage.S3Client
	if bucket := getEnv("S3_BUCKET", ""); bucket != "" {
		endpoint := getEnv("S3_ENDPOINT", "")
		cfg := storage.Config{
			Endpoint:         endpoint,
			Region:           getEnv("S3_REGION", getEnv("AWS_REGION", "eu-central-1")),
			AccessKey:        getEnv("S3_ACCESS_KEY", ""),
			SecretKey:        getEnv("S3_SECRET_KEY", ""),
			Bucket:           bucket,
			UseSSL:           getBoolEnv("S3_USE_SSL", endpoint == ""),
			ForcePathStyle:   getBoolEnv("S3_FORCE_PATH_STYLE", endpoint != ""),
			AutoCreateBucket: getBoolEnv("S3_AUTO_CREATE_BUCKET", endpoint != ""),
		}
		s3Client, err := storage.NewS3Client(cfg)
		if err != nil {
			log.Printf("WARNING: S3 storage not available: %v", err)
		} else {
			if err := s3Client.EnsureBucket(context.Background()); err != nil {
				log.Printf("WARNING: Failed to ensure S3 bucket: %v", err)
			} else {
				s3 = s3Client
				if cfg.Endpoint != "" {
					log.Printf("Connected to object storage via custom endpoint %s (bucket: %s)", cfg.Endpoint, cfg.Bucket)
				} else {
					log.Printf("Connected to AWS S3 bucket %s in %s using default credentials", cfg.Bucket, cfg.Region)
				}
			}
		}
	} else {
		log.Println("S3 storage not configured — attachments disabled")
	}

	// Google OAuth / session auth
	authCfg := auth.Config{
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:  getEnv("GOOGLE_REDIRECT_URI", "http://localhost:4000/auth/google/callback"),
		SessionSecret:      getEnv("SESSION_SECRET", "dev-secret-change-me"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000/app"),
		AllowedDomain:      getEnv("ALLOWED_EMAIL_DOMAIN", "near.foundation"),
		DevMode:            getBoolEnv("DEV_MODE", false),
	}
	authHandler := auth.NewHandler(authCfg, store)

	// MCP authenticator: accept both API keys and JWT session tokens
	mcpAuthenticator := func(ctx context.Context, token string) (*models.Agent, error) {
		if strings.HasPrefix(token, "hive_sk_") {
			return store.AuthenticateByAPIKey(ctx, token)
		}
		claims, err := authHandler.ValidateSessionToken(token)
		if err != nil {
			return nil, err
		}
		return store.GetAgentByID(ctx, claims.AgentID)
	}
	mcpServer := mcp.NewServer(mcpAuthenticator, mcp.NewExecutor(store, broker, s3))

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(corsMiddleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes (outside of authenticated middleware)
	r.Route("/auth", func(r chi.Router) {
		r.Get("/google", authHandler.HandleGoogleRedirect)
		r.Get("/google/callback", authHandler.HandleGoogleCallback)
		r.Get("/dev-login", authHandler.HandleDevLogin)
		r.Get("/logout", authHandler.HandleLogout)
	})

	r.Route("/mcp", func(r chi.Router) {
		mcpServer.Mount(r)
	})
	openapi.Mount(r, store, broker, authLimiter, s3, authHandler)

	// Server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("Taskboard API listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	return value
}

func getBoolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("WARNING: invalid boolean for %s=%q, using default %t", key, value, fallback)
		return fallback
	}
	return parsed
}

// parseAgentEnv parses "api_key|type[|email]".
// Also accepts legacy format "api_key|name|type|description[|email]" for backwards compatibility.
func parseAgentEnv(raw string) (db.AgentConfig, error) {
	parts := strings.SplitN(raw, "|", 5)
	if len(parts) < 2 {
		return db.AgentConfig{}, fmt.Errorf("expected at least 2 pipe-separated fields: api_key|type[|email]")
	}

	var cfg db.AgentConfig
	cfg.APIKey = strings.TrimSpace(parts[0])

	// Detect legacy format: if parts[2] is a valid type, parts[1] was the old name field
	if len(parts) >= 4 && (strings.TrimSpace(parts[2]) == "user" || strings.TrimSpace(parts[2]) == "service") {
		// Legacy: api_key|name|type|description[|email]
		cfg.Type = strings.TrimSpace(parts[2])
		if len(parts) >= 5 {
			cfg.Email = strings.TrimSpace(parts[4])
		}
	} else {
		// New: api_key|type[|email]
		cfg.Type = strings.TrimSpace(parts[1])
		if len(parts) >= 3 {
			cfg.Email = strings.TrimSpace(parts[2])
		}
	}

	if cfg.Type != "user" && cfg.Type != "service" {
		return db.AgentConfig{}, fmt.Errorf("type must be 'user' or 'service', got %q", cfg.Type)
	}
	return cfg, nil
}

func corsMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
