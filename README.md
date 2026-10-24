# Atlassian Jira & Confluence Multi-User MCP Server

A high-performance Golang Model Context Protocol (MCP) server providing structured tools for interacting with Atlassian Jira and Confluence Cloud over Server-Sent Events (SSE).

## Features

- **Multi-User Architecture**: Built for shared team deployment with per-user session isolation and dual authentication support (`Authorization: Bearer <token>` or `?token=<token>`).
- **Encrypted SQLite Vault**: Embedded CGO-free pure-Go SQLite (`modernc.org/sqlite` in WAL mode) with AES-256-GCM token encryption at rest.
- **Self-Service Onboarding**: User-friendly web flow (`/auth/login`) that performs 3-legged OAuth (3LO) with PKCE, accessible site selection, and delivers an instant client configuration snippet for Claude Desktop and Cursor.
- **8 Consolidated Action Dispatchers**: Minimizes prompt token overhead while providing complete coverage:
  - **`read_jira`**: Issues, projects, comments, worklogs, transitions, boards, sprints, filters, dashboards, users, attachments.
  - **`write_jira`**: Create/edit issues, transitions, comments, links, sprints, versions, watchers, file uploads.
  - **`search_jira`**: JQL query engine and JQL autocomplete.
  - **`delete_jira`**: Issues, comments, attachments, worklogs (gated by Atlassian Cloud RBAC).
  - **`manage_jira`**: Projects, workflows, screens (gated by Atlassian Cloud RBAC).
  - **`read_confluence`**: Pages, blogposts, spaces, comments, versions, attachments, tasks, templates, permissions.
  - **`write_confluence`**: Pages, blogposts, comments, labels, reactions, moving/archiving, public links.
  - **`search_confluence`**: CQL query engine.
- **Rich Text & Prompt Optimization**:
  - Transparent bidirectional conversion between Markdown and Atlassian Document Format (ADF) / Confluence Storage Format (XHTML).
  - Compact view projections that strip 80-90% of redundant metadata to preserve LLM context tokens.
- **Resilience**: Automatic retry with exponential backoff and jitter on HTTP 429 (`Retry-After`) and 503.

---

## Architecture

See [GLOSSARY.md](./GLOSSARY.md) and Architecture Decision Records in [docs/adr/](./docs/adr/):
- [ADR 0001: Consolidated Action Dispatch](./docs/adr/0001-consolidated-action-dispatch.md)
- [ADR 0003: Markdown Primary Rich Text](./docs/adr/0003-markdown-primary-rich-text.md)
- [ADR 0004: Multi-User HTTP MCP Server with Encrypted SQLite Vault](./docs/adr/0004-multi-user-http-sqlite-vault.md)
- [ADR 0005: Atlassian Native RBAC Delegation](./docs/adr/0005-atlassian-native-rbac.md)

---

## Getting Started

