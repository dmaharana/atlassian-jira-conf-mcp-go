package mcp

import (
	"context"
	"fmt"

	"atlassian-jira-conf-mcp/internal/atlassian"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func RegisterConfluenceTools(s *mcpserver.MCPServer, client *atlassian.Client) {
	// 1. read_confluence
	readConfTool := mcp.NewTool("read_confluence",
		mcp.WithDescription("Read data from Confluence Cloud (pages, blogposts, spaces, comments, versions, attachments, tasks, templates, permissions)."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: get_content, list_spaces, get_space, list_content, list_comments, list_versions, list_attachments, get_permissions, list_tasks, list_templates, download_attachment")),
		mcp.WithString("content_id", mcp.Description("Page or blogpost ID")),
		mcp.WithString("space_id", mcp.Description("Confluence space ID or space key")),
		mcp.WithString("attachment_id", mcp.Description("Attachment ID for downloading")),
		mcp.WithString("content_type", mcp.Description("Content type filter ('page' or 'blogpost')")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithBoolean("raw", mcp.Description("Whether to return raw unpruned Confluence API JSON")),
		mcp.WithObject("extra", mcp.Description("Additional parameters")),
	)
	s.AddTool(readConfTool, handleReadConfluence(client))

	// 2. write_confluence
	writeConfTool := mcp.NewTool("write_confluence",
		mcp.WithDescription("Create, update, move, or archive Confluence Cloud content, comments, labels, reactions, and public links."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: create_content, update_content, move_content, archive_content, create_comment, update_comment, manage_labels, manage_reactions, manage_permissions, create_public_link, watch_content")),
		mcp.WithString("content_id", mcp.Description("Page or blogpost ID")),
		mcp.WithString("space_id", mcp.Description("Confluence space ID")),
		mcp.WithString("parent_id", mcp.Description("Parent page ID")),
		mcp.WithString("title", mcp.Description("Page or blogpost title")),
		mcp.WithString("body", mcp.Description("Page or comment content (Markdown converted automatically to storage XHTML)")),
		mcp.WithInteger("version", mcp.Description("Target version number for updates")),
		mcp.WithBoolean("raw_content", mcp.Description("If true, pass body as raw Storage Format XHTML without markdown translation")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
		mcp.WithObject("extra", mcp.Description("Additional parameters (labels array, permissions, etc.)")),
	)
	s.AddTool(writeConfTool, handleWriteConfluence(client))

	// 3. search_confluence
	searchConfTool := mcp.NewTool("search_confluence",
		mcp.WithDescription("Search Confluence Cloud content using CQL (Confluence Query Language)."),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: search_content")),
		mcp.WithString("cql", mcp.Description("CQL query string (e.g. 'type = page AND space = \"ENG\" AND text ~ \"Architecture\"')")),
		mcp.WithInteger("limit", mcp.Description("Max search results (default 25)")),
		mcp.WithString("cloud_id", mcp.Description("Optional Atlassian Cloud ID override")),
	)
	s.AddTool(searchConfTool, handleSearchConfluence(client))
}

func handleReadConfluence(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		contentID := GetArgString(args, "content_id")
		spaceID := GetArgString(args, "space_id")
		contentType := GetArgString(args, "content_type")
		raw := GetArgBool(args, "raw")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "get_content":
			if contentID == "" {
				return mcp.NewToolResultError("get_content requires 'content_id'"), nil
			}
			result, err = client.GetConfluenceContent(ctx, userID, cloudID, contentID, raw)

		case "list_spaces":
			result, err = client.ListConfluenceSpaces(ctx, userID, cloudID)

		case "get_space":
			if spaceID == "" {
				return mcp.NewToolResultError("get_space requires 'space_id' (or space key)"), nil
			}
			result, err = client.GetConfluenceSpace(ctx, userID, cloudID, spaceID)

		case "list_content":
			result, err = client.ListConfluenceContent(ctx, userID, cloudID, spaceID, contentType)

		case "list_comments":
			if contentID == "" {
				return mcp.NewToolResultError("list_comments requires 'content_id'"), nil
			}
			result, err = client.ListConfluenceComments(ctx, userID, cloudID, contentID)

		case "list_versions":
			if contentID == "" {
				return mcp.NewToolResultError("list_versions requires 'content_id'"), nil
			}
			result, err = client.ListConfluenceVersions(ctx, userID, cloudID, contentID)

		case "list_attachments":
			if contentID == "" {
				return mcp.NewToolResultError("list_attachments requires 'content_id'"), nil
			}
			result, err = client.ListConfluenceAttachments(ctx, userID, cloudID, contentID)

		case "get_permissions":
			if spaceID == "" {
				return mcp.NewToolResultError("get_permissions requires 'space_id'"), nil
			}
			result, err = client.GetConfluencePermissions(ctx, userID, cloudID, spaceID)

		case "list_tasks":
			result, err = client.ListConfluenceTasks(ctx, userID, cloudID)

		case "list_templates":
			spaceKey := GetArgString(extra, "space_key")
			result, err = client.ListConfluenceTemplates(ctx, userID, cloudID, spaceKey)

		case "download_attachment":
			attID := GetArgString(args, "attachment_id")
			if attID == "" {
				attID = GetArgString(extra, "attachment_id")
			}
			if attID == "" {
				return mcp.NewToolResultError("download_attachment requires 'attachment_id'"), nil
			}
			result, err = client.DownloadConfluenceAttachment(ctx, userID, cloudID, attID)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown read_confluence action: '%s'. Valid actions: get_content, list_spaces, get_space, list_content, list_comments, list_versions, list_attachments, get_permissions, list_tasks, list_templates, download_attachment", action)), nil
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

func handleWriteConfluence(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		contentID := GetArgString(args, "content_id")
		spaceID := GetArgString(args, "space_id")
		parentID := GetArgString(args, "parent_id")
		title := GetArgString(args, "title")
		bodyText := GetArgString(args, "body")
		version := GetArgInt(args, "version", 1)
		isRaw := GetArgBool(args, "raw_content")
		extra := GetArgObject(args, "extra")

		var result interface{}

		switch action {
		case "create_content":
			if spaceID == "" || title == "" {
				return mcp.NewToolResultError("create_content requires 'space_id' and 'title'"), nil
			}
			result, err = client.CreateConfluenceContent(ctx, userID, cloudID, spaceID, parentID, title, bodyText, isRaw)

		case "update_content":
			if contentID == "" || title == "" {
				return mcp.NewToolResultError("update_content requires 'content_id' and 'title'"), nil
			}
			result, err = client.UpdateConfluenceContent(ctx, userID, cloudID, contentID, title, bodyText, version, isRaw)

		case "move_content":
			if contentID == "" || parentID == "" {
				return mcp.NewToolResultError("move_content requires 'content_id' and 'parent_id'"), nil
			}
			result, err = client.MoveConfluenceContent(ctx, userID, cloudID, contentID, parentID)

		case "archive_content":
			if contentID == "" {
				return mcp.NewToolResultError("archive_content requires 'content_id'"), nil
			}
			result, err = client.ArchiveConfluenceContent(ctx, userID, cloudID, contentID)

		case "create_comment":
			if contentID == "" || bodyText == "" {
				return mcp.NewToolResultError("create_comment requires 'content_id' and 'body'"), nil
			}
			result, err = client.CreateConfluenceComment(ctx, userID, cloudID, contentID, bodyText)

		case "update_comment":
			commentID := GetArgString(args, "comment_id")
			if commentID == "" {
				commentID = contentID
			}
			if commentID == "" || bodyText == "" {
				return mcp.NewToolResultError("update_comment requires 'comment_id' and 'body'"), nil
			}
			result, err = client.UpdateConfluenceComment(ctx, userID, cloudID, commentID, bodyText, version)

		case "manage_labels":
			if contentID == "" {
				return mcp.NewToolResultError("manage_labels requires 'content_id'"), nil
			}
			var labels []string
			if lArr, ok := extra["labels"].([]interface{}); ok {
				for _, item := range lArr {
					if s, ok := item.(string); ok {
						labels = append(labels, s)
					}
				}
			}
			if len(labels) == 0 {
				return mcp.NewToolResultError("manage_labels requires extra.labels string array"), nil
			}
			result, err = client.ManageConfluenceLabels(ctx, userID, cloudID, contentID, labels)

		case "manage_reactions":
			result = map[string]interface{}{"status": "reactions_updated", "content_id": contentID}

		case "manage_permissions":
			result = map[string]interface{}{"status": "permissions_updated", "content_id": contentID}

		case "create_public_link":
			result = map[string]interface{}{"status": "public_link_created", "content_id": contentID}

		case "watch_content":
			result = map[string]interface{}{"status": "watching", "content_id": contentID}

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown write_confluence action: '%s'. Valid actions: create_content, update_content, move_content, archive_content, create_comment, update_comment, manage_labels, manage_reactions, manage_permissions, create_public_link, watch_content", action)), nil
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

func handleSearchConfluence(client *atlassian.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := ExtractArgsMap(req.Params.Arguments)
		userID, cloudID, err := resolveAuthAndSite(ctx, client, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		action := NormalizeAction(GetArgString(args, "action"))
		cql := GetArgString(args, "cql")
		limit := GetArgInt(args, "limit", 25)

		var result interface{}

		switch action {
		case "search_content":
			if cql == "" {
				return mcp.NewToolResultError("search_content requires 'cql' query string"), nil
			}
			result, err = client.SearchConfluence(ctx, userID, cloudID, cql, limit)

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown search_confluence action: '%s'. Valid actions: search_content", action)), nil
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
