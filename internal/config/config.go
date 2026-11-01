package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Config is the root Argus configuration document.
type Config struct {
	Version       string        `yaml:"version"`
	Meta          Meta          `yaml:"meta"`
	Daemon        Daemon        `yaml:"daemon"`
	Inputs        Inputs        `yaml:"inputs"`
	Redis         Redis         `yaml:"redis"`
	LogLevel      LogLevel      `yaml:"log_level"`
	Normalization Normalization `yaml:"normalization"`
	Detection     Detection     `yaml:"detection"`
	Webhook       Webhook       `yaml:"webhook"`
	Control       Control       `yaml:"control"`
}

// Meta identifies the deployment context.
type Meta struct {
	Env      string `yaml:"env"`
	Service  string `yaml:"service"`
	ImageTag string `yaml:"image_tag"`
}

// Daemon tuning for workers and backpressure.
type Daemon struct {
	Workers        int `yaml:"workers"`
	BufferCapacity int `yaml:"buffer_capacity"`
}

// Inputs describes log sources.
type Inputs struct {
	Stdin bool     `yaml:"stdin"`
	Files []string `yaml:"files"`
}

// Redis connection and key layout.
type Redis struct {
	Address   string `yaml:"address"`
	KeyPrefix string `yaml:"key_prefix"`
	TTLDays   int    `yaml:"ttl_days"`
}

// LogLevel extraction settings.
type LogLevel struct {
	JSONField string `yaml:"json_field"`
	Regex     string `yaml:"regex"`
}

// Normalization covers Drain and masking.
type Normalization struct {
	DrainDepth      int              `yaml:"drain_depth"`
	SimThreshold    float64          `yaml:"sim_threshold"`
	SampleLinesMax  int              `yaml:"sample_lines_max"`
	MaskingPatterns []MaskingPattern `yaml:"masking_patterns"`
}

// MaskingPattern is a user-defined mask rule.
type MaskingPattern struct {
	Name  string `yaml:"name"`
	Regex string `yaml:"regex"`
}

// Detection holds anomaly rule parameters.
type Detection struct {
	GraceWindowDuration string `yaml:"grace_window_duration"`
	RegressionThreshold string `yaml:"regression_threshold"`
	SurgeMultipliers    []int  `yaml:"surge_multipliers"`
}

// Webhook delivery settings.
type Webhook struct {
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

// Control API settings.
type Control struct {
	Listen      string `yaml:"listen"`
	BearerToken string `yaml:"bearer_token"`
}

// Load reads and parses a YAML config file with ${ENV} expansion.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	expanded := expandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(&cfg)
	return &cfg, nil
}

// Validate checks required fields for any loaded config.
func (c *Config) Validate() error {
	if c.Meta.Env == "" {
		return fmt.Errorf("meta.env is required")
	}
	if c.Meta.Service == "" {
		return fmt.Errorf("meta.service is required")
	}
	if c.Redis.Address == "" {
		return fmt.Errorf("redis.address is required")
	}
	if c.Redis.KeyPrefix == "" {
		return fmt.Errorf("redis.key_prefix is required")
	}
	if c.Redis.TTLDays <= 0 {
		return fmt.Errorf("redis.ttl_days must be positive")
	}
	if !c.Inputs.Stdin && len(c.Inputs.Files) == 0 {
		return fmt.Errorf("at least one input source required (stdin or files)")
	}
	if _, err := time.ParseDuration(c.Detection.GraceWindowDuration); err != nil {
		return fmt.Errorf("detection.grace_window_duration: %w", err)
	}
	if _, err := time.ParseDuration(c.Detection.RegressionThreshold); err != nil {
		return fmt.Errorf("detection.regression_threshold: %w", err)
	}
	if len(c.Detection.SurgeMultipliers) == 0 {
		return fmt.Errorf("detection.surge_multipliers must not be empty")
	}
	for i, m := range c.Detection.SurgeMultipliers {
		if m <= 0 {
			return fmt.Errorf("detection.surge_multipliers[%d] must be positive", i)
		}
		if i > 0 && m <= c.Detection.SurgeMultipliers[i-1] {
			return fmt.Errorf("detection.surge_multipliers must be strictly increasing")
		}
	}
	if c.Webhook.Timeout != "" {
		if _, err := time.ParseDuration(c.Webhook.Timeout); err != nil {
			return fmt.Errorf("webhook.timeout: %w", err)
		}
	}
	return nil
}

// ValidateForRun adds checks required when starting the daemon.
func (c *Config) ValidateForRun() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Webhook.URL) == "" {
		return fmt.Errorf("webhook.url is required for run")
	}
	if strings.Contains(c.Webhook.URL, "${") {
		return fmt.Errorf("webhook.url contains unresolved ${ENV} placeholder")
	}
	if c.Daemon.Workers <= 0 {
		return fmt.Errorf("daemon.workers must be positive")
	}
	if c.Daemon.BufferCapacity <= 0 {
		return fmt.Errorf("daemon.buffer_capacity must be positive")
	}
	return nil
}

func expandEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return match
	})
}

func applyDefaults(cfg *Config) {
	if cfg.Version == "" {
		cfg.Version = "1.0"
	}
	if cfg.Daemon.Workers == 0 {
		cfg.Daemon.Workers = 8
	}
	if cfg.Daemon.BufferCapacity == 0 {
		cfg.Daemon.BufferCapacity = 10000
	}
	if cfg.Normalization.DrainDepth == 0 {
		cfg.Normalization.DrainDepth = 4
	}
	if cfg.Normalization.SimThreshold == 0 {
		cfg.Normalization.SimThreshold = 0.5
	}
	if cfg.Normalization.SampleLinesMax == 0 {
		cfg.Normalization.SampleLinesMax = 5
	}
	if cfg.Redis.TTLDays == 0 {
		cfg.Redis.TTLDays = 30
	}
	if cfg.LogLevel.Regex == "" {
		cfg.LogLevel.Regex = `(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`
	}
	if cfg.Detection.GraceWindowDuration == "" {
		cfg.Detection.GraceWindowDuration = "30m"
	}
	if cfg.Detection.RegressionThreshold == "" {
		cfg.Detection.RegressionThreshold = "24h"
	}
	if len(cfg.Detection.SurgeMultipliers) == 0 {
		cfg.Detection.SurgeMultipliers = []int{10, 100, 1000}
	}
	if cfg.Webhook.Timeout == "" {
		cfg.Webhook.Timeout = "5s"
	}
	if cfg.Control.Listen == "" {
		cfg.Control.Listen = "127.0.0.1:9090"
	}
}
