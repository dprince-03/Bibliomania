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
	ProfilePicture *string `json:"profile_picture" validate:"omitempty,https_url"`
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

// UpdateLibraryStatusRequest is PATCH /users/me/library/{bookId}'s body.
type UpdateLibraryStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=wishlist to_read reading completed dropped"`
}

// LibraryEntryResponse is the REST shape of one shelf entry. BookTitle is
// filled in by whoever has it: this service on a write (it just validated
// the book against catalog), the gateway on GET /users/me/library (live
// lookup, since a wishlist should show the book's current title).
type LibraryEntryResponse struct {
	BookID    uint64    `json:"book_id"`
	BookTitle string    `json:"book_title"`
	Status    string    `json:"status"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateUserStatusRequest is for the admin-only PATCH /users/{id}/status.
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}
