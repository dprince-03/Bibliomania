package user

import "time"

type UserResponse struct {
	ID        uint64 `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
}

type UpdateProfileRequest struct {
	PhoneNumber    *string `json:"phone_number"    validate:"omitempty,min=7,max=20"`
	Bio            *string `json:"bio"             validate:"omitempty,max=500"`
	ProfilePicture *string `json:"profile_picture" validate:"omitempty,url"`
}

type UserProfileResponse struct {
	UserResponse
	PhoneNumber    *string    `json:"phone_number"`
	Bio            *string    `json:"bio"`
	ProfilePicture *string    `json:"profile_picture"`
	LastOnlineAt   *time.Time `json:"last_online_at"`
	TotalBooksRead uint32     `json:"total_books_read"`
	TotalPagesRead uint32     `json:"total_pages_read"`
}

// UpdateLibraryStatusRequest — moved from internal/modules/reading (see
// UserLibrary in model.go for why).
type UpdateLibraryStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=wishlist to_read reading completed dropped"`
}

type LibraryEntryResponse struct {
	BookID    uint64    `json:"book_id"`
	BookTitle string    `json:"book_title"`
	Status    string    `json:"status"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HistoryEntryResponse is GET /users/me/history's shape — reading activity
// (from internal/modules/reading's session data), not borrow history (that's
// GET /borrows/my, a separate Step 16 concept: physical/digital checkout,
// not reading progress).
type HistoryEntryResponse struct {
	BookID      uint64    `json:"book_id"`
	BookTitle   string    `json:"book_title"`
	CurrentPage uint32    `json:"current_page"`
	TotalPages  uint32    `json:"total_pages"`
	ProgressPct float64   `json:"progress_pct"`
	IsCompleted bool      `json:"is_completed"`
	LastReadAt  time.Time `json:"last_read_at"`
}

// UpdateUserStatusRequest is for the admin-only PATCH /users/{id}/status.
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}
