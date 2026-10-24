# Multi-User HTTP MCP Server with Encrypted SQLite Token Vault

**Status**: Accepted (Supersedes [ADR 0002](file:///home/titu/sources/go-workspace/atlassian-jira-conf-mcp/docs/adr/0002-atlassian-cloud-oauth-loopback.md))

## Context & Decision
The MCP server must serve multiple concurrent users over HTTP rather than running as a single-user local stdio process. We decided to deploy the MCP server over HTTP/SSE, storing user tokens and approved Atlassian sites in an embedded SQLite database with AES-256-GCM column-level token encryption. Inbound MCP HTTP requests are authenticated via Bearer tokens mapped to specific user profiles and active Atlassian Cloud sites.

## Considered Options
- **Local Stdio Process per User**: Desktop-only single-tenant stdio processes. Rejected because a centralized multi-user HTTP service is required.
- **External Database (PostgreSQL/MySQL)**: Operating an external database cluster. Rejected in favor of embedded SQLite for simplicity, zero external network dependencies, and effortless self-hosting.
- **Unencrypted SQLite Storage**: Storing plain refresh tokens in SQLite. Rejected due to severe security exposure of long-lived Atlassian Cloud OAuth tokens.
