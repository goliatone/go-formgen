package submission

import (
	"strconv"
	"strings"
)

type pathSegment struct {
	Name   string
	Index  *int
	Append bool
}

func parsePath(raw string) []pathSegment {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "#/")
	clean = strings.TrimPrefix(clean, "$/")
	clean = strings.TrimPrefix(clean, "$.")
	for strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, ".") || strings.HasPrefix(clean, "$") {
		clean = strings.TrimPrefix(clean, "#")
		clean = strings.TrimPrefix(clean, "/")
		clean = strings.TrimPrefix(clean, ".")
		clean = strings.TrimPrefix(clean, "$")
	}
	clean = strings.Trim(clean, "./")
	if clean == "" {
		return nil
	}

	var out []pathSegment
	for _, part := range splitPathParts(clean) {
		parsePathPart(part, &out)
	}
	return out
}

func splitPathParts(path string) []string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '.' || r == '/'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parsePathPart(part string, out *[]pathSegment) {
	for {
		open := strings.IndexByte(part, '[')
		if open < 0 {
			if part != "" {
				appendPartSegment(part, out)
			}
			return
		}

		if open > 0 {
			appendPartSegment(part[:open], out)
		}

		close := strings.IndexByte(part[open+1:], ']')
		if close < 0 {
			appendPartSegment(part[open:], out)
			return
		}
		token := strings.TrimSpace(part[open+1 : open+1+close])
		if token == "" {
			*out = append(*out, pathSegment{Append: true})
		} else if index, err := strconv.Atoi(token); err == nil {
			*out = append(*out, pathSegment{Index: &index})
		} else {
			appendPartSegment(token, out)
		}
		part = part[open+1+close+1:]
		if part == "" {
			return
		}
	}
}

func appendPartSegment(part string, out *[]pathSegment) {
	part = strings.TrimSpace(part)
	if part == "" {
		return
	}
	if index, err := strconv.Atoi(part); err == nil {
		*out = append(*out, pathSegment{Index: &index})
		return
	}
	part = strings.ReplaceAll(part, "~1", "/")
	part = strings.ReplaceAll(part, "~0", "~")
	*out = append(*out, pathSegment{Name: part})
}

func canonicalPath(segments []pathSegment) string {
	var b strings.Builder
	for _, segment := range segments {
		switch {
		case segment.Name != "":
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(segment.Name)
		case segment.Index != nil:
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(*segment.Index))
			b.WriteByte(']')
		case segment.Append:
			b.WriteString("[]")
		}
	}
	return b.String()
}

// RendererPath drops array indexes and append suffixes from a precise
// submission path so existing renderers can consume field-level errors.
func RendererPath(path string) string {
	segments := parsePath(path)
	if len(segments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.Name == "" {
			continue
		}
		parts = append(parts, segment.Name)
	}
	return strings.Join(parts, ".")
}
