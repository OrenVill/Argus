package level

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Level is a normalized log severity.
type Level string

const (
	Debug Level = "DEBUG"
	Info  Level = "INFO"
	Warn  Level = "WARN"
	Error Level = "ERROR"
	Fatal Level = "FATAL"
)

// Parser extracts log level from a raw line.
type Parser struct {
	jsonField string
	re        *regexp.Regexp
}

// NewParser builds a level parser from config fields.
func NewParser(jsonField, regexPattern string) (*Parser, error) {
	if regexPattern == "" {
		regexPattern = `(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`
	}
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, err
	}
	return &Parser{jsonField: jsonField, re: re}, nil
}

// Parse returns normalized level per spec §3.
func (p *Parser) Parse(line string) Level {
	if p.jsonField != "" {
		if lv, ok := parseJSONField(line, p.jsonField); ok {
			return normalize(lv)
		}
	}
	matches := p.re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return normalize(matches[1])
	}
	return Info
}

func parseJSONField(line, field string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "{") {
		return "", false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return "", false
	}
	v, ok := obj[field]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func normalize(raw string) Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return Debug
	case "INFO":
		return Info
	case "WARN", "WARNING":
		return Warn
	case "ERROR":
		return Error
	case "FATAL", "PANIC":
		return Fatal
	default:
		return Info
	}
}

// String returns the level enum string.
func (l Level) String() string {
	return string(l)
}
