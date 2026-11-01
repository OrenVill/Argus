package drain_test

import (
	"testing"

	"github.com/OrenVill/Argus/internal/drain"
	"github.com/OrenVill/Argus/internal/meta"
)

func TestClusterStability(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m := meta.Meta{Env: "dev", Service: "pay"}

	line1 := "ERROR payment failed for user <UUID>"
	line2 := "ERROR payment failed for user <UUID>"

	c1 := f.Match(m, line1)
	c2 := f.Match(m, line2)
	if c1.Template != c2.Template {
		t.Fatalf("templates differ: %q vs %q", c1.Template, c2.Template)
	}
	if c1.Template != "ERROR payment failed for user <UUID>" {
		t.Fatalf("unexpected template: %q", c1.Template)
	}
}

func TestWildcardOnVariableToken(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m := meta.Meta{Env: "dev", Service: "pay"}

	_ = f.Match(m, "WARN retry attempt 1 for <UUID>")
	c2 := f.Match(m, "WARN retry attempt 2 for <UUID>")
	if c2.Template != "WARN retry attempt <*> for <UUID>" {
		t.Fatalf("wildcard template: got %q", c2.Template)
	}
}

func TestNewTemplateWhenDissimilar(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m := meta.Meta{Env: "dev", Service: "pay"}

	a := f.Match(m, "INFO started server")
	b := f.Match(m, "ERROR database connection lost")
	if a.Template == b.Template {
		t.Fatalf("expected distinct templates, both %q", a.Template)
	}
}

func TestForestIsolationByService(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m1 := meta.Meta{Env: "dev", Service: "a"}
	m2 := meta.Meta{Env: "dev", Service: "b"}

	c1 := f.Match(m1, "INFO hello world")
	c2 := f.Match(m2, "INFO hello world")
	if c1.Template != c2.Template {
		t.Fatalf("templates should match textually: %q vs %q", c1.Template, c2.Template)
	}
	_ = f.Match(m1, "INFO hello world")
	_ = f.Match(m1, "INFO hello universe")
	c3 := f.Match(m1, "INFO hello universe")
	if c3.Template != "INFO hello <*>" {
		t.Fatalf("m1 template after variant: %q", c3.Template)
	}
}

func TestSimThresholdBoundary(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m := meta.Meta{Env: "dev", Service: "pay"}

	_ = f.Match(m, "INFO a b c d e")
	cNear := f.Match(m, "INFO a b c d f")
	if cNear.Template != "INFO a b c d <*>" {
		t.Fatalf("near match should cluster: %q", cNear.Template)
	}

	fStrict := drain.NewForest(4, 0.9)
	_ = fStrict.Match(m, "WARN one two three four")
	cFar := fStrict.Match(m, "WARN one two three nine")
	if cFar.Template == "WARN one two three <*>" {
		t.Fatal("low similarity should not merge at 0.9 threshold")
	}
}

func TestProcessForestAPI(t *testing.T) {
	m := meta.Meta{Env: "prod", Service: "api"}
	pf := drain.NewProcessForest(m, 4, 0.5)
	c := pf.Match("ERROR payment failed for user <UUID>")
	if c.Template != "ERROR payment failed for user <UUID>" {
		t.Fatalf("unexpected template: %q", c.Template)
	}
}

func TestProcessForestIsolationByEnv(t *testing.T) {
	f := drain.NewForest(4, 0.5)
	m1 := meta.Meta{Env: "staging", Service: "pay"}
	m2 := meta.Meta{Env: "prod", Service: "pay"}

	_ = f.Match(m1, "INFO hello world")
	_ = f.Match(m1, "INFO hello universe")
	c1 := f.Match(m1, "INFO hello universe")
	_ = f.Match(m2, "INFO hello world")
	c2 := f.Match(m2, "INFO hello world")
	if c1.Template != "INFO hello <*>" {
		t.Fatalf("staging should have wildcard: %q", c1.Template)
	}
	if c2.Template != "INFO hello world" {
		t.Fatalf("prod tree should stay separate: %q", c2.Template)
	}
}
