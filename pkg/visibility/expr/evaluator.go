package expr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/visibility"
)

// Evaluator is a small, dependency-free visibility evaluator.
//
// Supported operators:
// - boolean checks: `enabled`
// - comparisons: `field == true`, `field != "value"`, `count == 3`
// - boolean composition: `a == true && b != false`, `a || b`
//
// Values are read from visibility.Context.Values (with dot-path traversal) and
// visibility.Context.Extras (via the `extras.` prefix).
type Evaluator struct{}

func New() *Evaluator { return &Evaluator{} }

func (e *Evaluator) Eval(fieldPath, rule string, ctx visibility.Context) (bool, error) {
	_ = fieldPath
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return true, nil
	}

	tokens, err := tokenize(trimmed)
	if err != nil {
		return false, err
	}
	if len(tokens) == 0 {
		return true, nil
	}

	expr, err := parseExpression(tokens)
	if err != nil {
		return false, err
	}
	if expr == nil {
		return true, nil
	}
	return expr.eval(ctx)
}

type tokenKind int

const (
	tokenIdentifier tokenKind = iota
	tokenString
	tokenNumber
	tokenBool
	tokenNull
	tokenEq
	tokenNeq
	tokenAnd
	tokenOr
	tokenNot
	tokenLParen
	tokenRParen
)

type token struct {
	kind tokenKind
	raw  string
}

func tokenize(input string) ([]token, error) {
	lexer := tokenLexer{input: input}
	var tokens []token
	for lexer.more() {
		ch := lexer.next()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			lexer.pos++
			continue
		}
		tok, ok, err := lexer.scanToken()
		if err != nil {
			return nil, err
		}
		if ok {
			tokens = append(tokens, tok)
		}
	}
	return tokens, nil
}

type tokenLexer struct {
	input string
	pos   int
}

func (l *tokenLexer) more() bool {
	return l.pos < len(l.input)
}

func (l *tokenLexer) next() byte {
	if !l.more() {
		return 0
	}
	return l.input[l.pos]
}

func (l *tokenLexer) consume() byte {
	if !l.more() {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *tokenLexer) scanToken() (token, bool, error) {
	switch l.next() {
	case '(':
		l.consume()
		return token{kind: tokenLParen, raw: "("}, true, nil
	case ')':
		l.consume()
		return token{kind: tokenRParen, raw: ")"}, true, nil
	case '!':
		return l.scanBang()
	case '=':
		return l.scanPair('=', tokenEq, "==", "visibility/expr: unexpected '='; use '=='")
	case '&':
		return l.scanPair('&', tokenAnd, "&&", "visibility/expr: unexpected '&'; use '&&'")
	case '|':
		return l.scanPair('|', tokenOr, "||", "visibility/expr: unexpected '|'; use '||'")
	case '"', '\'':
		return l.scanString()
	default:
		return l.scanBare()
	}
}

func (l *tokenLexer) scanBang() (token, bool, error) {
	l.consume()
	if l.next() == '=' {
		l.consume()
		return token{kind: tokenNeq, raw: "!="}, true, nil
	}
	return token{kind: tokenNot, raw: "!"}, true, nil
}

func (l *tokenLexer) scanPair(expected byte, kind tokenKind, raw, message string) (token, bool, error) {
	l.consume()
	if l.next() != expected {
		return token{}, false, errors.New(message)
	}
	l.consume()
	return token{kind: kind, raw: raw}, true, nil
}

func (l *tokenLexer) scanString() (token, bool, error) {
	quote := l.consume()
	start := l.pos
	escaped := false
	for l.more() {
		c := l.consume()
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == quote {
			raw := string(quote) + l.input[start:l.pos-1] + string(quote)
			value, err := strconv.Unquote(raw)
			if err != nil {
				return token{}, false, fmt.Errorf("visibility/expr: invalid string literal: %w", err)
			}
			return token{kind: tokenString, raw: value}, true, nil
		}
	}
	return token{}, false, errors.New("visibility/expr: unterminated string literal")
}

func (l *tokenLexer) scanBare() (token, bool, error) {
	start := l.pos
	for l.more() && !isTokenDelimiter(l.input[l.pos]) {
		l.pos++
	}
	raw := strings.TrimSpace(l.input[start:l.pos])
	if raw == "" {
		return token{}, false, nil
	}
	return bareToken(raw), true, nil
}

func bareToken(raw string) token {
	switch strings.ToLower(raw) {
	case "true", "false":
		return token{kind: tokenBool, raw: strings.ToLower(raw)}
	case "null", "nil":
		return token{kind: tokenNull, raw: "null"}
	default:
		if looksLikeNumber(raw) {
			return token{kind: tokenNumber, raw: raw}
		}
		return token{kind: tokenIdentifier, raw: raw}
	}
}

func isTokenDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == ')' || c == '!' || c == '=' || c == '&' || c == '|'
}

