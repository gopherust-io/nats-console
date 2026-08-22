package alerter

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/repo"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const recipientCacheTTL = 45 * time.Second

type Alert struct {
	st               *repo.DB
	recipientCacheMu sync.Mutex
	recipientCache   map[string]recipientCacheEntry
	clusterID        string
}

func New(st *repo.DB, clusterID string) *Alert {
	return &Alert{
		st:               st,
		recipientCacheMu: sync.Mutex{},
		recipientCache:   make(map[string]recipientCacheEntry),
		clusterID:        clusterID,
	}
}

type Options struct {
	Mailer        mail.Sender
	PublicBaseURL string
}

type recipientCacheEntry struct {
	expiresAt time.Time
	emails    []string
}

// Evaluate compares the latest metric samples for a cluster against enabled alert rules
func (a *Alert) Evaluate(ctx context.Context, samples []domain.MetricSample, opts Options) {
	if commonstrings.IsEmpty(a.clusterID) || len(samples) == 0 {
		return
	}

	rules, err := a.st.ListAlertRules(ctx, a.clusterID, true)
	if err != nil {
		tel.Warn().
			Err(err).
			Str("component", "alerter").
			Str("clusterID", a.clusterID).
			Msg("list alert rules failed")
		return
	}
	if len(rules) == 0 {
		return
	}

	openRules, err := a.st.ListOpenAlertRuleIDs(ctx, a.clusterID)
	if err != nil {
		tel.Warn().
			Err(err).
			Str("component", "alerter").
			Str("clusterID", a.clusterID).
			Msg("list open alerts failed")
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
			alertID, newlyOpened, err := a.st.UpsertOpenAlert(ctx, rule, a.clusterID, value, now)
			if err != nil {
				tel.Warn().Err(err).Str("component", "alerter").Str("rule_id", rule.ID).Msg("open alert failed")
				continue
			}
			if newlyOpened {
				a.notifyNewAlert(ctx, opts, alertID, rule, value, now)
			}
			continue
		}
		if _, open := openRules[rule.ID]; !open {
			continue
		}
		if err := a.st.CloseOpenAlert(ctx, rule.ID, a.clusterID, now); err != nil {
			tel.Warn().Err(err).Str("component", "alerter").Str("rule_id", rule.ID).Msg("close alert failed")
		}
	}
}

func (a *Alert) notifyNewAlert(
	ctx context.Context,
	opts Options,
	alertID string,
	rule domain.AlertRule,
	value float64,
	at time.Time) {
	if opts.Mailer == nil {
		return
	}
	claimed, err := a.st.ClaimAlertEmailNotify(ctx, alertID)
	if err != nil {
		tel.Warn().
			Err(err).
			Str("component", "alerter").
			Str("alertID", alertID).
			Msg("claim email notify failed")
		return
	}
	if !claimed {
		return
	}

	recipients, err := a.recipientEmails(ctx, a.clusterID)
	if err != nil {
		tel.Warn().
			Err(err).
			Str("component", "alerter").
			Msg("list alert email recipients failed")

		if err := a.st.ReleaseAlertEmailNotify(ctx, alertID); err != nil {
			tel.Warn().
				Err(err).
				Str("component", "alerter").
				Str("alertID", alertID).
				Msg("release email notify claim failed")
		}
		return
	}
	if len(recipients) == 0 {
		if err := a.st.ReleaseAlertEmailNotify(ctx, alertID); err != nil {
			tel.Warn().
				Err(err).
				Str("component", "alerter").
				Str("alertID", alertID).
				Msg("release email notify claim failed")
		}
		return
	}

	clusterName := ""
	if cluster, err := a.st.GetCluster(ctx, a.clusterID); err == nil {
		clusterName = cluster.Name
	}

	message := rule.Message
	if commonstrings.IsEmpty(message) {
		message = rule.Name
	}

	mailCh := make(chan mailJob, mailQueueSize)
	stop := make(chan struct{})
	wg := sync.WaitGroup{}

	mailCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	wg.Go(func() {
		for {
			select {
			case job, ok := <-mailCh:
				if !ok {
					return
				}
				safe.Run("alerter", func() {
					a.sendAlertEmail(mailCtx, job)
				})
			case <-stop:
				return
			}
		}
	})

	job := mailJob{
		mailer:      opts.Mailer,
		baseURL:     opts.PublicBaseURL,
		alertID:     alertID,
		clusterID:   a.clusterID,
		clusterName: clusterName,
		recipients:  recipients,
		alert: domain.Alert{
			ID:          alertID,
			RuleID:      rule.ID,
			ClusterID:   a.clusterID,
			Status:      domain.AlertStatusOpen,
			Severity:    rule.Severity,
			Metric:      rule.Metric,
			Message:     message,
			FiringValue: value,
			Threshold:   rule.Threshold,
			FirstSeenAt: at,
			LastSeenAt:  at,
			RuleName:    rule.Name,
		},
	}

	select {
	case mailCh <- job:
	default:
		tel.Warn().
			Str("component", "alerter").
			Str("alertID", job.alertID).
			Msg("alert email queue full; dropping notify")
		if err := a.st.ReleaseAlertEmailNotify(context.Background(), job.alertID); err != nil {
			tel.Warn().Err(err).Str("component", "alerter").Str("alertID", job.alertID).Msg("release email notify claim failed")
		}
	}
}

func (a *Alert) recipientEmails(ctx context.Context, clusterID string) ([]string, error) {
	now := time.Now()
	a.recipientCacheMu.Lock()
	if entry, ok := a.recipientCache[clusterID]; ok && now.Before(entry.expiresAt) {
		emails := append([]string(nil), entry.emails...)
		a.recipientCacheMu.Unlock()
		return emails, nil
	}
	a.recipientCacheMu.Unlock()

	users, err := a.st.ListUsers(ctx)
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
		if !auth.CanAccessCluster(auth.StoreUserToDomain(user), clusterID) {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, email)
	}

	a.recipientCacheMu.Lock()
	a.recipientCache[clusterID] = recipientCacheEntry{
		emails:    append([]string(nil), out...),
		expiresAt: now.Add(recipientCacheTTL),
	}
	for id, entry := range a.recipientCache {
		if now.After(entry.expiresAt) {
			delete(a.recipientCache, id)
		}
	}
	a.recipientCacheMu.Unlock()
	return out, nil
}
