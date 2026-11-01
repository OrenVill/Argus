package hash_test

import (
	"testing"

	"github.com/OrenVill/Argus/internal/hash"
	"github.com/OrenVill/Argus/internal/meta"
)

func TestTemplateHashStable(t *testing.T) {
	m := meta.Meta{Env: "prod", Service: "payment-svc"}
	tpl := "ERROR payment failed for user <UUID>"
	h1 := hash.TemplateHash(m, tpl)
	h2 := hash.TemplateHash(m, tpl)
	if h1 != h2 {
		t.Fatalf("hash not stable: %s vs %s", h1, h2)
	}
	if len(h1) != 16 {
		t.Fatalf("expected 16 hex chars, got %d: %q", len(h1), h1)
	}
}

func TestTemplateHashDiffersByEnv(t *testing.T) {
	tpl := "INFO ok"
	h1 := hash.TemplateHash(meta.Meta{Env: "a", Service: "s"}, tpl)
	h2 := hash.TemplateHash(meta.Meta{Env: "b", Service: "s"}, tpl)
	if h1 == h2 {
		t.Fatal("hash should differ by env")
	}
}
