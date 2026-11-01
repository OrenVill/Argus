package mask

import (
	"fmt"
	"regexp"
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
			// IPv6 (require hex; avoid matching clock times like 12:00:00)
			{`\b(?:(?:[0-9a-fA-F]{1,4}:){1,7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:)(?::\d+)?\b`, TokenIP},
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

// Apply runs all masking rules on the full line.
func (m *Masker) Apply(line string) string {
	out := line
	for _, r := range m.rules {
		out = r.re.ReplaceAllString(out, r.tok)
	}
	return out
}
