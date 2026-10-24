package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"atlassian-jira-conf-mcp/internal/vault"
	"golang.org/x/oauth2"
)

const (
	AtlassianAuthURL      = "https://auth.atlassian.com/authorize"
	AtlassianTokenURL     = "https://auth.atlassian.com/oauth/token"
	AtlassianResourcesURL = "https://api.atlassian.com/oauth/token/accessible-resources"
	AtlassianProfileURL   = "https://api.atlassian.com/me"
)

var DefaultScopes = []string{
	"read:me",
	"read:jira-work",
	"write:jira-work",
	"read:jira-user",
	"manage:jira-project",
	"manage:jira-configuration",
	"read:confluence-space",
	"read:confluence-content.all",
	"write:confluence-content",
	"read:confluence-user",
	"search:confluence",
	"offline_access",
}

type AccessibleResource struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Scopes    []string `json:"scopes"`
	AvatarURL string   `json:"avatarUrl"`
}

type UserProfile struct {
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
}

type PendingOAuthSession struct {
	State            string
	CodeVerifier     string
	MCPClientSession string
	CreatedAt        time.Time
}

type PendingSiteSelection struct {
	UserID           string
	Profile          UserProfile
	Resources        []AccessibleResource
	MCPClientSession string
	CreatedAt        time.Time
}

type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	Scopes       []string
}

type Service struct {
	cfg          Config
	db           *vault.DB
	oauthConfig  *oauth2.Config
	httpClient   *http.Client
	stateMu      sync.RWMutex
	pendingState map[string]PendingOAuthSession
	siteMu       sync.RWMutex
	pendingSites map[string]PendingSiteSelection
	refreshMu    sync.Map // per-user mutex for token refresh
	mcpOAuth     *MCPOAuthHandler
}

func NewService(cfg Config, db *vault.DB, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = DefaultScopes
	}

	redirectURI := fmt.Sprintf("%s/auth/callback", strings.TrimRight(cfg.BaseURL, "/"))
	oauthConf := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
		RedirectURL:  redirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL:  AtlassianAuthURL,
			TokenURL: AtlassianTokenURL,
		},
	}

	return &Service{
		cfg:          cfg,
		db:           db,
		oauthConfig:  oauthConf,
		httpClient:   httpClient,
		pendingState: make(map[string]PendingOAuthSession),
		pendingSites: make(map[string]PendingSiteSelection),
	}
}

func (s *Service) SetMCPOAuthHandler(h *MCPOAuthHandler) {
	s.mcpOAuth = h
}

// GeneratePKCE creates a cryptographic code_verifier and S256 code_challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}

// CreatePendingSession creates a random state string and records the pending session.
func (s *Service) CreatePendingSession(mcpClientSession string) (state, authURL string, err error) {
	stateBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, stateBytes); err != nil {
		return "", "", fmt.Errorf("generate random state: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(stateBytes)

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return "", "", fmt.Errorf("generate pkce: %w", err)
	}

	s.stateMu.Lock()
	s.pendingState[state] = PendingOAuthSession{
		State:            state,
		CodeVerifier:     verifier,
		MCPClientSession: mcpClientSession,
		CreatedAt:        time.Now().UTC(),
	}
	s.cleanupOldStates()
	s.stateMu.Unlock()

	params := url.Values{}
	params.Set("audience", "api.atlassian.com")
	params.Set("client_id", s.cfg.ClientID)
	params.Set("scope", strings.Join(s.cfg.Scopes, " "))
	params.Set("redirect_uri", s.oauthConfig.RedirectURL)
	params.Set("state", state)
	params.Set("response_type", "code")
	params.Set("prompt", "consent")
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	authURL = fmt.Sprintf("%s?%s", AtlassianAuthURL, params.Encode())
	return state, authURL, nil
}

func (s *Service) cleanupOldStates() {
	cutoff := time.Now().UTC().Add(-15 * time.Minute)
	for st, sess := range s.pendingState {
		if sess.CreatedAt.Before(cutoff) {
			delete(s.pendingState, st)
		}
	}
	for id, sess := range s.pendingSites {
		if sess.CreatedAt.Before(cutoff) {
			delete(s.pendingSites, id)
		}
	}
}

