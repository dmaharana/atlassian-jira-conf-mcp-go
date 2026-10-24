package format

import (
	"strings"
	"testing"
)

func TestMarkdownToADFAndBack(t *testing.T) {
	mdInput := `# Project Roadmap

Here is the description with [Atlassian Link](https://atlassian.com) and **bold text**.

* Task 1
* Task 2

` + "```go\nfmt.Println(\"hello\")\n```"

	adf := MarkdownToADF(mdInput)
	if adf["type"] != "doc" {
		t.Fatalf("expected doc type, got %v", adf["type"])
	}

	content, ok := adf["content"].([]interface{})
	if !ok || len(content) < 4 {
		t.Fatalf("expected at least 4 content nodes, got %d", len(content))
	}

	// Convert ADF back to Markdown
	convertedMD := ADFToMarkdown(adf)
	if !strings.Contains(convertedMD, "# Project Roadmap") {
		t.Fatalf("missing header in converted markdown: %s", convertedMD)
	}
	if !strings.Contains(convertedMD, "[Atlassian Link](https://atlassian.com)") {
		t.Fatalf("missing link in converted markdown: %s", convertedMD)
	}
	if !strings.Contains(convertedMD, "**bold text**") {
		t.Fatalf("missing bold in converted markdown: %s", convertedMD)
	}
	if !strings.Contains(convertedMD, "* Task 1") {
		t.Fatalf("missing bullet in converted markdown: %s", convertedMD)
	}
	if !strings.Contains(convertedMD, "fmt.Println") {
		t.Fatalf("missing code block in converted markdown: %s", convertedMD)
	}
}

func TestStorageFormatConversion(t *testing.T) {
	mdInput := "## Architecture\n\nThis is a *test*.\n\n* Item A\n* Item B"
	xhtml := MarkdownToStorageFormat(mdInput)

	if !strings.Contains(xhtml, "<h2>Architecture</h2>") {
		t.Fatalf("missing h2 tag in xhtml: %s", xhtml)
	}
	if !strings.Contains(xhtml, "<li>Item A</li>") {
		t.Fatalf("missing li tag in xhtml: %s", xhtml)
	}

	backToMD := StorageFormatToMarkdown(xhtml)
	if !strings.Contains(backToMD, "## Architecture") {
		t.Fatalf("failed roundtrip header: %s", backToMD)
	}
	if !strings.Contains(backToMD, "* Item A") {
		t.Fatalf("failed roundtrip list: %s", backToMD)
	}
}

func TestCompactJiraProjection(t *testing.T) {
	raw := map[string]interface{}{
		"key": "ENG-404",
		"fields": map[string]interface{}{
			"summary": "Fix critical race condition",
			"status": map[string]interface{}{
				"name": "In Progress",
			},
			"issuetype": map[string]interface{}{
				"name": "Bug",
			},
			"priority": map[string]interface{}{
				"name": "High",
			},
			"assignee": map[string]interface{}{
				"displayName": "Grace Hopper",
			},
			"description": map[string]interface{}{
				"type":    "doc",
				"version": 1.0,
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Race in database connection pool.",
							},
						},
					},
				},
			},
		},
	}

	compact := ProjectJiraIssue(raw)
	if compact.Key != "ENG-404" {
		t.Fatalf("expected ENG-404, got %s", compact.Key)
	}
	if compact.Summary != "Fix critical race condition" {
		t.Fatalf("unexpected summary: %s", compact.Summary)
	}
	if compact.Status != "In Progress" || compact.Assignee != "Grace Hopper" {
		t.Fatalf("unexpected fields: %+v", compact)
	}
	if compact.Description != "Race in database connection pool." {
		t.Fatalf("unexpected description: %s", compact.Description)
	}
}
