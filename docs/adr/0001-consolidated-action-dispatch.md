# Consolidated Action-Based MCP Tool Dispatch

## Context & Decision
Exposing individual MCP tools for all Jira and Confluence operations produces over 40 discrete tool definitions, cluttering the LLM's system prompt and degrading tool selection reliability. We decided to expose 8 consolidated action dispatcher tools (`read_jira`, `write_jira`, `search_jira`, `delete_jira`, `manage_jira`, `read_confluence`, `write_confluence`, `search_confluence`), each dispatching operations via an `action` argument. This minimizes token consumption while cleanly preserving read, write, and administrative security boundaries.

## Considered Options
- **Granular Individual Tools**: Separate tool for each endpoint (e.g. `get_jira_issue`, `create_jira_issue`). Rejected due to prompt token bloat and hallucinated tool cross-calls.
- **Single Monolithic Tool**: One general `atlassian_api` tool. Rejected because it destroys semantic categorization and makes administrative permission gating coarse.
