package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/dprince-03/Bibliomania/internal/cache"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/storage"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/jmoiron/sqlx"
)

const serviceName = "catalog-service"

type BookService struct {
	db             *sqlx.DB
	bookRepo       BookRepository
	authorRepo     AuthorRepository
	bookAuthorRepo BookAuthorRepository
	cache          cache.Cache
	store          storage.Store
	scanner        Scanner // nil = no virus scanning
	maxUploadMB    int64
	reads          ReadReplicas
}

// ReadReplicas are repositories bound to a read-only replica pool
// (READ_DATABASE_URL), used for the public, cacheable reads — list, search,
// detail, batch lookups. Writes, and every read that decides a write
// (ReserveCopy, Update's fetch, upload checks), stay on the primary: replica
// lag must never affect correctness, only how fresh a page looks. Without a
// replica configured these are the primary repositories.
type ReadReplicas struct {
	Books       BookRepository
	Authors     AuthorRepository
	BookAuthors BookAuthorRepository
}

// WithReadReplicas routes public reads to a replica (see ReadReplicas).
func (s *BookService) WithReadReplicas(r ReadReplicas) *BookService {
	s.reads = r
	return s
}

func NewBookService(
	db *sqlx.DB,
	bookRepo BookRepository,
	authorRepo AuthorRepository,
	bookAuthorRepo BookAuthorRepository,
	cache cache.Cache,
	store storage.Store,
	scanner Scanner,
	maxUploadMB int64,
) *BookService {
	return &BookService{
		db:             db,
		bookRepo:       bookRepo,
		authorRepo:     authorRepo,
		bookAuthorRepo: bookAuthorRepo,
		cache:          cache,
		store:          store,
		scanner:        scanner,
		maxUploadMB:    maxUploadMB,
		reads:          ReadReplicas{Books: bookRepo, Authors: authorRepo, BookAuthors: bookAuthorRepo},
	}
}

// ── Get All Books ─────────────────────────────────────────

func (s *BookService) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	resp, err := cache.ReadThrough(ctx, s.cache, cache.KeyBookList(pg.Page, pg.Limit),
		time.Duration(cache.TTLBookList)*time.Minute,
		func() (utils.PaginatedResponse, error) {
			books, total, err := s.reads.Books.GetAll(ctx, pg.Limit, pg.Offset)
			if err != nil {
				return utils.PaginatedResponse{}, err
			}
			items, err := s.enrichBooksFrom(ctx, s.reads.BookAuthors, books)
			if err != nil {
				return utils.PaginatedResponse{}, err
			}
			return utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit), nil
		})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Get Book By ID ────────────────────────────────────────

