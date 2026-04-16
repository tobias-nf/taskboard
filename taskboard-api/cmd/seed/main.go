package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nearintents/taskboard-api/internal/db"
	"github.com/nearintents/taskboard-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://hive:hive@localhost:5432/hive?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}

	store := db.NewStore(pool)
	ctx := context.Background()

	// --- Drop existing data (order matters due to FK constraints) ---
	fmt.Println("Dropping existing data...")
	tables := []string{
		"task_activity", "task_references", "task_attachments",
		"task_owed_to", "task_mentions", "task_tags",
		"tasks", "tags", "admin_audit", "agents",
	}
	for _, t := range tables {
		if _, err := pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", t)); err != nil {
			log.Fatalf("Failed to clear %s: %v", t, err)
		}
		fmt.Printf("  cleared %s\n", t)
	}
	// Reset task sequence
	if _, err := pool.Exec(ctx, "ALTER SEQUENCE task_seq RESTART WITH 1"); err != nil {
		log.Printf("  warning: could not reset task_seq: %v", err)
	}

	fmt.Println("\nSeeding Taskboard database...")

	// --- Admin agent (API key auth, configured via env in production) ---
	// The server's EnsureConfiguredAdmin hardcodes "hive-admin" as the admin ID,
	// so the seed must use the same ID.
	adminID := "hive-admin"
	adminSecret := getEnv("TASKBOARD_SEED_ADMIN_API_KEY_SECRET", "localdev")
	adminKey := fmt.Sprintf("hive_sk_%s_%s", adminID, adminSecret)
	adminPrefix := fmt.Sprintf("hive_sk_%s_", adminID)
	adminHash := mustHash(adminSecret)

	if err := store.CreateAgent(ctx, &models.Agent{
		ID: adminID, Type: "admin",
		Email: strPtr("admin@near.foundation"),
	}, adminHash, adminPrefix, true, nil); err != nil {
		log.Fatalf("Failed to create admin: %v", err)
	}
	fmt.Printf("  agent %s: created (admin)\n", adminID)

	// --- Google OAuth personal agents ---
	type googleAgent struct {
		GoogleSub string
		Email     string
		Name      string
		SlackID   string
	}

	googleAgents := []googleAgent{
		{GoogleSub: "google-alice", Email: "alice@near.foundation", Name: "Alice Johnson", SlackID: "U04ALICE001"},
		{GoogleSub: "google-bob", Email: "bob@near.foundation", Name: "Bob Smith", SlackID: "U04BOB00002"},
		{GoogleSub: "google-tobias", Email: "tobias.holenstein@near.foundation", Name: "Tobias Holenstein", SlackID: "U0AJ1HULDL3"},
	}

	apiKeys := map[string]string{adminID: adminKey}

	for _, ga := range googleAgents {
		agent, _, err := store.FindOrCreateAgentByGoogle(ctx, ga.GoogleSub, ga.Email, ga.Name)
		if err != nil {
			log.Fatalf("Failed to create agent %s: %v", ga.Email, err)
		}
		// Set slack_id
		if _, err := pool.Exec(ctx,
			`UPDATE agents SET slack_id = $2, updated_at = NOW() WHERE id = $1`,
			agent.ID, ga.SlackID); err != nil {
			log.Printf("  warning: failed to set slack_id for %s: %v", agent.ID, err)
		}
		// Generate an API key for each Google agent (for MCP / testing)
		key, err := store.RotateAPIKey(ctx, agent.ID)
		if err != nil {
			log.Printf("  warning: failed to generate API key for %s: %v", agent.ID, err)
		} else {
			apiKeys[agent.ID] = key
		}
		fmt.Printf("  agent %s (%s): created via Google OAuth\n", agent.ID, ga.Email)
	}

	// --- Meeting tracker (user agent with API key, no Google OAuth) ---
	svcSecret := "svc-meeting-secret"
	svcKey := fmt.Sprintf("hive_sk_meeting-tracker_%s", svcSecret)
	svcPrefix := "hive_sk_meeting-tracker_"
	svcHash := mustHash(svcSecret)

	if err := store.CreateAgent(ctx, &models.Agent{
		ID: "meeting-tracker", Type: "user",
	}, svcHash, svcPrefix, true, strPtr(adminID)); err != nil {
		log.Fatalf("Failed to create meeting-tracker: %v", err)
	}
	apiKeys["meeting-tracker"] = svcKey
	fmt.Printf("  agent meeting-tracker: created (user)\n")

	// --- Tags ---
	tagDefs := []struct {
		Name  string
		Color string
	}{
		{"legal", "#3B82F6"},
		{"compliance", "#EF4444"},
		{"engineering", "#10B981"},
		{"contracts", "#F59E0B"},
		{"meeting-action", "#8B5CF6"},
		{"urgent-review", "#DC2626"},
	}

	for _, td := range tagDefs {
		tag := &models.Tag{Name: td.Name, Color: strPtr(td.Color), CreatedBy: adminID}
		if err := store.CreateTag(ctx, tag); err != nil {
			fmt.Printf("  tag %s: error: %v\n", td.Name, err)
		} else {
			fmt.Printf("  tag %s: created\n", td.Name)
		}
	}

	// --- Tasks ---
	type taskDef struct {
		Title       string
		Description *string
		CreatedBy   string
		AssignedTo  *string
		Visibility  string
		Priority    string
		Deadline    *time.Time
		Status      string
		Tags        []string
		OwedTo      []string
	}

	parseTime := func(s string) *time.Time {
		t, _ := time.Parse(time.RFC3339, s)
		return &t
	}

	taskDefs := []taskDef{
		{Title: "Review partnership agreement — NEAR Foundation",
			Description: strPtr("Review and redline the partnership agreement draft from external counsel.\n\n## Playbook\n\n### Phase 1: Initial Review\n- [x] Read through partnership agreement draft\n- [x] Identify key clauses and obligations\n- [ ] Compare against standard template\n- [ ] Flag deviations from standard terms\n\n### Phase 2: Legal Analysis\n- [ ] Review indemnification clauses\n- [ ] Check IP assignment terms\n- [ ] Verify termination provisions"),
			CreatedBy: "alice", AssignedTo: strPtr("alice"),
			Priority: "standard", Deadline: parseTime("2026-04-25T23:59:59Z"),
			Status: "in_progress",
			Tags: []string{"legal", "contracts"}, OwedTo: []string{"tobias.holenstein"}},
		{Title: "Draft NDA for vendor onboarding",
			Description: strPtr("Prepare standard NDA for new infrastructure vendor."),
			CreatedBy: "alice", AssignedTo: strPtr("bob"),
			Priority: "standard", Deadline: parseTime("2026-04-28T23:59:59Z"),
			Status: "pending", Tags: []string{"legal", "contracts"}},
		{Title: "Review compliance policy update",
			Description: strPtr("Review updated compliance policy to reflect new NEAR Intents procedures.\n\n1. Compare against current policy\n2. Identify gaps\n3. Draft amendments"),
			CreatedBy: "tobias.holenstein", AssignedTo: strPtr("bob"),
			Priority: "standard", Deadline: parseTime("2026-04-30T23:59:59Z"),
			Status: "in_progress", Tags: []string{"compliance"}, OwedTo: []string{"tobias.holenstein"}},
		{Title: "Investigate failed transactions on mywallet.near",
			Description: strPtr("User reports failed transactions. Check wallet status and respond."),
			CreatedBy: "bob", AssignedTo: strPtr("bob"),
			Priority: "urgent", Deadline: parseTime("2026-04-18T23:59:59Z"),
			Status: "in_progress", Tags: []string{"compliance", "urgent-review"}},
		{Title: "Action item: Update onboarding documentation",
			Description: strPtr("From Weekly Sync — update the team onboarding docs to include Taskboard setup instructions."),
			CreatedBy: "meeting-tracker", AssignedTo: strPtr("tobias.holenstein"),
			Priority: "low", Deadline: parseTime("2026-04-25T23:59:59Z"),
			Status: "pending", Tags: []string{"meeting-action"}},
		{Title: "Action item: Schedule external counsel call",
			Description: strPtr("From Weekly Legal Sync — schedule call with outside counsel about cross-border requirements."),
			CreatedBy: "meeting-tracker", AssignedTo: strPtr("alice"),
			Priority: "standard", Deadline: parseTime("2026-04-21T23:59:59Z"),
			Status: "in_progress", Tags: []string{"meeting-action", "legal"}},
		{Title: "Prepare board presentation on compliance metrics",
			Description: strPtr("Compile Q1 compliance metrics and prepare slides for board review."),
			CreatedBy: "tobias.holenstein", AssignedTo: strPtr("bob"),
			Priority: "urgent", Status: "blocked",
			Tags: []string{"compliance"}, OwedTo: []string{"tobias.holenstein"}},
		{Title: "Complete vendor security assessment",
			Description: strPtr("Completed security review for cloud infrastructure vendor. All checks passed."),
			CreatedBy: "bob", AssignedTo: strPtr("bob"),
			Priority: "urgent", Status: "completed", Tags: []string{"compliance"}},
		{Title: "Finalize data processing agreement",
			Description: strPtr("DPA for cloud vendor finalized and signed."),
			CreatedBy: "alice", AssignedTo: strPtr("alice"),
			Priority: "standard", Status: "completed", Tags: []string{"legal", "contracts"}},
		{Title: "Fix COMPLIANCE_HOLD error code in 1Click API",
			Description: strPtr("Implement the COMPLIANCE_HOLD error code for blocked addresses in the 1Click API."),
			CreatedBy: "tobias.holenstein", AssignedTo: strPtr("tobias.holenstein"),
			Priority: "standard", Deadline: parseTime("2026-04-22T23:59:59Z"),
			Status: "in_progress", Tags: []string{"engineering"}},
		{Title: "Set up production AWS infrastructure",
			Description: strPtr("Provision ECS Fargate tasks, RDS Postgres, S3 bucket, CloudFront distribution, and Route 53 records."),
			CreatedBy: adminID, AssignedTo: strPtr("tobias.holenstein"),
			Priority: "standard", Deadline: parseTime("2026-04-25T23:59:59Z"),
			Status: "pending", Tags: []string{"engineering"}},
		{Title: "Configure Google OAuth for production",
			Description: strPtr("Create Google Cloud OAuth client for production deployment. Set redirect URIs and allowed domains."),
			CreatedBy: adminID, AssignedTo: strPtr("tobias.holenstein"),
			Priority: "standard", Deadline: parseTime("2026-04-22T23:59:59Z"),
			Status: "in_progress", Tags: []string{"engineering"}},
		{Title: "Action item: Define data retention policy",
			Description: strPtr("From Weekly Sync — draft data retention policy for task and audit data."),
			CreatedBy: "meeting-tracker", AssignedTo: strPtr(adminID),
			Priority: "standard", Deadline: parseTime("2026-04-28T23:59:59Z"),
			Status: "pending", Tags: []string{"meeting-action", "compliance"}},
		{Title: "Confidential: Review executive compensation package",
			Description: strPtr("Review and approve updated executive compensation details. Private to involved parties."),
			CreatedBy: "alice", AssignedTo: strPtr("tobias.holenstein"),
			Visibility: "private", Priority: "standard",
			Status: "pending", Tags: []string{"legal"},
			OwedTo: []string{"alice"}},
	}

	taskIDs := []string{}
	for _, td := range taskDefs {
		visibility := td.Visibility
		if visibility == "" {
			visibility = "public"
		}

		task := &models.Task{
			Title: td.Title, Description: td.Description,
			CreatedBy: td.CreatedBy, AssignedTo: td.AssignedTo,
			Visibility: visibility,
			Priority:   td.Priority, Deadline: td.Deadline,
		}
		id, err := store.CreateTask(ctx, task)
		if err != nil {
			fmt.Printf("  task '%s': error: %v\n", td.Title, err)
			taskIDs = append(taskIDs, "")
			continue
		}
		taskIDs = append(taskIDs, id)

		// Set status
		if td.Status != "pending" {
			switch td.Status {
			case "in_progress":
				store.UpdateTaskStatus(ctx, id, "in_progress")
			case "blocked":
				store.UpdateTaskStatus(ctx, id, "in_progress")
				store.UpdateTaskStatus(ctx, id, "blocked")
			case "completed":
				store.UpdateTaskStatus(ctx, id, "in_progress")
				store.UpdateTaskStatus(ctx, id, "completed")
			}
		}

		for _, tagName := range td.Tags {
			tag, err := store.GetTagByName(ctx, tagName)
			if err != nil {
				continue
			}
			store.AddTaskTag(ctx, id, tag.ID, td.CreatedBy)
		}

		for _, agentID := range td.OwedTo {
			store.AddOwedTo(ctx, id, agentID, td.CreatedBy)
		}

		store.AddActivity(ctx, &models.TaskActivity{
			TaskID: id, Type: "created", Actor: td.CreatedBy, ActorType: "agent",
			Summary: strPtr("Task created"),
		})

		fmt.Printf("  task %s: %s [%s] (%s)\n", id, td.Title[:min(50, len(td.Title))], td.Status, visibility)
	}

	// --- References ---
	if len(taskIDs) > 0 && taskIDs[0] != "" {
		refs := []models.TaskReference{
			{TaskID: taskIDs[0], Type: "origin", Source: "gmail", Title: "Email: Partnership agreement draft from external counsel", CreatedBy: "alice"},
			{TaskID: taskIDs[0], Type: "related", Source: "slack", Title: "Slack: #legal discussion", URL: strPtr("https://slack.com/archives/C0123456789"), CreatedBy: "alice"},
		}
		for _, ref := range refs {
			if err := store.AddReference(ctx, &ref); err != nil {
				fmt.Printf("  reference: error: %v\n", err)
			}
		}
	}

	// --- Audit entries ---
	store.LogAudit(ctx, "agent_approved", adminID, strPtr("agent"), strPtr("alice"), nil)
	store.LogAudit(ctx, "agent_approved", adminID, strPtr("agent"), strPtr("bob"), nil)
	store.LogAudit(ctx, "agent_approved", adminID, strPtr("agent"), strPtr("tobias.holenstein"), nil)

	// Print API keys
	fmt.Println("\n--- API Keys ---")
	for id, key := range apiKeys {
		fmt.Printf("  %-25s %s\n", id+":", key)
	}

	fmt.Println("\n--- Dev Login URLs ---")
	fmt.Println("  Alice:  http://localhost:4000/auth/dev-login?email=alice@near.foundation")
	fmt.Println("  Bob:    http://localhost:4000/auth/dev-login?email=bob@near.foundation")
	fmt.Println("  Tobias: http://localhost:4000/auth/dev-login?email=tobias.holenstein@near.foundation")
	fmt.Println("  Admin:  (use API key — admin is not a Google OAuth account)")

	fmt.Println("\nSeed complete!")
}

func strPtr(s string) *string { return &s }

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustHash(secret string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash: %v", err)
	}
	return string(hashed)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