// ExchangeAndOnboard completes the OAuth code exchange, stores tokens, and fetches resources.
func (s *Service) ExchangeAndOnboard(ctx context.Context, state, code string) (user *vault.User, resources []AccessibleResource, sessionID, mcpClientSession string, err error) {
	s.stateMu.Lock()
	pending, exists := s.pendingState[state]
	if !exists {
		s.stateMu.Unlock()
		return nil, nil, "", "", errors.New("invalid or expired oauth state parameter")
	}
	delete(s.pendingState, state)
	s.stateMu.Unlock()

	mcpClientSession = pending.MCPClientSession

	// Exchange code for tokens using verifier
	tokenResp, err := s.exchangeCode(ctx, code, pending.CodeVerifier)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("token exchange failed: %w", err)
	}

	// Fetch user profile from /me
	profile, err := s.FetchUserProfile(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("fetch user profile: %w", err)
	}

	// Upsert user in vault database
	user, err = s.db.UpsertUser(profile.Email, profile.Name)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("upsert user: %w", err)
	}

	// Save encrypted tokens
	tokenData := &vault.TokenData{
		UserID:       user.ID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
	}
	if err := s.db.SaveTokens(tokenData); err != nil {
		return nil, nil, "", "", fmt.Errorf("save encrypted tokens: %w", err)
	}

	// Fetch accessible resources (Atlassian Cloud sites)
	resources, err = s.FetchAccessibleResources(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("fetch accessible resources: %w", err)
	}

	if len(resources) == 0 {
		return nil, nil, "", "", errors.New("no accessible Atlassian Cloud sites found for this user account")
	}

	// If single site, auto-save as approved default site
	if len(resources) == 1 {
		r := resources[0]
		site := &vault.ApprovedSite{
			UserID:    user.ID,
			CloudID:   r.ID,
			SiteName:  r.Name,
			SiteURL:   r.URL,
			IsDefault: true,
			Scopes:    strings.Join(r.Scopes, " "),
		}
		if err := s.db.SaveApprovedSite(site); err != nil {
			return nil, nil, "", "", fmt.Errorf("save default site: %w", err)
		}
		return user, resources, "", mcpClientSession, nil
	}

	// Multiple sites: record pending site selection
	b := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, b)
	sessionID = base64.RawURLEncoding.EncodeToString(b)

	s.siteMu.Lock()
	s.pendingSites[sessionID] = PendingSiteSelection{
		UserID:           user.ID,
		Profile:          *profile,
		Resources:        resources,
		MCPClientSession: mcpClientSession,
		CreatedAt:        time.Now().UTC(),
	}
	s.siteMu.Unlock()

	return user, resources, sessionID, mcpClientSession, nil
}

type tokenResponseJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func (s *Service) exchangeCode(ctx context.Context, code, verifier string) (*tokenResponseJSON, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.cfg.ClientID)
	data.Set("client_secret", s.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.oauthConfig.RedirectURL)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AtlassianTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atlassian token error (status %d): %s", resp.StatusCode, string(body))
	}

	var tr tokenResponseJSON
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}
	return &tr, nil
}

func (s *Service) FetchUserProfile(ctx context.Context, accessToken string) (*UserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AtlassianProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var p UserProfile
			if err := json.NewDecoder(resp.Body).Decode(&p); err == nil && (p.Email != "" || p.AccountID != "") {
				if p.Email == "" {
					p.Email = fmt.Sprintf("user-%s@atlassian.cloud", p.AccountID)
				}
				if p.Name == "" {
					p.Name = "Atlassian User"
				}
				return &p, nil
			}
		}
	}

	// Fallback if read:me scope was not granted or /me returned 403:
	// Derive identity from token payload or hash so onboarding never fails.
	fallbackEmail := extractFallbackEmail(accessToken)
	return &UserProfile{
		AccountID: "atlassian-user",
		Email:     fallbackEmail,
		Name:      "Atlassian User",
	}, nil
}

