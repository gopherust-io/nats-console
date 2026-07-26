package api

import (
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

var staticClusterPathSegments = map[string]struct{}{
	"connections": {},
}

func clusterIDFromPath(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return ""
	}
	clusterID, _, _ := strings.Cut(rest, "/")
	if clusterID == "" {
		return ""
	}
	if _, ok := staticClusterPathSegments[clusterID]; ok {
		return ""
	}
	if !uuidPattern.MatchString(clusterID) {
		return ""
	}
	return clusterID
}

// isJetStreamResourcePath reports whether a cluster API path mutates streams,
// consumers, KV, or object stores (and thus requires CanManageJetStream).
func isJetStreamResourcePath(path string) bool {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	_, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return false
	}
	seg, _, _ := strings.Cut(after, "/")
	switch seg {
	case "streams", "kv", "objects":
		return true
	default:
		return false
	}
}

func filterClustersForActor(clusters []domain.Cluster, actor domain.User) []domain.Cluster {
	perms := domain.PermissionsFor(actor)
	if !shouldFilterClusters(perms) {
		return clusters
	}
	out := make([]domain.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		if allowsClusterWithGrants(actor, cluster.ID) {
			out = append(out, cluster)
		}
	}
	return out
}

func filterConnectionStatusesForActor(statuses []domain.NATSConnectionStatus, actor domain.User) []domain.NATSConnectionStatus {
	perms := domain.PermissionsFor(actor)
	if !shouldFilterClusters(perms) {
		return statuses
	}
	out := make([]domain.NATSConnectionStatus, 0, len(statuses))
	for _, status := range statuses {
		if allowsClusterWithGrants(actor, status.ClusterID) {
			out = append(out, status)
		}
	}
	return out
}

func allowsClusterWithGrants(actor domain.User, clusterID string) bool {
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

func auditFilterForActor(actor domain.User, clusterID string) (domain.AuditFilter, error) {
	perms := domain.PermissionsFor(actor)
	filter := domain.AuditFilter{}
	if clusterID != "" {
		if !allowsClusterWithGrants(actor, clusterID) {
			return filter, domain.ErrForbidden
		}
		filter.ClusterID = clusterID
		return filter, nil
	}
	if !shouldFilterClusters(perms) {
		return filter, nil
	}
	ids := clusterIDsForActor(actor)
	if ids == nil {
		ids = []string{}
	}
	filter.ClusterIDs = ids
	return filter, nil
}

func clusterIDsForActor(actor domain.User) []string {
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
		if id == "" {
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
