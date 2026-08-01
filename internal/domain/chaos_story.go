package domain

import (
	"sort"
	"strings"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Chaos story sources and act kinds.
const (
	ChaosStorySourceDemo     = "demo"
	ChaosStorySourceAI       = "ai"
	ChaosStorySourceLiveSeed = "live-seed"

	ChaosActClusterDown    = "cluster_down"
	ChaosActQuorumLoss     = "quorum_loss"
	ChaosActSchemaMismatch = "schema_mismatch"
	ChaosActConsumerDeploy = "consumer_deploy"
	ChaosActTrafficSpike   = "traffic_spike"
	ChaosActPartition      = "partition"
	ChaosActRecovery       = "recovery"

	ChaosSeverityInfo     = ArchSeverityInfo
	ChaosSeverityWarn     = ArchSeverityWarn
	ChaosSeverityCritical = ArchSeverityCritical

	chaosStorySeedCap  = 20
	chaosActDefaultSec = 5
)

// ChaosStoryAct is one timed beat in a disaster narrative.
type ChaosStoryAct struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Targets     []string `json:"targets,omitempty"`
	DurationSec int      `json:"durationSec"`
}

// ChaosStory is a multi-act disaster narrative (Docs simulation only).
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type ChaosStory struct {
	Title         string          `json:"title"`
	Setting       string          `json:"setting"`
	Severity      string          `json:"severity"`
	Summary       string          `json:"summary"`
	Source        string          `json:"source"`
	Acts          []ChaosStoryAct `json:"acts"`
	BlastRadius   []string        `json:"blastRadius,omitempty"`
	RecoveryHints []string        `json:"recoveryHints,omitempty"`
	Demo          bool            `json:"demo,omitempty"`
}

// ChaosStorySeed is capped inventory names for AI invent prompts.
type ChaosStorySeed struct {
	Streams   []string `json:"streams"`
	Consumers []string `json:"consumers"`
	Subjects  []string `json:"subjects"`
}

// ChaosStoryInventoryInput is one stream row from jsz for seed building.
type ChaosStoryInventoryInput struct {
	Name      string
	Subjects  []string
	Consumers []string
}

// DemoChaosStory returns the canned Black Friday multi-failure showcase.
func DemoChaosStory() ChaosStory {
	return ChaosStory{
		Title:    "Black Friday payment meltdown",
		Setting:  "Black Friday peak traffic",
		Severity: ChaosSeverityCritical,
		Summary:  "Payment cluster drops during Black Friday while one JetStream node loses quorum and a consumer deploy introduces a schema mismatch.",
		Acts: []ChaosStoryAct{
			{
				Title:       "Traffic surge hits payments",
				Description: "Checkout volume spikes; PAYMENTS stream lag climbs as shoppers flood the funnel.",
				Kind:        ChaosActTrafficSpike,
				Targets:     []string{"PAYMENTS", "payments.authorized"},
				DurationSec: 5,
			},
			{
				Title:       "Payment cluster down",
				Description: "The payment processing cluster becomes unreachable; publishers to PAYMENTS start timing out.",
				Kind:        ChaosActClusterDown,
				Targets:     []string{"PAYMENTS"},
				DurationSec: 6,
			},
			{
				Title:       "JetStream quorum loss",
				Description: "One JetStream replica drops out of R3; RAFT elections stall stream acknowledges on ORDERS.",
				Kind:        ChaosActQuorumLoss,
				Targets:     []string{"ORDERS"},
				DurationSec: 6,
			},
			{
				Title:       "Bad consumer deploy",
				Description: "A rush deploy of ORDERS/billing expects a new required field; older messages fail validation and DLQ fills.",
				Kind:        ChaosActSchemaMismatch,
				Targets:     []string{"ORDERS", "billing", "orders.created"},
				DurationSec: 7,
			},
			{
				Title:       "Stabilization",
				Description: "Rollback the consumer, restore the JetStream replica, and shed non-critical traffic until payments recover.",
				Kind:        ChaosActRecovery,
				Targets:     []string{"PAYMENTS", "ORDERS", "billing"},
				DurationSec: 5,
			},
		},
		BlastRadius: []string{
			"Checkout failures and abandoned carts",
			"ORDERS consumers stalled or poisoning on schema",
			"Downstream fulfillment delayed",
		},
		RecoveryHints: []string{
			"Roll back the billing consumer before fixing schema",
			"Restore JetStream replica / wait for quorum",
			"Pause non-critical publishers until PAYMENTS is healthy",
		},
		Source: ChaosStorySourceDemo,
		Demo:   true,
	}
}