func looksLikeNumber(raw string) bool {
	if raw == "" {
		return false
	}
	ch := raw[0]
	return (ch >= '0' && ch <= '9') || ch == '-' || ch == '+'
}

type exprNode interface {
	eval(ctx visibility.Context) (bool, error)
}

type exprOr struct {
	left  exprNode
	right exprNode
}

func (n exprOr) eval(ctx visibility.Context) (bool, error) {
	ok, err := n.left.eval(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return n.right.eval(ctx)
}

type exprAnd struct {
	left  exprNode
	right exprNode
}

func (n exprAnd) eval(ctx visibility.Context) (bool, error) {
	ok, err := n.left.eval(ctx)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return n.right.eval(ctx)
}

type exprNot struct {
	inner exprNode
}

func (n exprNot) eval(ctx visibility.Context) (bool, error) {
	ok, err := n.inner.eval(ctx)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

type literalKind int

const (
	litString literalKind = iota
	litNumber
	litBool
	litNull
)

type literal struct {
	kind literalKind
	raw  string
}

type exprCompare struct {
	identifier string
	op         tokenKind
	literal    literal
}

func (n exprCompare) eval(ctx visibility.Context) (bool, error) {
	value, ok := lookup(ctx, n.identifier)
	if !ok {
		value = nil
	}

	switch n.literal.kind {
	case litNull:
		return compareLiteralResult(n.op, value == nil, "null", n.opString())
	case litBool:
		want := n.literal.raw == "true"
		got, _ := coerceBool(value)
		return compareLiteralResult(n.op, got == want, "bool", n.opString())
	case litNumber:
		want, err := strconv.ParseFloat(n.literal.raw, 64)
		if err != nil {
			return false, fmt.Errorf("visibility/expr: invalid number literal %q", n.literal.raw)
		}
		got, ok := coerceNumber(value)
		if !ok {
			got = 0
		}
		return compareLiteralResult(n.op, got == want, "number", n.opString())
	case litString:
		want := n.literal.raw
		got := coerceString(value)
		return compareLiteralResult(n.op, got == want, "string", n.opString())
	default:
		return false, fmt.Errorf("visibility/expr: unsupported literal")
	}
}

func compareLiteralResult(op tokenKind, equal bool, literalType, opString string) (bool, error) {
	switch op {
	case tokenEq:
		return equal, nil
	case tokenNeq:
		return !equal, nil
	default:
		return false, fmt.Errorf("visibility/expr: unsupported operator %q for %s literal", opString, literalType)
	}
}

func (n exprCompare) opString() string {
	switch n.op {
	case tokenEq:
		return "=="
	case tokenNeq:
		return "!="
	default:
		return "?"
	}
}

type exprTruthy struct {
	identifier string
}

func (n exprTruthy) eval(ctx visibility.Context) (bool, error) {
	value, ok := lookup(ctx, n.identifier)
	if !ok {
		return false, nil
	}
	return truthy(value), nil
}

type tokenStream struct {
	tokens []token
	pos    int
}

func parseExpression(tokens []token) (exprNode, error) {
	stream := &tokenStream{tokens: tokens}
	node, err := parseOr(stream)
	if err != nil {
		return nil, err
	}
	if stream.pos < len(stream.tokens) {
		return nil, fmt.Errorf("visibility/expr: unexpected token %q", stream.tokens[stream.pos].raw)
	}
	return node, nil
}

func parseOr(stream *tokenStream) (exprNode, error) {
	left, err := parseAnd(stream)
	if err != nil {
		return nil, err
	}
	for stream.match(tokenOr) {
		right, err := parseAnd(stream)
		if err != nil {
			return nil, err
		}
		left = exprOr{left: left, right: right}
	}
	return left, nil
}

func parseAnd(stream *tokenStream) (exprNode, error) {
	left, err := parseUnary(stream)
	if err != nil {
		return nil, err
	}
	for stream.match(tokenAnd) {
		right, err := parseUnary(stream)
		if err != nil {
			return nil, err
		}
		left = exprAnd{left: left, right: right}
	}
	return left, nil
}

func parseUnary(stream *tokenStream) (exprNode, error) {
	if stream.match(tokenNot) {
		inner, err := parseUnary(stream)
		if err != nil {
			return nil, err
		}
		return exprNot{inner: inner}, nil
	}
	return parsePrimary(stream)
}

func parsePrimary(stream *tokenStream) (exprNode, error) {
	if stream.match(tokenLParen) {
		inner, err := parseOr(stream)
		if err != nil {
			return nil, err
		}
		if !stream.match(tokenRParen) {
			return nil, errors.New("visibility/expr: missing closing ')'")
		}
		return inner, nil
	}

	ident, ok := stream.consume(tokenIdentifier)
	if !ok {
		if stream.pos >= len(stream.tokens) {
			return nil, errors.New("visibility/expr: empty expression")
		}
		return nil, fmt.Errorf("visibility/expr: expected identifier, got %q", stream.tokens[stream.pos].raw)
	}

	if stream.match(tokenEq) {
		lit, err := stream.consumeLiteral()
		if err != nil {
			return nil, err
		}
		return exprCompare{identifier: ident.raw, op: tokenEq, literal: lit}, nil
	}
	if stream.match(tokenNeq) {
		lit, err := stream.consumeLiteral()
		if err != nil {
			return nil, err
		}
		return exprCompare{identifier: ident.raw, op: tokenNeq, literal: lit}, nil
	}

	return exprTruthy{identifier: ident.raw}, nil
}

func (s *tokenStream) match(kind tokenKind) bool {
	if s.pos >= len(s.tokens) {
		return false
	}
	if s.tokens[s.pos].kind != kind {
		return false
	}
	s.pos++
	return true
}

func (s *tokenStream) consume(kind tokenKind) (token, bool) {
	if s.pos >= len(s.tokens) {
		return token{}, false
	}
	if s.tokens[s.pos].kind != kind {
		return token{}, false
	}
	out := s.tokens[s.pos]
	s.pos++
	return out, true
}

func (s *tokenStream) consumeLiteral() (literal, error) {
	if s.pos >= len(s.tokens) {
		return literal{}, errors.New("visibility/expr: missing literal")
	}
	tok := s.tokens[s.pos]
	s.pos++
	switch tok.kind {
	case tokenString:
		return literal{kind: litString, raw: tok.raw}, nil
	case tokenNumber:
		return literal{kind: litNumber, raw: tok.raw}, nil
	case tokenBool:
		return literal{kind: litBool, raw: strings.ToLower(tok.raw)}, nil
	case tokenNull:
		return literal{kind: litNull, raw: "null"}, nil
	case tokenIdentifier:
		// Bare identifiers are treated as strings to keep the evaluator forgiving.
		return literal{kind: litString, raw: tok.raw}, nil
	default:
		return literal{}, fmt.Errorf("visibility/expr: expected literal, got %q", tok.raw)
	}
}

func lookup(ctx visibility.Context, key string) (any, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false
	}

	if strings.HasPrefix(strings.ToLower(key), "extras.") {
		path := strings.TrimSpace(key[len("extras."):])
		return lookupMap(ctx.Extras, path)
	}
	return lookupMap(ctx.Values, key)
}

func lookupMap(values map[string]any, path string) (any, bool) {
	if len(values) == 0 || strings.TrimSpace(path) == "" {
		return nil, false
	}
	path = strings.TrimSpace(path)

	// Prefer exact match for dotted keys (common with render values like "cta.headline").
	if v, ok := values[path]; ok {
		return v, true
	}

	parts := strings.Split(path, ".")
	var current any = values
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		case map[string]string:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		default:
			return nil, false
		}
	}
	return current, true
}

func truthy(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.TrimSpace(v) != ""
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case float32:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

func coerceBool(value any) (bool, bool) {
	if value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed, true
		}
		return strings.TrimSpace(v) != "", true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	default:
		return truthy(value), true
	}
}

func coerceNumber(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func coerceString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(value)
	}
}
