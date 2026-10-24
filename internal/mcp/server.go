package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"atlassian-jira-conf-mcp/internal/atlassian"
	"atlassian-jira-conf-mcp/internal/vault"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer        *mcpserver.MCPServer
	sseServer        *mcpserver.SSEServer
	streamableServer *mcpserver.StreamableHTTPServer
	db               *vault.DB
	client           *atlassian.Client
	baseURL          string
}

func NewServer(db *vault.DB, client *atlassian.Client, baseURL string) *Server {
	baseURL = strings.TrimRight(baseURL, "/")
	mcpSrv := mcpserver.NewMCPServer(
		"atlassian-jira-conf-mcp",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	// Register tools
	RegisterJiraTools(mcpSrv, client)
	RegisterConfluenceTools(mcpSrv, client)

	// Context injector for multi-user authentication
	contextFunc := func(ctx context.Context, r *http.Request) context.Context {
		apiKey := ExtractAPIKey(r)
		if apiKey == "" {
			return ctx
		}

		user, err := db.GetUserByAPIKey(apiKey)
		if err != nil || user == nil {
			return ctx
		}

		ctx = WithUser(ctx, user)
		site, err := db.GetDefaultSite(user.ID)
		if err == nil && site != nil {
			ctx = WithSite(ctx, site)
		}

		return ctx
	}

	opts := []mcpserver.SSEOption{
		mcpserver.WithSSEContextFunc(contextFunc),
		mcpserver.WithSSEEndpoint("/sse"),
		mcpserver.WithMessageEndpoint("/message"),
	}

	if baseURL != "" {
		opts = append(opts, mcpserver.WithBaseURL(baseURL))
	}

	sseSrv := mcpserver.NewSSEServer(mcpSrv, opts...)
	streamableSrv := mcpserver.NewStreamableHTTPServer(
		mcpSrv,
		mcpserver.WithHTTPContextFunc(contextFunc),
		mcpserver.WithDisableLocalhostProtection(true),
	)

	return &Server{
		mcpServer:        mcpSrv,
		sseServer:        sseSrv,
		streamableServer: streamableSrv,
		db:               db,
		client:           client,
		baseURL:          baseURL,
	}
}

// RegisterRoutes registers the SSE, Streamable HTTP, and message endpoints on an existing HTTP mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	endpointHandler := s.authMiddleware(http.HandlerFunc(s.handleEndpoint))
	mux.Handle("/sse", endpointHandler)
	mux.Handle("/mcp", endpointHandler)
	mux.Handle("/message", s.authMiddleware(s.sseServer.MessageHandler()))
}

func (s *Server) handleEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// If classic SSE message with sessionId
		if r.URL.Query().Get("sessionId") != "" {
			s.sseServer.MessageHandler().ServeHTTP(w, r)
			return
		}
		// Modern Streamable HTTP POST (e.g. initialize, tool call)
		s.streamableServer.ServeHTTP(w, r)
		return
	}

	if r.Method == http.MethodGet {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/event-stream") {
			s.sseServer.SSEHandler().ServeHTTP(w, r)
			return
		}
		s.streamableServer.ServeHTTP(w, r)
		return
	}

	if r.Method == http.MethodDelete {
		s.streamableServer.ServeHTTP(w, r)
		return
	}

	s.streamableServer.ServeHTTP(w, r)
}

// authMiddleware enforces valid MCP API keys before establishing SSE or message handling.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := ExtractAPIKey(r)
		if apiKey == "" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, s.baseURL))
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate")
			http.Error(w, "Unauthorized: Missing MCP API key. Provide via 'Authorization: Bearer <key>' or '?token=<key>'", http.StatusUnauthorized)
			return
		}

		user, err := s.db.GetUserByAPIKey(apiKey)
		if err != nil || user == nil {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="invalid_token", resource_metadata="%s/.well-known/oauth-protected-resource"`, s.baseURL))
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate")
			http.Error(w, "Unauthorized: Invalid or revoked MCP API key", http.StatusUnauthorized)
			return
		}

		ctx := WithUser(r.Context(), user)
		site, err := s.db.GetDefaultSite(user.ID)
		if err == nil && site != nil {
			ctx = WithSite(ctx, site)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ServeHTTP delegates to the SSE server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.sseServer.ServeHTTP(w, r)
}

// GetMCPServer returns the underlying MCPServer for direct inspection/testing.
func (s *Server) GetMCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}
