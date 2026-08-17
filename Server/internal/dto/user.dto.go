package dto

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
