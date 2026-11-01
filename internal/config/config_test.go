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

func TestValidateRejectsInvalidMaskRegex(t *testing.T) {
	cfg := &config.Config{
		Meta:   config.Meta{Env: "e", Service: "s"},
		Redis:  config.Redis{Address: "127.0.0.1:6379", KeyPrefix: "argus:", TTLDays: 30},
		Inputs: config.Inputs{Stdin: true},
		Detection: config.Detection{
			GraceWindowDuration: "30m",
			RegressionThreshold: "24h",
			SurgeMultipliers:    []int{10, 100, 1000},
		},
		Normalization: config.Normalization{
			DrainDepth:   4,
			SimThreshold: 0.5,
			MaskingPatterns: []config.MaskingPattern{
				{Name: "bad", Regex: "[unclosed"},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid mask regex error")
	}
}

func TestValidateRejectsNonPositiveSimThreshold(t *testing.T) {
	cfg := &config.Config{
		Meta:   config.Meta{Env: "e", Service: "s"},
		Redis:  config.Redis{Address: "127.0.0.1:6379", KeyPrefix: "argus:", TTLDays: 30},
		Inputs: config.Inputs{Stdin: true},
		Detection: config.Detection{
			GraceWindowDuration: "30m",
			RegressionThreshold: "24h",
			SurgeMultipliers:    []int{10, 100, 1000},
		},
		Normalization: config.Normalization{DrainDepth: 4, SimThreshold: -0.1},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected sim_threshold error")
	}
}

func TestToMeta(t *testing.T) {
	cfg := &config.Config{
		Meta: config.Meta{Env: "prod", Service: "pay", ImageTag: "v3"},
	}
	m := cfg.ToMeta()
	if m.Env != "prod" || m.Service != "pay" || m.ImageTag != "v3" {
		t.Fatalf("ToMeta: %+v", m)
	}
}

func TestMetaEnvOverrides(t *testing.T) {
	t.Setenv("ARGUS_ENV", "from-env")
	t.Setenv("ARGUS_SERVICE", "svc-env")
	t.Setenv("ARGUS_IMAGE_TAG", "tag-v9")
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `version: "1.0"
meta:
  env: yaml-env
  service: yaml-svc
  image_tag: yaml-tag
inputs:
  stdin: true
redis:
  address: "127.0.0.1:6379"
  key_prefix: "argus:"
  ttl_days: 7
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Meta.Env != "from-env" || cfg.Meta.Service != "svc-env" || cfg.Meta.ImageTag != "tag-v9" {
		t.Fatalf("overrides: %+v", cfg.Meta)
	}
}

func TestValidateRejectsUnresolvedMetaPlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `version: "1.0"
meta:
  env: prod
  service: ${MISSING_SERVICE}
inputs:
  stdin: true
redis:
  address: "127.0.0.1:6379"
  key_prefix: "argus:"
  ttl_days: 7
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected unresolved meta.service error")
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
