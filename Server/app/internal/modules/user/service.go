package user

import (
	"context"

	"github.com/dprince-03/Bibliomania/internal/modules/catalog"
	"github.com/dprince-03/Bibliomania/internal/modules/reading"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

type Service struct {
	userRepo    Repository
	profileRepo ProfileRepository
	libraryRepo LibraryRepository
	sessionRepo reading.SessionRepository
	bookRepo    catalog.BookRepository
}

func NewService(
	userRepo Repository,
	profileRepo ProfileRepository,
	libraryRepo LibraryRepository,
	sessionRepo reading.SessionRepository,
	bookRepo catalog.BookRepository,
) *Service {
	return &Service{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		libraryRepo: libraryRepo,
		sessionRepo: sessionRepo,
		bookRepo:    bookRepo,
	}
}

// ── Profile ───────────────────────────────────────────────

func (s *Service) GetMe(ctx context.Context, userID uint64) (*UserProfileResponse, error) {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := mapToProfileResponse(u, profile)
	return &resp, nil
}

func (s *Service) UpdateMe(ctx context.Context, userID uint64, req UpdateProfileRequest) (*UserProfileResponse, error) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.PhoneNumber != nil {
		profile.PhoneNumber = req.PhoneNumber
	}
	if req.Bio != nil {
		profile.Bio = req.Bio
	}
	if req.ProfilePicture != nil {
		profile.ProfilePicture = req.ProfilePicture
	}

	if err := s.profileRepo.Update(ctx, profile); err != nil {
		return nil, err
	}

	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := mapToProfileResponse(u, profile)
	return &resp, nil
}

func mapToProfileResponse(u *User, p *UserProfile) UserProfileResponse {
	return UserProfileResponse{
		UserResponse: UserResponse{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Email:     u.Email,
			Role:      u.Role,
			IsActive:  u.IsActive,
		},
		PhoneNumber:    p.PhoneNumber,
		Bio:            p.Bio,
		ProfilePicture: p.ProfilePicture,
		LastOnlineAt:   p.LastOnlineAt,
		TotalBooksRead: p.TotalBooksRead,
		TotalPagesRead: p.TotalPagesRead,
	}
}

// ── User Library ──────────────────────────────────────────

func (s *Service) GetLibrary(ctx context.Context, userID uint64, status string, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	entries, total, err := s.libraryRepo.GetByUserID(ctx, userID, status, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items := make([]LibraryEntryResponse, len(entries))
	for i, e := range entries {
		title := ""
		if book, err := s.bookRepo.GetByID(ctx, e.BookID); err == nil {
			title = book.Title
		}
		items[i] = LibraryEntryResponse{
			BookID:    e.BookID,
			BookTitle: title,
			Status:    e.Status,
			AddedAt:   e.AddedAt,
			UpdatedAt: e.UpdatedAt,
		}
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

func (s *Service) UpdateLibraryStatus(ctx context.Context, userID, bookID uint64, req UpdateLibraryStatusRequest) (*LibraryEntryResponse, error) {
	book, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return nil, err
	}

	entry := &UserLibrary{
		UserID: userID,
		BookID: bookID,
		Status: req.Status,
	}
	if err := s.libraryRepo.Upsert(ctx, entry); err != nil {
		return nil, err
	}

	final, err := s.libraryRepo.GetEntry(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	resp := LibraryEntryResponse{
		BookID:    final.BookID,
		BookTitle: book.Title,
		Status:    final.Status,
		AddedAt:   final.AddedAt,
		UpdatedAt: final.UpdatedAt,
	}
	return &resp, nil
}

// ── Reading History ───────────────────────────────────────

// GetHistory is reading activity (internal/modules/reading's session data),
// distinct from borrow history (GET /borrows/my, Step 16) — physical/digital
// checkout is a different concept from reading progress.
func (s *Service) GetHistory(ctx context.Context, userID uint64, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	sessions, total, err := s.sessionRepo.GetAllByUserID(ctx, userID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items := make([]HistoryEntryResponse, len(sessions))
	for i, sess := range sessions {
		title := ""
		if book, err := s.bookRepo.GetByID(ctx, sess.BookID); err == nil {
			title = book.Title
		}
		items[i] = HistoryEntryResponse{
			BookID:      sess.BookID,
			BookTitle:   title,
			CurrentPage: sess.CurrentPage,
			TotalPages:  sess.TotalPages,
			ProgressPct: sess.ProgressPct,
			IsCompleted: sess.IsCompleted,
			LastReadAt:  sess.LastReadAt,
		}
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

// ── Admin ─────────────────────────────────────────────────

func (s *Service) GetAllUsers(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	users, total, err := s.userRepo.GetAll(ctx, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items := make([]UserResponse, len(users))
	for i, u := range users {
		items[i] = UserResponse{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Email:     u.Email,
			Role:      u.Role,
			IsActive:  u.IsActive,
		}
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

func (s *Service) UpdateUserStatus(ctx context.Context, targetUserID uint64, req UpdateUserStatusRequest) error {
	return s.userRepo.UpdateStatus(ctx, targetUserID, req.IsActive)
}
