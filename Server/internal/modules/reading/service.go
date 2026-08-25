package reading

import (
	"context"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
)

// Service is intentionally scoped to just what Step 14 (offline sync) needs.
// Step 15 extends this same type with session-get, progress-only-update, and
// bookmark endpoints rather than introducing a second service in this package.
type Service struct {
	sessionRepo SessionRepository
	bookRepo    catalog.BookRepository
}

func NewService(sessionRepo SessionRepository, bookRepo catalog.BookRepository) *Service {
	return &Service{
		sessionRepo: sessionRepo,
		bookRepo:    bookRepo,
	}
}

// Sync creates or updates a user's reading session for a book. Conflict
// resolution (last write wins, compared by ClientUpdatedAt) happens in
// SessionRepository.Upsert's SQL, not here — this just re-fetches afterward
// so the response always reflects the authoritative post-merge state, which
// may differ from what the client just sent if its own data was stale.
func (s *Service) Sync(ctx context.Context, userID, bookID uint64, req UpdateProgressRequest) (*ReadingSessionResponse, error) {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	var progressPct float64
	if req.TotalPages > 0 {
		progressPct = float64(req.CurrentPage) / float64(req.TotalPages) * 100
	}
	isCompleted := req.TotalPages > 0 && req.CurrentPage >= req.TotalPages

	session := &ReadingSession{
		UserID:          userID,
		BookID:          bookID,
		CurrentPage:     req.CurrentPage,
		TotalPages:      req.TotalPages,
		ProgressPct:     progressPct,
		CurrentChapter:  req.CurrentChapter,
		IsCompleted:     isCompleted,
		ClientUpdatedAt: req.ClientUpdatedAt,
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
