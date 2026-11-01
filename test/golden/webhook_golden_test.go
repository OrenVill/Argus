package golden_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OrenVill/Argus/internal/meta"
	iredis "github.com/OrenVill/Argus/internal/redis"
	"github.com/OrenVill/Argus/internal/webhook"
)

func TestWebhookPayloadGolden(t *testing.T) {
	m := meta.Meta{Env: "dev", Service: "payment-svc"}
	res := iredis.LineResult{
		FireWebhook:   true,
		AnomalyType:   "NEW_SIGNATURE",
		Count:         1,
		ReportedCount: 0,
		SampleLines:   []string{"ERROR connection failed host=<IP>"},
		FirstSeen:     "2026-11-01T12:00:00Z",
		LastSeen:      "2026-11-01T12:00:00Z",
	}
	payload := webhook.NewPayload(m, "0123456789abcdef", "ERROR connection failed host=<IP>", "ERROR", res)
	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "webhook_new_signature.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var gotObj, wantObj any
	if err := json.Unmarshal(got, &gotObj); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantObj); err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(gotObj, wantObj) {
		t.Fatalf("payload mismatch:\ngot  %s\nwant %s", got, want)
	}
	key := webhook.IdempotencyKey(m, "0123456789abcdef", "NEW_SIGNATURE", 1)
	if key != "dev:payment-svc:0123456789abcdef:NEW_SIGNATURE:1" {
		t.Fatalf("idempotency key=%q", key)
	}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
