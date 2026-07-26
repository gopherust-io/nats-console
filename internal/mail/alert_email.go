package mail

import (
	"fmt"
	htmlpkg "html"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

// AlertEmailContent holds subject and bodies for an opened alert.
type AlertEmailContent struct {
	Subject  string
	TextBody string
	HTMLBody string
}

type severityTheme struct {
	label      string
	accent     string
	accentSoft string
	badgeBg    string
	badgeText  string
}

func themeForSeverity(severity string) severityTheme {
	switch strings.ToLower(severity) {
	case domain.AlertSeverityCritical:
		return severityTheme{
			label:      "Critical",
			accent:     "#B91C1C",
			accentSoft: "#FEF2F2",
			badgeBg:    "#FEE2E2",
			badgeText:  "#991B1B",
		}
	case domain.AlertSeverityWarning:
		return severityTheme{
			label:      "Warning",
			accent:     "#B45309",
			accentSoft: "#FFFBEB",
			badgeBg:    "#FEF3C7",
			badgeText:  "#92400E",
		}
	default:
		return severityTheme{
			label:      "Info",
			accent:     "#0F766E",
			accentSoft: "#F0FDFA",
			badgeBg:    "#CCFBF1",
			badgeText:  "#115E59",
		}
	}
}

func BuildAlertEmail(alert domain.Alert, clusterName, publicBaseURL string) AlertEmailContent {
	ruleLabel := alert.RuleName
	if ruleLabel == "" {
		ruleLabel = alert.Message
	}
	subject := fmt.Sprintf("[nats-consol] %s: %s", alert.Severity, ruleLabel)

	clusterLabel := clusterName
	if clusterLabel == "" {
		clusterLabel = alert.ClusterID
	}
	link := strings.TrimRight(publicBaseURL, "/") + "/admin/alerts"
	seen := alert.FirstSeenAt
	if seen.IsZero() {
		seen = time.Now().UTC()
	}
	seenFmt := seen.UTC().Format("2 Jan 2006 · 15:04 UTC")

	var text strings.Builder
	text.WriteString("An alert opened in NATS Consol.\n\n")
	fmt.Fprintf(&text, "Severity: %s\n", alert.Severity)
	fmt.Fprintf(&text, "Rule: %s\n", ruleLabel)
	fmt.Fprintf(&text, "Message: %s\n", alert.Message)
	fmt.Fprintf(&text, "Metric: %s\n", alert.Metric)
	fmt.Fprintf(&text, "Value: %g (threshold %g)\n", alert.FiringValue, alert.Threshold)
	fmt.Fprintf(&text, "Cluster: %s\n", clusterLabel)
	fmt.Fprintf(&text, "First seen: %s\n", seen.UTC().Format(time.RFC3339))
	fmt.Fprintf(&text, "\nView alerts: %s\n", link)

	return AlertEmailContent{
		Subject:  subject,
		TextBody: text.String(),
		HTMLBody: buildAlertHTML(alert, ruleLabel, clusterLabel, link, seenFmt),
	}
}

func buildAlertHTML(alert domain.Alert, ruleLabel, clusterLabel, link, seenFmt string) string {
	theme := themeForSeverity(alert.Severity)
	esc := htmlpkg.EscapeString
	valueStr := fmt.Sprintf("%g", alert.FiringValue)
	thresholdStr := fmt.Sprintf("%g", alert.Threshold)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="color-scheme" content="light">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#F4F6F8;-webkit-font-smoothing:antialiased;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F4F6F8;padding:32px 16px;">
  <tr>
    <td align="center">
      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;background-color:#FFFFFF;border-radius:16px;overflow:hidden;border:1px solid #E5E9EF;box-shadow:0 8px 24px rgba(15,23,42,0.06);">

        <!-- Accent bar -->
        <tr>
          <td style="height:4px;background-color:%s;font-size:0;line-height:0;">&nbsp;</td>
        </tr>

        <!-- Header -->
        <tr>
          <td style="padding:28px 32px 8px 32px;background-color:#FFFFFF;">
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td>
                  <span style="display:inline-block;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:12px;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:#64748B;">NATS Consol</span>
                </td>
                <td align="right">
                  <span style="display:inline-block;padding:6px 12px;border-radius:999px;background-color:%s;color:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:12px;font-weight:700;letter-spacing:0.04em;text-transform:uppercase;">%s</span>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Title -->
        <tr>
          <td style="padding:8px 32px 24px 32px;">
            <h1 style="margin:0 0 10px 0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:24px;line-height:1.3;font-weight:700;color:#0F172A;">%s</h1>
            <p style="margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:15px;line-height:1.55;color:#475569;">%s</p>
          </td>
        </tr>

        <!-- Metric highlight -->
        <tr>
          <td style="padding:0 32px 24px 32px;">
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:%s;border-radius:12px;border:1px solid #E2E8F0;">
              <tr>
                <td style="padding:20px 22px;width:50%%;vertical-align:top;border-right:1px solid rgba(15,23,42,0.06);">
                  <div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:11px;font-weight:700;letter-spacing:0.06em;text-transform:uppercase;color:#64748B;margin-bottom:6px;">Current value</div>
                  <div style="font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:28px;font-weight:700;color:%s;line-height:1.1;">%s</div>
                </td>
                <td style="padding:20px 22px;width:50%%;vertical-align:top;">
                  <div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:11px;font-weight:700;letter-spacing:0.06em;text-transform:uppercase;color:#64748B;margin-bottom:6px;">Threshold</div>
                  <div style="font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:28px;font-weight:700;color:#0F172A;line-height:1.1;">%s</div>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Details -->
        <tr>
          <td style="padding:0 32px 8px 32px;">
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="border-collapse:collapse;">
              %s
              %s
              %s
            </table>
          </td>
        </tr>

        <!-- CTA -->
        <tr>
          <td style="padding:20px 32px 32px 32px;" align="left">
            <a href="%s" style="display:inline-block;padding:14px 22px;background-color:%s;color:#FFFFFF;text-decoration:none;border-radius:10px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:14px;font-weight:700;letter-spacing:0.01em;">View alert in console →</a>
            <p style="margin:14px 0 0 0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:12px;line-height:1.5;color:#94A3B8;">Or open <span style="color:#64748B;">%s</span></p>
          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="padding:16px 32px;background-color:#F8FAFC;border-top:1px solid #E5E9EF;">
            <p style="margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:12px;line-height:1.5;color:#94A3B8;">You received this because your console account can access this cluster. Acknowledge the alert in the UI to silence the badge; email is sent once per open incident.</p>
          </td>
        </tr>

      </table>
    </td>
  </tr>
</table>
</body>
</html>`,
		esc(ruleLabel),
		theme.accent,
		theme.badgeBg,
		theme.badgeText,
		esc(theme.label),
		esc(ruleLabel),
		esc(alert.Message),
		theme.accentSoft,
		theme.accent,
		esc(valueStr),
		esc(thresholdStr),
		detailRow("Metric", `<code style="font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:13px;background-color:#F1F5F9;padding:3px 8px;border-radius:6px;color:#0F172A;">`+esc(alert.Metric)+`</code>`),
		detailRow("Cluster", esc(clusterLabel)),
		detailRow("First seen", esc(seenFmt)),
		esc(link),
		theme.accent,
		esc(link),
	)
}

func detailRow(label, valueHTML string) string {
	return fmt.Sprintf(`
              <tr>
                <td style="padding:12px 0;border-bottom:1px solid #EEF2F6;width:120px;vertical-align:top;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:13px;font-weight:600;color:#64748B;">%s</td>
                <td style="padding:12px 0;border-bottom:1px solid #EEF2F6;vertical-align:top;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:14px;color:#0F172A;">%s</td>
              </tr>`, label, valueHTML)
}

// IsPlaceholderEmail reports bootstrap/local placeholder addresses that should not receive mail.
func IsPlaceholderEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return true
	}
	return strings.HasSuffix(email, "@local")
}
