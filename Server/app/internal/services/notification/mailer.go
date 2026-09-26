// Package notification is notification-service: a pure event consumer
// that turns domain events into email. No public API and deliberately no
// database — "what did we send" is a log/trace concern (Loki/Tempo), not a
// queryable business entity.
//
// Delivery is at-least-once (every broker here redelivers on failure), so
// a rare duplicate email is possible; with no database there's nothing to
// dedupe against, and a duplicate reminder was judged cheaper than a
// notification store. See Server/app/docs/plan.md.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// NewMailer picks the transport: Resend's HTTP API when an API key is set
// (production), otherwise plain SMTP (Mailpit in dev). With neither, mail
// is only logged — so a bare `go run` doesn't need a mail server.
func NewMailer(resendAPIKey, smtpHost, smtpPort, from string) Mailer {
	switch {
	case resendAPIKey != "":
		return &resendMailer{apiKey: resendAPIKey, from: from, client: &http.Client{Timeout: 10 * time.Second}}
	case smtpHost != "":
		return &smtpMailer{addr: smtpHost + ":" + smtpPort, from: from}
	default:
		return logMailer{}
	}
}

type smtpMailer struct {
	addr string
	from string
}

func (m *smtpMailer) Send(_ context.Context, msg Message) error {
	var body strings.Builder
	fmt.Fprintf(&body, "From: %s\r\n", m.from)
	fmt.Fprintf(&body, "To: %s\r\n", msg.To)
	fmt.Fprintf(&body, "Subject: %s\r\n", msg.Subject)
	body.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	body.WriteString(msg.Text)
	return smtp.SendMail(m.addr, nil, envelopeAddress(m.from), []string{msg.To}, []byte(body.String()))
}

// envelopeAddress extracts "a@b" from "Name <a@b>".
func envelopeAddress(from string) string {
	if i, j := strings.Index(from, "<"), strings.Index(from, ">"); i >= 0 && j > i {
		return from[i+1 : j]
	}
	return from
}

type resendMailer struct {
	apiKey string
	from   string
	client *http.Client
}

func (m *resendMailer) Send(ctx context.Context, msg Message) error {
	payload, _ := json.Marshal(map[string]any{
		"from":    m.from,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("resend: HTTP %d", resp.StatusCode)
	}
	return nil
}

type logMailer struct{}

func (logMailer) Send(ctx context.Context, msg Message) error {
	slog.InfoContext(ctx, "email (no mail transport configured, logging only)", "to", msg.To, "subject", msg.Subject)
	return nil
}
