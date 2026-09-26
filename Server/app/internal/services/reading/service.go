package reading

import (
	"context"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

const serviceName = "reading-service"

// Catalog is reading-service's one cross-service dependency: validate a
// book (and get its title to snapshot) when a session or bookmark is
// created.
type Catalog interface {
	BookTitle(ctx context.Context, bookID uint64) (string, error)
}

type Service struct {
	store   Store
	catalog Catalog
}

func NewService(store Store, catalog Catalog) *Service {
	return &Service{store: store, catalog: catalog}
}

// ── Reading Session ────────────────────────────────────────

// GetSession returns the caller's own reading session for a book — 404 if
// they haven't started one yet. (No catalog call: before the split this
// also 404'd on an unknown book; a session can't exist for one anyway.)
func (s *Service) GetSession(ctx context.Context, userID, bookID uint64) (*ReadingSessionResponse, error) {
	session, err := s.store.GetSession(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	resp := mapSessionToResponse(session)
	return &resp, nil
}

// Sync creates or updates a user's reading session for a book with the
// client's own clock. Last write wins (see Store.UpsertProgress); the
// response is always the authoritative post-merge state, which may differ
// from what the client sent if its data was stale.
func (s *Service) Sync(ctx context.Context, userID, bookID uint64, req UpdateProgressRequest) (*ReadingSessionResponse, error) {
	return s.upsertProgress(ctx, userID, bookID, req.CurrentPage, req.TotalPages, req.CurrentChapter, *req.ClientUpdatedAt)
}

// UpdateProgress is the always-online counterpart to Sync: the server's
// clock ("now") feeds the same last-write-wins comparison.
func (s *Service) UpdateProgress(ctx context.Context, userID, bookID uint64, req ProgressUpdateRequest) (*ReadingSessionResponse, error) {
	return s.upsertProgress(ctx, userID, bookID, req.CurrentPage, req.TotalPages, req.CurrentChapter, time.Now())
}

func (s *Service) upsertProgress(ctx context.Context, userID, bookID uint64, currentPage, totalPages uint32, currentChapter *string, clientUpdatedAt time.Time) (*ReadingSessionResponse, error) {
	// Validate the book once, on the session's first write — not on every
	// progress tick (the hot path). An existing session proves the book
	// was valid when it started.
	var title string
	if existing, err := s.store.GetSession(ctx, userID, bookID); err == nil {
		title = existing.BookTitle
	} else if isNotFound(err) {
		if title, err = s.catalog.BookTitle(ctx, bookID); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	var progressPct float64
	if totalPages > 0 {
		progressPct = float64(currentPage) / float64(totalPages) * 100
	}
	isCompleted := totalPages > 0 && currentPage >= totalPages

	session := &ReadingSession{
		UserID:            userID,
		BookID:            bookID,
		BookTitle:         title,
		CurrentPage:       currentPage,
		TotalPages:        totalPages,
		ProgressPct:       progressPct,
		CurrentChapter:    currentChapter,
		IsCompleted:       isCompleted,
		ClientUpdatedAtNs: clientUpdatedAt.UnixNano(),
	}
	if isCompleted {
		now := time.Now().UTC()
		session.CompletedAt = &now
	}

	err := s.store.UpsertProgress(ctx, session, func(prev *ReadingSession, applied bool) ([]events.Event, error) {
		if !applied {
			return nil, nil
		}
		key := utils.Uint64Key(userID)
		progress, err := events.New(ctx, events.TypeProgressUpdated, serviceName, key, events.ProgressUpdated{
			UserID: userID, BookID: bookID, CurrentPage: currentPage, TotalPages: totalPages,
		})
		if err != nil {
			return nil, err
		}
		evs := []events.Event{progress}

		// Completed on this write, and wasn't before → user-service bumps
		// its cached total_books_read/total_pages_read.
		if isCompleted && (prev == nil || !prev.IsCompleted) {
			completed, err := events.New(ctx, events.TypeBookCompleted, serviceName, key, events.BookCompleted{
				UserID: userID, BookID: bookID, TotalPages: totalPages,
			})
			if err != nil {
				return nil, err
			}
			evs = append(evs, completed)
		}
		return evs, nil
	})
	if err != nil {
		return nil, err
	}

	final, err := s.store.GetSession(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	resp := mapSessionToResponse(final)
	return &resp, nil
}

// GetHistory is reading activity for GET /users/me/history, newest first.
func (s *Service) GetHistory(ctx context.Context, userID uint64, pg utils.Pagination) ([]HistoryEntryResponse, int, error) {
	sessions, total, err := s.store.ListSessions(ctx, userID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, 0, err
	}
	items := make([]HistoryEntryResponse, len(sessions))
	for i, sess := range sessions {
		items[i] = HistoryEntryResponse{
			BookID:      sess.BookID,
			BookTitle:   sess.BookTitle,
			CurrentPage: sess.CurrentPage,
			TotalPages:  sess.TotalPages,
			ProgressPct: sess.ProgressPct,
			IsCompleted: sess.IsCompleted,
			LastReadAt:  sess.LastReadAt,
		}
	}
	return items, total, nil
}

func mapSessionToResponse(s *ReadingSession) ReadingSessionResponse {
	return ReadingSessionResponse{
		BookID:         s.BookID,
		CurrentPage:    s.CurrentPage,
		TotalPages:     s.TotalPages,
		ProgressPct:    s.ProgressPct,
		CurrentChapter: s.CurrentChapter,
		IsCompleted:    s.IsCompleted,
		LastReadAt:     s.LastReadAt,
	}
}

// ── Bookmarks ─────────────────────────────────────────────

func (s *Service) GetBookmarks(ctx context.Context, userID, bookID uint64) ([]BookmarkResponse, error) {
	bookmarks, err := s.store.ListBookmarks(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	items := make([]BookmarkResponse, len(bookmarks))
	for i, b := range bookmarks {
		items[i] = mapBookmarkToResponse(b)
	}
	return items, nil
}

// CreateBookmark validates the book against catalog-service — creation is
// the one point a bookmark's book_id is checked.
func (s *Service) CreateBookmark(ctx context.Context, userID, bookID uint64, req BookmarkRequest) (*BookmarkResponse, error) {
	if _, err := s.catalog.BookTitle(ctx, bookID); err != nil {
		return nil, err
	}

	color := req.Color
	if color == "" {
		color = "yellow" // the pre-split column default
	}
	bookmark := &Bookmark{
		UserID:    userID,
		BookID:    bookID,
		Page:      req.Page,
		Note:      req.Note,
		Highlight: req.Highlight,
		Color:     color,
	}
	if err := s.store.CreateBookmark(ctx, bookmark); err != nil {
		return nil, err
	}

	resp := mapBookmarkToResponse(bookmark)
	return &resp, nil
}

// DeleteBookmark checks both ownership (the bookmark belongs to this user)
// and that it belongs to the book named in the URL, returning the same
// Forbidden for either mismatch — no admin bypass, since a bookmark is a
// personal reading note, not a shared library resource.
func (s *Service) DeleteBookmark(ctx context.Context, userID, bookID, bookmarkID uint64) error {
	bookmark, err := s.store.GetBookmark(ctx, bookmarkID)
	if err != nil {
		return err
	}

	if bookmark.UserID != userID || bookmark.BookID != bookID {
		return apperrors.Forbidden("you do not have permission to delete this bookmark")
	}

	return s.store.DeleteBookmark(ctx, bookmarkID)
}

func mapBookmarkToResponse(b *Bookmark) BookmarkResponse {
	return BookmarkResponse{
		ID:        b.ID,
		BookID:    b.BookID,
		Page:      b.Page,
		Note:      b.Note,
		Highlight: b.Highlight,
		Color:     b.Color,
	}
}

func isNotFound(err error) bool {
	appErr, ok := err.(*apperrors.AppError)
	return ok && appErr.Code == 404
}

// ── catalog-service client ────────────────────────────────

type grpcCatalog struct {
	client catalogv1.CatalogServiceClient
}

func NewCatalogClient(client catalogv1.CatalogServiceClient) Catalog {
	return &grpcCatalog{client: client}
}

func (c *grpcCatalog) BookTitle(ctx context.Context, bookID uint64) (string, error) {
	book, err := c.client.GetBook(ctx, &catalogv1.GetBookRequest{BookId: bookID})
	if err != nil {
		return "", grpcx.FromStatus(err)
	}
	return book.GetTitle(), nil
}
