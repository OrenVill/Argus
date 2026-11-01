package mask

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/OrenVill/Argus/internal/config"
)

// Token constants per product spec §4.1.
const (
	TokenUUID = "<UUID>"
	TokenIP   = "<IP>"
	TokenTS   = "<TS>"
	TokenHEX  = "<HEX>"
	TokenPII  = "<PII>"
	TokenNUM  = "<NUM>"
)

type rule struct {
	re  *regexp.Regexp
	tok string
}

// Masker applies built-in then user-defined masking patterns.
type Masker struct {
	rules []rule
}

var (
	builtinOnce  sync.Once
	builtinRules []rule
)

func builtinMaskRules() []rule {
	builtinOnce.Do(func() {
		patterns := []struct {
			re  string
			tok string
		}{
			// UUID / GUID
			{`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, TokenUUID},
			// IPv4 with optional port
			{`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`, TokenIP},
			// ISO-8601 timestamps
			{`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b`, TokenTS},
			// Common date/time
			{`\b\d{4}[-/]\d{2}[-/]\d{2}(?:[ T]\d{2}:\d{2}:\d{2})?\b`, TokenTS},
			{`\b\d{2}[-/]\d{2}[-/]\d{4}(?:[ T]\d{2}:\d{2}:\d{2})?\b`, TokenTS},
			// Bare clock times (HH:MM:SS)
			{`\b\d{1,2}:\d{2}:\d{2}\b`, TokenTS},
			// Unix epoch seconds or milliseconds
			{`\b1[0-9]{9,12}\b`, TokenTS},
			// Hex pointers
			{`\b0x[0-9a-fA-F]+\b`, TokenHEX},
			// Email-like
			{`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, TokenPII},
			// Bearer tokens
			{`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`, TokenPII},
			// Basic auth credentials (headers, JSON fields, query params)
			{`(?i)Basic\s+[A-Za-z0-9+/=]+`, TokenPII},
			// Basic auth in URLs
			{`(?i)://[^:\s]+:[^@\s]+@`, TokenPII},
			// Integer / float metrics (last built-in; decimals before integers)
			{`\b-?\d+\.\d+(?:[eE][+-]?\d+)?`, TokenNUM},
			{`\b-?\d+(?:[eE][+-]?\d+)?\b`, TokenNUM},
		}
		builtinRules = make([]rule, 0, len(patterns))
		for _, p := range patterns {
			re, err := regexp.Compile(p.re)
			if err != nil {
				panic(fmt.Sprintf("builtin mask %q: %v", p.re, err))
			}
			builtinRules = append(builtinRules, rule{re: re, tok: p.tok})
		}
	})
	return builtinRules
}

// New builds a masker from normalization config (user patterns appended after built-ins).
func New(cfg config.Normalization) (*Masker, error) {
	rules := make([]rule, 0, len(builtinMaskRules())+len(cfg.MaskingPatterns))
	rules = append(rules, builtinMaskRules()...)
	for _, p := range cfg.MaskingPatterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			return nil, fmt.Errorf("masking pattern %q: %w", p.Name, err)
		}
		tok := "<" + p.Name + ">"
		if p.Name == "uuid" {
			tok = TokenUUID
		}
		if p.Name == "ip" {
			tok = TokenIP
		}
		rules = append(rules, rule{re: re, tok: tok})
	}
	return &Masker{rules: rules}, nil
}

// Apply runs all masking rules on the full line (built-in order per spec §4.1).
func (m *Masker) Apply(line string) string {
	out := line
	builtinN := len(builtinMaskRules())
	for i, r := range m.rules {
		if r.tok == TokenNUM {
			out = r.re.ReplaceAllString(out, r.tok)
			continue
		}
		// Built-in IPv4 is index 1; mask maximal IPv6 literals before dotted-quad so mapped forms stay one <IP>.
		if i == 1 && builtinN > 1 {
			out = maskIPv6(out)
		}
		out = r.re.ReplaceAllString(out, r.tok)
	}
	return out
}

func maskIPv6(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	for i := 0; i < len(line); {
		if end, ok := maximalIPv6Span(line, i); ok {
			b.WriteString(TokenIP)
			i = end
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}

const maxIPv6LiteralLen = 49 // longest textual forms incl. brackets and :port

func maximalIPv6Span(line string, start int) (end int, ok bool) {
	if start >= len(line) {
		return 0, false
	}
	switch line[start] {
	case '[':
		return maximalBracketedIPv6(line, start)
	case ':':
		return growIPv6Literal(line, start)
	default:
		if !isHexDigit(line[start]) {
			return 0, false
		}
		return growIPv6Literal(line, start)
	}
}

func maximalBracketedIPv6(line string, start int) (int, bool) {
	close := strings.IndexByte(line[start+1:], ']')
	if close < 0 {
		return 0, false
	}
	end := start + 1 + close + 1
	if !isIPv6Literal(line[start:end]) {
		return 0, false
	}
	end = extendPortSuffix(line, start, end)
	if !hasIPv6Boundary(line, start, end) {
		return 0, false
	}
	return end, true
}

func growIPv6Literal(line string, start int) (int, bool) {
	limit := start + maxIPv6LiteralLen
	if limit > len(line) {
		limit = len(line)
	}
	best := -1
	for e := start + 1; e <= limit; e++ {
		if !isIPv6LiteralChar(line[e-1]) {
			break
		}
		if !isIPv6Literal(line[start:e]) {
			continue
		}
		spanEnd := extendPortSuffix(line, start, e)
		if hasIPv6Boundary(line, start, spanEnd) {
			best = spanEnd
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}

func extendPortSuffix(line string, start, hostEnd int) int {
	if hostEnd >= len(line) || line[hostEnd] != ':' {
		return hostEnd
	}
	if hostEnd > start && line[hostEnd-1] == ':' {
		return hostEnd
	}
	p := hostEnd + 1
	for p < len(line) && line[p] >= '0' && line[p] <= '9' {
		p++
	}
	if p == hostEnd+1 {
		return hostEnd
	}
	if isIPv6Literal(line[start:p]) {
		return p
	}
	return hostEnd
}

func hasIPv6Boundary(line string, start, end int) bool {
	if start > 0 {
		prev := line[start-1]
		if (prev >= '0' && prev <= '9') || (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || prev == ':' {
			return false
		}
	}
	if end < len(line) {
		next := line[end]
		if (next >= '0' && next <= '9') || (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == ':' {
			return false
		}
	}
	return true
}

func isIPv6Literal(s string) bool {
	host := ipv6HostForParse(s)
	if strings.Contains(host, "::") || strings.Count(host, ":") >= 2 {
		if ip := net.ParseIP(host); ip != nil && strings.Contains(host, ":") {
			return true
		}
	}
	return false
}

func ipv6HostForParse(s string) string {
	if strings.HasPrefix(s, "[") {
		host := s
		if i := strings.Index(host, "]:"); i >= 0 {
			host = host[:i+1]
		}
		host = strings.TrimPrefix(host, "[")
		return strings.TrimSuffix(host, "]")
	}
	if ip := net.ParseIP(s); ip != nil && strings.Contains(s, ":") {
		return s
	}
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return s
	}
	port := s[i+1:]
	if port == "" || !allDecimal(port) {
		return s
	}
	host := s[:i]
	if ip := net.ParseIP(host); ip != nil {
		return host
	}
	return s
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isIPv6LiteralChar(b byte) bool {
	return isHexDigit(b) || b == ':' || b == '.'
}

func allDecimal(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
