//go:build !goexperiment.simd

package fingerprint

func sum64SIMD(data []byte) uint64 {
	return Sum64(data)
}
