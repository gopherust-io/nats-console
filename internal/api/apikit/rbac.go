package apikit

import (
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var staticClusterPathSegments = map[string]struct{}{
	"connections": {},
}

func ClusterIDFromPath(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if commonstrings.IsEmpty(rest) {
		return ""
	}
	clusterID, _, _ := strings.Cut(rest, "/")
	if commonstrings.IsEmpty(clusterID) {
		return ""
	}
	if _, ok := staticClusterPathSegments[clusterID]; ok {
		return ""
	}
	if !uuidPattern.MatchString(clusterID) {
		return ""
	}
	return strings.ToLower(clusterID)
}

// IsJetStreamResourcePath reports whether a cluster API path mutates streams,
// consumers, KV, or object stores (and thus requires CanManageJetStream).
func IsJetStreamResourcePath(path string) bool {
	seg := ClusterSubResource(path)
	switch seg {
	case "streams", "kv", "objects", "request-reply", "zombies", "subject-naming", "event-genome", "event-catalog", "event-wikipedia", "incident-capsules":
		return true
	default:
		return false
	}
}

// ClusterSubResource extracts the first path segment following the
// clusterId in a /api/v1/clusters/{clusterId}/... path, or "" when the path
// is exactly /api/v1/clusters/{clusterId} (no sub-resource). Returns "" also
// when path does not match the cluster prefix at all; callers that need to
// distinguish "no sub-resource" from "not a cluster path" should call
// ClusterIDFromPath first.
func ClusterSubResource(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	_, after, _ := strings.Cut(rest, "/")
	seg, _, _ := strings.Cut(after, "/")
	return seg
}

func FilterClustersForActor(clusters []domain.Cluster, actor domain.User) []domain.Cluster {
	perms := domain.PermissionsFor(actor)
	if !shouldFilterClusters(perms) {
		return clusters
	}
	out := make([]domain.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		if AllowsClusterWithGrants(actor, cluster.ID) {
			out = append(out, cluster)
		}
	}
	return out
}

func FilterConnectionStatusesForActor(statuses []domain.NATSConnectionStatus, actor domain.User) []domain.NATSConnectionStatus {
	perms := domain.PermissionsFor(actor)
	if !shouldFilterClusters(perms) {
		return statuses
	}
	out := make([]domain.NATSConnectionStatus, 0, len(statuses))
	for _, status := range statuses {
		if AllowsClusterWithGrants(actor, status.ClusterID) {
			out = append(out, status)
		}
	}
	return out
}

func AllowsClusterWithGrants(actor domain.User, clusterID string) bool {
	if domain.PermissionsFor(actor).AllowsCluster(clusterID) {
		return true
	}
	for _, g := range actor.Grants {
		switch g.ResourceType {
		case domain.ResourceSystem:
			if g.ResourceKey == clusterID {
				return true
			}
		case domain.ResourceAccount, domain.ResourceNATSUser:
			if g.ResourceKey == clusterID || strings.HasPrefix(g.ResourceKey, clusterID+":") {
				return true
			}
		}
	}
	return false
}

func AuditFilterForActor(actor domain.User, clusterID string) (domain.AuditFilter, error) {
	perms := domain.PermissionsFor(actor)
	filter := domain.AuditFilter{}
	if !commonstrings.IsEmpty(clusterID) {
		if !AllowsClusterWithGrants(actor, clusterID) {
			return filter, domain.ErrForbidden
		}
		filter.ClusterID = clusterID
		return filter, nil
	}
	if !shouldFilterClusters(perms) {
		return filter, nil
	}
	ids := ClusterIDsForActor(actor)
	if ids == nil {
		ids = []string{}
	}
	filter.ClusterIDs = ids
	return filter, nil
}

func ClusterIDsForActor(actor domain.User) []string {
	perms := domain.PermissionsFor(actor)
	ids := append([]string(nil), perms.ClusterIDs...)
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	for _, g := range actor.Grants {
		id := g.ResourceKey
		if i := strings.IndexByte(id, ':'); i >= 0 {
			id = id[:i]
		}
		if commonstrings.IsEmpty(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func shouldFilterClusters(perms domain.Permissions) bool {
	return !perms.IsRoot && !perms.AllClusters
}
