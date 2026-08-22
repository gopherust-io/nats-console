package ops

import (
	"errors"
	_ "net/http/pprof" //nolint:gosec // G108: gated by PPROF_ENABLED and admin auth
	"strconv"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/profiler"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var errPprofDisabled = errors.New("pprof is disabled")

var supportedPprofProfiles = []string{
	"heap", "goroutine", "allocs", "block", "mutex", "threadcreate", "cpu",
}

// PprofConfig godoc
//
// @Summary Pprof Config
// @Tags Profiling
// @Produce json
// @Success 200 {object} api.PprofConfigEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/pprof/config [get]
func (h *Handler) PprofConfig(ctx *fasthttp.RequestCtx) {
	if !h.Cfg.Pprof.Enabled {
		httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.PprofConfig{Enabled: false})
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.PprofConfig{
		Enabled:      true,
		AuthRequired: h.Cfg.Pprof.AuthEnabled,
		Profiles:     append([]string(nil), supportedPprofProfiles...),
		MaxCPUSecs:   h.Cfg.Pprof.CPUMaxSeconds,
	})
}

// PprofRuntime godoc
//
// @Summary Pprof Runtime
// @Tags Profiling
// @Produce json
// @Success 200 {object} api.PprofRuntimeEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/pprof/runtime [get]
func (h *Handler) PprofRuntime(ctx *fasthttp.RequestCtx) {
	if !h.Cfg.Pprof.Enabled {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, profiler.ReadRuntimeStats())
}

// PprofProfileSummary godoc
//
// @Summary Pprof Profile Summary
// @Tags Profiling
// @Param profile path string true "profile"
// @Produce json
// @Success 200 {object} api.PprofSummaryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/pprof/profile/{profile} [get]
func (h *Handler) PprofProfileSummary(ctx *fasthttp.RequestCtx) {
	if !h.Cfg.Pprof.Enabled {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}

	profileType, ok := pprofProfileParam(ctx)
	if !ok {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("missing profile type"))
		return
	}

	seconds := parseCPUSeconds(ctx, h.Cfg.Pprof.CPUMaxSeconds)
	data, durationSecs, err := collectProfile(profileType, seconds)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	summary, err := profiler.Summarize(profileType, data, durationSecs)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, summary)
}

// PprofProfileDownload godoc
//
// @Summary Pprof Profile Download
// @Tags Profiling
// @Param profile path string true "profile"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/pprof/profile/{profile}/download [get]
func (h *Handler) PprofProfileDownload(ctx *fasthttp.RequestCtx) {
	if !h.Cfg.Pprof.Enabled {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, errPprofDisabled)
		return
	}

	profileType, ok := pprofProfileParam(ctx)
	if !ok {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("missing profile type"))
		return
	}
	seconds := parseCPUSeconds(ctx, h.Cfg.Pprof.CPUMaxSeconds)

	data, _, err := collectProfile(profileType, seconds)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
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

func pprofProfileParam(ctx *fasthttp.RequestCtx) (string, bool) {
	value := ctx.UserValue("profile")
	name, ok := value.(string)
	return name, ok && !strings.IsEmpty(name)
}

func parseCPUSeconds(ctx *fasthttp.RequestCtx, maxSeconds int) int {
	raw := strings.BytesToString(ctx.QueryArgs().Peek("seconds"))
	if strings.IsEmpty(raw) {
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
