// cmd/seed inserts a small set of sample data for local development
// (`make seed`), against a running stack. It skips each part if data
// already looks present (the admin email, or any author at all) rather
// than upserting field by field — meant for a fresh dev stack.
//
// Since the microservices split it no longer writes straight into one
// shared database:
//   - the admin account is inserted into auth-service's database together
//     with its auth.user_registered outbox event (registration via the API
//     always yields a member), so user-service learns about it the normal
//     way;
//   - the catalog is created through the gateway's REST API as that admin,
//     so it goes through catalog-service's validation, cache invalidation
//     and events exactly like real data.
//
// Env: SEED_AUTH_DATABASE_URL (auth's MySQL DSN, reachable from where this
// runs) and SEED_GATEWAY_URL (default http://localhost:9081 — the dev
// stack's published gateway port).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/services/auth"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/mysqlclient"

	"github.com/jmoiron/sqlx"
)

const seedAdminEmail = "admin@bibliomania.local"
const seedAdminPassword = "ChangeMe123!"

func main() {
	ctx := context.Background()

	authDSN := os.Getenv("SEED_AUTH_DATABASE_URL")
	if authDSN == "" {
		log.Fatal("SEED_AUTH_DATABASE_URL is required (auth-service's MySQL DSN)")
	}
	gatewayURL := os.Getenv("SEED_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:9081"
	}

	db, err := mysqlclient.Connect(ctx, authDSN)
	if err != nil {
		log.Fatalf("auth database error: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, database.DriverMySQL, auth.Migrations); err != nil {
		log.Fatalf("auth migration error: %v", err)
	}

	seedAdmin(ctx, db)

	api := &client{base: gatewayURL + "/api/v1", http: &http.Client{Timeout: 10 * time.Second}}
	if err := api.login(seedAdminEmail, seedAdminPassword); err != nil {
		log.Fatalf("logging in as the seed admin via %s: %v", gatewayURL, err)
	}
	seedCatalog(api)

	log.Println("seed complete")
}

func seedAdmin(ctx context.Context, db *sqlx.DB) {
	if _, err := auth.NewAccountRepository(db).GetByEmail(ctx, seedAdminEmail); err == nil {
		log.Printf("admin user %s already exists, skipping", seedAdminEmail)
		return
	}

	hashed, err := utils.HashPassword(seedAdminPassword)
	if err != nil {
		log.Fatalf("failed to hash seed admin password: %v", err)
	}

	admin := &auth.Account{
		FirstName: "Admin",
		LastName:  "User",
		Email:     seedAdminEmail,
		Password:  hashed,
		Role:      "admin",
		IsActive:  true,
	}
	if err := auth.CreateAccount(ctx, db, admin); err != nil {
		log.Fatalf("failed to create seed admin: %v", err)
	}
	log.Printf("seeded admin user: %s / %s (change this password)", seedAdminEmail, seedAdminPassword)
}

func seedCatalog(api *client) {
	var authors struct {
		TotalCount int `json:"total_count"`
	}
	if err := api.do(http.MethodGet, "/authors?limit=1", nil, &authors); err != nil {
		log.Fatalf("failed to check existing authors: %v", err)
	}
	if authors.TotalCount > 0 {
		log.Println("catalog already has authors, skipping author/book seed")
		return
	}

	type seedBook struct {
		title, isbn, genre, description string
		year, copies                    int
	}
	seeds := []struct {
		first, last, dob string
		book             seedBook
	}{
		{"J.R.R.", "Tolkien", "1892-01-03", seedBook{"The Hobbit", "9780547928227", "Fantasy", "Bilbo Baggins is swept into an epic quest.", 1937, 3}},
		{"J.K.", "Rowling", "1965-07-31", seedBook{"Harry Potter and the Philosopher's Stone", "9780747532699", "Fantasy", "A young wizard begins his magical education.", 1997, 4}},
		{"Frank", "Herbert", "1920-10-08", seedBook{"Dune", "9780441013593", "Science Fiction", "A desert planet, a prophecy, and a fight for survival.", 1965, 2}},
	}

	for _, s := range seeds {
		var author struct {
			ID uint64 `json:"id"`
		}
		if err := api.do(http.MethodPost, "/authors", map[string]any{
			"first_name": s.first, "last_name": s.last, "date_of_birth": s.dob,
		}, &author); err != nil {
			log.Fatalf("failed to create seed author %s %s: %v", s.first, s.last, err)
		}

		if err := api.do(http.MethodPost, "/books", map[string]any{
			"title": s.book.title, "isbn": s.book.isbn, "genre": s.book.genre,
			"description": s.book.description, "published_year": s.book.year,
			"total_copies": s.book.copies, "author_ids": []uint64{author.ID},
		}, nil); err != nil {
			log.Fatalf("failed to create seed book %q: %v", s.book.title, err)
		}
		log.Printf("seeded book: %q by %s %s", s.book.title, s.first, s.last)
	}
}

// ── Tiny REST client ──────────────────────────────────────

type client struct {
	base  string
	token string
	http  *http.Client
}

func (c *client) login(email, password string) error {
	// A freshly inserted account can take a moment to be visible; retry
	// briefly (also covers the stack still starting up).
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		var resp struct {
			Token struct {
				AccessToken string `json:"access_token"`
			} `json:"token"`
		}
		if err = c.do(http.MethodPost, "/auth/login", map[string]string{"email": email, "password": password}, &resp); err == nil {
			c.token = resp.Token.AccessToken
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

func (c *client) do(method, path string, body, out any) error {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("%s %s: HTTP %d, unreadable body", method, path, resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, envelope.Error)
	}
	if out != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
