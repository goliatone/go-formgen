package model

import (
	"regexp"
	"strings"
)

var splitWordsPattern = regexp.MustCompile(`[_\-\s]+`)

// DefaultLabeler converts a field name into a human-friendly label. It splits
// on underscores/dashes and camelCase boundaries.
func DefaultLabeler(name string) string {
	if name == "" {
		return ""
	}

	words := splitWordsPattern.Split(name, -1)
	var segments []string
	for _, word := range words {
		if word == "" {
			continue
		}
		segments = append(segments, titleCase(splitCamel(word)))
	}
	return strings.TrimSpace(strings.Join(segments, " "))
}

func splitCamel(input string) string {
	var out strings.Builder
	for i, r := range input {
		if i > 0 && isBoundary(input, i, r) {
			out.WriteRune(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

func isBoundary(input string, index int, r rune) bool {
	prev := rune(input[index-1])
	return (isLower(prev) && isUpper(r)) || (isLetter(prev) && isDigit(r)) || (isDigit(prev) && isLetter(r))
}

func isUpper(r rune) bool  { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool  { return r >= 'a' && r <= 'z' }
func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return isUpper(r) || isLower(r) }

func titleCase(word string) string {
	if word == "" {
		return ""
	}
	lower := strings.ToLower(word)
	return strings.ToUpper(lower[:1]) + lower[1:]
}
