# Atlassian Cloud 3LO with Local Loopback Authorization

**Status**: Superseded by [ADR 0004](file:///home/titu/sources/go-workspace/atlassian-jira-conf-mcp/docs/adr/0004-multi-user-http-sqlite-vault.md)

## Context & Decision
MCP clients operate as local processes over stdio without native interactive browser windows. We decided to implement Atlassian 3-legged OAuth (3LO) with PKCE using a short-lived local HTTP callback listener (`localhost:8080/callback`) triggered by a CLI command or first-run initialization, storing the encrypted refresh token at `~/.config/atlassian-mcp/credentials.json`. This provides secure headless token refresh during stdio MCP sessions without requiring users to manually manage API tokens.

## Considered Options
- **Static API Tokens**: Using Atlassian email + API token basic auth. Rejected because the prompt explicitly mandated Atlassian OAuth authorization, which supports granular OAuth scopes and automated token lifecycle.
- **In-Band MCP Auth Tools**: Interactive auth inside the MCP tool loop. Rejected due to poor UX in stdio environments where background redirect servers cannot easily coordinate with conversational turns.
