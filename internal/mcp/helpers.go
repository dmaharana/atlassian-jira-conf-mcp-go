package mcp

import (
	"encoding/json"
	"regexp"
	"strings"
)

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

// NormalizeAction converts camelCase, PascalCase, and aliases to canonical snake_case.
func NormalizeAction(action string) string {
	a := strings.TrimSpace(action)
	if a == "" {
		return ""
	}

	// Direct alias mappings for prompt specifics
	switch strings.ToLower(strings.ReplaceAll(a, "_", "")) {
	case "getjiraissue", "getissue":
		return "get_issue"
	case "createjiraissue", "createissue":
		return "create_issue"
	case "editjiraissue", "editissue":
		return "edit_issue"
	case "transitionjiraissue", "transitionissue":
		return "transition_issue"
	case "addoreditjiraissuecomment":
		return "add_or_edit_comment"
	case "addcomment", "addjiraissuecomment":
		return "add_comment"
	case "editcomment", "editjiraissuecomment":
		return "edit_comment"
	case "searchjiraissuesusingjql", "searchissues", "searchjiraissues":
		return "search_issues"
	case "getconfluencecontent", "getcontent":
		return "get_content"
	case "createconfluencecontent", "createcontent":
		return "create_content"
	case "updateconfluencecontent", "updatecontent":
		return "update_content"
	case "searchconfluence", "searchcontent":
		return "search_content"
	}

	// Convert camelCase to snake_case
	snake := matchFirstCap.ReplaceAllString(a, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	snake = strings.ReplaceAll(snake, "-", "_")
	return strings.ToLower(snake)
}

// ExtractArgsMap safely converts any arguments container to map[string]interface{}.
func ExtractArgsMap(args any) map[string]interface{} {
	if args == nil {
		return make(map[string]interface{})
	}
	if m, ok := args.(map[string]interface{}); ok {
		return m
	}
	if m, ok := args.(map[string]any); ok {
		res := make(map[string]interface{}, len(m))
		for k, v := range m {
			res[k] = v
		}
		return res
	}
	return make(map[string]interface{})
}

func GetArgString(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func GetArgBool(args map[string]interface{}, key string) bool {
	if args == nil {
		return false
	}
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func GetArgInt(args map[string]interface{}, key string, defaultVal int) int {
	if args == nil {
		return defaultVal
	}
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(int); ok {
		return v
	}
	return defaultVal
}

func GetArgObject(args map[string]interface{}, key string) map[string]interface{} {
	if args == nil {
		return nil
	}
	if v, ok := args[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func FormatJSONResult(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
