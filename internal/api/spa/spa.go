package spa

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/valyala/fasthttp"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Handler struct {
	staticDir string
	index     string
}

func NewSPAHandler(staticDir string) *Handler {
	absDir, err := filepath.Abs(staticDir)
	if err != nil {
		absDir = staticDir
	}
	return &Handler{
		staticDir: absDir,
		index:     filepath.Join(absDir, "index.html"),
	}
}

func (h *Handler) ServeHTTP(ctx *fasthttp.RequestCtx) {
	path := commonstrings.BytesToString(ctx.Path())
	if path == "/" {
		fasthttp.ServeFile(ctx, h.index)
		return
	}

	if strings.HasPrefix(path, "/api/") {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	filePath, ok := safeStaticFilePath(h.staticDir, path)
	if ok {
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fasthttp.ServeFile(ctx, filePath)
			return
		}
	}

	fasthttp.ServeFile(ctx, h.index)
}

// safeStaticFilePath resolves a URL path under rootDir, rejecting traversal outside the root.
func safeStaticFilePath(rootDir, urlPath string) (string, bool) {
	cleaned := path.Clean("/" + urlPath)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if commonstrings.IsEmpty(cleaned) || cleaned == "." || strings.Contains(cleaned, "..") {
		return "", false
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", false
	}

	candidate := filepath.Join(absRoot, filepath.FromSlash(cleaned))
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}

	rel, err := filepath.Rel(absRoot, absCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return absCandidate, true
}
