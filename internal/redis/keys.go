package redis

import "github.com/OrenVill/Argus/internal/meta"

// TemplateKey returns the Redis HASH key for a template (§4.3).
func TemplateKey(prefix, env, service, templateHash string) string {
	return prefix + env + ":" + service + ":" + templateHash
}

// GraceKey returns the deploy grace deadline key (§6).
func GraceKey(prefix, env, service string) string {
	return prefix + env + ":grace:" + service
}

// TemplateKeyMeta builds a template key from process metadata.
func TemplateKeyMeta(prefix string, m meta.Meta, templateHash string) string {
	return TemplateKey(prefix, m.Env, m.Service, templateHash)
}

// GraceKeyMeta builds a grace key from process metadata.
func GraceKeyMeta(prefix string, m meta.Meta, service string) string {
	return GraceKey(prefix, m.Env, service)
}
