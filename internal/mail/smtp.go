package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Sender delivers outbound email.
type Sender interface {
	Send(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

// NopSender discards messages (used when SMTP is disabled).
type NopSender struct{}

func (NopSender) Send(context.Context, []string, string, string, string) error {
	return nil
}

// goalign:ignore
type SMTPConfig struct {
	Host     string
	Username string
	Password string
	From     string
	Port     int
	TLS      bool
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if commonstrings.IsEmpty(strings.TrimSpace(cfg.Host)) {
		return nil, errors.New("smtp host is required")
	}
	if commonstrings.IsEmpty(strings.TrimSpace(cfg.From)) {
		return nil, errors.New("smtp from is required")
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	return &SMTPSender{cfg: cfg}, nil
}

func (s *SMTPSender) Send(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	if len(to) == 0 {
		return nil
	}
	msg := buildMIME(s.cfg.From, to, subject, textBody, htmlBody)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	var client *smtp.Client
	if s.cfg.Port == 465 || (s.cfg.TLS && s.cfg.Port == 465) {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12})
		client, err = smtp.NewClient(tlsConn, s.cfg.Host)
	} else {
		client, err = smtp.NewClient(conn, s.cfg.Host)
	}
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if s.cfg.TLS && s.cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}

	if !commonstrings.IsEmpty(s.cfg.Username) {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMIME(from string, to []string, subject, textBody, htmlBody string) []byte {
	var b strings.Builder
	boundary := "natsconsol-boundary"
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(textBody)
	b.WriteString("\r\n")
	if !commonstrings.IsEmpty(htmlBody) {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(htmlBody)
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return commonstrings.StringToBytes(b.String())
}

func sanitizeHeader(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, v)
}

// NewSenderFromConfig returns a real SMTP sender when enabled, otherwise NopSender.
func NewSenderFromConfig(enabled bool, cfg SMTPConfig) (Sender, error) {
	if !enabled {
		return NopSender{}, nil
	}
	return NewSMTPSender(cfg)
}
