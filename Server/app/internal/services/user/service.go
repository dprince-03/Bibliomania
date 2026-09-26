package user

import (
	"context"
	"log/slog"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

const serviceName = "user-service"

// Catalog is user-service's only cross-service dependency: validating a
// book when it's added to someone's shelf.
type Catalog interface {
	BookTitle(ctx context.Context, bookID uint64) (string, error)
}

type Service struct {
	userRepo    Repository
	profileRepo ProfileRepository
	libraryRepo LibraryRepository
	catalog     Catalog
}

func NewService(userRepo Repository, profileRepo ProfileRepository, libraryRepo LibraryRepository, catalog Catalog) *Service {
	return &Service{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		libraryRepo: libraryRepo,
		catalog:     catalog,
	}
}

// ── Profile ───────────────────────────────────────────────

// GetMe 404s for a deactivated account, and — for a few milliseconds after
// registration — for a brand-new one whose auth.user_registered event
// hasn't been applied yet (eventual consistency; see the package doc).
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
		UserResponse:   mapUser(u),
		PhoneNumber:    p.PhoneNumber,
		Bio:            p.Bio,
		ProfilePicture: p.ProfilePicture,
		LastOnlineAt:   p.LastOnlineAt,
		TotalBooksRead: p.TotalBooksRead,
		TotalPagesRead: p.TotalPagesRead,
	}
}

func mapUser(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Role:      u.Role,
		IsActive:  u.IsActive,
	}
}

// ── User Library ──────────────────────────────────────────

// ListLibrary returns shelf entries without titles (see LibraryEntryResponse).
func (s *Service) ListLibrary(ctx context.Context, userID uint64, status string, pg utils.Pagination) ([]*UserLibrary, int, error) {
	return s.libraryRepo.GetByUserID(ctx, userID, status, pg.Limit, pg.Offset)
}

func (s *Service) GetLibraryEntry(ctx context.Context, userID, bookID uint64) (*UserLibrary, error) {
	return s.libraryRepo.GetEntry(ctx, userID, bookID)
}

// UpdateLibraryStatus validates the book against catalog-service (a write-
// time correctness check) before upserting the shelf entry.
func (s *Service) UpdateLibraryStatus(ctx context.Context, userID, bookID uint64, req UpdateLibraryStatusRequest) (*LibraryEntryResponse, error) {
	title, err := s.catalog.BookTitle(ctx, bookID)
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
		BookTitle: title,
		Status:    final.Status,
		AddedAt:   final.AddedAt,
		UpdatedAt: final.UpdatedAt,
	}
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
		items[i] = mapUser(u)
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

// UpdateUserStatus (de)activates an account here and, through the outbox,
// in auth-service — which then refuses logins and revokes refresh tokens.
func (s *Service) UpdateUserStatus(ctx context.Context, targetUserID uint64, req UpdateUserStatusRequest) error {
	e, err := events.New(ctx, events.TypeUserStatusChanged, serviceName, utils.Uint64Key(targetUserID), events.UserStatusChanged{
		UserID: targetUserID, IsActive: req.IsActive,
	})
	if err != nil {
		return err
	}
	return s.userRepo.UpdateStatus(ctx, targetUserID, req.IsActive, e)
}

// ── Event consumers ───────────────────────────────────────

func (s *Service) OnUserRegistered(ctx context.Context, e events.Event) error {
	var p events.UserRegistered
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.userRepo.ApplyUserRegistered(ctx, e.ID, &User{
		ID:        p.UserID,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		Email:     p.Email,
		Role:      p.Role,
		IsActive:  p.IsActive,
	})
}

func (s *Service) OnBookCompleted(ctx context.Context, e events.Event) error {
	var p events.BookCompleted
	if err := e.Decode(&p); err != nil {
		return err
	}
	applied, err := s.userRepo.ApplyBookCompleted(ctx, e.ID, p.UserID, p.TotalPages)
	if applied {
		slog.InfoContext(ctx, "reading counters updated", "user_id", p.UserID, "book_id", p.BookID)
	}
	return err
}

func (s *Service) OnProgressUpdated(ctx context.Context, e events.Event) error {
	var p events.ProgressUpdated
	if err := e.Decode(&p); err != nil {
		return err
	}
	return s.userRepo.ApplyProgress(ctx, p.UserID, p.BookID)
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
