# Atlassian Jira & Confluence MCP Server

A Model Context Protocol (MCP) server providing structured LLM tools to interact with Atlassian Jira and Confluence Cloud workspaces.

## Language

**Cloud Resource**:
An Atlassian Cloud site identified by a unique `cloudId` and URL domain (e.g., `https://example.atlassian.net`).
_Avoid_: Tenant, Server, Realm, Organization

**Approved Site**:
The specific Atlassian Cloud site (`cloudId`, URL) selected and bound to a user profile in the database.
_Avoid_: Default tenant, Connected workspace

**User Token Vault**:
The local SQLite database storing user OAuth tokens encrypted at rest with AES-256-GCM.
_Avoid_: Keyring, Secret store, Credential file

**MCP User API Key**:
A high-entropy bearer token issued to an authenticated user to authorize their HTTP/SSE MCP connections.
_Avoid_: Session cookie, Client token, Personal token

**Action Dispatcher**:
A consolidated MCP tool that routes incoming requests to a specific Atlassian operation based on an `action` identifier.
_Avoid_: God tool, Generic endpoint, Multi-tool

**Admin Tool**:
A privileged MCP tool performing destructive deletions or modifying site configuration (`delete_jira`, `manage_jira`), whose execution permissions are delegated directly to Atlassian Cloud's native RBAC.
_Avoid_: Sudo tool, Root tool, Local admin tool

**Storage Format**:
Confluence's canonical XHTML-based storage representation for pages, blog posts, and macros.
_Avoid_: Confluence HTML, Wiki markup

**Atlassian Document Format (ADF)**:
Jira's JSON-based schema representing rich text in issue descriptions and comments.
_Avoid_: Jira markdown, Rich text JSON

**Compact View**:
A token-efficient projection of an Atlassian entity (issue, page) retaining only core fields.
_Avoid_: Summary view, Short schema, Truncated JSON

**Top-Level Parameter**:
A high-frequency argument exposed directly at the tool's root schema level alongside an extra parameter bag.
_Avoid_: Root prop, Primary argument

**Backoff Handler**:
An HTTP transport decorator that intercepts 429 and 503 responses and delays retries according to Atlassian's `Retry-After` directive.
_Avoid_: Throttler, Waiter

