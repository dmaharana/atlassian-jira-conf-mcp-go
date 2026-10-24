package mcp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"atlassian-jira-conf-mcp/internal/vault"
)

type contextKey string

const (
	userContextKey contextKey = "atlassian_mcp_user"
	siteContextKey contextKey = "atlassian_mcp_site"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated: missing or invalid MCP API key")
)

// ExtractAPIKey extracts the MCP API key from Bearer Authorization header or query parameter (?token= or ?apiKey=).
func ExtractAPIKey(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	if token := r.URL.Query().Get("token"); token != "" {
		return strings.TrimSpace(token)
	}
	if apiKey := r.URL.Query().Get("apiKey"); apiKey != "" {
		return strings.TrimSpace(apiKey)
	}

	return ""
}

func WithUser(ctx context.Context, user *vault.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*vault.User, bool) {
	u, ok := ctx.Value(userContextKey).(*vault.User)
	return u, ok && u != nil
}

func WithSite(ctx context.Context, site *vault.ApprovedSite) context.Context {
	return context.WithValue(ctx, siteContextKey, site)
}

func SiteFromContext(ctx context.Context) (*vault.ApprovedSite, bool) {
	s, ok := ctx.Value(siteContextKey).(*vault.ApprovedSite)
	return s, ok && s != nil
}
