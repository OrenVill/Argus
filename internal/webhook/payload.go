package webhook

import (
	"encoding/json"

	"github.com/OrenVill/Argus/internal/meta"
	iredis "github.com/OrenVill/Argus/internal/redis"
)

// Payload is the outbound webhook JSON body (§7.2).
type Payload struct {
	Env             string   `json:"env"`
	Service         string   `json:"service"`
	ImageTag        string   `json:"image_tag,omitempty"`
	TemplateHash    string   `json:"template_hash"`
	Template        string   `json:"template"`
	Level           string   `json:"level"`
	AnomalyType     string   `json:"anomaly_type"`
	SurgeMultiplier int      `json:"surge_multiplier,omitempty"`
	Count           int      `json:"count"`
	ReportedCount   int      `json:"reported_count"`
	SampleLines     []string `json:"sample_lines"`
	FirstSeen       string   `json:"first_seen"`
	LastSeen        string   `json:"last_seen"`
}

// NewPayload builds a webhook payload from detection output.
func NewPayload(m meta.Meta, templateHash, template, level string, res iredis.LineResult) Payload {
	p := Payload{
		Env:           m.Env,
		Service:       m.Service,
		TemplateHash:  templateHash,
		Template:      template,
		Level:         level,
		AnomalyType:   res.AnomalyType,
		Count:         res.Count,
		ReportedCount: res.ReportedCount,
		SampleLines:   res.SampleLines,
		FirstSeen:     res.FirstSeen,
		LastSeen:      res.LastSeen,
	}
	if m.ImageTag != "" {
		p.ImageTag = m.ImageTag
	}
	if res.SurgeMultiplier > 0 {
		p.SurgeMultiplier = res.SurgeMultiplier
	}
	return p
}

// MarshalJSON returns compact JSON for POST body.
func (p Payload) MarshalJSON() ([]byte, error) {
	type alias Payload
	return json.Marshal(alias(p))
}

// IdempotencyKey returns the §7.1 header value.
func IdempotencyKey(m meta.Meta, templateHash, anomalyType string, count int) string {
	return m.Env + ":" + m.Service + ":" + templateHash + ":" + anomalyType + ":" + itoa(count)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
