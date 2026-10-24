package format

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// MarkdownToStorageFormat transforms Markdown into Confluence Storage Format XHTML.
func MarkdownToStorageFormat(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return "<p></p>"
	}

	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var out strings.Builder

	var inCodeBlock bool
	var codeBlockLines []string
	var codeBlockLang string

	var inList bool

	closeList := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
	}

	for _, line := range lines {
		// Code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			closeList()
			if inCodeBlock {
				macro := fmt.Sprintf("<ac:structured-macro ac:name=\"code\"><ac:parameter ac:name=\"language\">%s</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>\n",
					html.EscapeString(codeBlockLang), strings.Join(codeBlockLines, "\n"))
				out.WriteString(macro)
				inCodeBlock = false
				codeBlockLines = nil
				codeBlockLang = ""
			} else {
				inCodeBlock = true
				codeBlockLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
			}
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			closeList()
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "#") {
			closeList()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				headerText := html.EscapeString(strings.TrimSpace(trimmed[level:]))
				out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, headerText, level))
				continue
			}
		}

		// Bullet lists
		if (strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ")) && len(trimmed) > 2 {
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			itemText := html.EscapeString(strings.TrimSpace(trimmed[2:]))
			out.WriteString(fmt.Sprintf("<li>%s</li>\n", itemText))
			continue
		}

		closeList()
		escaped := html.EscapeString(trimmed)
		out.WriteString(fmt.Sprintf("<p>%s</p>\n", escaped))
	}

	closeList()
	return out.String()
}

var (
	tagRegex    = regexp.MustCompile(`<[^>]+>`)
	macroRegex  = regexp.MustCompile(`(?s)<ac:structured-macro.*?<ac:plain-text-body><!\[CDATA\[(.*?)\]\]></ac:plain-text-body>.*?</ac:structured-macro>`)
	headerRegex = regexp.MustCompile(`(?i)<h([1-6])>(.*?)</h[1-6]>`)
	liRegex     = regexp.MustCompile(`(?i)<li>(.*?)</li>`)
	pRegex      = regexp.MustCompile(`(?i)<p>(.*?)</p>`)
)

// StorageFormatToMarkdown extracts clean Markdown from Confluence Storage Format XHTML.
func StorageFormatToMarkdown(storageXHTML string) string {
	if strings.TrimSpace(storageXHTML) == "" {
		return ""
	}

	text := storageXHTML

	// Extract code macros
	text = macroRegex.ReplaceAllString(text, "```\n$1\n```\n")

	// Convert headers
	text = headerRegex.ReplaceAllStringFunc(text, func(m string) string {
		sub := headerRegex.FindStringSubmatch(m)
		if len(sub) == 3 {
			lvl := 1
			if n := sub[1][0] - '0'; n >= 1 && n <= 6 {
				lvl = int(n)
			}
			hText := tagRegex.ReplaceAllString(sub[2], "")
			return fmt.Sprintf("\n%s %s\n", strings.Repeat("#", lvl), strings.TrimSpace(hText))
		}
		return m
	})

	// Convert list items
	text = liRegex.ReplaceAllStringFunc(text, func(m string) string {
		sub := liRegex.FindStringSubmatch(m)
		if len(sub) == 2 {
			liText := tagRegex.ReplaceAllString(sub[1], "")
			return fmt.Sprintf("* %s\n", strings.TrimSpace(liText))
		}
		return m
	})

	// Convert paragraphs
	text = pRegex.ReplaceAllStringFunc(text, func(m string) string {
		sub := pRegex.FindStringSubmatch(m)
		if len(sub) == 2 {
			pText := tagRegex.ReplaceAllString(sub[1], "")
			return fmt.Sprintf("\n%s\n", strings.TrimSpace(pText))
		}
		return m
	})

	// Strip remaining XML tags
	text = tagRegex.ReplaceAllString(text, "")
	text = html.UnescapeString(text)

	// Clean up duplicate newlines
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" || (len(cleaned) > 0 && cleaned[len(cleaned)-1] != "") {
			cleaned = append(cleaned, trimmed)
		}
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
