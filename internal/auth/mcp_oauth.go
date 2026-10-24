package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"atlassian-jira-conf-mcp/internal/vault"
)

type PendingClientAuth struct {
	SessionID     string
	ClientID      string
	RedirectURI   string
	State         string
	CodeChallenge string
	CreatedAt     time.Time
}

type IssuedAuthCode struct {
	Code          string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	CreatedAt     time.Time
}

type MCPOAuthHandler struct {
	baseURL       string
	authSvc       *Service
	db            *vault.DB
	clientAuthMu  sync.RWMutex
	pendingClient map[string]PendingClientAuth
	authCodeMu    sync.RWMutex
	issuedCodes   map[string]IssuedAuthCode
}

func NewMCPOAuthHandler(baseURL string, authSvc *Service, db *vault.DB) *MCPOAuthHandler {
	return &MCPOAuthHandler{
		baseURL:       strings.TrimRight(baseURL, "/"),
		authSvc:       authSvc,
		db:            db,
		pendingClient: make(map[string]PendingClientAuth),
		issuedCodes:   make(map[string]IssuedAuthCode),
	}
}

func (h *MCPOAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	// RFC 9728 Protected Resource Metadata
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.handleProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/sse", h.handleProtectedResourceMetadata)

	// RFC 8414 Authorization Server Metadata & OpenID Connect Discovery
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.handleAuthServerMetadata)
	mux.HandleFunc("GET /.well-known/openid-configuration", h.handleAuthServerMetadata)

	// RFC 7591 Dynamic Client Registration
	mux.HandleFunc("POST /oauth/register", h.handleClientRegister)

	// Authorization endpoint (called by MCP client / opened in browser)
	mux.HandleFunc("GET /oauth/authorize", h.handleAuthorize)

	// Token endpoint (called by MCP client to exchange code for access token)
	mux.HandleFunc("POST /oauth/token", h.handleToken)
}

func (h *MCPOAuthHandler) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data := map[string]interface{}{
		"resource": fmt.Sprintf("%s/sse", h.baseURL),
		"authorization_servers": []string{
			h.baseURL,
		},
		"scopes_supported":         DefaultScopes,
		"bearer_methods_supported": []string{"header", "query"},
		"resource_name":            "Atlassian Jira & Confluence MCP Server",
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (h *MCPOAuthHandler) handleAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data := map[string]interface{}{
		"issuer":                                h.baseURL,
		"authorization_endpoint":                fmt.Sprintf("%s/oauth/authorize", h.baseURL),
		"token_endpoint":                        fmt.Sprintf("%s/oauth/token", h.baseURL),
		"registration_endpoint":                 fmt.Sprintf("%s/oauth/register", h.baseURL),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post", "client_secret_basic"},
		"scopes_supported":                      DefaultScopes,
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (h *MCPOAuthHandler) handleClientRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)

	redirectURIs := []string{}
	if uris, ok := body["redirect_uris"].([]interface{}); ok {
		for _, u := range uris {
			if s, ok := u.(string); ok {
				redirectURIs = append(redirectURIs, s)
			}
		}
	}

	clientResp := map[string]interface{}{
		"client_id":     "mcp-client-" + base64.RawURLEncoding.EncodeToString(randomBytes(8)),
		"client_secret": "mcp-secret-" + base64.RawURLEncoding.EncodeToString(randomBytes(16)),
		"redirect_uris": redirectURIs,
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(clientResp)
}

func (h *MCPOAuthHandler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")

	if redirectURI == "" {
		http.Error(w, "Missing redirect_uri", http.StatusBadRequest)
		return
	}

	sessionID := base64.RawURLEncoding.EncodeToString(randomBytes(16))
	h.clientAuthMu.Lock()
	h.pendingClient[sessionID] = PendingClientAuth{
		SessionID:     sessionID,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		State:         state,
		CodeChallenge: codeChallenge,
		CreatedAt:     time.Now().UTC(),
	}
	h.clientAuthMu.Unlock()

	// Redirect into Atlassian login, preserving mcp_session
	loginURL := fmt.Sprintf("/auth/login?mcp_client_session=%s", sessionID)
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// CompleteClientAuth generates an authorization code for the pending MCP client session and returns the redirect URL.
func (h *MCPOAuthHandler) CompleteClientAuth(sessionID, userID string) (string, error) {
	h.clientAuthMu.Lock()
	pending, exists := h.pendingClient[sessionID]
	if !exists {
		h.clientAuthMu.Unlock()
		return "", fmt.Errorf("invalid or expired MCP client auth session")
	}
	delete(h.pendingClient, sessionID)
	h.clientAuthMu.Unlock()

	code := "mcp_code_" + base64.RawURLEncoding.EncodeToString(randomBytes(24))

	h.authCodeMu.Lock()
	h.issuedCodes[code] = IssuedAuthCode{
		Code:          code,
		UserID:        userID,
		RedirectURI:   pending.RedirectURI,
		CodeChallenge: pending.CodeChallenge,
		CreatedAt:     time.Now().UTC(),
	}
	h.authCodeMu.Unlock()

	sep := "?"
	if strings.Contains(pending.RedirectURI, "?") {
		sep = "&"
	}
	targetURL := fmt.Sprintf("%s%scode=%s&state=%s", pending.RedirectURI, sep, code, pending.State)
	return targetURL, nil
}

func (h *MCPOAuthHandler) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"invalid_request","error_description":"failed to parse form"}`, http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")

	if grantType != "authorization_code" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"unsupported_grant_type","error_description":"only authorization_code is supported"}`))
		return
	}

	h.authCodeMu.Lock()
	issued, exists := h.issuedCodes[code]
	if !exists {
		h.authCodeMu.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"invalid or expired authorization code"}`))
		return
	}
	delete(h.issuedCodes, code)
	h.authCodeMu.Unlock()

	// Verify PKCE if challenge was provided
	if issued.CodeChallenge != "" {
		hash := sha256.Sum256([]byte(codeVerifier))
		computedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
		if computedChallenge != issued.CodeChallenge && codeVerifier != issued.CodeChallenge {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"PKCE code_verifier does not match"}`))
			return
		}
	}

	user, err := h.db.GetUserByID(issued.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error","error_description":"user record missing"}`))
		return
	}

	resp := map[string]interface{}{
		"access_token": user.MCPAPIKey,
		"token_type":   "Bearer",
		"expires_in":   315360000,
		"scope":        strings.Join(DefaultScopes, " "),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, b)
	return b
}