// GetByID is cached read-through; during a database outage it serves the
// last-known copy (cache.ReadThrough).
func (s *BookService) GetByID(ctx context.Context, id uint64) (*BookResponse, error) {
	resp, err := cache.ReadThrough(ctx, s.cache, cache.KeyBookSingle(id),
		time.Duration(cache.TTLBookSingle)*time.Minute,
		func() (BookResponse, error) {
			book, err := s.reads.Books.GetByID(ctx, id)
			if err != nil {
				return BookResponse{}, err
			}
			items, err := s.enrichBooksFrom(ctx, s.reads.BookAuthors, []*Book{book})
			if err != nil {
				return BookResponse{}, err
			}
			return items[0], nil
		})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetByIDs is the gRPC batch read (no cache — callers ask for arbitrary sets).
func (s *BookService) GetByIDs(ctx context.Context, ids []uint64) ([]BookResponse, error) {
	books, err := s.reads.Books.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return s.enrichBooksFrom(ctx, s.reads.BookAuthors, books)
}

// ── Create Book ───────────────────────────────────────────

func (s *BookService) Create(ctx context.Context, req CreateBookRequest) (*BookResponse, error) {
	// 1. Validate all author IDs exist before creating the book
	authorDTOs := make([]AuthorResponse, 0, len(req.AuthorIDs))
	for _, authorID := range req.AuthorIDs {
		author, err := s.authorRepo.GetByID(ctx, authorID)
		if err != nil {
			return nil, apperrors.BadRequest(
				fmt.Sprintf("author with id %d not found", authorID), nil,
			)
		}
		authorDTOs = append(authorDTOs, mapAuthorToResponse(author))
	}

	book := &Book{
		Title:           req.Title,
		ISBN:            req.ISBN,
		Genre:           req.Genre,
		Description:     req.Description,
		CoverImage:      req.CoverImage,
		PublishedYear:   req.PublishedYear,
		TotalCopies:     req.TotalCopies,
		AvailableCopies: req.TotalCopies, // available = total on creation
		IsDigital:       req.IsDigital,
		PriceCents:      req.PriceCents,
		Currency:        req.Currency,
	}

	// 2. Book, author links and catalog.book_added commit together — the
	// old code could leave a book with no authors if an assignment failed.
	err := events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		bookID, err := s.bookRepo.WithTx(tx).Create(ctx, book)
		if err != nil {
			return err
		}
		book.ID = bookID

		for i, authorID := range req.AuthorIDs {
			role := "primary"
			if req.AuthorRoles != nil && i < len(req.AuthorRoles) {
				role = req.AuthorRoles[i]
			}
			if err := s.bookAuthorRepo.WithTx(tx).AssignAuthor(ctx, &BookAuthor{
				BookID: bookID, AuthorID: authorID, Role: role,
			}); err != nil {
				return err
			}
		}
		return s.emit(ctx, tx, events.TypeBookAdded, book)
	})
	if err != nil {
		return nil, err
	}

	s.invalidateBookListCache(ctx)

	resp := mapBookToResponse(book, authorDTOs)
	return &resp, nil
}

// ── Update Book ───────────────────────────────────────────

func (s *BookService) Update(ctx context.Context, id uint64, req UpdateBookRequest) (*BookResponse, error) {
	book, err := s.bookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		book.Title = *req.Title
	}
	if req.Genre != nil {
		book.Genre = *req.Genre
	}
	if req.Description != nil {
		book.Description = req.Description
	}
	if req.CoverImage != nil {
		book.CoverImage = req.CoverImage
	}
	if req.PublishedYear != nil {
		book.PublishedYear = req.PublishedYear
	}
	// Changing total_copies moves available_copies by the same delta —
	// applied relatively in SQL (BookRepository.Update), so a borrow or
	// return racing with this edit isn't overwritten.
	copiesDelta := 0
	if req.TotalCopies != nil {
		copiesDelta = *req.TotalCopies - book.TotalCopies
		book.TotalCopies = *req.TotalCopies
	}
	if req.IsDigital != nil {
		book.IsDigital = *req.IsDigital
	}
	if req.PriceCents != nil {
		book.PriceCents = req.PriceCents
	}
	if req.Currency != nil {
		book.Currency = req.Currency
	}
	if book.PriceCents != nil && book.Currency == nil {
		return nil, apperrors.UnprocessableEntity("currency is required when price_cents is set")
	}

	err = events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := s.bookRepo.WithTx(tx).Update(ctx, book, copiesDelta); err != nil {
			return err
		}
		return s.emit(ctx, tx, events.TypeBookUpdated, book)
	})
	if err != nil {
		return nil, err
	}

	s.invalidateBookCache(ctx, id)
	s.invalidateBookListCache(ctx)

	return s.withAuthors(ctx, book)
}

// ── Delete Book ───────────────────────────────────────────

func (s *BookService) Delete(ctx context.Context, id uint64) error {
	book, err := s.bookRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	err = events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := s.bookRepo.WithTx(tx).Delete(ctx, id); err != nil {
			return err
		}
		return s.emit(ctx, tx, events.TypeBookDeleted, book)
	})
	if err != nil {
		return err
	}

	s.invalidateBookCache(ctx, id)
	s.invalidateBookListCache(ctx)
	return nil
}

// ── Assign Author to Book ─────────────────────────────────

