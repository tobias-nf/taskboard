package db

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateAPIKeyFromSecretRoundTrip(t *testing.T) {
	key, hash, prefix, err := GenerateAPIKeyFromSecret("agent-1", "super-secret")
	if err != nil {
		t.Fatalf("GenerateAPIKeyFromSecret returned error: %v", err)
	}

	if want := "hive_sk_agent-1_super-secret"; key != want {
		t.Fatalf("expected key %q, got %q", want, key)
	}
	if want := "hive_sk_agent-1_"; prefix != want {
		t.Fatalf("expected prefix %q, got %q", want, prefix)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("super-secret")); err != nil {
		t.Fatalf("expected hash to verify secret, got %v", err)
	}

	agentID, secret, gotPrefix, err := ParseAPIKey(key)
	if err != nil {
		t.Fatalf("ParseAPIKey returned error: %v", err)
	}
	if agentID != "agent-1" || secret != "super-secret" || gotPrefix != prefix {
		t.Fatalf("unexpected parse result: agentID=%q secret=%q prefix=%q", agentID, secret, gotPrefix)
	}
}

func TestGenerateAPIKeyFromSecretRejectsEmptySecret(t *testing.T) {
	_, _, _, err := GenerateAPIKeyFromSecret("agent-1", "")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty-secret error, got %v", err)
	}
}

func TestParseAPIKeyRejectsInvalidFormat(t *testing.T) {
	for _, key := range []string{
		"",
		"nope",
		"hive-key-agent-secret",
		"hive_sk_onlythree",
	} {
		t.Run(key, func(t *testing.T) {
			_, _, _, err := ParseAPIKey(key)
			if err == nil {
				t.Fatalf("expected error for %q", key)
			}
		})
	}
}

func TestValidateStatusTransition(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		next            string
		isOwnerOrAdmin  bool
		wantErrContains string
	}{
		{name: "draft to pending", current: "draft", next: "pending"},
		{name: "draft to cancelled", current: "draft", next: "cancelled"},
		{name: "draft to in_progress blocked", current: "draft", next: "in_progress", wantErrContains: "invalid transition"},
		{name: "pending to in_progress", current: "pending", next: "in_progress"},
		{name: "pending to completed", current: "pending", next: "completed"},
		{name: "pending to blocked", current: "pending", next: "blocked"},
		{name: "in progress to blocked", current: "in_progress", next: "blocked"},
		{name: "review to completed", current: "review", next: "completed"},
		{name: "reopen denied", current: "completed", next: "pending", wantErrContains: "only task creator or admin can reopen"},
		{name: "reopen allowed for owner", current: "failed", next: "pending", isOwnerOrAdmin: true},
		{name: "unknown status", current: "archived", next: "pending", wantErrContains: "cannot transition from terminal status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusTransition(tt.current, tt.next, tt.isOwnerOrAdmin)
			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("expected transition to be allowed, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErrContains)
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrContains, err)
			}
		})
	}
}
