package fingerprint_test

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/cloudwego/base64x"

	"github.com/gopherust-io/nats-consol/pkg/common/fingerprint"
)

func BenchmarkFingerprintSHA256(b *testing.B) {
	data := make([]byte, 256<<10)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		_ = sha256.Sum256(data)
	}
}

func BenchmarkFingerprintXXHash(b *testing.B) {
	data := make([]byte, 256<<10)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		_ = xxhash.Sum64(data)
	}
}

func BenchmarkFingerprintSum64SIMD(b *testing.B) {
	data := make([]byte, 256<<10)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		_ = fingerprint.Sum64SIMD(data)
	}
}

func BenchmarkBase64Stdlib(b *testing.B) {
	src := make([]byte, 4<<10)
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		base64.StdEncoding.Encode(dst, src)
	}
}

func BenchmarkBase64X(b *testing.B) {
	src := make([]byte, 4<<10)
	dst := make([]byte, base64x.StdEncoding.EncodedLen(len(src)))
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		base64x.StdEncoding.Encode(dst, src)
	}
}
