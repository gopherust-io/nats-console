package fingerprint

// Sum64SIMD is an experimental portable-SIMD XOR-fold fingerprint.
// Without GOEXPERIMENT=simd it delegates to xxhash (Sum64).
func Sum64SIMD(data []byte) uint64 {
	return sum64SIMD(data)
}
