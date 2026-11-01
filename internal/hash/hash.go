package hash

import (
	"encoding/hex"

	"github.com/cespare/xxhash/v2"

	"github.com/OrenVill/Argus/internal/meta"
)

// TemplateHash returns lowercase hex xxHash64 of env\0service\0template.
func TemplateHash(m meta.Meta, template string) string {
	var b []byte
	b = append(b, m.Env...)
	b = append(b, 0)
	b = append(b, m.Service...)
	b = append(b, 0)
	b = append(b, template...)
	sum := xxhash.Sum64(b)
	digest := uint64ToBytes(sum)
	var buf [16]byte
	hex.Encode(buf[:], digest[:])
	return string(buf[:])
}

func uint64ToBytes(v uint64) [8]byte {
	var out [8]byte
	for i := 0; i < 8; i++ {
		out[i] = byte(v >> (56 - i*8))
	}
	return out
}
