package alerter

import (
	"context"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	"github.com/gopherust-io/tel"
)

const mailQueueSize = 64

type mailJob struct {
	st          *store.Store
	mailer      mail.Sender
	baseURL     string
	alertID     string
	clusterID   string
	clusterName string
	recipients  []string
	alert       domain.Alert
}

var (
	mailOnce sync.Once
	mailCh   chan mailJob
	mailStop chan struct{}
	mailWG   sync.WaitGroup
)

func startMailWorker() {
	mailOnce.Do(func() {
		mailCh = make(chan mailJob, mailQueueSize)
		mailStop = make(chan struct{})
		mailWG.Go(func() {
			for {
				select {
				case job, ok := <-mailCh:
					if !ok {
						return
					}
					safe.Run("alerter", func() { sendAlertEmail(job) })
				case <-mailStop:
					return
				}
			}
		})
	})
}

func enqueueAlertEmail(job mailJob) {
	startMailWorker()
	select {
	case mailCh <- job:
	default:
		tel.Warn().
			Str("component", "alerter").
			Str("alert_id", job.alertID).
			Msg("alert email queue full; dropping notify")
		if job.st != nil {
			if err := job.st.ReleaseAlertEmailNotify(context.Background(), job.alertID); err != nil {
				tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", job.alertID).Msg("release email notify claim failed")
			}
		}
	}
}

func sendAlertEmail(job mailJob) {
	content := mail.BuildAlertEmail(job.alert, job.clusterName, job.baseURL)
	mailCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := job.mailer.Send(mailCtx, job.recipients, content.Subject, content.TextBody, content.HTMLBody); err != nil {
		tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", job.alertID).Int("recipients", len(job.recipients)).Msg("send alert email failed")
		if job.st != nil {
			if err := job.st.ReleaseAlertEmailNotify(context.Background(), job.alertID); err != nil {
				tel.Warn().Err(err).Str("component", "alerter").Str("alert_id", job.alertID).Msg("release email notify claim failed")
			}
		}
		return
	}
	tel.Info().Str("component", "alerter").Str("alert_id", job.alertID).Int("recipients", len(job.recipients)).Msg("alert email sent")
}
