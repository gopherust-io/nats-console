package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/config"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Sender interface {
	Send(ctx context.Context, to []string, subject, textBody, htmlBody string) error
	Stop() error
}

type SMTPSender struct {
	cfg    config.Config
	conn   net.Conn
	client *smtp.Client
}

func NewSMTPSenderFromConfig(ctx context.Context, cfg config.Config) (*SMTPSender, error) {
	if !cfg.SMTP.Enabled {
		return nil, nil
	}

	dialer := &net.Dialer{Timeout: cfg.SMTP.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port))
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		err = conn.SetDeadline(deadline)
		if err != nil {
			return nil, err
		}
	}

	var client *smtp.Client
	if cfg.SMTP.TLS {
		client, err = smtp.NewClient(tls.Client(conn, &tls.Config{
			ServerName: cfg.SMTP.Host,
			MinVersion: tls.VersionTLS12,
		}), cfg.SMTP.Host)
		if err != nil {
			return nil, err
		}

		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{
				ServerName: cfg.SMTP.Host,
				MinVersion: tls.VersionTLS12,
			}); err != nil {
				return nil, err
			}
		}
	} else {
		client, err = smtp.NewClient(conn, cfg.SMTP.Host)
		if err != nil {
			return nil, err
		}
	}

	if !commonstrings.IsEmpty(cfg.SMTP.Username) && !commonstrings.IsEmpty(cfg.SMTP.Password) {
		err = client.Auth(smtp.PlainAuth("", cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Host))
		if err != nil {
			return nil, err
		}
	}

	return &SMTPSender{
		cfg:    cfg,
		client: client,
		conn:   conn,
	}, nil

}

func (s *SMTPSender) Send(_ context.Context, to []string, subject, textBody, htmlBody string) error {
	if len(to) == 0 {
		return nil
	}

	if err := s.client.Mail(s.cfg.SMTP.From); err != nil {
		return err
	}
	for _, addr := range to {
		if err := s.client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := s.client.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(buildMIME(s.cfg.SMTP.From, to, subject, textBody, htmlBody)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return s.client.Quit()
}

func (s *SMTPSender) Stop() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.conn.Close(), s.client.Close())
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
