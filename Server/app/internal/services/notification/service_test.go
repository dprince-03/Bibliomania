package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/dprince-03/Bibliomania/internal/events"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type countingMailer struct {
	sent int
	fail bool
}

func (m *countingMailer) Send(context.Context, Message) error {
	if m.fail {
		return errors.New("smtp down")
	}
	m.sent++
	return nil
}

func TestRedeliveredEventEmailsOnce(t *testing.T) {
	mr := miniredis.RunT(t)
	mailer := &countingMailer{}
	svc := NewService(mailer, NewRedisSentLog(redis.NewClient(&redis.Options{Addr: mr.Addr()})))
	ctx := context.Background()

	e, _ := events.New(ctx, events.TypeUserRegistered, "test", "1", events.UserRegistered{UserID: 1, Email: "a@b.c", FirstName: "A"})
	handler := svc.Handlers()[events.TypeUserRegistered]

	// A failed send must not be recorded as sent…
	mailer.fail = true
	if err := handler(ctx, e); err == nil {
		t.Fatal("expected send failure")
	}
	mailer.fail = false
	// …so the redelivery sends, and further redeliveries don't.
	for i := 0; i < 3; i++ {
		if err := handler(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if mailer.sent != 1 {
		t.Fatalf("sent %d emails, want 1", mailer.sent)
	}
}
