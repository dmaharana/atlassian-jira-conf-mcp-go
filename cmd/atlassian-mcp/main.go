package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"atlassian-jira-conf-mcp/internal/atlassian"
	"atlassian-jira-conf-mcp/internal/auth"
	mcpbridge "atlassian-jira-conf-mcp/internal/mcp"
	"atlassian-jira-conf-mcp/internal/vault"
)

var Version = "1.0.0"

func main() {
	port := flag.String("port", getEnv("PORT", "8080"), "Server HTTP port")
	host := flag.String("host", getEnv("HOST", "0.0.0.0"), "Server bind host")
	baseURL := flag.String("base-url", getEnv("ATLASSIAN_MCP_BASE_URL", "http://localhost:8080"), "Public base URL for OAuth callbacks")
	clientID := flag.String("client-id", getEnv("ATLASSIAN_CLIENT_ID", ""), "Atlassian OAuth 2.0 Client ID")
	clientSecret := flag.String("client-secret", getEnv("ATLASSIAN_CLIENT_SECRET", ""), "Atlassian OAuth 2.0 Client Secret")
	dbPath := flag.String("db-path", getEnv("ATLASSIAN_MCP_DB_PATH", "data/mcp_vault.db"), "Path to SQLite database")
	keyPath := flag.String("key-path", getEnv("ATLASSIAN_MCP_KEY_PATH", "data/master.key"), "Path to master encryption key file")
	downloadDir := flag.String("download-dir", getEnv("ATLASSIAN_MCP_DOWNLOAD_DIR", "data/downloads"), "Directory for downloaded attachments")
	showVersion := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("atlassian-jira-conf-mcp v%s\n", Version)
		os.Exit(0)
	}

	log.Printf("[MCP] Starting Atlassian Jira & Confluence MCP Server v%s", Version)

	// 1. Resolve master encryption key
	masterKey, err := vault.LoadOrGenerateKey(*keyPath)
	if err != nil {
		log.Fatalf("[Vault] Failed to load or generate master key: %v", err)
	}

	// 2. Init AES-256-GCM Encryptor
	encryptor, err := vault.NewEncryptor(masterKey)
	if err != nil {
		log.Fatalf("[Vault] Failed to initialize AES encryptor: %v", err)
	}

	// 3. Open SQLite vault database
	db, err := vault.OpenDB(*dbPath, encryptor)
	if err != nil {
		log.Fatalf("[Vault] Failed to open SQLite database (%s): %v", *dbPath, err)
	}
	defer db.Close()
	log.Printf("[Vault] SQLite vault connected (%s)", *dbPath)

	// 4. Initialize OAuth service
	authCfg := auth.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		BaseURL:      *baseURL,
	}
	authSvc := auth.NewService(authCfg, db, nil)
	mcpOAuth := auth.NewMCPOAuthHandler(*baseURL, authSvc, db)
	authSvc.SetMCPOAuthHandler(mcpOAuth)

	if *clientID == "" || *clientSecret == "" {
		log.Printf("[Warning] ATLASSIAN_CLIENT_ID or ATLASSIAN_CLIENT_SECRET is not set. OAuth login will fail until configured.")
	}

	// 5. Initialize Atlassian Client
	atlClient := atlassian.NewClient(authSvc, db, nil, *downloadDir)

	// 6. Initialize MCP Server
	mcpServer := mcpbridge.NewServer(db, atlClient, *baseURL)

	// 7. Setup HTTP router
	mux := http.NewServeMux()

	// Landing & health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok","version":"`+Version+`"}`)
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Atlassian Jira & Confluence MCP Server</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f4f5f7; display: flex; justify-content: center; padding: 50px 20px; }
    .card { background: white; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.08); padding: 36px; max-width: 600px; width: 100%%; }
    h1 { color: #0052cc; margin-top: 0; }
    .btn { display: inline-block; background: #0052cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; font-weight: 600; margin-top: 16px; }
    .btn:hover { background: #0747a6; }
    code { background: #ebecf0; padding: 2px 6px; border-radius: 4px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Atlassian MCP Server</h1>
    <p>Multi-user Model Context Protocol (MCP) server for Jira and Confluence Cloud.</p>
    <p>Connect your Atlassian Cloud workspace and generate an MCP API key for Claude Desktop, Cursor, or Antigravity:</p>
    <a href="/auth/login" class="btn">Connect Atlassian Account</a>
    <hr style="margin: 28px 0; border: none; border-top: 1px solid #dfe1e6;">
    <p style="font-size: 13px; color: #5e6c84;">SSE Endpoint: <code>%s/sse</code></p>
  </div>
</body>
</html>`, strings.TrimRight(*baseURL, "/"))
	})

	// Register Onboarding web routes
	authSvc.RegisterRoutes(mux)

	// Register RFC 9728 & RFC 8414 MCP OAuth discovery and token routes
	mcpOAuth.RegisterRoutes(mux)

	// Register MCP SSE & Message routes
	mcpServer.RegisterRoutes(mux)

	// HTTP Server
	addr := fmt.Sprintf("%s:%s", *host, *port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[HTTP] Listening on http://%s (Public URL: %s)", addr, *baseURL)
		log.Printf("[HTTP] Onboarding URL: %s/auth/login", strings.TrimRight(*baseURL, "/"))
		log.Printf("[HTTP] MCP SSE Endpoint: %s/sse", strings.TrimRight(*baseURL, "/"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[HTTP] Server failed: %v", err)
		}
	}()

	<-stopChan
	log.Println("[HTTP] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[HTTP] Shutdown error: %v", err)
	}
	log.Println("[MCP] Server stopped.")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
