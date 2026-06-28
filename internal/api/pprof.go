package api

import (
	"errors"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: gated by PPROF_ENABLED and admin auth
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/profiler"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

const pprofPathPrefix = "/debug/pprof"

var (
	errPprofDisabled = errors.New("pprof is disabled")
)

var supportedPprofProfiles = []string{
	"heap", "goroutine", "allocs", "block", "mutex", "threadcreate", "cpu",
}

var stdPprofHandler = fasthttpadaptor.NewFastHTTPHandler(http.DefaultServeMux)

func isPprofPath(path string) bool {
	return path == pprofPathPrefix || strings.HasPrefix(path, pprofPathPrefix+"/")
}

func isLongRunningProfilePath(path string) bool {
	if isPprofPath(path) {
		return strings.HasPrefix(path, pprofPathPrefix+"/profile")
	}
	return strings.HasPrefix(path, "/api/v1/pprof/profile/cpu") ||
		strings.HasPrefix(path, "/api/v1/pprof/profile/profile")
}

func (h *Handler) PprofConfig(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.PprofConfig{Enabled: false})
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.PprofConfig{
		Enabled:            true,
		AuthRequired:       h.cfg.PprofAuthEnabled,
		ContinuousEnabled:  h.cfg.PprofContinuous(),
		Profiles:           append([]string(nil), supportedPprofProfiles...),
		MaxCPUSecs:         h.cfg.MaxPprofCPUSecs(),
		ContinuousInterval: int(h.cfg.ContinuousPprofInterval().Seconds()),
		ContinuousCPUSlice: int(h.cfg.ContinuousPprofCPUSlice().Seconds()),
	})
}

func (h *Handler) PprofContinuous(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}
	if !h.cfg.PprofContinuous() || profiler.Default == nil {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errors.New("continuous profiling is disabled"))
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, profiler.Default.Snapshot())
}

func (h *Handler) PprofRuntime(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}
	if h.cfg.PprofContinuous() && profiler.Default != nil {
		snapshot := profiler.Default.Snapshot()
		if n := len(snapshot.RuntimeHistory); n > 0 {
			serializer.WriteJSON(ctx, fasthttp.StatusOK, snapshot.RuntimeHistory[n-1])
			return
		}
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, readRuntimeStats())
}

func (h *Handler) PprofProfileSummary(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}

	profileType, ok := pprofProfileParam(ctx)
	if !ok {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("missing profile type"))
		return
	}
	if useContinuousSource(ctx) && profiler.Default != nil {
		if summary, ok := profiler.Default.Profile(profileType); ok {
			serializer.WriteJSON(ctx, fasthttp.StatusOK, summary)
			return
		}
	}

	seconds := parseCPUSeconds(ctx, h.cfg.MaxPprofCPUSecs())
	data, durationSecs, err := collectProfile(profileType, seconds)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	summary, err := profiler.Summarize(profileType, data, durationSecs)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, summary)
}

func (h *Handler) PprofProfileDownload(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}

	profileType, ok := pprofProfileParam(ctx)
	if !ok {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("missing profile type"))
		return
	}
	seconds := parseCPUSeconds(ctx, h.cfg.MaxPprofCPUSecs())

	data, _, err := collectProfile(profileType, seconds)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	filename := profileType + ".pb.gz"
	if profileType == "cpu" {
		filename = "cpu.pprof"
	}
	ctx.Response.Header.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SetContentType("application/octet-stream")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(data)
}

func serveStdPprof(ctx *fasthttp.RequestCtx) {
	stdPprofHandler(ctx)
}

func pprofProfileParam(ctx *fasthttp.RequestCtx) (string, bool) {
	value := ctx.UserValue("profile")
	name, ok := value.(string)
	return name, ok && name != ""
}

func useContinuousSource(ctx *fasthttp.RequestCtx) bool {
	raw := strings.ToLower(string(ctx.QueryArgs().Peek("source")))
	if raw == "continuous" || raw == "cache" {
		return true
	}
	if len(ctx.QueryArgs().Peek("source")) == 0 && string(ctx.QueryArgs().Peek("continuous")) == "1" {
		return true
	}
	return false
}

func readRuntimeStats() domain.PprofRuntimeStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return domain.PprofRuntimeStats{
		FetchedAt:  time.Now().UTC(),
		Goroutines: runtime.NumGoroutine(),
		Memory: domain.PprofMemoryStats{
			Alloc:       stats.Alloc,
			TotalAlloc:  stats.TotalAlloc,
			Sys:         stats.Sys,
			HeapAlloc:   stats.HeapAlloc,
			HeapInuse:   stats.HeapInuse,
			HeapObjects: stats.HeapObjects,
			NumGC:       stats.NumGC,
		},
	}
}

func parseCPUSeconds(ctx *fasthttp.RequestCtx, maxSeconds int) int {
	raw := string(ctx.QueryArgs().Peek("seconds"))
	if raw == "" {
		return 30
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 30
	}
	if seconds > maxSeconds {
		return maxSeconds
	}
	return seconds
}

func collectProfile(name string, seconds int) ([]byte, int, error) {
	switch name {
	case "cpu", "profile":
		return profiler.CollectCPU(seconds)
	default:
		return profiler.CollectNamed(name)
	}
}
