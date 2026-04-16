package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nearintents/taskboard-api/internal/db"
)

// Config holds all auth-related configuration.
type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string // e.g. "http://localhost:4000/auth/google/callback"
	SessionSecret      string // HMAC key for JWT signing
	FrontendURL        string // e.g. "http://localhost:3000/app"
	AllowedDomain      string // e.g. "near.foundation"
	DevMode            bool   // enables /auth/dev-login
}

// Handler provides HTTP handlers for Google OAuth and session management.
type Handler struct {
	cfg   Config
	store *db.Store
	oauth *oauth2.Config
}

func NewHandler(cfg Config, store *db.Store) *Handler {
	return &Handler{
		cfg:   cfg,
		store: store,
		oauth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURI,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// SessionClaims are the JWT claims for dashboard sessions.
type SessionClaims struct {
	AgentID string `json:"agent_id"`
	Email   string `json:"email"`
	jwt.RegisteredClaims
}

// --- Handlers ---

// HandleGoogleRedirect starts the OAuth flow by redirecting to Google.
func (h *Handler) HandleGoogleRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Store state in a short-lived cookie for CSRF protection
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	url := h.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", h.cfg.AllowedDomain))
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// googleUserInfo is the response from Google's userinfo endpoint.
type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	HD            string `json:"hd"` // hosted domain
}

// HandleGoogleCallback exchanges the auth code, validates the user, and redirects to the frontend.
func (h *Handler) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}
	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/auth",
		MaxAge: -1,
	})

	// Check for error from Google
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("Google OAuth error: %s", errParam)
		http.Redirect(w, r, h.cfg.FrontendURL+"?auth_error="+errParam, http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("OAuth code exchange failed: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Fetch user info from Google
	client := h.oauth.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		log.Printf("Failed to fetch Google user info: %v", err)
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		log.Printf("Failed to decode Google user info: %v", err)
		http.Error(w, "failed to parse user info", http.StatusInternalServerError)
		return
	}

	// Validate domain
	if !userInfo.EmailVerified {
		http.Redirect(w, r, h.cfg.FrontendURL+"?auth_error=email_not_verified", http.StatusTemporaryRedirect)
		return
	}
	emailDomain := ""
	if idx := strings.LastIndex(userInfo.Email, "@"); idx > 0 {
		emailDomain = userInfo.Email[idx+1:]
	}
	if emailDomain != h.cfg.AllowedDomain {
		http.Redirect(w, r, h.cfg.FrontendURL+"?auth_error=domain_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	// Find or create agent
	agent, created, err := h.store.FindOrCreateAgentByGoogle(r.Context(), userInfo.Sub, userInfo.Email, userInfo.Name)
	if err != nil {
		log.Printf("Failed to find/create agent for %s: %v", userInfo.Email, err)
		http.Error(w, "account creation failed", http.StatusInternalServerError)
		return
	}
	if created {
		log.Printf("Created new agent %s for %s via Google OAuth", agent.ID, userInfo.Email)
	}

	if !agent.Active {
		http.Redirect(w, r, h.cfg.FrontendURL+"?auth_error=account_inactive", http.StatusTemporaryRedirect)
		return
	}

	// Issue session JWT
	sessionToken, err := h.generateSessionToken(agent.ID, userInfo.Email)
	if err != nil {
		log.Printf("Failed to generate session token: %v", err)
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	// Redirect to frontend with token
	http.Redirect(w, r, h.cfg.FrontendURL+"/auth/callback?token="+sessionToken, http.StatusTemporaryRedirect)
}

// HandleDevLogin creates a session for a test user without Google. Only available when DEV_MODE=true.
func (h *Handler) HandleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.DevMode {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		email = "dev@" + h.cfg.AllowedDomain
	}

	// Validate domain even in dev mode
	emailDomain := ""
	if idx := strings.LastIndex(email, "@"); idx > 0 {
		emailDomain = email[idx+1:]
	}
	if emailDomain != h.cfg.AllowedDomain {
		http.Error(w, "domain not allowed", http.StatusBadRequest)
		return
	}

	// Find or create a dev agent
	devSub := "dev-" + email // fake google sub for dev
	agent, _, err := h.store.FindOrCreateAgentByGoogle(r.Context(), devSub, email, "Dev User")
	if err != nil {
		log.Printf("Dev login failed: %v", err)
		http.Error(w, "dev login failed", http.StatusInternalServerError)
		return
	}

	sessionToken, err := h.generateSessionToken(agent.ID, email)
	if err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.cfg.FrontendURL+"/auth/callback?token="+sessionToken, http.StatusTemporaryRedirect)
}

// HandleLogout is a no-op redirect — the frontend clears the token client-side.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusTemporaryRedirect)
}

// --- Session token helpers ---

func (h *Handler) generateSessionToken(agentID, email string) (string, error) {
	now := time.Now()
	claims := SessionClaims{
		AgentID: agentID,
		Email:   email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)), // 7 days
			Issuer:    "taskboard",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.SessionSecret))
}

// ValidateSessionToken parses and validates a JWT session token.
func (h *Handler) ValidateSessionToken(tokenStr string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &SessionClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.cfg.SessionSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*SessionClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
