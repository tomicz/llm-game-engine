package ui

import (
	"strings"
)

// ParseCSS parses a primitive CSS file: selectors .class or #id and blocks of "key: value;" .
// No combinators, no @rules. Later rules override earlier for the same selector.
func ParseCSS(content string) (*Stylesheet, error) {
	sheet := &Stylesheet{Rules: nil}
	content = stripCSSComments(content)
	for {
		rule, rest, ok := parseOneRule(content)
		if !ok {
			break
		}
		sheet.Rules = append(sheet.Rules, rule)
		content = rest
	}
	return sheet, nil
}

func stripCSSComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			j := i + 2
			for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '/') {
				j++
			}
			if j+1 < len(s) {
				j += 2
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// parseOneRule finds the next "selector { ... }" and returns the rule and the rest of the string.
func parseOneRule(s string) (Rule, string, bool) {
	open := strings.Index(s, "{")
	if open == -1 {
		return Rule{}, "", false
	}
	selector := strings.TrimSpace(s[:open])
	if selector == "" || (selector[0] != '.' && selector[0] != '#') || len(selector) < 2 {
		// Skip this block and continue after the matching "}"
		close := findMatchingBrace(s, open)
		if close == -1 {
			return Rule{}, "", false
		}
		return parseOneRule(s[close+1:])
	}
	close := findMatchingBrace(s, open)
	if close == -1 {
		return Rule{}, "", false
	}
	body := strings.TrimSpace(s[open+1 : close])
	props := parseDeclarations(body)
	rest := strings.TrimSpace(s[close+1:])
	return Rule{Selector: selector, Props: props}, rest, true
}

func findMatchingBrace(s string, openIdx int) int {
	depth := 1
	for i := openIdx + 1; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseDeclarations(body string) map[string]string {
	props := make(map[string]string)
	for _, part := range strings.Split(body, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colon := strings.Index(part, ":")
		if colon == -1 {
			continue
		}
		k := strings.TrimSpace(part[:colon])
		v := strings.TrimSpace(part[colon+1:])
		if k != "" {
			props[k] = v
		}
	}
	return props
}
