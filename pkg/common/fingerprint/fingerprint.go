package fingerprint

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/cespare/xxhash/v2"
)

// Sum64 returns a non-cryptographic 64-bit fingerprint of data.
// Suitable for change-detection and cache etags, not for security.
func Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// Sum64String returns a non-cryptographic 64-bit fingerprint of s.
func Sum64String(s string) uint64 {
	return xxhash.Sum64String(s)
}

// ETag quotes an xxhash fingerprint as an HTTP weak-style opaque tag.
func ETag(data []byte) string {
	return formatETag(Sum64(data))
}

// ETagString quotes an xxhash fingerprint of s.
func ETagString(s string) string {
	return formatETag(Sum64String(s))
}

func formatETag(sum uint64) string {
	var buf [16]byte
	hex.Encode(buf[:], uint64Bytes(sum))
	return `"` + string(buf[:]) + `"`
}

func uint64Bytes(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	return b[:]
}
