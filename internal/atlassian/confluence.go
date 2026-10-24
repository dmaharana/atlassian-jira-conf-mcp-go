package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"atlassian-jira-conf-mcp/internal/format"
)

func (c *Client) confV2URL(cloudID string) string {
	return fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s/wiki/api/v2", cloudID)
}

func (c *Client) confLegacyURL(cloudID string) string {
	return fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s/wiki/rest/api", cloudID)
}

// GetConfluenceContent retrieves a single page or blogpost.
func (c *Client) GetConfluenceContent(ctx context.Context, userID, cloudID, contentID string, raw bool) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/pages/%s?body-format=storage,atlas_doc_format", c.confV2URL(cloudID), url.PathEscape(contentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		// If 404, try blogpost
		apiURL = fmt.Sprintf("%s/blogposts/%s?body-format=storage,atlas_doc_format", c.confV2URL(cloudID), url.PathEscape(contentID))
		var blogErr error
		body, _, blogErr = c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
		if blogErr != nil {
			return nil, err
		}
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("unmarshal confluence content: %w", err)
	}

	if raw {
		return data, nil
	}
	return format.ProjectConfluenceContent(data), nil
}

// ListConfluenceSpaces lists spaces in the workspace.
func (c *Client) ListConfluenceSpaces(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/spaces", c.confV2URL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// GetConfluenceSpace retrieves space details by space key or ID.
func (c *Client) GetConfluenceSpace(ctx context.Context, userID, cloudID, spaceIDOrKey string) (interface{}, error) {
	// First try v2 by ID
	apiURL := fmt.Sprintf("%s/spaces/%s", c.confV2URL(cloudID), url.PathEscape(spaceIDOrKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err == nil {
		var data interface{}
		_ = json.Unmarshal(body, &data)
		return data, nil
	}

	// Try query by space key
	apiURL = fmt.Sprintf("%s/spaces?keys=%s", c.confV2URL(cloudID), url.QueryEscape(spaceIDOrKey))
	body, _, err = c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceContent lists pages or blogposts in a space.
func (c *Client) ListConfluenceContent(ctx context.Context, userID, cloudID, spaceID, contentType string) (interface{}, error) {
	endpoint := "pages"
	if contentType == "blogpost" {
		endpoint = "blogposts"
	}

	apiURL := fmt.Sprintf("%s/%s", c.confV2URL(cloudID), endpoint)
	if spaceID != "" {
		apiURL = fmt.Sprintf("%s?space-id=%s", apiURL, url.QueryEscape(spaceID))
	}

	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceComments lists footer comments on a page.
func (c *Client) ListConfluenceComments(ctx context.Context, userID, cloudID, contentID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/pages/%s/footer-comments?body-format=storage", c.confV2URL(cloudID), url.PathEscape(contentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceVersions lists versions of a page.
func (c *Client) ListConfluenceVersions(ctx context.Context, userID, cloudID, contentID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/pages/%s/versions", c.confV2URL(cloudID), url.PathEscape(contentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceAttachments lists attachments for a page.
func (c *Client) ListConfluenceAttachments(ctx context.Context, userID, cloudID, contentID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/pages/%s/attachments", c.confV2URL(cloudID), url.PathEscape(contentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// GetConfluencePermissions gets space permissions.
func (c *Client) GetConfluencePermissions(ctx context.Context, userID, cloudID, spaceID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/spaces/%s/permissions", c.confV2URL(cloudID), url.PathEscape(spaceID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceTasks lists inline tasks in Confluence.
func (c *Client) ListConfluenceTasks(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/tasks", c.confV2URL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListConfluenceTemplates lists page templates.
func (c *Client) ListConfluenceTemplates(ctx context.Context, userID, cloudID, spaceKey string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/template/page", c.confLegacyURL(cloudID))
	if spaceKey != "" {
		apiURL = fmt.Sprintf("%s?spaceKey=%s", apiURL, url.QueryEscape(spaceKey))
	}
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// DownloadConfluenceAttachment downloads an attachment from Confluence to local disk.
func (c *Client) DownloadConfluenceAttachment(ctx context.Context, userID, cloudID, attachmentID string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/attachments/%s", c.confV2URL(cloudID), url.PathEscape(attachmentID))
	metaBody, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}

	var meta map[string]interface{}
	_ = json.Unmarshal(metaBody, &meta)

	downloadLink, _ := meta["downloadLink"].(string)
	title, _ := meta["title"].(string)
	if title == "" {
		title = fmt.Sprintf("confluence-attachment-%s.bin", attachmentID)
	}

	if !strings.HasPrefix(downloadLink, "http") {
		downloadLink = fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s%s", cloudID, downloadLink)
	}

	fileBytes, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, downloadLink, nil, nil)
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll(c.downloadDir, 0700)
	targetPath := filepath.Join(c.downloadDir, title)
	if err := os.WriteFile(targetPath, fileBytes, 0600); err != nil {
		return nil, fmt.Errorf("write attachment file: %w", err)
	}

	return map[string]interface{}{
		"attachment_id": attachmentID,
		"filename":      title,
		"file_path":     targetPath,
		"size_bytes":    len(fileBytes),
		"media_type":    meta["mediaType"],
	}, nil
}

// CreateConfluenceContent creates a page in Confluence.
func (c *Client) CreateConfluenceContent(ctx context.Context, userID, cloudID, spaceID, parentID, title, bodyText string, isRaw bool) (interface{}, error) {
	storageValue := bodyText
	if !isRaw {
		storageValue = format.MarkdownToStorageFormat(bodyText)
	}

	payload := map[string]interface{}{
		"spaceId": spaceID,
		"status":  "current",
		"title":   title,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          storageValue,
		},
	}
	if parentID != "" {
		payload["parentId"] = parentID
	}

	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/pages", c.confV2URL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// UpdateConfluenceContent updates an existing page.
func (c *Client) UpdateConfluenceContent(ctx context.Context, userID, cloudID, pageID, title, bodyText string, version int, isRaw bool) (interface{}, error) {
	storageValue := bodyText
	if !isRaw {
		storageValue = format.MarkdownToStorageFormat(bodyText)
	}

	payload := map[string]interface{}{
		"id":     pageID,
		"status": "current",
		"title":  title,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          storageValue,
		},
		"version": map[string]interface{}{
			"number": version,
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/pages/%s", c.confV2URL(cloudID), url.PathEscape(pageID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// MoveConfluenceContent changes parent of a page.
func (c *Client) MoveConfluenceContent(ctx context.Context, userID, cloudID, pageID, newParentID string) (interface{}, error) {
	// First get existing page metadata to preserve title & version
	existing, err := c.GetConfluenceContent(ctx, userID, cloudID, pageID, true)
	if err != nil {
		return nil, err
	}

	pageMap, _ := existing.(map[string]interface{})
	title := pageMap["title"]
	verObj, _ := pageMap["version"].(map[string]interface{})
	currentVer, _ := verObj["number"].(float64)

	payload := map[string]interface{}{
		"id":       pageID,
		"status":   "current",
		"title":    title,
		"parentId": newParentID,
		"version": map[string]interface{}{
			"number": int(currentVer) + 1,
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/pages/%s", c.confV2URL(cloudID), url.PathEscape(pageID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// ArchiveConfluenceContent archives a page.
func (c *Client) ArchiveConfluenceContent(ctx context.Context, userID, cloudID, pageID string) (interface{}, error) {
	payload := map[string]interface{}{
		"pages": []map[string]interface{}{
			{"id": pageID},
		},
	}
	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/pages/archive", c.confV2URL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// CreateConfluenceComment adds a comment to a page.
func (c *Client) CreateConfluenceComment(ctx context.Context, userID, cloudID, pageID, bodyText string) (interface{}, error) {
	storageValue := format.MarkdownToStorageFormat(bodyText)
	payload := map[string]interface{}{
		"pageId": pageID,
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          storageValue,
		},
	}
	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/footer-comments", c.confV2URL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// UpdateConfluenceComment updates an existing footer comment.
func (c *Client) UpdateConfluenceComment(ctx context.Context, userID, cloudID, commentID, bodyText string, version int) (interface{}, error) {
	if version <= 0 {
		version = 1
	}
	storageValue := format.MarkdownToStorageFormat(bodyText)
	payload := map[string]interface{}{
		"id":     commentID,
		"status": "current",
		"body": map[string]interface{}{
			"representation": "storage",
			"value":          storageValue,
		},
		"version": map[string]interface{}{
			"number": version,
		},
	}
	jsonBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/footer-comments/%s", c.confV2URL(cloudID), url.PathEscape(commentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// ManageConfluenceLabels adds labels to a page.
func (c *Client) ManageConfluenceLabels(ctx context.Context, userID, cloudID, pageID string, labels []string) (interface{}, error) {
	var labelPayload []map[string]interface{}
	for _, l := range labels {
		labelPayload = append(labelPayload, map[string]interface{}{
			"prefix": "global",
			"name":   l,
		})
	}
	jsonBytes, _ := json.Marshal(labelPayload)
	apiURL := fmt.Sprintf("%s/pages/%s/labels", c.confV2URL(cloudID), url.PathEscape(pageID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// SearchConfluence executes a CQL (Confluence Query Language) query.
func (c *Client) SearchConfluence(ctx context.Context, userID, cloudID, cql string, limit int) (interface{}, error) {
	if limit <= 0 {
		limit = 25
	}
	apiURL := fmt.Sprintf("%s/search?cql=%s&limit=%d", c.confLegacyURL(cloudID), url.QueryEscape(cql), limit)
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal confluence search: %w", err)
	}

	return raw, nil
}
