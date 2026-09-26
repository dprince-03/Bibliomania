//go:build integration

package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dprince-03/Bibliomania/internal/testsupport"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
)

// Of N concurrent refreshes presenting the same token, exactly one may
// succeed; the rest trip reuse detection, which revokes every session —
// including the one the winner just got.
func TestRefreshTokenConcurrentRotation(t *testing.T) {
	db := testsupport.MySQL(t, Migrations)
	svc := NewService(db, NewAccountRepository(db), NewTokenRepository(db),
		jwt.NewManager("integration-test-secret-integration-test", 15*time.Minute), time.Hour)
	ctx := context.Background()

	reg, err := svc.Register(ctx, RegisterRequest{FirstName: "Ada", LastName: "Lovelace", Email: " Ada@Example.com ", Password: "correct-horse"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.User.Email != "ada@example.com" {
		t.Fatalf("email not normalized: %q", reg.User.Email)
	}

	const n = 20
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners []*AuthResponse
	)
	start := make(chan struct{})
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if resp, err := svc.RefreshToken(ctx, reg.Token.RefreshToken); err == nil {
				mu.Lock()
				winners = append(winners, resp)
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(winners) != 1 {
		t.Fatalf("expected exactly 1 successful refresh, got %d", len(winners))
	}
	// Losers presented a used token → all sessions revoked, so the winner's
	// fresh token is dead too.
	if _, err := svc.RefreshToken(ctx, winners[0].Token.RefreshToken); err == nil {
		t.Fatal("reuse detection should have revoked the winner's new token")
	}
}

// Sequential happy path: each rotation works once, and the old token is
// then rejected (and burns the family).
func TestRefreshTokenReuseRevokesFamily(t *testing.T) {
	db := testsupport.MySQL(t, Migrations)
	svc := NewService(db, NewAccountRepository(db), NewTokenRepository(db),
		jwt.NewManager("integration-test-secret-integration-test", 15*time.Minute), time.Hour)
	ctx := context.Background()

	reg, err := svc.Register(ctx, RegisterRequest{FirstName: "Alan", LastName: "Turing", Email: "alan@example.com", Password: "correct-horse"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.RefreshToken(ctx, reg.Token.RefreshToken)
	if err != nil {
		t.Fatalf("first rotation: %v", err)
	}
	third, err := svc.RefreshToken(ctx, second.Token.RefreshToken)
	if err != nil {
		t.Fatalf("second rotation: %v", err)
	}
	if _, err := svc.RefreshToken(ctx, reg.Token.RefreshToken); err == nil {
		t.Fatal("replaying the original token must fail")
	}
	if _, err := svc.RefreshToken(ctx, third.Token.RefreshToken); err == nil {
		t.Fatal("after reuse, the latest token must be revoked too")
	}
}
