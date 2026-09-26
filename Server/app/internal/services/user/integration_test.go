//go:build integration

package user

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/testsupport"
)

type staticAccounts []Account

func (s staticAccounts) ListAccounts(_ context.Context, afterID uint64, limit int) ([]Account, error) {
	sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	var out []Account
	for _, a := range s {
		if a.ID > afterID && len(out) < limit {
			out = append(out, a)
		}
	}
	return out, nil
}

// Seeds each kind of drift, runs one pass, checks every repair — then a
// second pass finds nothing left (the job is idempotent).
func TestReconcilerRepairsDrift(t *testing.T) {
	db := testsupport.Postgres(t, Migrations)
	ctx := context.Background()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// 1: in sync. 2: identity drift. 3: is_active drift (user-service owns
	// it, so auth must be told). 99: orphan. 4: missing here.
	mustExec(`INSERT INTO users (id, first_name, last_name, email, role, is_active) VALUES
		(1, 'Ada', 'Lovelace', 'ada@example.com', 'member', TRUE),
		(2, 'Old', 'Name', 'old@example.com', 'member', TRUE),
		(3, 'Deac', 'Tivated', 'deac@example.com', 'member', FALSE),
		(99, 'Or', 'Phan', 'orphan@example.com', 'member', TRUE)`)

	source := staticAccounts{
		{ID: 1, Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace", Role: "member", IsActive: true},
		{ID: 2, Email: "new@example.com", FirstName: "New", LastName: "Name", Role: "librarian", IsActive: true},
		{ID: 3, Email: "deac@example.com", FirstName: "Deac", LastName: "Tivated", Role: "member", IsActive: true},
		{ID: 4, Email: "missing@example.com", FirstName: "Miss", LastName: "Ing", Role: "member", IsActive: true},
	}
	rec := NewReconciler(db, source)

	rep, ran, err := rec.ReconcileOnce(ctx)
	if err != nil || !ran {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
	want := ReconcileReport{Checked: 4, Created: 1, Updated: 1, StatusRepublished: 1, Orphans: 1}
	if rep != want {
		t.Fatalf("report %+v, want %+v", rep, want)
	}

	var u User
	if err := db.Get(&u, `SELECT `+userColumns+` FROM users WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if u.Email != "new@example.com" || u.Role != "librarian" {
		t.Fatalf("identity not repaired: %+v", u)
	}
	var profiles int
	_ = db.Get(&profiles, `SELECT count(*) FROM users_profile WHERE user_id = 4`)
	if profiles != 1 {
		t.Fatal("missing user created without a profile")
	}
	var status int
	_ = db.Get(&status, `SELECT count(*) FROM outbox_events WHERE event_type = $1`, events.TypeUserStatusChanged)
	if status != 1 {
		t.Fatalf("user.status_changed outbox rows: %d, want 1", status)
	}

	// Second pass: only the status drift persists (auth hasn't consumed the
	// event in this test) and the orphan is re-flagged; nothing re-created.
	rep, _, err = rec.ReconcileOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Created != 0 || rep.Updated != 0 {
		t.Fatalf("second pass repaired again: %+v", rep)
	}
}

// Two replicas firing at once: the advisory lock lets exactly one run.
func TestReconcilerSingleRunner(t *testing.T) {
	db := testsupport.Postgres(t, Migrations)
	source := make(staticAccounts, 0, 300)
	for i := uint64(1); i <= 300; i++ {
		source = append(source, Account{ID: i, Email: "u@example.com", FirstName: "U", LastName: "U", Role: "member", IsActive: true})
	}
	// Distinct emails (users.email is unique).
	for i := range source {
		source[i].Email = "u" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "@example.com"
	}
	rec := NewReconciler(db, source)

	var wg sync.WaitGroup
	var mu sync.Mutex
	runs, created := 0, 0
	start := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rep, ran, err := rec.ReconcileOnce(context.Background())
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if ran {
				runs++
				created += rep.Created
			}
		}()
	}
	close(start)
	wg.Wait()

	var n int
	_ = db.Get(&n, `SELECT count(*) FROM users`)
	if n != 300 || created != 300 {
		t.Fatalf("users=%d created=%d (runs=%d), want 300/300", n, created, runs)
	}
}
