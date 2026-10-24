package mcp

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

	"atlassian-jira-conf-mcp/internal/atlassian"
	"atlassian-jira-conf-mcp/internal/auth"
	"atlassian-jira-conf-mcp/internal/vault"
	mcpmodel "github.com/mark3labs/mcp-go/mcp"
)

type mockTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func setupTestMCPServer(t *testing.T) (*Server, *vault.DB, *vault.User) {
	tmpDir := t.TempDir()
	enc, _ := vault.NewEncryptor(bytes.Repeat([]byte("m"), 32))
	db, err := vault.OpenDB(filepath.Join(tmpDir, "mcp_test.db"), enc)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	user, _ := db.UpsertUser("dev@acme.com", "Developer")
	_ = db.SaveTokens(&vault.TokenData{
		UserID:       user.ID,
		AccessToken:  "mock-token",
		RefreshToken: "mock-refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
	})
	_ = db.SaveApprovedSite(&vault.ApprovedSite{
		UserID:    user.ID,
		CloudID:   "cloud-123",
		SiteName:  "Acme Prod",
		SiteURL:   "https://acme.atlassian.net",
		IsDefault: true,
	})

	mock := &mockTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/issue/PROJ-1") {
				body := `{"key":"PROJ-1","fields":{"summary":"Test Issue","status":{"name":"Open"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			}

			if strings.Contains(req.URL.Path, "/pages/page-42") {
				body := `{"id":"page-42","title":"Home Page","status":"current","body":{"storage":{"value":"<p>Welcome</p>"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			}

			if strings.Contains(req.URL.Path, "/search") {
				body := `{"total":1,"startAt":0,"maxResults":25,"issues":[{"key":"PROJ-1","fields":{"summary":"Found Issue"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
				Header:     make(http.Header),
			}, nil
		},
	}

	authSvc := auth.NewService(auth.Config{ClientID: "c", ClientSecret: "s"}, db, &http.Client{Transport: mock})
	client := atlassian.NewClient(authSvc, db, &http.Client{Transport: mock}, filepath.Join(tmpDir, "downloads"))
	server := NewServer(db, client, "http://localhost:8080")

	return server, db, user
}

func TestActionNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"getJiraIssue", "get_issue"},
		{"getIssue", "get_issue"},
		{"createJiraIssue", "create_issue"},
		{"searchJiraIssuesUsingJql", "search_issues"},
		{"getConfluenceContent", "get_content"},
		{"searchConfluence", "search_content"},
		{"list_projects", "list_projects"},
		{"listProjects", "list_projects"},
	}

	for _, tt := range tests {
		actual := NormalizeAction(tt.input)
		if actual != tt.expected {
			t.Errorf("NormalizeAction(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestExtractAPIKey(t *testing.T) {
	// From Bearer header
	req1 := httptest.NewRequest("GET", "/sse", nil)
	req1.Header.Set("Authorization", "Bearer mcp_test_key_123")
	if k := ExtractAPIKey(req1); k != "mcp_test_key_123" {
		t.Fatalf("expected mcp_test_key_123, got %s", k)
	}

	// From query param ?token=
	req2 := httptest.NewRequest("GET", "/sse?token=mcp_test_key_456", nil)
	if k := ExtractAPIKey(req2); k != "mcp_test_key_456" {
		t.Fatalf("expected mcp_test_key_456, got %s", k)
	}

	// From query param ?apiKey=
	req3 := httptest.NewRequest("GET", "/sse?apiKey=mcp_test_key_789", nil)
	if k := ExtractAPIKey(req3); k != "mcp_test_key_789" {
		t.Fatalf("expected mcp_test_key_789, got %s", k)
	}
}

func TestAuthMiddleware(t *testing.T) {
	server, db, user := setupTestMCPServer(t)
	defer db.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// 1. Unauthenticated request to /sse should fail with 401
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/sse", nil)
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec1.Code)
	}
	wwwAuth := rec1.Header().Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "resource_metadata") {
		t.Fatalf("expected resource_metadata in WWW-Authenticate, got %s", wwwAuth)
	}

	// 2. Authenticated request with query param token
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/sse?token="+user.MCPAPIKey, nil)
	// We run through mux - since it's an SSE handler it initiates an event stream
	go mux.ServeHTTP(rec2, req2)
	time.Sleep(50 * time.Millisecond)

	// 3. Invalid token
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/sse?token=invalid-key", nil)
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rec3.Code)
	}
}

func TestDirectToolExecution(t *testing.T) {
	server, db, user := setupTestMCPServer(t)
	defer db.Close()

	mcpSrv := server.GetMCPServer()
	ctx := WithUser(context.Background(), user)

	// 1. Test read_jira with get_issue
	req := mcpmodel.CallToolRequest{}
	req.Params.Name = "read_jira"
	req.Params.Arguments = map[string]interface{}{
		"action":    "get_issue",
		"issue_key": "PROJ-1",
	}

	readTool := mcpSrv.GetTool("read_jira")
	if readTool == nil {
		t.Fatalf("read_jira tool not registered")
	}
	res, err := readTool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected call tool error: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res)
	}

	var jsonOut map[string]interface{}
	textResult, ok := res.Content[0].(mcpmodel.TextContent)
	if !ok {
		t.Fatalf("expected TextContent result, got %+v", res.Content[0])
	}
	_ = json.Unmarshal([]byte(textResult.Text), &jsonOut)
	if jsonOut["key"] != "PROJ-1" {
		t.Fatalf("expected issue key PROJ-1, got %v", jsonOut["key"])
	}

	// 2. Test search_jira with search_issues
	reqSearch := mcpmodel.CallToolRequest{}
	reqSearch.Params.Name = "search_jira"
	reqSearch.Params.Arguments = map[string]interface{}{
		"action": "search_issues",
		"jql":    "project = PROJ",
	}
	searchTool := mcpSrv.GetTool("search_jira")
	if searchTool == nil {
		t.Fatalf("search_jira tool not registered")
	}
	resSearch, err := searchTool.Handler(ctx, reqSearch)
	if err != nil {
		t.Fatalf("unexpected search tool error: %v", err)
	}
	if resSearch.IsError {
		t.Fatalf("search returned error: %+v", resSearch)
	}

	// 3. Test read_confluence with get_content
	reqConf := mcpmodel.CallToolRequest{}
	reqConf.Params.Name = "read_confluence"
	reqConf.Params.Arguments = map[string]interface{}{
		"action":     "get_content",
		"content_id": "page-42",
	}
	readConfTool := mcpSrv.GetTool("read_confluence")
	if readConfTool == nil {
		t.Fatalf("read_confluence tool not registered")
	}
	resConf, err := readConfTool.Handler(ctx, reqConf)
	if err != nil {
		t.Fatalf("unexpected read_confluence error: %v", err)
	}
	if resConf.IsError {
		t.Fatalf("read_confluence returned error: %+v", resConf)
	}
}

func TestStreamableHttpPostInitialize(t *testing.T) {
	server, db, user := setupTestMCPServer(t)
	defer db.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0",
			},
		},
	}
	body, _ := json.Marshal(initReq)

	req := httptest.NewRequest("POST", "/sse", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+user.MCPAPIKey)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("POST /sse must NOT return 405 Method Not Allowed")
	}
	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Fatalf("expected 200 OK or 202 Accepted, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}
