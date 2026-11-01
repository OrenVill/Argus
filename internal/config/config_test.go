package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OrenVill/Argus/internal/config"
)

func TestLoadExampleConfig(t *testing.T) {
	t.Setenv("ALERT_WEBHOOK_URL", "https://example.com/hook")
	path := filepath.Join("..", "..", "config", "argus.yaml.example")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Meta.Env != "dev" || cfg.Meta.Service != "payment-svc" {
		t.Fatalf("meta: %+v", cfg.Meta)
	}
	if cfg.Webhook.URL != "https://example.com/hook" {
		t.Fatalf("webhook url: %q", cfg.Webhook.URL)
	}
	if err := cfg.ValidateForRun(); err != nil {
		t.Fatal(err)
	}
}

func TestExpandEnvUnresolved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `version: "1.0"
meta:
  env: prod
  service: svc
inputs:
  stdin: true
redis:
  address: "127.0.0.1:6379"
  key_prefix: "argus:"
  ttl_days: 7
webhook:
  url: "${MISSING_VAR}"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Webhook.URL != "${MISSING_VAR}" {
		t.Fatalf("expected unresolved placeholder, got %q", cfg.Webhook.URL)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateForRun(); err == nil {
		t.Fatal("expected error for unresolved webhook url")
	}
}

func TestValidateRequiredMeta(t *testing.T) {
	cfg := &config.Config{
		Redis:  config.Redis{Address: "127.0.0.1:6379", KeyPrefix: "argus:", TTLDays: 30},
		Inputs: config.Inputs{Stdin: true},
		Detection: config.Detection{
			GraceWindowDuration: "30m",
			RegressionThreshold: "24h",
			SurgeMultipliers:    []int{10, 100, 1000},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected meta validation error")
	}
}
