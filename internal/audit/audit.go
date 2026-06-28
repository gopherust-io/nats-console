package audit

import (
	"context"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/valyala/fasthttp"
)

type Writer struct {
	store *store.Store
}

func NewWriter(st *store.Store) *Writer {
	return &Writer{store: st}
}

func (w *Writer) Log(ctx context.Context, in store.AuditCreate) {
	if w == nil || w.store == nil {
		return
	}
	_ = w.store.InsertAudit(ctx, in)
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
