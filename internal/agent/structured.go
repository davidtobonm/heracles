package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DecodeStructuredJSON extracts one JSON object or array from provider text output.
func DecodeStructuredJSON(text string, target any) error {
	candidates := []string{strings.TrimSpace(text)}
	if fenced := fencedJSON(text); fenced != "" {
		candidates = append([]string{fenced}, candidates...)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if tryDecodeJSON(candidate, target) == nil {
			return nil
		}
		if extracted := extractJSONValue(candidate); extracted != "" && extracted != candidate {
			if err := tryDecodeJSON(extracted, target); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("provider did not return valid JSON")
}

func tryDecodeJSON(text string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("extra trailing content after JSON value")
	}
	return nil
}

func fencedJSON(text string) string {
	value := strings.TrimSpace(text)
	if !strings.HasPrefix(value, "```") {
		return ""
	}
	value = strings.TrimPrefix(value, "```")
	if newline := strings.IndexByte(value, '\n'); newline >= 0 {
		header := strings.TrimSpace(value[:newline])
		if header == "" || strings.EqualFold(header, "json") {
			value = value[newline+1:]
		}
	}
	if end := strings.LastIndex(value, "```"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func extractJSONValue(text string) string {
	for start, char := range text {
		if char != '{' && char != '[' {
			continue
		}
		if extracted := balancedJSON(text[start:]); extracted != "" {
			return extracted
		}
	}
	return ""
}

func balancedJSON(text string) string {
	if text == "" {
		return ""
	}
	var stack []rune
	inString := false
	escaped := false
	for index, char := range text {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, char)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return ""
			}
			stack = stack[:len(stack)-1]
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return ""
			}
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			return strings.TrimSpace(text[:index+1])
		}
	}
	return ""
}
