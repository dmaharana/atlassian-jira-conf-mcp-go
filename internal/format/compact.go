package format

type CompactJiraIssue struct {
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	IssueType   string   `json:"issue_type"`
	Priority    string   `json:"priority,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Reporter    string   `json:"reporter,omitempty"`
	Description string   `json:"description,omitempty"`
	Created     string   `json:"created,omitempty"`
	Updated     string   `json:"updated,omitempty"`
	Comments    []string `json:"comments,omitempty"`
}

type CompactJiraSearchResult struct {
	Total      int                `json:"total"`
	StartAt    int                `json:"start_at"`
	MaxResults int                `json:"max_results"`
	Issues     []CompactJiraIssue `json:"issues"`
}

type CompactConfluenceContent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	SpaceKey  string `json:"space_key,omitempty"`
	SpaceID   string `json:"space_id,omitempty"`
	Version   int    `json:"version,omitempty"`
	Author    string `json:"author,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Body      string `json:"body,omitempty"`
}

// ProjectJiraIssue extracts the compact view from a raw Jira issue payload.
func ProjectJiraIssue(raw map[string]interface{}) CompactJiraIssue {
	out := CompactJiraIssue{
		Key: getString(raw, "key"),
	}

	fields, _ := raw["fields"].(map[string]interface{})
	if fields == nil {
		return out
	}

	out.Summary = getString(fields, "summary")
	out.Created = getString(fields, "created")
	out.Updated = getString(fields, "updated")

	if status, ok := fields["status"].(map[string]interface{}); ok {
		out.Status = getString(status, "name")
	}
	if it, ok := fields["issuetype"].(map[string]interface{}); ok {
		out.IssueType = getString(it, "name")
	}
	if prio, ok := fields["priority"].(map[string]interface{}); ok {
		out.Priority = getString(prio, "name")
	}
	if assignee, ok := fields["assignee"].(map[string]interface{}); ok {
		out.Assignee = getString(assignee, "displayName")
	}
	if reporter, ok := fields["reporter"].(map[string]interface{}); ok {
		out.Reporter = getString(reporter, "displayName")
	}

	// ADF description to markdown
	if descRaw := fields["description"]; descRaw != nil {
		out.Description = ADFToMarkdown(descRaw)
	}

	// Extract comments
	if commentContainer, ok := fields["comment"].(map[string]interface{}); ok {
		if commentsList, ok := commentContainer["comments"].([]interface{}); ok {
			for _, c := range commentsList {
				if cMap, ok := c.(map[string]interface{}); ok {
					author := "Unknown"
					if a, ok := cMap["author"].(map[string]interface{}); ok {
						author = getString(a, "displayName")
					}
					bodyText := ADFToMarkdown(cMap["body"])
					created := getString(cMap, "created")
					out.Comments = append(out.Comments, author+" ("+created+"): "+bodyText)
				}
			}
		}
	}

	return out
}

// ProjectConfluenceContent extracts the compact view from a Confluence REST v2 page/blogpost.
func ProjectConfluenceContent(raw map[string]interface{}) CompactConfluenceContent {
	out := CompactConfluenceContent{
		ID:        getString(raw, "id"),
		Type:      getString(raw, "type"),
		Status:    getString(raw, "status"),
		Title:     getString(raw, "title"),
		SpaceID:   getString(raw, "spaceId"),
		CreatedAt: getString(raw, "createdAt"),
	}

	if ver, ok := raw["version"].(map[string]interface{}); ok {
		if num, ok := ver["number"].(float64); ok {
			out.Version = int(num)
		}
		if author, ok := ver["authorId"].(string); ok {
			out.Author = author
		}
	}

	// Confluence v2 body: body.storage.value or body.atlas_doc_format.value
	if bodyObj, ok := raw["body"].(map[string]interface{}); ok {
		if storage, ok := bodyObj["storage"].(map[string]interface{}); ok {
			val := getString(storage, "value")
			out.Body = StorageFormatToMarkdown(val)
		} else if atlasDoc, ok := bodyObj["atlas_doc_format"].(map[string]interface{}); ok {
			// atlas doc format
			if val := atlasDoc["value"]; val != nil {
				out.Body = ADFToMarkdown(val)
			}
		}
	}

	return out
}

func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}
