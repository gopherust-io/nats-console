package api

import (
	"errors"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: gated by PPROF_ENABLED and admin auth
	"strconv"
	"strings"

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
		Enabled:      true,
		AuthRequired: h.cfg.PprofAuthEnabled,
		Profiles:     append([]string(nil), supportedPprofProfiles...),
		MaxCPUSecs:   h.cfg.MaxPprofCPUSecs(),
	})
}

func (h *Handler) PprofRuntime(ctx *fasthttp.RequestCtx) {
	if !h.cfg.PprofEnabled {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, profiler.ReadRuntimeStats())
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
