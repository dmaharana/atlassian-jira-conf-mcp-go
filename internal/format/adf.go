package format

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownToADF converts Markdown text into a valid Jira Atlassian Document Format (ADF) JSON structure.
func MarkdownToADF(markdown string) map[string]interface{} {
	if strings.TrimSpace(markdown) == "" {
		return map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{},
		}
	}

	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var content []interface{}

	var inCodeBlock bool
	var codeBlockLines []string
	var codeBlockLang string

	var inBulletList bool
	var bulletItems []interface{}

	flushBulletList := func() {
		if inBulletList && len(bulletItems) > 0 {
			content = append(content, map[string]interface{}{
				"type":    "bulletList",
				"content": bulletItems,
			})
			bulletItems = nil
			inBulletList = false
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Handle Code Block
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			flushBulletList()
			if inCodeBlock {
				// Close code block
				content = append(content, map[string]interface{}{
					"type": "codeBlock",
					"attrs": map[string]interface{}{
						"language": codeBlockLang,
					},
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": strings.Join(codeBlockLines, "\n"),
						},
					},
				})
				inCodeBlock = false
				codeBlockLines = nil
				codeBlockLang = ""
				continue
			} else {
				// Start code block
				inCodeBlock = true
				codeBlockLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
				codeBlockLines = nil
				continue
			}
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushBulletList()
			continue
		}

		// Handle Headings (#, ##, ###, etc.)
		if strings.HasPrefix(trimmed, "#") {
			flushBulletList()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				headerText := strings.TrimSpace(trimmed[level:])
				content = append(content, map[string]interface{}{
					"type": "heading",
					"attrs": map[string]interface{}{
						"level": level,
					},
					"content": parseInlineMarks(headerText),
				})
				continue
			}
		}

		// Handle Bullet Points (* or -)
		if (strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ")) && len(trimmed) > 2 {
			inBulletList = true
			itemText := strings.TrimSpace(trimmed[2:])
			bulletItems = append(bulletItems, map[string]interface{}{
				"type": "listItem",
				"content": []interface{}{
					map[string]interface{}{
						"type":    "paragraph",
						"content": parseInlineMarks(itemText),
					},
				},
			})
			continue
		}

		flushBulletList()

		// Standard Paragraph
		content = append(content, map[string]interface{}{
			"type":    "paragraph",
			"content": parseInlineMarks(trimmed),
		})
	}

	flushBulletList()
	if inCodeBlock && len(codeBlockLines) > 0 {
		content = append(content, map[string]interface{}{
			"type": "codeBlock",
			"attrs": map[string]interface{}{
				"language": codeBlockLang,
			},
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": strings.Join(codeBlockLines, "\n"),
				},
			},
		})
	}

	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

var linkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// parseInlineMarks handles text, links, bold, and italic inline styling.
func parseInlineMarks(text string) []interface{} {
	if text == "" {
		return []interface{}{}
	}

	var nodes []interface{}
	lastIdx := 0

	matches := linkRegex.FindAllStringSubmatchIndex(text, -1)
	for _, m := range matches {
		if m[0] > lastIdx {
			prefix := text[lastIdx:m[0]]
			nodes = append(nodes, parseTextFormatting(prefix)...)
		}
		linkText := text[m[2]:m[3]]
		linkURL := text[m[4]:m[5]]
		nodes = append(nodes, map[string]interface{}{
			"type": "text",
			"text": linkText,
			"marks": []interface{}{
				map[string]interface{}{
					"type": "link",
					"attrs": map[string]interface{}{
						"href": linkURL,
					},
				},
			},
		})
		lastIdx = m[1]
	}

	if lastIdx < len(text) {
		remainder := text[lastIdx:]
		nodes = append(nodes, parseTextFormatting(remainder)...)
	}

	return nodes
}

func parseTextFormatting(text string) []interface{} {
	var nodes []interface{}
	// Check for bold **...**
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	lastIdx := 0
	matches := boldRegex.FindAllStringSubmatchIndex(text, -1)
	for _, m := range matches {
		if m[0] > lastIdx {
			nodes = append(nodes, map[string]interface{}{
				"type": "text",
				"text": text[lastIdx:m[0]],
			})
		}
		boldText := text[m[2]:m[3]]
		nodes = append(nodes, map[string]interface{}{
			"type": "text",
			"text": boldText,
			"marks": []interface{}{
				map[string]interface{}{
					"type": "strong",
				},
			},
		})
		lastIdx = m[1]
	}
	if lastIdx < len(text) {
		nodes = append(nodes, map[string]interface{}{
			"type": "text",
			"text": text[lastIdx:],
		})
	}
	return nodes
}

// ADFToMarkdown recursively traverses an ADF tree and extracts clean Markdown representation.
func ADFToMarkdown(node interface{}) string {
	if node == nil {
		return ""
	}

	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return ""
	}

	nodeType, _ := nodeMap["type"].(string)
	contentRaw, _ := nodeMap["content"].([]interface{})

	switch nodeType {
	case "doc":
		var parts []string
		for _, child := range contentRaw {
			if md := ADFToMarkdown(child); md != "" {
				parts = append(parts, md)
			}
		}
		return strings.Join(parts, "\n\n")

	case "paragraph":
		var textParts []string
		for _, child := range contentRaw {
			textParts = append(textParts, ADFToMarkdown(child))
		}
		return strings.Join(textParts, "")

	case "heading":
		attrs, _ := nodeMap["attrs"].(map[string]interface{})
		level := 1
		if l, ok := attrs["level"].(float64); ok {
			level = int(l)
		}
		prefix := strings.Repeat("#", level) + " "
		var textParts []string
		for _, child := range contentRaw {
			textParts = append(textParts, ADFToMarkdown(child))
		}
		return prefix + strings.Join(textParts, "")

	case "bulletList":
		var items []string
		for _, child := range contentRaw {
			itemText := ADFToMarkdown(child)
			if itemText != "" {
				items = append(items, "* "+itemText)
			}
		}
		return strings.Join(items, "\n")

	case "orderedList":
		var items []string
		for idx, child := range contentRaw {
			itemText := ADFToMarkdown(child)
			if itemText != "" {
				items = append(items, fmt.Sprintf("%d. %s", idx+1, itemText))
			}
		}
		return strings.Join(items, "\n")

	case "listItem":
		var textParts []string
		for _, child := range contentRaw {
			textParts = append(textParts, ADFToMarkdown(child))
		}
		return strings.Join(textParts, " ")

	case "codeBlock":
		attrs, _ := nodeMap["attrs"].(map[string]interface{})
		lang, _ := attrs["language"].(string)
		var textParts []string
		for _, child := range contentRaw {
			textParts = append(textParts, ADFToMarkdown(child))
		}
		return fmt.Sprintf("```%s\n%s\n```", lang, strings.Join(textParts, ""))

	case "text":
		text, _ := nodeMap["text"].(string)
		marks, _ := nodeMap["marks"].([]interface{})
		for _, m := range marks {
			if markMap, ok := m.(map[string]interface{}); ok {
				mType, _ := markMap["type"].(string)
				switch mType {
				case "strong":
					text = "**" + text + "**"
				case "em":
					text = "*" + text + "*"
				case "link":
					attrs, _ := markMap["attrs"].(map[string]interface{})
					if href, ok := attrs["href"].(string); ok {
						text = fmt.Sprintf("[%s](%s)", text, href)
					}
				}
			}
		}
		return text

	default:
		// Fallback for unknown containers: traverse children
		var parts []string
		for _, child := range contentRaw {
			parts = append(parts, ADFToMarkdown(child))
		}
		return strings.Join(parts, " ")
	}
}
