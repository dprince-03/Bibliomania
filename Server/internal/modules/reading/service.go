package reading

import (
	"context"
	"time"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
)

type Service struct {
	sessionRepo  SessionRepository
	bookmarkRepo BookmarkRepository
	bookRepo     catalog.BookRepository
}

func NewService(sessionRepo SessionRepository, bookmarkRepo BookmarkRepository, bookRepo catalog.BookRepository) *Service {
	return &Service{
		sessionRepo:  sessionRepo,
		bookmarkRepo: bookmarkRepo,
		bookRepo:     bookRepo,
	}
}

// ── Reading Session ────────────────────────────────────────

// GetSession returns the caller's own reading session for a book — 404 if
// they haven't started one yet.
func (s *Service) GetSession(ctx context.Context, userID, bookID uint64) (*ReadingSessionResponse, error) {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	session, err := s.sessionRepo.GetByUserAndBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	resp := mapSessionToResponse(session)
	return &resp, nil
}

// Sync creates or updates a user's reading session for a book. Conflict
// resolution (last write wins, compared by ClientUpdatedAt) happens in
// SessionRepository.Upsert's SQL, not here — this just re-fetches afterward
// so the response always reflects the authoritative post-merge state, which
// may differ from what the client just sent if its own data was stale.
func (s *Service) Sync(ctx context.Context, userID, bookID uint64, req UpdateProgressRequest) (*ReadingSessionResponse, error) {
	return s.upsertProgress(ctx, userID, bookID, req.CurrentPage, req.TotalPages, req.CurrentChapter, *req.ClientUpdatedAt)
}

// UpdateProgress is the plain, always-online counterpart to Sync — no
// conflict to resolve, so it just stamps "now" as the update time (which
// always wins over whatever an offline client might sync in later with an
// older timestamp — that's intentional: the two endpoints share the same
// last-write-wins mechanism, they just differ in whose clock feeds it).
func (s *Service) UpdateProgress(ctx context.Context, userID, bookID uint64, req ProgressUpdateRequest) (*ReadingSessionResponse, error) {
	return s.upsertProgress(ctx, userID, bookID, req.CurrentPage, req.TotalPages, req.CurrentChapter, time.Now())
}

func (s *Service) upsertProgress(ctx context.Context, userID, bookID uint64, currentPage, totalPages uint32, currentChapter *string, clientUpdatedAt time.Time) (*ReadingSessionResponse, error) {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	var progressPct float64
	if totalPages > 0 {
		progressPct = float64(currentPage) / float64(totalPages) * 100
	}
	isCompleted := totalPages > 0 && currentPage >= totalPages

	session := &ReadingSession{
		UserID:          userID,
		BookID:          bookID,
		CurrentPage:     currentPage,
		TotalPages:      totalPages,
		ProgressPct:     progressPct,
		CurrentChapter:  currentChapter,
		IsCompleted:     isCompleted,
		ClientUpdatedAt: &clientUpdatedAt,
	}
	if isCompleted {
		now := time.Now()
		session.CompletedAt = &now
	}

	if err := s.sessionRepo.Upsert(ctx, session); err != nil {
		return nil, err
	}

	final, err := s.sessionRepo.GetByUserAndBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	resp := mapSessionToResponse(final)
	return &resp, nil
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
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	bookmarks, err := s.bookmarkRepo.GetByUserAndBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	items := make([]BookmarkResponse, len(bookmarks))
	for i, b := range bookmarks {
		items[i] = mapBookmarkToResponse(b)
	}
	return items, nil
}

func (s *Service) CreateBookmark(ctx context.Context, userID, bookID uint64, req BookmarkRequest) (*BookmarkResponse, error) {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	bookmark := &Bookmark{
		UserID:    userID,
		BookID:    bookID,
		Page:      req.Page,
		Note:      req.Note,
		Highlight: req.Highlight,
		Color:     req.Color,
	}

	id, err := s.bookmarkRepo.Create(ctx, bookmark)
	if err != nil {
		return nil, err
	}
	bookmark.ID = id

	resp := mapBookmarkToResponse(bookmark)
	return &resp, nil
}

// DeleteBookmark checks both ownership (the bookmark belongs to this user)
// and that it belongs to the book named in the URL, returning the same
// Forbidden for either mismatch — no admin bypass, since a bookmark is a
// personal reading note, not a shared library resource.
func (s *Service) DeleteBookmark(ctx context.Context, userID, bookID, bookmarkID uint64) error {
	bookmark, err := s.bookmarkRepo.GetByID(ctx, bookmarkID)
	if err != nil {
		return err
	}

	if bookmark.UserID != userID || bookmark.BookID != bookID {
		return apperrors.Forbidden("you do not have permission to delete this bookmark")
	}

	return s.bookmarkRepo.Delete(ctx, bookmarkID)
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
