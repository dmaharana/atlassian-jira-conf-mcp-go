package mcp

import (
	"context"
	"fmt"

	"atlassian-jira-conf-mcp/internal/atlassian"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func RegisterJiraTools(s *mcpserver.MCPServer, client *atlassian.Client) {
	// 1. read_jira
	readJiraTool := mcp.NewTool("read_jira",
		mcp.WithDescription("Read data from Jira Cloud (issues, projects, comments, worklogs, transitions, boards, sprints, filters, dashboards, users, attachments)."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: get_issue, list_projects, list_issue_types, list_transitions, list_comments, list_worklogs, list_boards, list_sprints, list_filters, list_dashboards, lookup_users, download_attachment")),
		mcp.WithString("issue_key", mcp.Description("Jira issue key (e.g., 'PROJ-123')")),
		mcp.WithString("project_key", mcp.Description("Jira project key (e.g., 'PROJ')")),
		mcp.WithString("board_id", mcp.Description("Board ID for sprint queries")),
		mcp.WithString("attachment_id", mcp.Description("Attachment ID for downloading files")),
		mcp.WithString("query", mcp.Description("Search text for lookup_users")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithBoolean("raw", mcp.Description("Whether to return raw unpruned Jira API JSON")),
		mcp.WithObject("extra", mcp.Description("Additional action-specific parameters")),
	)
	s.AddTool(readJiraTool, handleReadJira(client))

	// 2. write_jira
	writeJiraTool := mcp.NewTool("write_jira",
		mcp.WithDescription("Create or update data in Jira Cloud (create/edit issue, transition issue, add/edit comments, link issues, manage sprints/versions, watch issues, upload attachments)."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: create_issue, edit_issue, transition_issue, add_comment, edit_comment, create_link, edit_link, manage_sprint, manage_version, watch_issue, upload_attachment, convert_hierarchy")),
		mcp.WithString("issue_key", mcp.Description("Jira issue key (e.g., 'PROJ-123')")),
		mcp.WithString("project_key", mcp.Description("Jira project key (e.g., 'PROJ')")),
		mcp.WithString("title", mcp.Description("Issue summary / title")),
		mcp.WithString("body", mcp.Description("Markdown description or comment text")),
		mcp.WithString("issue_type", mcp.Description("Issue type name (e.g. 'Task', 'Bug', 'Story')")),
		mcp.WithString("transition_id", mcp.Description("Workflow transition ID")),
		mcp.WithString("comment_id", mcp.Description("Comment ID to edit")),
		mcp.WithString("file_path", mcp.Description("Local file path for upload_attachment")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithObject("extra", mcp.Description("Additional parameters (e.g., custom fields, link types, sprint/version details)")),
	)
	s.AddTool(writeJiraTool, handleWriteJira(client))

	// 3. search_jira
	searchJiraTool := mcp.NewTool("search_jira",
		mcp.WithDescription("Search Jira Cloud issues using JQL (Jira Query Language) or autocomplete JQL clauses."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: search_issues, get_filters, autocomplete_jql")),
		mcp.WithString("jql", mcp.Description("JQL query string (e.g., 'project = PROJ AND status = \"In Progress\" ORDER BY created DESC')")),
		mcp.WithInteger("start_at", mcp.Description("Pagination start offset (default 0)")),
		mcp.WithInteger("max_results", mcp.Description("Max results to return (default 25)")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithBoolean("raw", mcp.Description("Whether to return raw unpruned Jira API JSON")),
		mcp.WithObject("extra", mcp.Description("Additional query parameters")),
	)
	s.AddTool(searchJiraTool, handleSearchJira(client))

	// 4. delete_jira (Admin-enabled)
	deleteJiraTool := mcp.NewTool("delete_jira",
		mcp.WithDescription("Permanently delete Jira Cloud resources (issues, comments, attachments, worklogs). Requires appropriate Jira administrative or project permissions."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: delete_issue, delete_comment, delete_attachment, delete_worklog")),
		mcp.WithString("issue_key", mcp.Description("Jira issue key")),
		mcp.WithString("comment_id", mcp.Description("Comment ID to delete")),
		mcp.WithString("attachment_id", mcp.Description("Attachment ID to delete")),
		mcp.WithString("worklog_id", mcp.Description("Worklog ID to delete")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
	)
	s.AddTool(deleteJiraTool, handleDeleteJira(client))

	// 5. manage_jira (Admin-enabled)
	manageJiraTool := mcp.NewTool("manage_jira",
		mcp.WithDescription("Manage Jira Cloud workspace configuration (create/update projects, inspect workflows, inspect screens). Requires Jira administration privileges."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: create_project, update_project, get_workflows, update_workflows, get_screens, update_screens")),
		mcp.WithString("project_key", mcp.Description("Jira project key")),
		mcp.WithString("title", mcp.Description("Project name")),
		mcp.WithString("body", mcp.Description("Project description")),
		mcp.WithString("lead_account_id", mcp.Description("Account ID for project lead")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithObject("extra", mcp.Description("Additional project or scheme configuration parameters")),
	)
	s.AddTool(manageJiraTool, handleManageJira(client))
}

func resolveAuthAndSite(ctx context.Context, client *atlassian.Client, args map[string]interface{}) (userID, cloudID string, err error) {
	user, ok := UserFromContext(ctx)
	if !ok || user == nil {
		return "", "", ErrUnauthenticated
	}

	overrideCloudID := GetArgString(args, "cloud_id")
	site, err := client.ResolveSite(user.ID, overrideCloudID)
	if err != nil {
		return "", "", fmt.Errorf("resolve active atlassian site: %w", err)
	}

	return user.ID, site.CloudID, nil
}

func handleReadJira(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		raw := GetArgBool(args, "raw")
		issueKey := GetArgString(args, "issue_key")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "get_issue":
			if issueKey == "" {
				return mcp.NewToolResultError("Missing required parameter: 'issue_key'"), nil
			}
			result, err = client.GetJiraIssue(ctx, userID, cloudID, issueKey, raw)

		case "list_projects":
			result, err = client.ListJiraProjects(ctx, userID, cloudID)

		case "list_issue_types":
			result, err = client.ListJiraIssueTypes(ctx, userID, cloudID)

		case "list_transitions":
			if issueKey == "" {
				return mcp.NewToolResultError("Missing required parameter: 'issue_key'"), nil
			}
			result, err = client.ListJiraTransitions(ctx, userID, cloudID, issueKey)

		case "list_comments":
			if issueKey == "" {
				return mcp.NewToolResultError("Missing required parameter: 'issue_key'"), nil
			}
			result, err = client.ListJiraComments(ctx, userID, cloudID, issueKey)

		case "list_worklogs":
			if issueKey == "" {
				return mcp.NewToolResultError("Missing required parameter: 'issue_key'"), nil
			}
			result, err = client.ListJiraWorklogs(ctx, userID, cloudID, issueKey)

		case "list_boards":
			result, err = client.ListJiraBoards(ctx, userID, cloudID)

		case "list_sprints":
			boardID := GetArgString(args, "board_id")
			if boardID == "" {
				boardID = GetArgString(extra, "board_id")
			}
			if boardID == "" {
				return mcp.NewToolResultError("Missing required parameter: 'board_id'"), nil
			}
			result, err = client.ListJiraSprints(ctx, userID, cloudID, boardID)

		case "list_filters":
			result, err = client.ListJiraFilters(ctx, userID, cloudID)

		case "list_dashboards":
			result, err = client.ListJiraDashboards(ctx, userID, cloudID)

		case "lookup_users":
			q := GetArgString(args, "query")
			if q == "" {
				q = GetArgString(extra, "query")
			}
			result, err = client.LookupJiraUsers(ctx, userID, cloudID, q)

		case "download_attachment":
			attID := GetArgString(args, "attachment_id")
			if attID == "" {
				attID = GetArgString(extra, "attachment_id")
			}
			if attID == "" {
				return mcp.NewToolResultError("Missing required parameter: 'attachment_id'"), nil
			}
			result, err = client.DownloadJiraAttachment(ctx, userID, cloudID, attID)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown read_jira action: '%s'. Valid actions are: get_issue, list_projects, list_issue_types, list_transitions, list_comments, list_worklogs, list_boards, list_sprints, list_filters, list_dashboards, lookup_users, download_attachment", action)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := FormatJSONResult(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Format result error: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}
}

func handleWriteJira(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		issueKey := GetArgString(args, "issue_key")
		projectKey := GetArgString(args, "project_key")
		title := GetArgString(args, "title")
		bodyText := GetArgString(args, "body")
		issueType := GetArgString(args, "issue_type")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "create_issue":
			if projectKey == "" || title == "" {
				return mcp.NewToolResultError("create_issue requires 'project_key' and 'title' (summary)"), nil
			}
			result, err = client.CreateJiraIssue(ctx, userID, cloudID, projectKey, title, issueType, bodyText, extra)

		case "edit_issue":
			if issueKey == "" {
				return mcp.NewToolResultError("edit_issue requires 'issue_key'"), nil
			}
			result, err = client.EditJiraIssue(ctx, userID, cloudID, issueKey, title, bodyText, extra)

		case "transition_issue":
			transID := GetArgString(args, "transition_id")
			if transID == "" {
				transID = GetArgString(extra, "transition_id")
			}
			if issueKey == "" || transID == "" {
				return mcp.NewToolResultError("transition_issue requires 'issue_key' and 'transition_id'"), nil
			}
			result, err = client.TransitionJiraIssue(ctx, userID, cloudID, issueKey, transID)

		case "add_comment":
			if issueKey == "" || bodyText == "" {
				return mcp.NewToolResultError("add_comment requires 'issue_key' and 'body' (comment text)"), nil
			}
			result, err = client.AddJiraIssueComment(ctx, userID, cloudID, issueKey, bodyText)

		case "edit_comment":
			commID := GetArgString(args, "comment_id")
			if commID == "" {
				commID = GetArgString(extra, "comment_id")
			}
			if issueKey == "" || commID == "" || bodyText == "" {
				return mcp.NewToolResultError("edit_comment requires 'issue_key', 'comment_id', and 'body'"), nil
			}
			result, err = client.EditJiraIssueComment(ctx, userID, cloudID, issueKey, commID, bodyText)

		case "add_or_edit_comment":
			commID := GetArgString(args, "comment_id")
			if commID == "" {
				commID = GetArgString(extra, "comment_id")
			}
			if issueKey == "" || bodyText == "" {
				return mcp.NewToolResultError("add_or_edit_comment requires 'issue_key' and 'body'"), nil
			}
			if commID != "" {
				result, err = client.EditJiraIssueComment(ctx, userID, cloudID, issueKey, commID, bodyText)
			} else {
				result, err = client.AddJiraIssueComment(ctx, userID, cloudID, issueKey, bodyText)
			}

		case "create_link", "edit_link":
			linkType := GetArgString(extra, "type")
			inward := GetArgString(extra, "inward_key")
			outward := GetArgString(extra, "outward_key")
			if linkType == "" {
				linkType = "Relates"
			}
			if inward == "" || outward == "" {
				return mcp.NewToolResultError("create_link requires extra.inward_key and extra.outward_key"), nil
			}
			result, err = client.CreateJiraLink(ctx, userID, cloudID, linkType, inward, outward)

		case "manage_sprint":
			sprintID := GetArgString(extra, "sprint_id")
			var issues []string
			if issList, ok := extra["issues"].([]interface{}); ok {
				for _, isItem := range issList {
					if s, ok := isItem.(string); ok {
						issues = append(issues, s)
					}
				}
			}
			if issueKey != "" && len(issues) == 0 {
				issues = append(issues, issueKey)
			}
			if sprintID == "" || len(issues) == 0 {
				return mcp.NewToolResultError("manage_sprint requires extra.sprint_id and issues list"), nil
			}
			result, err = client.ManageJiraSprint(ctx, userID, cloudID, sprintID, issues)

		case "manage_version":
			vName := GetArgString(extra, "version_name")
			vDesc := GetArgString(extra, "description")
			vRel := GetArgBool(extra, "released")
			if projectKey == "" || vName == "" {
				return mcp.NewToolResultError("manage_version requires 'project_key' and extra.version_name"), nil
			}
			result, err = client.ManageJiraVersion(ctx, userID, cloudID, projectKey, vName, vDesc, vRel)

		case "watch_issue":
			if issueKey == "" {
				return mcp.NewToolResultError("watch_issue requires 'issue_key'"), nil
			}
			accID := GetArgString(extra, "account_id")
			result, err = client.WatchJiraIssue(ctx, userID, cloudID, issueKey, accID)

		case "upload_attachment":
			filePath := GetArgString(args, "file_path")
			if filePath == "" {
				filePath = GetArgString(extra, "file_path")
			}
			if issueKey == "" || filePath == "" {
				return mcp.NewToolResultError("upload_attachment requires 'issue_key' and 'file_path'"), nil
			}
			result, err = client.UploadJiraAttachment(ctx, userID, cloudID, issueKey, filePath)

		case "convert_hierarchy":
			if issueKey == "" {
				return mcp.NewToolResultError("convert_hierarchy requires 'issue_key'"), nil
			}
			result, err = client.EditJiraIssue(ctx, userID, cloudID, issueKey, "", "", extra)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown write_jira action: '%s'. Valid actions are: create_issue, edit_issue, transition_issue, add_comment, edit_comment, create_link, edit_link, manage_sprint, manage_version, watch_issue, upload_attachment, convert_hierarchy", action)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := FormatJSONResult(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Format result error: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}
}

func handleSearchJira(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		jql := GetArgString(args, "jql")
		startAt := GetArgInt(args, "start_at", 0)
		maxResults := GetArgInt(args, "max_results", 25)
		raw := GetArgBool(args, "raw")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "search_issues":
			result, err = client.SearchJiraIssues(ctx, userID, cloudID, jql, startAt, maxResults, raw)

		case "get_filters":
			result, err = client.ListJiraFilters(ctx, userID, cloudID)

		case "autocomplete_jql":
			fieldName := GetArgString(extra, "field_name")
			fieldValue := GetArgString(extra, "field_value")
			result, err = client.AutocompleteJQL(ctx, userID, cloudID, fieldName, fieldValue)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown search_jira action: '%s'. Valid actions: search_issues, get_filters, autocomplete_jql", action)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := FormatJSONResult(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Format result error: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}
}

func handleDeleteJira(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		issueKey := GetArgString(args, "issue_key")
		commentID := GetArgString(args, "comment_id")
		attachmentID := GetArgString(args, "attachment_id")
		worklogID := GetArgString(args, "worklog_id")

		var result interface{}

		switch action {
		case "delete_issue":
			if issueKey == "" {
				return mcp.NewToolResultError("delete_issue requires 'issue_key'"), nil
			}
			result, err = client.DeleteJiraIssue(ctx, userID, cloudID, issueKey)

		case "delete_comment":
			if issueKey == "" || commentID == "" {
				return mcp.NewToolResultError("delete_comment requires 'issue_key' and 'comment_id'"), nil
			}
			result, err = client.DeleteJiraComment(ctx, userID, cloudID, issueKey, commentID)

		case "delete_attachment":
			if attachmentID == "" {
				return mcp.NewToolResultError("delete_attachment requires 'attachment_id'"), nil
			}
			result, err = client.DeleteJiraAttachment(ctx, userID, cloudID, attachmentID)

		case "delete_worklog":
			if issueKey == "" || worklogID == "" {
				return mcp.NewToolResultError("delete_worklog requires 'issue_key' and 'worklog_id'"), nil
			}
			result, err = client.DeleteJiraWorklog(ctx, userID, cloudID, issueKey, worklogID)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown delete_jira action: '%s'. Valid actions: delete_issue, delete_comment, delete_attachment, delete_worklog", action)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := FormatJSONResult(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Format result error: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}
}

func handleManageJira(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		projectKey := GetArgString(args, "project_key")
		title := GetArgString(args, "title")
		bodyText := GetArgString(args, "body")
		leadID := GetArgString(args, "lead_account_id")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "create_project":
			if projectKey == "" || title == "" || leadID == "" {
				return mcp.NewToolResultError("create_project requires 'project_key', 'title' (project name), and 'lead_account_id'"), nil
			}
			projType := GetArgString(extra, "project_type_key")
			result, err = client.CreateJiraProject(ctx, userID, cloudID, projectKey, title, projType, leadID)

		case "update_project":
			if projectKey == "" {
				return mcp.NewToolResultError("update_project requires 'project_key'"), nil
			}
			result, err = client.UpdateJiraProject(ctx, userID, cloudID, projectKey, title, bodyText)

		case "get_workflows", "update_workflows":
			result, err = client.GetJiraWorkflows(ctx, userID, cloudID)

		case "get_screens", "update_screens":
			result, err = client.GetJiraScreens(ctx, userID, cloudID)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown manage_jira action: '%s'. Valid actions: create_project, update_project, get_workflows, update_workflows, get_screens, update_screens", action)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := FormatJSONResult(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Format result error: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}
}
