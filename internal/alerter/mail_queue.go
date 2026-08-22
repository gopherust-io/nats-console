package alerter

import (
	"context"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/mail"
)

const mailQueueSize = 64

type mailJob struct {
	mailer      mail.Sender
	baseURL     string
	alertID     string
	clusterID   string
	clusterName string
	recipients  []string
	alert       domain.Alert
}

func (a *Alert) sendAlertEmail(ctx context.Context, job mailJob) {
	content := mail.BuildAlertEmail(job.alert, job.clusterName, job.baseURL)

	if err := job.mailer.Send(ctx, job.recipients, content.Subject, content.TextBody, content.HTMLBody); err != nil {
		tel.Warn().
			Err(err).
			Str("component", "alerter").
			Str("alertID", job.alertID).
			Int("recipients", len(job.recipients)).
			Msg("send alert email failed")
		if err := a.st.ReleaseAlertEmailNotify(context.Background(), job.alertID); err != nil {
			tel.Warn().
				Err(err).
				Str("component", "alerter").
				Str("alertID", job.alertID).
				Msg("release email notify claim failed")
		}
		return
	}
	tel.Info().
		Str("component", "alerter").
		Str("alertID", job.alertID).
		Int("recipients", len(job.recipients)).
		Msg("alert email sent")
}