func extractFallbackEmail(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) >= 2 {
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err == nil {
			var claims map[string]interface{}
			if err := json.Unmarshal(payloadBytes, &claims); err == nil {
				if email, ok := claims["email"].(string); ok && email != "" {
					return email
				}
				if sub, ok := claims["sub"].(string); ok && sub != "" {
					return fmt.Sprintf("user-%s@atlassian.cloud", sub)
				}
			}
		}
	}
	h := sha256.Sum256([]byte(accessToken))
	return fmt.Sprintf("user-%x@atlassian.cloud", h[:6])
}

func (s *Service) FetchAccessibleResources(ctx context.Context, accessToken string) ([]AccessibleResource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AtlassianResourcesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch resources status %d: %s", resp.StatusCode, string(b))
	}

	var res []AccessibleResource
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPendingSiteSelection retrieves pending sites for user approval.
func (s *Service) GetPendingSiteSelection(sessionID string) (*PendingSiteSelection, bool) {
	s.siteMu.RLock()
	defer s.siteMu.RUnlock()
	sess, exists := s.pendingSites[sessionID]
	if !exists {
		return nil, false
	}
	return &sess, true
}

// CompleteSiteSelection approves a site for a user session.
func (s *Service) CompleteSiteSelection(sessionID, selectedCloudID string) (*vault.User, *vault.ApprovedSite, string, error) {
	s.siteMu.Lock()
	sess, exists := s.pendingSites[sessionID]
	if !exists {
		s.siteMu.Unlock()
		return nil, nil, "", errors.New("invalid or expired site selection session")
	}
	delete(s.pendingSites, sessionID)
	s.siteMu.Unlock()

	user, err := s.db.GetUserByID(sess.UserID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("user not found: %w", err)
	}

	var matched *AccessibleResource
	for i := range sess.Resources {
		if sess.Resources[i].ID == selectedCloudID {
			matched = &sess.Resources[i]
			break
		}
	}
	if matched == nil {
		return nil, nil, "", errors.New("selected cloud_id was not among authorized accessible resources")
	}

	site := &vault.ApprovedSite{
		UserID:    user.ID,
		CloudID:   matched.ID,
		SiteName:  matched.Name,
		SiteURL:   matched.URL,
		IsDefault: true,
		Scopes:    strings.Join(matched.Scopes, " "),
	}
	if err := s.db.SaveApprovedSite(site); err != nil {
		return nil, nil, "", fmt.Errorf("save approved site: %w", err)
	}

	return user, site, sess.MCPClientSession, nil
}

// GetValidAccessToken returns a valid access token for the user, refreshing if near expiry.
func (s *Service) GetValidAccessToken(ctx context.Context, userID string) (string, error) {
	val, _ := s.refreshMu.LoadOrStore(userID, &sync.Mutex{})
	userMu := val.(*sync.Mutex)
	userMu.Lock()
	defer userMu.Unlock()

	tokens, err := s.db.GetTokens(userID)
	if err != nil {
		return "", err
	}

	// If token expires in > 3 minutes, use it directly
	if time.Until(tokens.Expiry) > 3*time.Minute {
		return tokens.AccessToken, nil
	}

	// Refresh token
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", s.cfg.ClientID)
	data.Set("client_secret", s.cfg.ClientSecret)
	data.Set("refresh_token", tokens.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AtlassianTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create refresh req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read refresh body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh token failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tr tokenResponseJSON
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("unmarshal refresh response: %w", err)
	}

	newRefreshToken := tr.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = tokens.RefreshToken // preserve existing if rotation did not emit new refresh token
	}

	tokens.AccessToken = tr.AccessToken
	tokens.RefreshToken = newRefreshToken
	tokens.Expiry = time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second)
	tokens.TokenType = tr.TokenType

	if err := s.db.SaveTokens(tokens); err != nil {
		return "", fmt.Errorf("save refreshed tokens: %w", err)
	}

	return tokens.AccessToken, nil
}