func (s *BookService) AssignAuthor(ctx context.Context, bookID uint64, req AssignAuthorRequest) error {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return err
	}
	if _, err := s.authorRepo.GetByID(ctx, req.AuthorID); err != nil {
		return err
	}

	ba := &BookAuthor{
		BookID:   bookID,
		AuthorID: req.AuthorID,
		Role:     req.Role,
	}
	if err := s.bookAuthorRepo.AssignAuthor(ctx, ba); err != nil {
		return err
	}

	s.invalidateBookCache(ctx, bookID)
	s.invalidateBookListCache(ctx)
	return nil
}

// ── Remove Author from Book ───────────────────────────────

func (s *BookService) RemoveAuthor(ctx context.Context, bookID, authorID uint64) error {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return err
	}

	// Make sure at least one author remains
	authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, bookID)
	if err != nil {
		return err
	}
	if len(authors) <= 1 {
		return apperrors.BadRequest("a book must have at least one author", nil)
	}

	if err := s.bookAuthorRepo.RemoveAuthor(ctx, bookID, authorID); err != nil {
		return err
	}

	s.invalidateBookCache(ctx, bookID)
	s.invalidateBookListCache(ctx)
	return nil
}

// ── Search ────────────────────────────────────────────────

func (s *BookService) Search(
	ctx context.Context,
	params BookSearchParams,
	pg utils.Pagination,
) (*utils.PaginatedResponse, error) {
	key := cache.KeySearchBooks(params.Query, params.Genre, params.Format, params.AuthorID, params.Year, pg.Page, pg.Limit)
	resp, err := cache.ReadThrough(ctx, s.cache, key, time.Duration(cache.TTLSearchResult)*time.Minute,
		func() (utils.PaginatedResponse, error) {
			items, total, err := s.SearchRaw(ctx, params, pg)
			if err != nil {
				return utils.PaginatedResponse{}, err
			}
			return utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit), nil
		})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchRaw is Search without the response envelope — for gRPC.
func (s *BookService) SearchRaw(ctx context.Context, params BookSearchParams, pg utils.Pagination) ([]BookResponse, int, error) {
	books, total, err := s.reads.Books.Search(ctx, params, pg.Limit, pg.Offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.enrichBooksFrom(ctx, s.reads.BookAuthors, books)
	return items, total, err
}

// ── Borrow Saga: reserve / release a copy ─────────────────

// ReserveCopy is called by borrow-service (gRPC) before it creates a borrow
// record. See BookRepository.ReserveCopy for the idempotency rules.
func (s *BookService) ReserveCopy(ctx context.Context, bookID uint64, key string) (bool, error) {
	if key == "" {
		return false, apperrors.BadRequest("idempotency_key is required", nil)
	}
	var reserved bool
	err := events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		var err error
		reserved, err = s.bookRepo.WithTx(tx).ReserveCopy(ctx, bookID, key)
		if err != nil || !reserved {
			return err
		}
		return s.emitReservation(ctx, tx, events.TypeCopyReserved, bookID, key)
	})
	if err != nil {
		return false, err
	}
	if reserved {
		s.invalidateBookCache(ctx, bookID)
		s.invalidateBookListCache(ctx)
	}
	return reserved, nil
}

// ReleaseCopy is the Saga's compensating step, and also the normal path
// when a borrowed book is returned.
func (s *BookService) ReleaseCopy(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, apperrors.BadRequest("idempotency_key is required", nil)
	}
	var bookID uint64
	var released bool
	err := events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		var err error
		bookID, released, err = s.bookRepo.WithTx(tx).ReleaseCopy(ctx, key)
		if err != nil || !released {
			return err
		}
		return s.emitReservation(ctx, tx, events.TypeCopyReleased, bookID, key)
	})
	if err != nil {
		return false, err
	}
	if released {
		s.invalidateBookCache(ctx, bookID)
		s.invalidateBookListCache(ctx)
	}
	return released, nil
}

// ── E-Library: Upload / Download ──────────────────────────

