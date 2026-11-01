package mask_test

import (
	"testing"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/mask"
)

func TestMaskGolden(t *testing.T) {
	m, err := mask.New(config.Normalization{
		MaskingPatterns: []config.MaskingPattern{
			{Name: "uuid", Regex: `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		in, want string
	}{
		{
			"user 550e8400-e29b-41d4-a716-446655440000 logged in",
			"user <UUID> logged in",
		},
		{
			"connect 10.0.0.5:8080 ok",
			"connect <IP> ok",
		},
		{
			"at 2024-01-15T12:00:00Z event",
			"at <TS> event",
		},
		{
			"ptr 0xdeadbeef fault",
			"ptr <HEX> fault",
		},
		{
			"auth Bearer secret-token-xyz",
			"auth <PII>",
		},
		{
			"latency 42.5ms count 1000",
			"latency <NUM>ms count <NUM>",
		},
		{
			"mail user@example.com sent",
			"mail <PII> sent",
		},
	}
	for _, tc := range cases {
		got := m.Apply(tc.in)
		if got != tc.want {
			t.Errorf("Apply(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuiltinOrderUUIDBeforeNum(t *testing.T) {
	m, err := mask.New(config.Normalization{})
	if err != nil {
		t.Fatal(err)
	}
	line := "id 12345"
	got := m.Apply(line)
	if got != "id <NUM>" {
		t.Fatalf("got %q", got)
	}
}