### 1. Register Atlassian OAuth App
1. Go to the [Atlassian Developer Console](https://developer.atlassian.com/console/myapps/).
2. Create an **OAuth 2.0 (3LO)** app.
3. Under **Permissions**, add:
   - **User Identity API**: `read:me` (View user profile)
   - **Jira API**: `read:jira-work`, `write:jira-work`, `read:jira-user`, `manage:jira-project`, `manage:jira-configuration`, `offline_access`.
   - **Confluence API**: `read:confluence-space`, `read:confluence-content.all`, `write:confluence-content`, `read:confluence-user`, `search:confluence`, `offline_access`.
4. Under **Authorization**, set the Callback URL:
   - `http://localhost:8080/auth/callback` (or `https://your-domain.com/auth/callback`).
5. Copy your **Client ID** and **Client Secret**.

### 2. Run the Server

```bash
export ATLASSIAN_CLIENT_ID="your-client-id"
export ATLASSIAN_CLIENT_SECRET="your-client-secret"
export ATLASSIAN_MCP_BASE_URL="http://localhost:8080" # or your public domain

# Build and run
go build -o bin/atlassian-mcp ./cmd/atlassian-mcp
./bin/atlassian-mcp
```

### 3. Connect User Account
1. Open `http://localhost:8080` in your browser and click **Connect Atlassian Account** (or visit `/auth/login`).
2. Authorize with your Atlassian account and select your active Jira/Confluence workspace.
3. Copy your unique **MCP API Key**.

### 4. Configure Your LLM Client

#### Claude Desktop (`claude_desktop_config.json`)
```json
{
  "mcpServers": {
    "atlassian": {
      "url": "http://localhost:8080/sse",
      "headers": {
        "Authorization": "Bearer mcp_YOUR_USER_API_KEY"
      }
    }
  }
}
```

#### Clients Supporting URL Query Token (e.g. Cursor, Web Gateways)
```text
http://localhost:8080/sse?token=mcp_YOUR_USER_API_KEY
```

---

## Tool Reference

Every tool accepts case-insensitive action names (`get_issue`, `getIssue`, `getJiraIssue`).

### 1. `read_jira`
```json
{
  "action": "get_issue",
  "issue_key": "PROJ-101"
}
```
Available actions: `get_issue`, `list_projects`, `list_issue_types`, `list_transitions`, `list_comments`, `list_worklogs`, `list_boards`, `list_sprints`, `list_filters`, `list_dashboards`, `lookup_users`, `download_attachment`.

### 2. `write_jira`
```json
{
  "action": "create_issue",
  "project_key": "PROJ",
  "title": "Implement OAuth flow",
  "issue_type": "Task",
  "body": "## Overview\n* Write auth handlers\n* Encrypt tokens"
}
```
Available actions: `create_issue`, `edit_issue`, `transition_issue`, `add_comment`, `edit_comment`, `create_link`, `edit_link`, `manage_sprint`, `manage_version`, `watch_issue`, `upload_attachment`, `convert_hierarchy`.

### 3. `search_jira`
```json
{
  "action": "search_issues",
  "jql": "project = PROJ AND status = 'In Progress' ORDER BY updated DESC",
  "max_results": 10
}
```

### 4. `delete_jira` (Admin-enabled)
```json
{
  "action": "delete_issue",
  "issue_key": "PROJ-999"
}
```

### 5. `manage_jira` (Admin-enabled)
```json
{
  "action": "create_project",
  "project_key": "NEWPROJ",
  "title": "New Initiative",
  "lead_account_id": "account-id-123"
}
```

### 6. `read_confluence`
```json
{
  "action": "get_content",
  "content_id": "12345678"
}
```
Available actions: `get_content`, `list_spaces`, `get_space`, `list_content`, `list_comments`, `list_versions`, `list_attachments`, `get_permissions`, `list_tasks`, `list_templates`, `download_attachment`.

### 7. `write_confluence`
```json
{
  "action": "create_content",
  "space_id": "12345",
  "title": "System Architecture",
  "body": "# Architecture Overview\n\n```go\nfunc main() {}\n```"
}
```

### 8. `search_confluence`
```json
{
  "action": "search_content",
  "cql": "type = page AND space = 'ENG' AND text ~ 'Architecture'"
}
```

---

## Configuration Reference

| Environment Variable | CLI Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | `-port` | `8080` | HTTP listen port |
| `HOST` | `-host` | `0.0.0.0` | Bind IP address |
| `ATLASSIAN_MCP_BASE_URL` | `-base-url` | `http://localhost:8080` | Public base URL for OAuth callbacks |
| `ATLASSIAN_CLIENT_ID` | `-client-id` | `""` | Atlassian OAuth 2.0 Client ID |
| `ATLASSIAN_CLIENT_SECRET` | `-client-secret` | `""` | Atlassian OAuth 2.0 Client Secret |
| `ATLASSIAN_MCP_DB_PATH` | `-db-path` | `data/mcp_vault.db` | Path to SQLite vault database |
| `ATLASSIAN_MCP_KEY_PATH` | `-key-path` | `data/master.key` | Path to master encryption key file |
| `ATLASSIAN_MCP_ENCRYPTION_KEY` | - | `""` | Optional 32-byte master encryption key |
| `ATLASSIAN_MCP_DOWNLOAD_DIR` | `-download-dir` | `data/downloads` | Directory for downloaded attachments |

---

## Testing

Run unit and integration test suite:
```bash
go test -v ./...
```
