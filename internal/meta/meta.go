package meta

// Meta holds fixed process metadata from configuration.
type Meta struct {
	Env      string `yaml:"env"`
	Service  string `yaml:"service"`
	ImageTag string `yaml:"image_tag"`
}

// Key returns a stable identifier for per-service in-memory structures.
func (m Meta) Key() string {
	return m.Env + "\x00" + m.Service
}
