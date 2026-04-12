package adf

import (
	"fmt"
	"reflect"
	"strings"
)

func PlainTextDoc(text string) map[string]any {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	content := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			content = append(content, map[string]any{
				"type":    "paragraph",
				"content": []map[string]any{},
			})
			continue
		}
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []map[string]any{
				{
					"type": "text",
					"text": line,
				},
			},
		})
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

func ExtractPlainText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			text := ExtractPlainText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case []map[string]any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			text := ExtractPlainText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		typ, _ := v["type"].(string)
		switch typ {
		case "text":
			if text, ok := v["text"].(string); ok {
				return text
			}
		case "hardBreak":
			return "\n"
		}
		if content, ok := v["content"]; ok {
			text := ExtractPlainText(content)
			if typ == "paragraph" || typ == "heading" {
				return strings.TrimRight(text, "\n") + "\n"
			}
			return text
		}
	}
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Slice {
		parts := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			text := ExtractPlainText(rv.Index(i).Interface())
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return fmt.Sprint(value)
}
