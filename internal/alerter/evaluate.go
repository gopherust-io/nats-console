package alerter

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/tel"
)

const recipientCacheTTL = 45 * time.Second

// Options configures alert evaluation side effects (email).
type Options struct {
	Mailer        mail.Sender
	PublicBaseURL string
}

type recipientCacheEntry struct {
	expiresAt time.Time
	emails    []string
}

var (
	recipientCacheMu sync.Mutex
	recipientCache   = make(map[string]recipientCacheEntry)
)

// Evaluate compares the latest metric samples for a cluster against enabled alert rules.
func Evaluate(ctx context.Context, st *store.Store, clusterID string, samples []domain.MetricSample, opts Options) {
	if st == nil || clusterID == "" || len(samples) == 0 {
		return
	}

	rules, err := st.ListAlertRules(ctx, clusterID, true)
	if err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Str("cluster_id", clusterID).Msg("list alert rules failed")
		return
	}
	if len(rules) == 0 {
		return
	}

	openRules, err := st.ListOpenAlertRuleIDs(ctx, clusterID)
	if err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Str("cluster_id", clusterID).Msg("list open alerts failed")
		return
	}

	byMetric := make(map[string]float64, len(samples))
	for _, sample := range samples {
		byMetric[sample.Metric] = sample.Value
	}

	now := time.Now().UTC()
	for _, rule := range rules {
		value, ok := byMetric[rule.Metric]
		if !ok {
			continue
		}
		if domain.ThresholdMet(rule.Comparator, value, rule.Threshold) {
			alertID, newlyOpened, err := st.UpsertOpenAlert(ctx, rule, clusterID, value, now)
			if err != nil {
				tel.Warn().Err(err).Str("component", "alerter").Str("rule_id", rule.ID).Msg("open alert failed")
				continue
			}
			if newlyOpened {
				notifyNewAlert(ctx, st, opts, alertID, rule, clusterID, value, now)
			}
			continue
		}
		if _, open := openRules[rule.ID]; !open {
			continue
		}
		if err := st.CloseOpenAlert(ctx, rule.ID, clusterID, now); err != nil {
			tel.Warn().Err(err).Str("component", "alerter").Str("rule_id", rule.ID).Msg("close alert failed")
		}
	}
}

func notifyNewAlert(ctx context.Context, st *store.Store, opts Options, alertID string, rule domain.AlertRule, clusterID string, value float64, at time.Time) {
	if opts.Mailer == nil {
		return
	}
	claimed, err := st.ClaimAlertEmailNotify(ctx, alertID)
	if err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", alertID).Msg("claim email notify failed")
		return
	}
	if !claimed {
		return
	}

	release := func() {
		if err := st.ReleaseAlertEmailNotify(ctx, alertID); err != nil {
			tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", alertID).Msg("release email notify claim failed")
		}
	}

	recipients, err := recipientEmails(ctx, st, clusterID)
	if err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Msg("list alert email recipients failed")
		release()
		return
	}
	if len(recipients) == 0 {
		release()
		return
	}

	clusterName := ""
	if cluster, err := st.GetCluster(ctx, clusterID); err == nil {
		clusterName = cluster.Name
	}

	message := rule.Message
	if message == "" {
		message = rule.Name
	}
	alert := domain.Alert{
		ID:          alertID,
		RuleID:      rule.ID,
		ClusterID:   clusterID,
		Status:      domain.AlertStatusOpen,
		Severity:    rule.Severity,
		Metric:      rule.Metric,
		Message:     message,
		FiringValue: value,
		Threshold:   rule.Threshold,
		FirstSeenAt: at,
		LastSeenAt:  at,
		RuleName:    rule.Name,
	}
	content := mail.BuildAlertEmail(alert, clusterName, opts.PublicBaseURL)

	mailCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := opts.Mailer.Send(mailCtx, recipients, content.Subject, content.TextBody, content.HTMLBody); err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", alertID).Int("recipients", len(recipients)).Msg("send alert email failed")
		release()
		return
	}
	tel.Info().Str("component", "alerter").Str("alert_id", alertID).Int("recipients", len(recipients)).Msg("alert email sent")
}

func recipientEmails(ctx context.Context, st *store.Store, clusterID string) ([]string, error) {
	now := time.Now()
	recipientCacheMu.Lock()
	if entry, ok := recipientCache[clusterID]; ok && now.Before(entry.expiresAt) {
		emails := append([]string(nil), entry.emails...)
		recipientCacheMu.Unlock()
		return emails, nil
	}
	recipientCacheMu.Unlock()

	users, err := st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, user := range users {
		email := strings.TrimSpace(user.Email)
		if mail.IsPlaceholderEmail(email) {
			continue
		}
		if !auth.CanAccessCluster(user, clusterID) {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, email)
	}

	recipientCacheMu.Lock()
	recipientCache[clusterID] = recipientCacheEntry{
		emails:    append([]string(nil), out...),
		expiresAt: now.Add(recipientCacheTTL),
	}
	// Opportunistic purge of expired cluster entries.
	for id, entry := range recipientCache {
		if now.After(entry.expiresAt) {
			delete(recipientCache, id)
		}
	}
	recipientCacheMu.Unlock()
	return out, nil
}
