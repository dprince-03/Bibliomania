// cmd/seed inserts a small set of sample data for local development
// (`make seed`). It is deliberately not fully idempotent per-row — it skips
// entirely if data already looks present (an admin user by a fixed email,
// or any author at all), rather than trying to upsert every field. Intended
// for a fresh dev database, not for repeated runs against one already in use.
package main

import (
	"context"
	"log"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/config"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
	"github.com/dprince-03/Bibliotheca/internal/modules/user"
	"github.com/dprince-03/Bibliotheca/internal/utils"
	"github.com/dprince-03/Bibliotheca/pkg/mysqlclient"
)

const seedAdminEmail = "admin@bibliotheca.local"
const seedAdminPassword = "ChangeMe123!"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := mysqlclient.ConnectMySqlClient(cfg)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	userRepo := user.NewRepository(db)
	profileRepo := user.NewProfileRepository(db)
	authorRepo := catalog.NewAuthorRepository(db)
	bookRepo := catalog.NewBookRepository(db)
	bookAuthorRepo := catalog.NewBookAuthorRepository(db)

	seedAdmin(ctx, userRepo, profileRepo)
	seededCatalog := seedCatalog(ctx, authorRepo, bookRepo, bookAuthorRepo)

	// InnoDB FULLTEXT indexes cache newly-inserted rows in memory
	// (innodb_ft_cache_size) and only merge them into the on-disk index on
	// a size threshold or an explicit OPTIMIZE TABLE — not immediately on
	// insert. Without this, GET /api/v1/search finds nothing for the books
	// just seeded above until something else happens to trigger a flush.
	// Found by testing search against a freshly-seeded local database.
	if seededCatalog {
		if _, err := db.ExecContext(ctx, "OPTIMIZE TABLE books"); err != nil {
			log.Printf("warning: failed to optimize books table's fulltext index: %v", err)
		}
	}

	log.Println("seed complete")
}

func seedAdmin(ctx context.Context, userRepo user.Repository, profileRepo user.ProfileRepository) {
	if _, err := userRepo.GetByEmail(ctx, seedAdminEmail); err == nil {
		log.Printf("admin user %s already exists, skipping", seedAdminEmail)
		return
	}

	hashed, err := utils.HashPassword(seedAdminPassword)
	if err != nil {
		log.Fatalf("failed to hash seed admin password: %v", err)
	}

	admin := &user.User{
		FirstName: "Admin",
		LastName:  "User",
		Email:     seedAdminEmail,
		Password:  hashed,
		Role:      "admin",
		IsActive:  true,
	}

	id, err := userRepo.Create(ctx, admin)
	if err != nil {
		log.Fatalf("failed to create seed admin: %v", err)
	}

	if err := profileRepo.Create(ctx, &user.UserProfile{UserID: id}); err != nil {
		log.Fatalf("failed to create seed admin's profile: %v", err)
	}

	log.Printf("seeded admin user: %s / %s (change this password)", seedAdminEmail, seedAdminPassword)
}

// seedCatalog returns whether it actually inserted new books, so the
// caller knows whether the fulltext index needs an OPTIMIZE TABLE pass.
func seedCatalog(ctx context.Context, authorRepo catalog.AuthorRepository, bookRepo catalog.BookRepository, bookAuthorRepo catalog.BookAuthorRepository) bool {
	_, total, err := authorRepo.GetAll(ctx, 1, 0)
	if err != nil {
		log.Fatalf("failed to check existing authors: %v", err)
	}
	if total > 0 {
		log.Println("catalog already has authors, skipping author/book seed")
		return false
	}

	dob := func(s string) *time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			log.Fatalf("bad seed date %q: %v", s, err)
		}
		return &t
	}

	type seedBook struct {
		title, isbn, genre, description string
		year, copies                    int
	}

	seeds := []struct {
		author catalog.Author
		book   seedBook
	}{
		{
			author: catalog.Author{FirstName: "J.R.R.", LastName: "Tolkien", DateOfBirth: dob("1892-01-03")},
			book:   seedBook{"The Hobbit", "9780547928227", "Fantasy", "Bilbo Baggins is swept into an epic quest.", 1937, 3},
		},
		{
			author: catalog.Author{FirstName: "J.K.", LastName: "Rowling", DateOfBirth: dob("1965-07-31")},
			book:   seedBook{"Harry Potter and the Philosopher's Stone", "9780747532699", "Fantasy", "A young wizard begins his magical education.", 1997, 4},
		},
		{
			author: catalog.Author{FirstName: "Frank", LastName: "Herbert", DateOfBirth: dob("1920-10-08")},
			book:   seedBook{"Dune", "9780441013593", "Science Fiction", "A desert planet, a prophecy, and a fight for survival.", 1965, 2},
		},
	}

	for _, s := range seeds {
		authorID, err := authorRepo.Create(ctx, &s.author)
		if err != nil {
			log.Fatalf("failed to create seed author %s %s: %v", s.author.FirstName, s.author.LastName, err)
		}

		year := s.book.year
		book := &catalog.Book{
			Title:           s.book.title,
			ISBN:            s.book.isbn,
			Genre:           s.book.genre,
			Description:     &s.book.description,
			PublishedYear:   &year,
			TotalCopies:     s.book.copies,
			AvailableCopies: s.book.copies,
		}
		bookID, err := bookRepo.Create(ctx, book)
		if err != nil {
			log.Fatalf("failed to create seed book %q: %v", s.book.title, err)
		}

		if err := bookAuthorRepo.AssignAuthor(ctx, &catalog.BookAuthor{
			BookID: bookID, AuthorID: authorID, Role: "primary",
		}); err != nil {
			log.Fatalf("failed to assign seed author to %q: %v", s.book.title, err)
		}

		log.Printf("seeded book: %q by %s %s", s.book.title, s.author.FirstName, s.author.LastName)
	}

	return true
}
