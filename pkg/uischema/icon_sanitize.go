package uischema

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strings"
)

var allowedSVGElements = map[string]struct{}{
	"a":              {},
	"circle":         {},
	"clipPath":       {},
	"defs":           {},
	"desc":           {},
	"ellipse":        {},
	"g":              {},
	"line":           {},
	"linearGradient": {},
	"marker":         {},
	"mask":           {},
	"path":           {},
	"pattern":        {},
	"polygon":        {},
	"polyline":       {},
	"radialGradient": {},
	"rect":           {},
	"stop":           {},
	"svg":            {},
	"symbol":         {},
	"title":          {},
	"use":            {},
}

var allowedSVGAttrs = map[string]struct{}{
	"aria-hidden":         {},
	"aria-label":          {},
	"aria-labelledby":     {},
	"class":               {},
	"clip-path":           {},
	"clip-rule":           {},
	"cx":                  {},
	"cy":                  {},
	"d":                   {},
	"fill":                {},
	"fill-opacity":        {},
	"fill-rule":           {},
	"focusable":           {},
	"height":              {},
	"id":                  {},
	"marker-end":          {},
	"marker-mid":          {},
	"marker-start":        {},
	"mask":                {},
	"opacity":             {},
	"points":              {},
	"preserveAspectRatio": {},
	"r":                   {},
	"refX":                {},
	"refY":                {},
	"role":                {},
	"rx":                  {},
	"ry":                  {},
	"spreadMethod":        {},
	"stop-color":          {},
	"stop-opacity":        {},
	"stroke":              {},
	"stroke-dasharray":    {},
	"stroke-dashoffset":   {},
	"stroke-linecap":      {},
	"stroke-linejoin":     {},
	"stroke-miterlimit":   {},
	"stroke-opacity":      {},
	"stroke-width":        {},
	"transform":           {},
	"viewBox":             {},
	"viewbox":             {},
	"width":               {},
	"x":                   {},
	"x1":                  {},
	"x2":                  {},
	"xmlns":               {},
	"xmlns:xlink":         {},
	"y":                   {},
	"y1":                  {},
	"y2":                  {},
}

var allowedFragmentAttrs = map[string]struct{}{
	"href":       {},
	"xlink:href": {},
}

func sanitizeIconMarkup(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	sanitized, ok := sanitizeSVG(trimmed)
	if !ok {
		return ""
	}
	return sanitized
}

func sanitizeSVG(raw string) (string, bool) {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	state := svgSanitizeState{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}

		if !state.handleToken(token) {
			return "", false
		}
	}

	if !state.validEnd() {
		return "", false
	}
	cleaned := strings.TrimSpace(state.out.String())
	if cleaned == "" {
		return "", false
	}
	return cleaned, true
}

type svgSanitizeState struct {
	out       strings.Builder
	stack     []string
	skipDepth int
	sawRoot   bool
}

func (s *svgSanitizeState) handleToken(token xml.Token) bool {
	switch tok := token.(type) {
	case xml.StartElement:
		return s.handleStartElement(tok)
	case xml.EndElement:
		s.handleEndElement()
	case xml.CharData:
		s.handleCharData(tok)
	case xml.Comment, xml.ProcInst, xml.Directive:
	}
	return true
}

func (s *svgSanitizeState) handleStartElement(tok xml.StartElement) bool {
	name := tok.Name.Local
	if s.skipDepth > 0 {
		s.skipDepth++
		return true
	}
	if !allowedSVGElement(name) {
		s.skipDepth = 1
		return true
	}
	if !s.sawRoot {
		if name != "svg" {
			return false
		}
		s.sawRoot = true
	}
	writeStartElement(&s.out, name, sanitizeSVGAttrs(tok.Attr))
	s.stack = append(s.stack, name)
	return true
}

func (s *svgSanitizeState) handleEndElement() {
	if s.skipDepth > 0 {
		s.skipDepth--
		return
	}
	if len(s.stack) == 0 {
		return
	}
	name := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	writeEndElement(&s.out, name)
}

func (s *svgSanitizeState) handleCharData(tok xml.CharData) {
	if s.skipDepth > 0 || len(s.stack) == 0 {
		return
	}
	writeEscapedText(&s.out, string(tok))
}

func (s *svgSanitizeState) validEnd() bool {
	return s.sawRoot && s.skipDepth == 0 && len(s.stack) == 0
}

func sanitizeSVGAttrs(attrs []xml.Attr) []xml.Attr {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]xml.Attr, 0, len(attrs))
	for _, attr := range attrs {
		name := attrName(attr.Name)
		if name == "" || strings.HasPrefix(strings.ToLower(name), "on") {
			continue
		}
		value := strings.TrimSpace(attr.Value)
		if _, ok := allowedFragmentAttrs[name]; ok {
			if !isSafeFragmentIRI(value) {
				continue
			}
			out = append(out, xml.Attr{Name: xml.Name{Local: name}, Value: value})
			continue
		}
		if !allowedSVGAttr(name) || isUnsafeAttrValue(value) {
			continue
		}
		out = append(out, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name.Local < out[j].Name.Local
	})
	return out
}

func attrName(name xml.Name) string {
	local := strings.TrimSpace(name.Local)
	if local == "" {
		return ""
	}
	if name.Space == "" {
		return local
	}
	switch name.Space {
	case "http://www.w3.org/2000/xmlns/":
		if local == "xmlns" {
			return "xmlns"
		}
		return "xmlns:" + local
	case "http://www.w3.org/1999/xlink", "xlink":
		return "xlink:" + local
	default:
		return local
	}
}

func allowedSVGElement(name string) bool {
	_, ok := allowedSVGElements[name]
	return ok
}

func allowedSVGAttr(name string) bool {
	_, ok := allowedSVGAttrs[name]
	return ok
}

func isSafeFragmentIRI(value string) bool {
	if value == "" {
		return false
	}
	return strings.HasPrefix(value, "#") && !strings.ContainsAny(value, "\"'<>`")
}

func isUnsafeAttrValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "data:") ||
		strings.Contains(lower, "vbscript:") ||
		strings.Contains(lower, "expression(") ||
		strings.ContainsAny(lower, "<>`")
}

func writeStartElement(out *strings.Builder, name string, attrs []xml.Attr) {
	out.WriteByte('<')
	out.WriteString(name)
	for _, attr := range attrs {
		out.WriteByte(' ')
		out.WriteString(attr.Name.Local)
		out.WriteString(`="`)
		out.WriteString(escapeXMLAttr(attr.Value))
		out.WriteByte('"')
	}
	out.WriteByte('>')
}

func writeEndElement(out *strings.Builder, name string) {
	out.WriteString("</")
	out.WriteString(name)
	out.WriteByte('>')
}

func writeEscapedText(out *strings.Builder, value string) {
	if value == "" {
		return
	}
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return
	}
	out.WriteString(buf.String())
}

func escapeXMLAttr(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return strings.ReplaceAll(buf.String(), `"`, "&#34;")
}
