package atlassian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"atlassian-jira-conf-mcp/internal/format"
)

func (c *Client) jiraBaseURL(cloudID string) string {
	return fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3", cloudID)
}

func (c *Client) jiraAgileURL(cloudID string) string {
	return fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/agile/1.0", cloudID)
}

// GetJiraIssue retrieves a single Jira issue.
func (c *Client) GetJiraIssue(ctx context.Context, userID, cloudID, issueKey string, raw bool) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("unmarshal issue response: %w", err)
	}

	if raw {
		return data, nil
	}
	return format.ProjectJiraIssue(data), nil
}

// ListJiraProjects lists available projects.
func (c *Client) ListJiraProjects(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/project", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraIssueTypes lists available issue types.
func (c *Client) ListJiraIssueTypes(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issuetype", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraTransitions lists available workflow transitions for an issue.
func (c *Client) ListJiraTransitions(ctx context.Context, userID, cloudID, issueKey string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/transitions", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraComments lists comments for an issue.
func (c *Client) ListJiraComments(ctx context.Context, userID, cloudID, issueKey string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/comment", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraWorklogs lists worklogs for an issue.
func (c *Client) ListJiraWorklogs(ctx context.Context, userID, cloudID, issueKey string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/worklog", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraBoards lists agile boards.
func (c *Client) ListJiraBoards(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/board", c.jiraAgileURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraSprints lists sprints for a given board ID.
func (c *Client) ListJiraSprints(ctx context.Context, userID, cloudID string, boardID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/board/%s/sprint", c.jiraAgileURL(cloudID), url.PathEscape(boardID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraFilters lists filters owned by or shared with the user.
func (c *Client) ListJiraFilters(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/filter/my", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// ListJiraDashboards lists system dashboards.
func (c *Client) ListJiraDashboards(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/dashboard", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// LookupJiraUsers searches for users by query string.
func (c *Client) LookupJiraUsers(ctx context.Context, userID, cloudID, query string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/user/search?query=%s", c.jiraBaseURL(cloudID), url.QueryEscape(query))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// DownloadJiraAttachment downloads an attachment by ID to the local filesystem bridge.
func (c *Client) DownloadJiraAttachment(ctx context.Context, userID, cloudID, attachmentID string) (map[string]interface{}, error) {
	// First get attachment metadata
	metaURL := fmt.Sprintf("%s/attachment/%s", c.jiraBaseURL(cloudID), url.PathEscape(attachmentID))
	metaBody, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, metaURL, nil, nil)
	if err != nil {
		return nil, err
	}

	var meta map[string]interface{}
	_ = json.Unmarshal(metaBody, &meta)

	filename, _ := meta["filename"].(string)
	if filename == "" {
		filename = fmt.Sprintf("attachment-%s.bin", attachmentID)
	}
	contentURL, _ := meta["content"].(string)
	if contentURL == "" {
		contentURL = fmt.Sprintf("%s/attachment/content/%s", c.jiraBaseURL(cloudID), url.PathEscape(attachmentID))
	}

	fileBytes, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, contentURL, nil, nil)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(c.downloadDir, 0700); err != nil {
		return nil, fmt.Errorf("create download dir: %w", err)
	}

	targetPath := filepath.Join(c.downloadDir, filename)
	if err := os.WriteFile(targetPath, fileBytes, 0600); err != nil {
		return nil, fmt.Errorf("write attachment file: %w", err)
	}

	return map[string]interface{}{
		"attachment_id": attachmentID,
		"filename":      filename,
		"file_path":     targetPath,
		"size_bytes":    len(fileBytes),
		"mime_type":     meta["mimeType"],
	}, nil
}

// CreateJiraIssue creates a new issue.
func (c *Client) CreateJiraIssue(ctx context.Context, userID, cloudID, projectKey, summary, issueType, description string, extra map[string]interface{}) (interface{}, error) {
	if issueType == "" {
		issueType = "Task"
	}

	fields := map[string]interface{}{
		"project": map[string]interface{}{
			"key": projectKey,
		},
		"summary": summary,
		"issuetype": map[string]interface{}{
			"name": issueType,
		},
	}

	if description != "" {
		fields["description"] = format.MarkdownToADF(description)
	}

	for k, v := range extra {
		fields[k] = v
	}

	payload := map[string]interface{}{"fields": fields}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issue", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// EditJiraIssue updates fields on an existing issue.
func (c *Client) EditJiraIssue(ctx context.Context, userID, cloudID, issueKey, summary, description string, extra map[string]interface{}) (interface{}, error) {
	fields := make(map[string]interface{})
	if summary != "" {
		fields["summary"] = summary
	}
	if description != "" {
		fields["description"] = format.MarkdownToADF(description)
	}
	for k, v := range extra {
		fields[k] = v
	}

	payload := map[string]interface{}{"fields": fields}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issue/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]interface{}{"success": true, "issue_key": issueKey}, nil
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// TransitionJiraIssue transitions an issue to another workflow state.
func (c *Client) TransitionJiraIssue(ctx context.Context, userID, cloudID, issueKey, transitionID string) (interface{}, error) {
	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionID,
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issue/%s/transitions", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]interface{}{"success": true, "issue_key": issueKey, "transition_id": transitionID}, nil
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// AddJiraIssueComment adds a new comment to an issue.
func (c *Client) AddJiraIssueComment(ctx context.Context, userID, cloudID, issueKey, bodyText string) (interface{}, error) {
	payload := map[string]interface{}{
		"body": format.MarkdownToADF(bodyText),
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issue/%s/comment", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// EditJiraIssueComment updates an existing comment.
func (c *Client) EditJiraIssueComment(ctx context.Context, userID, cloudID, issueKey, commentID, bodyText string) (interface{}, error) {
	payload := map[string]interface{}{
		"body": format.MarkdownToADF(bodyText),
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issue/%s/comment/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey), url.PathEscape(commentID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// CreateJiraLink links two issues together.
func (c *Client) CreateJiraLink(ctx context.Context, userID, cloudID, linkType, inwardKey, outwardKey string) (interface{}, error) {
	payload := map[string]interface{}{
		"type": map[string]interface{}{
			"name": linkType,
		},
		"inwardIssue": map[string]interface{}{
			"key": inwardKey,
		},
		"outwardIssue": map[string]interface{}{
			"key": outwardKey,
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/issueLink", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]interface{}{"success": true, "inward": inwardKey, "outward": outwardKey}, nil
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// ManageJiraSprint moves issues into a sprint.
func (c *Client) ManageJiraSprint(ctx context.Context, userID, cloudID string, sprintID string, issues []string) (interface{}, error) {
	payload := map[string]interface{}{
		"issues": issues,
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/sprint/%s/issue", c.jiraAgileURL(cloudID), url.PathEscape(sprintID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]interface{}{"success": true, "sprint_id": sprintID, "issues": issues}, nil
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// ManageJiraVersion creates or updates a release version.
func (c *Client) ManageJiraVersion(ctx context.Context, userID, cloudID, projectKey, versionName, description string, released bool) (interface{}, error) {
	payload := map[string]interface{}{
		"name":        versionName,
		"project":     projectKey,
		"description": description,
		"released":    released,
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/version", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// WatchJiraIssue adds the current or specified user to watch an issue.
func (c *Client) WatchJiraIssue(ctx context.Context, userID, cloudID, issueKey, accountID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/watchers", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	var payloadBytes []byte
	if accountID != "" {
		payloadBytes = []byte(strconv.Quote(accountID))
	}

	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, payloadBytes, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return map[string]interface{}{"success": true, "watched": issueKey}, nil
	}
	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// UploadJiraAttachment uploads a local file as an attachment to an issue.
func (c *Client) UploadJiraAttachment(ctx context.Context, userID, cloudID, issueKey, filePath string) (interface{}, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read local attachment file %s: %w", filePath, err)
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("write multipart data: %w", err)
	}
	w.Close()

	apiURL := fmt.Sprintf("%s/issue/%s/attachments", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	headers := map[string]string{
		"Content-Type":        w.FormDataContentType(),
		"X-Atlassian-Token":   "no-check",
	}

	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, b.Bytes(), headers)
	if err != nil {
		return nil, err
	}

	var resp interface{}
	_ = json.Unmarshal(body, &resp)
	return resp, nil
}

// SearchJiraIssues executes a JQL search query.
func (c *Client) SearchJiraIssues(ctx context.Context, userID, cloudID, jql string, startAt, maxResults int, raw bool) (interface{}, error) {
	if maxResults <= 0 {
		maxResults = 25
	}

	payload := map[string]interface{}{
		"jql":        jql,
		"startAt":    startAt,
		"maxResults": maxResults,
		"fields": []string{
			"summary", "status", "issuetype", "priority", "assignee", "reporter", "description", "created", "updated", "comment",
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/search", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}

	var rawResult map[string]interface{}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, fmt.Errorf("unmarshal search result: %w", err)
	}

	if raw {
		return rawResult, nil
	}

	total := 0
	if t, ok := rawResult["total"].(float64); ok {
		total = int(t)
	}

	var compactIssues []format.CompactJiraIssue
	if issuesList, ok := rawResult["issues"].([]interface{}); ok {
		for _, iss := range issuesList {
			if issMap, ok := iss.(map[string]interface{}); ok {
				compactIssues = append(compactIssues, format.ProjectJiraIssue(issMap))
			}
		}
	}

	return format.CompactJiraSearchResult{
		Total:      total,
		StartAt:    startAt,
		MaxResults: maxResults,
		Issues:     compactIssues,
	}, nil
}

// AutocompleteJQL retrieves JQL autocomplete suggestions.
func (c *Client) AutocompleteJQL(ctx context.Context, userID, cloudID, fieldName, fieldValue string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/jql/autocompletedata/suggestions?fieldName=%s&fieldValue=%s",
		c.jiraBaseURL(cloudID), url.QueryEscape(fieldName), url.QueryEscape(fieldValue))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// DeleteJiraIssue deletes an issue permanently (Admin-enabled).
func (c *Client) DeleteJiraIssue(ctx context.Context, userID, cloudID, issueKey string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey))
	_, _, err := c.ExecuteRequest(ctx, userID, http.MethodDelete, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "issue_key": issueKey}, nil
}

// DeleteJiraComment deletes an issue comment (Admin-enabled).
func (c *Client) DeleteJiraComment(ctx context.Context, userID, cloudID, issueKey, commentID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/comment/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey), url.PathEscape(commentID))
	_, _, err := c.ExecuteRequest(ctx, userID, http.MethodDelete, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "comment_id": commentID}, nil
}

// DeleteJiraAttachment deletes an attachment (Admin-enabled).
func (c *Client) DeleteJiraAttachment(ctx context.Context, userID, cloudID, attachmentID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/attachment/%s", c.jiraBaseURL(cloudID), url.PathEscape(attachmentID))
	_, _, err := c.ExecuteRequest(ctx, userID, http.MethodDelete, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "attachment_id": attachmentID}, nil
}

// DeleteJiraWorklog deletes a worklog entry (Admin-enabled).
func (c *Client) DeleteJiraWorklog(ctx context.Context, userID, cloudID, issueKey, worklogID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/issue/%s/worklog/%s", c.jiraBaseURL(cloudID), url.PathEscape(issueKey), url.PathEscape(worklogID))
	_, _, err := c.ExecuteRequest(ctx, userID, http.MethodDelete, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "worklog_id": worklogID}, nil
}

// CreateJiraProject creates a new Jira project (Admin-enabled).
func (c *Client) CreateJiraProject(ctx context.Context, userID, cloudID, key, name, projectTypeKey, leadAccountID string) (interface{}, error) {
	if projectTypeKey == "" {
		projectTypeKey = "software"
	}
	payload := map[string]interface{}{
		"key":            key,
		"name":           name,
		"projectTypeKey": projectTypeKey,
		"leadAccountId":  leadAccountID,
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/project", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPost, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// UpdateJiraProject updates project details (Admin-enabled).
func (c *Client) UpdateJiraProject(ctx context.Context, userID, cloudID, projectKey, name, description string) (interface{}, error) {
	payload := make(map[string]interface{})
	if name != "" {
		payload["name"] = name
	}
	if description != "" {
		payload["description"] = description
	}
	jsonBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/project/%s", c.jiraBaseURL(cloudID), url.PathEscape(projectKey))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodPut, apiURL, jsonBytes, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// GetJiraWorkflows inspects workflows (Admin-enabled).
func (c *Client) GetJiraWorkflows(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/workflow/search", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}

// GetJiraScreens inspects screens (Admin-enabled).
func (c *Client) GetJiraScreens(ctx context.Context, userID, cloudID string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/screens", c.jiraBaseURL(cloudID))
	body, _, err := c.ExecuteRequest(ctx, userID, http.MethodGet, apiURL, nil, nil)
	if err != nil {
		return nil, err
	}
	var data interface{}
	_ = json.Unmarshal(body, &data)
	return data, nil
}