// DemoChaosStorySeed is inventory names matching the sample story.
func DemoChaosStorySeed() ChaosStorySeed {
	return ChaosStorySeed{
		Streams:   []string{"PAYMENTS", "ORDERS", "BILLING"},
		Consumers: []string{"billing", "shipping", "analytics"},
		Subjects:  []string{"payments.authorized", "orders.created", "orders.shipped"},
	}
}

// BuildChaosStorySeed caps and sorts inventory names for prompts.
func BuildChaosStorySeed(inputs []ChaosStoryInventoryInput) ChaosStorySeed {
	streamSet := map[string]struct{}{}
	consumerSet := map[string]struct{}{}
	subjectSet := map[string]struct{}{}

	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if !commonstrings.IsEmpty(name) {
			streamSet[name] = struct{}{}
		}
		for _, s := range in.Subjects {
			s = strings.TrimSpace(s)
			if commonstrings.IsEmpty(s) || strings.ContainsAny(s, "*>") {
				continue
			}
			subjectSet[s] = struct{}{}
		}
		for _, c := range in.Consumers {
			c = strings.TrimSpace(c)
			if !commonstrings.IsEmpty(c) {
				consumerSet[c] = struct{}{}
			}
		}
	}

	return ChaosStorySeed{
		Streams:   cappedSortedKeys(streamSet, chaosStorySeedCap),
		Consumers: cappedSortedKeys(consumerSet, chaosStorySeedCap),
		Subjects:  cappedSortedKeys(subjectSet, chaosStorySeedCap),
	}
}

func cappedSortedKeys(set map[string]struct{}, capN int) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) > capN {
		out = out[:capN]
	}
	return out
}

// NormalizeChaosStory fills defaults and clamps act durations.
func NormalizeChaosStory(story ChaosStory, source string) ChaosStory {
	if commonstrings.IsEmpty(story.Source) {
		story.Source = source
	}
	if commonstrings.IsEmpty(story.Severity) {
		story.Severity = ChaosSeverityWarn
	}
	if commonstrings.IsEmpty(story.Title) {
		story.Title = "Chaos story"
	}
	for i := range story.Acts {
		if story.Acts[i].DurationSec <= 0 {
			story.Acts[i].DurationSec = chaosActDefaultSec
		}
		if story.Acts[i].DurationSec > 30 {
			story.Acts[i].DurationSec = 30
		}
		if commonstrings.IsEmpty(story.Acts[i].Kind) {
			story.Acts[i].Kind = ChaosActTrafficSpike
		}
	}
	return story
}

// FilterChaosStoryTargets drops act targets not present in the seed (when seed non-empty).
func FilterChaosStoryTargets(story ChaosStory, seed ChaosStorySeed) ChaosStory {
	allowed := map[string]struct{}{}
	for _, s := range seed.Streams {
		allowed[s] = struct{}{}
	}
	for _, c := range seed.Consumers {
		allowed[c] = struct{}{}
	}
	for _, s := range seed.Subjects {
		allowed[s] = struct{}{}
	}
	if len(allowed) == 0 {
		return story
	}
	for i := range story.Acts {
		kept := make([]string, 0, len(story.Acts[i].Targets))
		for _, t := range story.Acts[i].Targets {
			t = strings.TrimSpace(t)
			if _, ok := allowed[t]; ok {
				kept = append(kept, t)
			}
		}
		story.Acts[i].Targets = kept
	}
	return story
}

// NextChaosActIndex advances the simulate playbook; returns done when past the last act.
func NextChaosActIndex(current, actCount int) (next int, done bool) {
	if actCount <= 0 {
		return 0, true
	}
	next = current + 1
	if next >= actCount {
		return actCount - 1, true
	}
	return next, false
}
