# Atlassian Native RBAC Delegation for Admin Operations

**Status**: Accepted

## Context & Decision
The MCP server exposes both read/write and privileged administrative tools (`delete_jira`, `manage_jira`). Rather than maintaining an internal user role table and custom permission hierarchy in the local SQLite database—which introduces heavy administrative overhead and drifts from actual Atlassian Cloud permissions—we decided to expose the tools to all authenticated sessions and delegate authorization enforcement directly to Atlassian Cloud's native Role-Based Access Control (RBAC).

## Consequences
If a user without Atlassian administrative permissions invokes `delete_jira` or `manage_jira`, Atlassian Cloud returns an HTTP 403 Forbidden with exact diagnostic details. The MCP server translates this into a structured MCP error explaining the exact missing permission to the calling LLM.

## Considered Options
- **Local SQLite Admin Role Table**: Storing an `is_admin` boolean flag per user. Rejected because it requires constant manual operator sync when organization roles change in Atlassian.
- **Pre-flight Permission Probing**: Calling Jira's `/rest/api/3/mypermissions` before every tool invocation. Rejected due to latency and rate-limit overhead.
