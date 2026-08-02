package middleware

import (
	"fmt"
	"testing"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func BenchmarkResponseCompression(b *testing.B) {
	// Must be strictly larger than 32 KiB to exercise compression.
	sizes := []int{40 << 10, 64 << 10, 512 << 10}
	modes := []struct {
		name   string
		accept string
		on     bool
	}{
		{name: "off", accept: "", on: false},
		{name: "gzip", accept: "gzip", on: true},
		{name: "br", accept: "br", on: true},
	}

	for _, size := range sizes {
		payload := largeJSON(size)
		for _, mode := range modes {
			b.Run(fmt.Sprintf("%s/%d", mode.name, size), func(b *testing.B) {
				mw := New(config.Config{HTTP: config.HTTPConfig{ResponseCompression: mode.on}}, nil, nil)
				h := mw.ApplyResponseCompression(func(ctx *fasthttp.RequestCtx) {
					ctx.SetStatusCode(fasthttp.StatusOK)
					ctx.SetContentType("application/json")
					ctx.SetBody(payload)
				})

				ctx := &fasthttp.RequestCtx{}
				if !commonstrings.IsEmpty(mode.accept) {
					ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, mode.accept)
				}
				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()

				var outLen int
				b.ResetTimer()
				for b.Loop() {
					ctx.Response.Reset()
					if !commonstrings.IsEmpty(mode.accept) {
						ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, mode.accept)
					}
					h(ctx)
					outLen = len(ctx.Response.Body())
				}
				b.ReportMetric(float64(outLen), "bytes_out")
				b.ReportMetric(float64(outLen)/float64(len(payload)), "ratio")
			})
		}
	}
}

func BenchmarkRequestDecompression(b *testing.B) {
	plain := largeJSON(64 << 10)
	gzipped := fasthttp.AppendGzipBytes(nil, plain)

	b.Run("off", func(b *testing.B) {
		mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 1 << 20}}, nil, nil)
		h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {})
		ctx := &fasthttp.RequestCtx{}
		b.SetBytes(int64(len(plain)))
		b.ReportAllocs()
		for b.Loop() {
			ctx.Request.Reset()
			ctx.Request.SetBody(plain)
			h(ctx)
		}
	})

	b.Run("gzip", func(b *testing.B) {
		mw := New(config.Config{HTTP: config.HTTPConfig{MaxRequestBodySize: 1 << 20}}, nil, nil)
		h := mw.DecompressRequestBody(func(ctx *fasthttp.RequestCtx) {})
		ctx := &fasthttp.RequestCtx{}
		b.ReportMetric(float64(len(gzipped)), "bytes_in")
		b.SetBytes(int64(len(gzipped)))
		b.ReportAllocs()
		for b.Loop() {
			ctx.Request.Reset()
			ctx.Request.Header.SetContentEncoding("gzip")
			ctx.Request.SetBody(gzipped)
			h(ctx)
		}
	})
}
