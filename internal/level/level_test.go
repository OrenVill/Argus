package level_test

import (
	"testing"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/level"
)

func TestJSONField(t *testing.T) {
	p, err := level.NewParser("level", "")
	if err != nil {
		t.Fatal(err)
	}
	lv := p.Parse(`{"level":"error","msg":"boom"}`)
	if lv != level.Error {
		t.Fatalf("got %s", lv)
	}
}

func TestRegexDefault(t *testing.T) {
	p, err := level.NewParser("", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Parse("something WARNING happened") != level.Warn {
		t.Fatal("expected WARN")
	}
}

func TestNormalizePanictoFatal(t *testing.T) {
	p, err := level.NewParser("", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Parse("PANIC stack trace") != level.Fatal {
		t.Fatal("expected FATAL")
	}
}

func TestUnknownDefaultsInfo(t *testing.T) {
	p, err := level.NewParser("", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Parse("no level here") != level.Info {
		t.Fatal("expected INFO default")
	}
}

func TestRegexWithoutCaptureGroupUsesFullMatch(t *testing.T) {
	p, err := level.NewParser("", `(?i)^ERROR$`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Parse("ERROR") != level.Error {
		t.Fatalf("expected ERROR from full match, got %s", p.Parse("ERROR"))
	}
}

func TestParseLineFromConfig(t *testing.T) {
	lv, err := level.ParseLine("something WARNING happened", config.LogLevel{})
	if err != nil {
		t.Fatal(err)
	}
	if lv != level.Warn {
		t.Fatalf("got %s", lv)
	}
}

func TestWarningNormalized(t *testing.T) {
	p, err := level.NewParser("", `(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Parse("line with WARNING in it") != level.Warn {
		t.Fatal("expected WARN not WARNING")
	}
}
