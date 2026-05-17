package dto

import "time"

type CreateAuthorRequest struct {
	FirstName   string  `json:"first_name"   validate:"required,min=2,max=100"`
	LastName    string  `json:"last_name"    validate:"required,min=2,max=100"`
	MiddleName  *string `json:"middle_name"  validate:"omitempty,max=100"`
	Image       *string `json:"image"        validate:"omitempty,url"`
	DateOfBirth *string `json:"date_of_birth" validate:"omitempty,datetime=2006-01-02"`
	Biography   *string `json:"biography"    validate:"omitempty,max=2000"`
	Phone       *string `json:"phone"        validate:"omitempty,max=20"`
	Email       *string `json:"email"        validate:"omitempty,email"`
}

type UpdateAuthorRequest struct {
	FirstName   *string `json:"first_name"   validate:"omitempty,min=2,max=100"`
	LastName    *string `json:"last_name"    validate:"omitempty,min=2,max=100"`
	MiddleName  *string `json:"middle_name"  validate:"omitempty,max=100"`
	Image       *string `json:"image"        validate:"omitempty,url"`
	DateOfBirth *string `json:"date_of_birth" validate:"omitempty,datetime=2006-01-02"`
	Biography   *string `json:"biography"    validate:"omitempty,max=2000"`
	Phone       *string `json:"phone"        validate:"omitempty,max=20"`
	Email       *string `json:"email"        validate:"omitempty,email"`
}

type AuthorResponse struct {
	ID          uint64     `json:"id"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	MiddleName  *string    `json:"middle_name,omitempty"`
	Image       *string    `json:"image,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Biography   *string    `json:"biography,omitempty"`
	Email       *string    `json:"email,omitempty"`
}
