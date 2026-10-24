package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlassian-jira-conf-mcp/internal/vault"
)

func TestPKCEGeneration(t *testing.T) {
	v, c, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("unexpected error generating PKCE: %v", err)
	}
	if len(v) < 32 || len(c) < 32 {
		t.Fatalf("pkce strings too short: v=%s c=%s", v, c)
	}
}

func setupTestAuthService(t *testing.T, roundTripper http.RoundTripper) (*Service, *vault.DB) {
	tmpDir := t.TempDir()
	enc, _ := vault.NewEncryptor(bytes.Repeat([]byte("a"), 32))
	db, err := vault.OpenDB(filepath.Join(tmpDir, "auth_test.db"), enc)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	httpClient := &http.Client{
		Transport: roundTripper,
		Timeout:   5 * time.Second,
	}

	svc := NewService(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		BaseURL:      "http://localhost:8080",
	}, db, httpClient)

	return svc, db
}

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestOAuthFlowAndTokenRefresh(t *testing.T) {
	mock := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			switch {
			case req.URL.Path == "/oauth/token":
				// Token exchange or refresh
				respData := map[string]interface{}{
					"access_token":  "mock-access-token-123",
					"refresh_token": "mock-refresh-token-456",
					"expires_in":    3600,
					"token_type":    "Bearer",
				}
				b, _ := json.Marshal(respData)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil

			case req.URL.Path == "/me":
				profile := UserProfile{
					AccountID: "acc-123",
					Email:     "sarah@acme.com",
					Name:      "Sarah Connor",
				}
				b, _ := json.Marshal(profile)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil

			case req.URL.Path == "/oauth/token/accessible-resources":
				resources := []AccessibleResource{
					{
						ID:     "cloud-uuid-1",
						Name:   "Acme Production",
						URL:    "https://acme.atlassian.net",
						Scopes: []string{"read:jira-work", "write:jira-work"},
					},
				}
				b, _ := json.Marshal(resources)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
			}, nil
		},
	}

	svc, db := setupTestAuthService(t, mock)
	defer db.Close()

	// 1. Create Pending Session
	state, authURL, err := svc.CreatePendingSession("")
	if err != nil {
		t.Fatalf("failed to create pending session: %v", err)
	}
	if !strings.Contains(authURL, "client_id=test-client-id") {
		t.Fatalf("authURL missing client_id: %s", authURL)
	}

	// 2. Exchange and Onboard
	user, resources, sessionID, _, err := svc.ExchangeAndOnboard(context.Background(), state, "mock-auth-code")
	if err != nil {
		t.Fatalf("failed exchange and onboard: %v", err)
	}
	if user.Email != "sarah@acme.com" {
		t.Fatalf("expected email sarah@acme.com, got %s", user.Email)
	}
	if sessionID != "" {
		t.Fatalf("expected empty sessionID for single resource auto-approval, got %s", sessionID)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	// Verify site saved
	site, err := db.GetDefaultSite(user.ID)
	if err != nil {
		t.Fatalf("expected default site to be saved: %v", err)
	}
	if site.CloudID != "cloud-uuid-1" {
		t.Fatalf("expected cloudID cloud-uuid-1, got %s", site.CloudID)
	}

	// 3. Test GetValidAccessToken (cache hit)
	token, err := svc.GetValidAccessToken(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to get valid access token: %v", err)
	}
	if token != "mock-access-token-123" {
		t.Fatalf("expected cached token mock-access-token-123, got %s", token)
	}
}

func TestWebHandlers(t *testing.T) {
	svc, db := setupTestAuthService(t, http.DefaultTransport)
	defer db.Close()

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	// Test GET /auth/login redirects
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/login", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from /auth/login, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, AtlassianAuthURL) {
		t.Fatalf("redirect does not point to Atlassian auth: %s", location)
	}
}

func TestMCPOAuthDiscovery(t *testing.T) {
	svc, db := setupTestAuthService(t, http.DefaultTransport)
	defer db.Close()

	mcpOAuth := NewMCPOAuthHandler("http://localhost:8080", svc, db)
	mux := http.NewServeMux()
	mcpOAuth.RegisterRoutes(mux)

	// 1. Check /.well-known/oauth-protected-resource
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var meta map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &meta)
	if meta["resource"] != "http://localhost:8080/sse" {
		t.Fatalf("unexpected resource: %v", meta["resource"])
	}

	// 2. Check /.well-known/oauth-authorization-server
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	var authMeta map[string]interface{}
	_ = json.Unmarshal(rec2.Body.Bytes(), &authMeta)
	if authMeta["authorization_endpoint"] != "http://localhost:8080/oauth/authorize" {
		t.Fatalf("unexpected auth endpoint: %v", authMeta["authorization_endpoint"])
	}
}
