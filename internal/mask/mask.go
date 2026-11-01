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

// ipv6Candidate finds bracketed or colon-hex substrings; net.ParseIP filters false positives (e.g. 12:00:00).
var ipv6Candidate = regexp.MustCompile(`(?i)(?:\[(?:[0-9a-f]{0,4}:){1,7}[0-9a-f]{0,4}\]|(?:[0-9a-f]{1,4}:){1,7}[0-9a-f]{0,4}|(?:[0-9a-f]{1,4}:){1,7}:|:(?:[0-9a-f]{1,4}:){0,6}[0-9a-f]{0,4})(?::\d+)?`)

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
			// Unix epoch seconds or milliseconds
			{`\b1[0-9]{9,12}\b`, TokenTS},
			// Hex pointers
			{`\b0x[0-9a-fA-F]+\b`, TokenHEX},
			// Email-like
			{`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, TokenPII},
			// Bearer tokens
			{`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`, TokenPII},
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
			out = replaceIntegers(out, r.re, r.tok)
			continue
		}
		out = r.re.ReplaceAllString(out, r.tok)
		// Built-in IPv4 is index 1; IPv6 masking follows immediately per §4.1 IP before TS/NUM.
		if i == 1 && builtinN > 1 {
			out = maskIPv6(out)
		}
	}
	return out
}

func replaceIntegers(line string, re *regexp.Regexp, tok string) string {
	clock := regexp.MustCompile(`\b\d{1,2}:\d{2}:\d{2}\b`)
	spans := clock.FindAllStringIndex(line, -1)
	insideClock := func(start, end int) bool {
		for _, sp := range spans {
			if start >= sp[0] && end <= sp[1] {
				return true
			}
		}
		return false
	}
	var b strings.Builder
	last := 0
	for _, loc := range re.FindAllStringSubmatchIndex(line, -1) {
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		b.WriteString(line[last:start])
		if insideClock(start, end) {
			b.WriteString(line[start:end])
		} else {
			b.WriteString(tok)
		}
		last = end
	}
	b.WriteString(line[last:])
	return b.String()
}

func maskIPv6(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	remaining := line
	for {
		loc := ipv6Candidate.FindStringIndex(remaining)
		if loc == nil {
			b.WriteString(remaining)
			break
		}
		start, end := loc[0], loc[1]
		b.WriteString(remaining[:start])
		candidate := remaining[start:end]
		if isIPv6Literal(candidate) && hasIPv6Boundary(remaining, start, end) {
			b.WriteString(TokenIP)
		} else {
			b.WriteString(candidate)
		}
		remaining = remaining[end:]
	}
	return b.String()
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
	host := s
	if i := strings.Index(host, "]:"); i >= 0 {
		host = host[:i+1]
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if strings.Contains(host, "::") || strings.Count(host, ":") >= 2 {
		if ip := net.ParseIP(host); ip != nil && strings.Contains(host, ":") {
			return true
		}
	}
	return false
}
