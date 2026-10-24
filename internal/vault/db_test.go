package vault

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestDBLifecycleAndVault(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_vault.db")

	key := bytes.Repeat([]byte("v"), 32)
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to init encryptor: %v", err)
	}

	db, err := OpenDB(dbPath, enc)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// 1. Upsert User
	user, err := db.UpsertUser("john.doe@company.com", "John Doe")
	if err != nil {
		t.Fatalf("failed to upsert user: %v", err)
	}
	if user.Email != "john.doe@company.com" || user.Name != "John Doe" {
		t.Fatalf("unexpected user fields: %+v", user)
	}
	if len(user.MCPAPIKey) < 10 {
		t.Fatalf("invalid generated mcp api key: %s", user.MCPAPIKey)
	}

	// 2. Lookup by API Key
	foundUser, err := db.GetUserByAPIKey(user.MCPAPIKey)
	if err != nil {
		t.Fatalf("failed to lookup user by api key: %v", err)
	}
	if foundUser.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, foundUser.ID)
	}

	// 3. Save Encrypted Tokens
	originalAccessToken := "atlassian-access-token-live-abc"
	originalRefreshToken := "atlassian-refresh-token-live-xyz"
	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)

	err = db.SaveTokens(&TokenData{
		UserID:       user.ID,
		AccessToken:  originalAccessToken,
		RefreshToken: originalRefreshToken,
		Expiry:       expiry,
		TokenType:    "Bearer",
	})
	if err != nil {
		t.Fatalf("failed to save tokens: %v", err)
	}

	// 4. Retrieve & Decrypt Tokens
	tokens, err := db.GetTokens(user.ID)
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if tokens.AccessToken != originalAccessToken {
		t.Fatalf("expected access token %s, got %s", originalAccessToken, tokens.AccessToken)
	}
	if tokens.RefreshToken != originalRefreshToken {
		t.Fatalf("expected refresh token %s, got %s", originalRefreshToken, tokens.RefreshToken)
	}

	// 5. Save & List Approved Sites
	site1 := &ApprovedSite{
		UserID:    user.ID,
		CloudID:   "cloud-id-123",
		SiteName:  "Production Jira",
		SiteURL:   "https://prod.atlassian.net",
		IsDefault: true,
		Scopes:    "read:jira-work write:jira-work",
	}
	site2 := &ApprovedSite{
		UserID:    user.ID,
		CloudID:   "cloud-id-456",
		SiteName:  "Staging Jira",
		SiteURL:   "https://staging.atlassian.net",
		IsDefault: false,
		Scopes:    "read:jira-work",
	}

	if err := db.SaveApprovedSite(site1); err != nil {
		t.Fatalf("failed to save site1: %v", err)
	}
	if err := db.SaveApprovedSite(site2); err != nil {
		t.Fatalf("failed to save site2: %v", err)
	}

	sites, err := db.ListSites(user.ID)
	if err != nil {
		t.Fatalf("failed to list sites: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}

	defaultSite, err := db.GetDefaultSite(user.ID)
	if err != nil {
		t.Fatalf("failed to get default site: %v", err)
	}
	if defaultSite.CloudID != "cloud-id-123" {
		t.Fatalf("expected default site cloud-id-123, got %s", defaultSite.CloudID)
	}

	// Switch default site
	site2.IsDefault = true
	if err := db.SaveApprovedSite(site2); err != nil {
		t.Fatalf("failed to update site2 as default: %v", err)
	}

	updatedDefault, err := db.GetDefaultSite(user.ID)
	if err != nil {
		t.Fatalf("failed to get updated default site: %v", err)
	}
	if updatedDefault.CloudID != "cloud-id-456" {
		t.Fatalf("expected default site cloud-id-456, got %s", updatedDefault.CloudID)
	}
}
