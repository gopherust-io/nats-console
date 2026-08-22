//go:build goexperiment.simd

package fingerprint

import "simd"

// sum64SIMD XOR-folds bytes with portable SIMD loads. POC for GOEXPERIMENT=simd;
// production change-detection uses xxhash (Sum64).
func sum64SIMD(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}

	var acc simd.Uint8s
	for i := 0; i < len(data); {
		v, n := simd.LoadUint8sPart(data[i:])
		if n == 0 {
			break
		}
		acc = acc.Xor(v)
		i += n
	}

	// Max practical vector width today is 64 bytes (AVX-512).
	var tmp [64]byte
	stored := acc.StorePart(tmp[:])
	var out uint64
	for i := 0; i < stored; i++ {
		out ^= uint64(tmp[i]) << (8 * (i & 7))
	}
	return out
}
