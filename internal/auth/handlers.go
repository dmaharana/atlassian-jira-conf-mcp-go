package auth

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// RegisterRoutes registers the self-service onboarding routes on the provided mux.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", s.handleLogin)
	mux.HandleFunc("GET /auth/callback", s.handleCallback)
	mux.HandleFunc("GET /auth/select-site", s.handleSelectSite)
	mux.HandleFunc("POST /auth/select-site", s.handleCompleteSelectSite)
	mux.HandleFunc("GET /auth/success", s.handleSuccess)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	mcpClientSession := r.URL.Query().Get("mcp_client_session")
	_, authURL, err := s.CreatePendingSession(mcpClientSession)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initiate login: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("Atlassian authorization error: %s (%s)", errParam, errDesc), http.StatusBadRequest)
		return
	}

	if state == "" || code == "" {
		http.Error(w, "Missing state or authorization code", http.StatusBadRequest)
		return
	}

	user, _, sessionID, mcpClientSession, err := s.ExchangeAndOnboard(r.Context(), state, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	if sessionID != "" {
		http.Redirect(w, r, fmt.Sprintf("/auth/select-site?session=%s", sessionID), http.StatusFound)
		return
	}

	if mcpClientSession != "" && s.mcpOAuth != nil {
		targetURL, err := s.mcpOAuth.CompleteClientAuth(mcpClientSession, user.ID)
		if err == nil {
			http.Redirect(w, r, targetURL, http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/auth/success?user_id=%s", user.ID), http.StatusFound)
}

func (s *Service) handleSelectSite(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "Missing session parameter", http.StatusBadRequest)
		return
	}

	pending, exists := s.GetPendingSiteSelection(sessionID)
	if !exists {
		http.Error(w, "Invalid or expired site selection session", http.StatusBadRequest)
		return
	}

	tmpl := template.Must(template.New("select-site").Parse(selectSiteHTML))
	data := map[string]interface{}{
		"SessionID": sessionID,
		"UserName":  pending.Profile.Name,
		"Email":     pending.Profile.Email,
		"Resources": pending.Resources,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

func (s *Service) handleCompleteSelectSite(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	sessionID := r.FormValue("session_id")
	selectedCloudID := r.FormValue("cloud_id")

	if sessionID == "" || selectedCloudID == "" {
		http.Error(w, "Missing session_id or cloud_id selection", http.StatusBadRequest)
		return
	}

	user, _, mcpClientSession, err := s.CompleteSiteSelection(sessionID, selectedCloudID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to approve site: %v", err), http.StatusBadRequest)
		return
	}

	if mcpClientSession != "" && s.mcpOAuth != nil {
		targetURL, err := s.mcpOAuth.CompleteClientAuth(mcpClientSession, user.ID)
		if err == nil {
			http.Redirect(w, r, targetURL, http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/auth/success?user_id=%s", user.ID), http.StatusFound)
}

func (s *Service) handleSuccess(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id", http.StatusBadRequest)
		return
	}

	user, err := s.db.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	site, _ := s.db.GetDefaultSite(userID)
	baseURL := strings.TrimRight(s.cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", r.Host)
	}

	sseURL := fmt.Sprintf("%s/sse", baseURL)
	claudeSnippet := fmt.Sprintf(`{
  "mcpServers": {
    "atlassian": {
      "url": "%s",
      "headers": {
        "Authorization": "Bearer %s"
      }
    }
  }
}`, sseURL, user.MCPAPIKey)

	siteName := "Atlassian Cloud Site"
	if site != nil {
		siteName = fmt.Sprintf("%s (%s)", site.SiteName, site.SiteURL)
	}

	tmpl := template.Must(template.New("success").Parse(successHTML))
	data := map[string]interface{}{
		"UserName":      user.Name,
		"Email":         user.Email,
		"APIKey":        user.MCPAPIKey,
		"ApprovedSite":  siteName,
		"SSEURL":        sseURL,
		"ClaudeSnippet": claudeSnippet,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

const selectSiteHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Select Atlassian Site - MCP Server</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background: #f4f5f7; display: flex; justify-content: center; padding: 40px 20px; }
    .card { background: white; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.08); padding: 32px; max-width: 540px; width: 100%; }
    h2 { margin-top: 0; color: #172b4d; }
    p { color: #5e6c84; font-size: 14px; }
    .site-option { border: 1px solid #dfe1e6; border-radius: 6px; padding: 14px; margin-bottom: 12px; display: flex; align-items: center; cursor: pointer; transition: border-color 0.2s; }
    .site-option:hover { border-color: #0052cc; background: #f9fbfd; }
    .site-option input { margin-right: 14px; }
    .btn { background: #0052cc; color: white; border: none; border-radius: 4px; padding: 10px 20px; font-weight: 600; cursor: pointer; width: 100%; margin-top: 16px; font-size: 14px; }
    .btn:hover { background: #0747a6; }
  </style>
</head>
<body>
  <div class="card">
    <h2>Select Approved Atlassian Site</h2>
    <p>Hello <strong>{{.UserName}}</strong> ({{.Email}}). Please select which Atlassian Cloud workspace this MCP server should interact with:</p>
    <form method="POST" action="/auth/select-site">
      <input type="hidden" name="session_id" value="{{.SessionID}}">
      {{range $i, $res := .Resources}}
      <label class="site-option">
        <input type="radio" name="cloud_id" value="{{$res.ID}}" {{if eq $i 0}}checked{{end}}>
        <div>
          <strong>{{$res.Name}}</strong><br>
          <small style="color:#6b778c">{{$res.URL}}</small>
        </div>
      </label>
      {{end}}
      <button type="submit" class="btn">Connect Workspace</button>
    </form>
  </div>
</body>
</html>`

const successHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>MCP Authorization Success</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background: #f4f5f7; display: flex; justify-content: center; padding: 40px 20px; }
    .card { background: white; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.08); padding: 32px; max-width: 650px; width: 100%; }
    h2 { color: #36b37e; margin-top: 0; }
    .token-box { background: #ebecf0; border-radius: 4px; padding: 12px; font-family: monospace; font-size: 15px; word-break: break-all; margin: 10px 0; user-select: all; }
    pre { background: #091e420f; border-radius: 6px; padding: 16px; font-size: 13px; overflow-x: auto; }
    .meta { font-size: 14px; color: #5e6c84; line-height: 1.6; }
  </style>
</head>
<body>
  <div class="card">
    <h2>✓ Connected to {{.ApprovedSite}}</h2>
    <p class="meta">Account: <strong>{{.UserName}}</strong> ({{.Email}})</p>
    
    <h3>Your MCP API Key</h3>
    <p class="meta">Add this token to your MCP client configuration:</p>
    <div class="token-box">{{.APIKey}}</div>

    <h3>Client Configuration (Claude Desktop / Cursor)</h3>
    <p class="meta">Add the following block to your <code>claude_desktop_config.json</code>:</p>
    <pre>{{.ClaudeSnippet}}</pre>

    <p class="meta">For clients that do not support custom headers, you can use the direct URL with token:<br>
    <code>{{.SSEURL}}?token={{.APIKey}}</code></p>
  </div>
</body>
</html>`
