package atlassian

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"atlassian-jira-conf-mcp/internal/auth"
	"atlassian-jira-conf-mcp/internal/vault"
)

type mockTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func setupTestAtlassianClient(t *testing.T, roundTripper http.RoundTripper) (*Client, *vault.DB, string, string) {
	tmpDir := t.TempDir()
	enc, _ := vault.NewEncryptor(bytes.Repeat([]byte("c"), 32))
	db, err := vault.OpenDB(filepath.Join(tmpDir, "atl_test.db"), enc)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	authSvc := auth.NewService(auth.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		BaseURL:      "http://localhost:8080",
	}, db, &http.Client{Transport: roundTripper})

	user, _ := db.UpsertUser("test@company.com", "Tester")
	_ = db.SaveTokens(&vault.TokenData{
		UserID:       user.ID,
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
	})
	_ = db.SaveApprovedSite(&vault.ApprovedSite{
		UserID:    user.ID,
		CloudID:   "cloud-uuid-prod",
		SiteName:  "Test Site",
		SiteURL:   "https://test.atlassian.net",
		IsDefault: true,
	})

	httpClient := &http.Client{
		Transport: roundTripper,
		Timeout:   5 * time.Second,
	}

	client := NewClient(authSvc, db, httpClient, filepath.Join(tmpDir, "downloads"))
	return client, db, user.ID, "cloud-uuid-prod"
}

func TestJiraGetIssueAnd403Translation(t *testing.T) {
	mock := &mockTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/issue/SEC-100") {
				// Simulate 403 Forbidden
				errBody := `{"errorMessages":["You do not have permission to view issue SEC-100 in project SEC"]}`
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader(errBody)),
					Header:     make(http.Header),
				}, nil
			}

			if strings.Contains(req.URL.Path, "/issue/ENG-1") {
				issueData := map[string]interface{}{
					"key": "ENG-1",
					"fields": map[string]interface{}{
						"summary": "Fix auth flow",
						"status":  map[string]interface{}{"name": "To Do"},
					},
				}
				b, _ := json.Marshal(issueData)
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

	client, db, userID, cloudID := setupTestAtlassianClient(t, mock)
	defer db.Close()

	// 1. Success case
	res, err := client.GetJiraIssue(context.Background(), userID, cloudID, "ENG-1", false)
	if err != nil {
		t.Fatalf("failed to get jira issue: %v", err)
	}
	compactIssue, ok := res.(interface{})
	if !ok || compactIssue == nil {
		t.Fatalf("expected compact issue result")
	}

	// 2. 403 Permission Denied case
	_, err = client.GetJiraIssue(context.Background(), userID, cloudID, "SEC-100", false)
	if err == nil {
		t.Fatalf("expected 403 permission denied error, got nil")
	}
	if !strings.Contains(err.Error(), "Permission Denied (403)") {
		t.Fatalf("error should contain actionable 403 message: %v", err)
	}
}

func TestRateLimit429Retry(t *testing.T) {
	var attempts int32
	mock := &mockTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			cur := atomic.AddInt32(&attempts, 1)
			if cur == 1 {
				// First attempt: 429
				resp := &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Body:       io.NopCloser(strings.NewReader(`{"message":"Rate limit exceeded"}`)),
					Header:     make(http.Header),
				}
				resp.Header.Set("Retry-After", "1")
				return resp, nil
			}

			// Second attempt: 200 OK
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"key":"ENG-2","fields":{"summary":"Retry Success"}}`)),
				Header:     make(http.Header),
			}, nil
		},
	}

	client, db, userID, cloudID := setupTestAtlassianClient(t, mock)
	defer db.Close()

	res, err := client.GetJiraIssue(context.Background(), userID, cloudID, "ENG-2", true)
	if err != nil {
		t.Fatalf("expected successful retry after 429, got error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	rawMap, ok := res.(map[string]interface{})
	if !ok || rawMap["key"] != "ENG-2" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
