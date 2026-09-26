package notification

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dprince-03/Bibliomania/internal/events"
)

// SentLog remembers which events already produced an email, so a
// redelivered event (every broker here is at-least-once) doesn't email the
// reader twice. Backed by Redis with a TTL — notification-service still
// owns no database.
type SentLog interface {
	WasSent(ctx context.Context, eventID string) (bool, error)
	MarkSent(ctx context.Context, eventID string) error
}

type Service struct {
	mailer Mailer
	sent   SentLog // nil = no dedupe
}

func NewService(mailer Mailer, sent SentLog) *Service {
	return &Service{mailer: mailer, sent: sent}
}

// Handlers maps each event type this service reacts to onto its handler.
func (s *Service) Handlers() map[string]events.Handler {
	return map[string]events.Handler{
		events.TypeUserRegistered: s.onUserRegistered,
		events.TypeBookBorrowed:   s.onBookBorrowed,
		events.TypeBorrowOverdue:  s.onBorrowOverdue,
		events.TypeBookPurchased:  s.onBookPurchased,
	}
}

// send checks the sent log before mailing and records only after a
// successful send: a crash between the two can still (rarely) repeat an
// email, but a failed send is never recorded as sent. If the log itself is
// unreachable the email goes out anyway — a possible duplicate beats a
// missing overdue notice.
func (s *Service) send(ctx context.Context, e events.Event, m Message) error {
	if s.sent != nil {
		already, err := s.sent.WasSent(ctx, e.ID)
		if err != nil {
			slog.WarnContext(ctx, "sent-log unavailable, sending without dedupe", "error", err)
		} else if already {
			slog.InfoContext(ctx, "email already sent for this event, skipping", "event_type", e.Type, "event_id", e.ID)
			return nil
		}
	}
	if err := s.mailer.Send(ctx, m); err != nil {
		return err
	}
	if s.sent != nil {
		if err := s.sent.MarkSent(ctx, e.ID); err != nil {
			slog.WarnContext(ctx, "could not record sent email", "event_id", e.ID, "error", err)
		}
	}
	slog.InfoContext(ctx, "email sent", "event_type", e.Type, "event_id", e.ID, "to", m.To)
	return nil
}

func (s *Service) onUserRegistered(ctx context.Context, e events.Event) error {
	var p events.UserRegistered
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.send(ctx, e, Message{
		To:      p.Email,
		Subject: "Welcome to Bibliomania",
		Text: fmt.Sprintf("Hi %s,\n\nYour Bibliomania account is ready. Browse the catalog, borrow a book, "+
			"or pick up reading where you left off on any device.\n\nHappy reading!\n", p.FirstName),
	})
}

func (s *Service) onBookBorrowed(ctx context.Context, e events.Event) error {
	var p events.BorrowEvent
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.send(ctx, e, Message{
		To:      p.UserEmail,
		Subject: fmt.Sprintf("You borrowed %q", p.BookTitle),
		Text: fmt.Sprintf("You've borrowed %q. It's due back on %s.\n",
			p.BookTitle, p.DueAt.Format("Monday, 2 January 2006")),
	})
}

func (s *Service) onBorrowOverdue(ctx context.Context, e events.Event) error {
	var p events.BorrowEvent
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.send(ctx, e, Message{
		To:      p.UserEmail,
		Subject: fmt.Sprintf("%q is overdue", p.BookTitle),
		Text: fmt.Sprintf("%q was due back on %s. Please return it so the next reader can enjoy it.\n",
			p.BookTitle, p.DueAt.Format("Monday, 2 January 2006")),
	})
}

func (s *Service) onBookPurchased(ctx context.Context, e events.Event) error {
	var p events.PaymentEvent
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.send(ctx, e, Message{
		To:      p.UserEmail,
		Subject: fmt.Sprintf("Your copy of %q", p.BookTitle),
		Text: fmt.Sprintf("Thanks for your purchase of %q (%.2f %s). It's yours to read for life.\n",
			p.BookTitle, float64(p.AmountCents)/100, p.Currency),
	})
}
