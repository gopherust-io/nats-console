package domain

import (
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"sort"
	"strings"
)

const (
	// MetadataServiceKey groups consumers under a logical service name.
	MetadataServiceKey = "nats-consol/service"
	// MetadataCriticalKey marks a consumer (and its service) as critical.
	MetadataCriticalKey = "nats-consol/critical"
	// MetadataCriticalTrue is the metadata value that marks a consumer critical.
	MetadataCriticalTrue = "true"
)

// BlastRadius summarizes delete impact for a stream.
type BlastRadius struct {
	Stream         string   `json:"stream"`
	Critical       []string `json:"critical"`
	ServiceNames   []string `json:"serviceNames"`
	RelatedStreams []string `json:"relatedStreams"`
	ConsumerNames  []string `json:"consumerNames"`
	Services       int      `json:"services"`
	Streams        int      `json:"streams"`
	Consumers      int      `json:"consumers"`
}

// ServiceID returns the logical service identity for a consumer.
// Prefer metadata nats-consol/service; otherwise durable name, then consumer name.
func ServiceID(c ConsumerInfo) string {
	if c.Config.Metadata != nil {
		if s := strings.TrimSpace(c.Config.Metadata[MetadataServiceKey]); !commonstrings.IsEmpty(s) {
			return s
		}
	}
	if s := strings.TrimSpace(c.Config.DurableName); !commonstrings.IsEmpty(s) {
		return s
	}
	if s := strings.TrimSpace(c.Config.Name); !commonstrings.IsEmpty(s) {
		return s
	}
	return strings.TrimSpace(c.Name)
}

// IsCriticalConsumer reports whether consumer metadata marks it critical.
func IsCriticalConsumer(c ConsumerInfo) bool {
	if c.Config.Metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(c.Config.Metadata[MetadataCriticalKey]), MetadataCriticalTrue)
}

// ComputeBlastRadius builds delete-impact stats from the target stream,
// its consumers, and all streams in the account (for reverse mirror/source + DLQ).
func ComputeBlastRadius(target StreamInfo, consumers []ConsumerInfo, allStreams []StreamInfo) BlastRadius {
	targetName := strings.TrimSpace(target.Config.Name)
	serviceSet := make(map[string]struct{})
	criticalSet := make(map[string]struct{})
	consumerSet := make(map[string]struct{})

	for _, c := range consumers {
		if cn := consumerDisplayName(c); !commonstrings.IsEmpty(cn) {
			consumerSet[cn] = struct{}{}
		}
		id := ServiceID(c)
		if commonstrings.IsEmpty(id) {
			continue
		}
		serviceSet[id] = struct{}{}
		if IsCriticalConsumer(c) {
			criticalSet[id] = struct{}{}
		}
	}

	relatedSet := make(map[string]struct{})
	for _, s := range allStreams {
		name := strings.TrimSpace(s.Config.Name)
		if commonstrings.IsEmpty(name) || name == targetName {
			continue
		}
		if streamDependsOn(s, targetName) {
			relatedSet[name] = struct{}{}
			continue
		}
		if isCompanionDLQ(targetName, s) {
			relatedSet[name] = struct{}{}
		}
	}

	consumerNames := sortedKeys(consumerSet)
	return BlastRadius{
		Stream:         targetName,
		Services:       len(serviceSet),
		Streams:        len(relatedSet),
		Consumers:      len(consumerNames),
		Critical:       sortedKeys(criticalSet),
		ServiceNames:   sortedKeys(serviceSet),
		RelatedStreams: sortedKeys(relatedSet),
		ConsumerNames:  consumerNames,
	}
}

func consumerDisplayName(c ConsumerInfo) string {
	if s := strings.TrimSpace(c.Name); !commonstrings.IsEmpty(s) {
		return s
	}
	if s := strings.TrimSpace(c.Config.DurableName); !commonstrings.IsEmpty(s) {
		return s
	}
	return strings.TrimSpace(c.Config.Name)
}

func streamDependsOn(s StreamInfo, targetName string) bool {
	if s.Config.Mirror != nil && strings.TrimSpace(s.Config.Mirror.Name) == targetName {
		return true
	}
	for _, src := range s.Config.Sources {
		if strings.TrimSpace(src.Name) == targetName {
			return true
		}
	}
	return false
}

func isCompanionDLQ(targetName string, candidate StreamInfo) bool {
	if !IsDLQStream(candidate.Config.Name, candidate.Config.Metadata) {
		return false
	}
	// Conventional companion: {target}_DLQ
	if strings.EqualFold(candidate.Config.Name, targetName+DLQNameSuffix) {
		return true
	}
	return false
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
