package audit

import (
	"context"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/valyala/fasthttp"
)

const auditQueueSize = 512

type Writer struct {
	store *store.Store
	ch    chan store.AuditCreate
}

func NewWriter(st *store.Store) *Writer {
	w := &Writer{
		store: st,
		ch:    make(chan store.AuditCreate, auditQueueSize),
	}
	go w.worker()
	return w
}

func (w *Writer) worker() {
	for in := range w.ch {
		_ = w.store.InsertAudit(context.Background(), in)
	}
}

func (w *Writer) Log(ctx context.Context, in store.AuditCreate) {
	if w == nil || w.store == nil {
		return
	}
	select {
	case w.ch <- in:
	default:
		// Drop under backpressure rather than blocking request completion.
	}
}

func ParseResource(path string) (resourceType, resourceName string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return "", ""
	}
	// /api/v1/clusters/{id}/...
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