// UploadFile validates, scans and stores a book's digital copy (see
// storeUpload), then points the book at it. The previous file, if any, is
// deleted only after the database update succeeds, so a failed upload
// never leaves the book without its old file.
func (s *BookService) UploadFile(ctx context.Context, bookID uint64, filename string, src io.Reader) (*BookResponse, error) {
	book, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return nil, err
	}

	key, size, format, err := s.storeUpload(ctx, bookID, filename, src)
	if err != nil {
		return nil, err
	}

	if err := s.bookRepo.UpdateFilePath(ctx, bookID, key, size, format); err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	if book.FilePath != nil && *book.FilePath != "" && *book.FilePath != key {
		if err := s.store.Delete(ctx, *book.FilePath); err != nil {
			slog.WarnContext(ctx, "could not delete replaced book file", "key", *book.FilePath, "error", err)
		}
	}

	s.invalidateBookCache(ctx, bookID)
	s.invalidateBookListCache(ctx)

	updated, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return nil, err
	}
	return s.withAuthors(ctx, updated)
}

// OpenDownload opens a book's digital copy, or 404s if it has none.
// Caller closes Download.Object.
func (s *BookService) OpenDownload(ctx context.Context, bookID uint64) (*Download, error) {
	book, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if book.FilePath == nil || *book.FilePath == "" || book.FileFormat == nil {
		return nil, apperrors.NotFound("digital file for this book")
	}

	obj, modTime, err := s.store.Open(ctx, *book.FilePath)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, apperrors.NotFound("digital file for this book")
	}
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	return &Download{
		Object:      obj,
		Name:        downloadName(book.Title, *book.FileFormat),
		ContentType: contentTypes[*book.FileFormat],
		ModTime:     modTime,
	}, nil
}

// ── Helpers ───────────────────────────────────────────────

func (s *BookService) withAuthors(ctx context.Context, book *Book) (*BookResponse, error) {
	items, err := s.enrichBooks(ctx, []*Book{book})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// enrichBooks attaches authors to a slice of books with one query for the
// whole slice (the pre-split version ran one query per book).
func (s *BookService) enrichBooks(ctx context.Context, books []*Book) ([]BookResponse, error) {
	return s.enrichBooksFrom(ctx, s.bookAuthorRepo, books)
}

func (s *BookService) enrichBooksFrom(ctx context.Context, bookAuthors BookAuthorRepository, books []*Book) ([]BookResponse, error) {
	ids := make([]uint64, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	byBook, err := bookAuthors.GetAuthorsByBookIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]BookResponse, len(books))
	for i, b := range books {
		authors := byBook[b.ID]
		authorDTOs := make([]AuthorResponse, len(authors))
		for j, a := range authors {
			authorDTOs[j] = mapAuthorToResponse(a)
		}
		items[i] = mapBookToResponse(b, authorDTOs)
	}
	return items, nil
}

func (s *BookService) emit(ctx context.Context, tx *sqlx.Tx, eventType string, book *Book) error {
	e, err := events.New(ctx, eventType, serviceName, utils.Uint64Key(book.ID), events.BookChanged{
		BookID: book.ID, Title: book.Title,
	})
	if err != nil {
		return err
	}
	return events.AddToOutbox(ctx, tx, e)
}

func (s *BookService) emitReservation(ctx context.Context, tx *sqlx.Tx, eventType string, bookID uint64, key string) error {
	e, err := events.New(ctx, eventType, serviceName, utils.Uint64Key(bookID), events.CopyReservation{
		BookID: bookID, IdempotencyKey: key,
	})
	if err != nil {
		return err
	}
	return events.AddToOutbox(ctx, tx, e)
}

func (s *BookService) invalidateBookCache(ctx context.Context, id uint64) {
	_ = s.cache.Delete(ctx, cache.KeyBookSingle(id))
}

// invalidateBookListCache drops every cached page of every list that can
// contain a book — the book list, search results, and per-author pages.
func (s *BookService) invalidateBookListCache(ctx context.Context) {
	for _, prefix := range []string{cache.PrefixBookList, cache.PrefixSearch, cache.PrefixAuthorBooks} {
		_ = s.cache.DeletePrefix(ctx, prefix)
	}
}
