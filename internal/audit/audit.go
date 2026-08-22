package audit

import (
	"context"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/repo"
)

const auditQueueSize = 512

type Writer struct {
	db *repo.DB
	ch chan repo.AuditCreate
}

func NewWriter(db *repo.DB) *Writer {
	return &Writer{
		db: db,
		ch: make(chan repo.AuditCreate, auditQueueSize),
	}
}

func (w *Writer) Start(ctx context.Context) {
	for in := range w.ch {
		_ = w.db.InsertAudit(ctx, in)
	}
}

func (w *Writer) Stop() {
	close(w.ch)
}

func (w *Writer) Log(in repo.AuditCreate) {
	if w == nil || w.db == nil {
		return
	}
	select {
	case w.ch <- in:
	default: // Drop under backpressure rather than blocking request completion.
	}
}

func ParseResource(path string) (resourceType, resourceName string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return "", ""
	}
	if parts[2] != "clusters" {
		return "", ""
	}
	if len(parts) == 4 {
		return "cluster", parts[3]
	}
	resourceType = parts[4]
	if len(parts) > 5 {
		resourceName = parts[5]
	}
	return resourceType, resourceName
}

func ActionForMethod(method string) string {
	switch method {
	case fasthttp.MethodPost:
		return "create"
	case fasthttp.MethodPut:
		return "update"
	case fasthttp.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

func ClusterIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 4 && parts[2] == "clusters" {
		return parts[3]
	}
	return ""
}
