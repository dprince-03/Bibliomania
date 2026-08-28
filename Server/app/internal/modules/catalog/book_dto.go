package catalog

type CreateBookRequest struct {
	Title         string   `json:"title"          validate:"required,min=1,max=255"`
	ISBN          string   `json:"isbn"           validate:"required,max=20"`
	Genre         string   `json:"genre"          validate:"required,max=100"`
	Description   *string  `json:"description"    validate:"omitempty,max=2000"`
	CoverImage    *string  `json:"cover_image"    validate:"omitempty,url"`
	PublishedYear *int     `json:"published_year" validate:"omitempty,min=1000,max=2100"`
	TotalCopies   int      `json:"total_copies"   validate:"required,min=1"`
	IsDigital     bool     `json:"is_digital"`
	AuthorIDs     []uint64 `json:"author_ids"    validate:"required,min=1"`
	AuthorRoles   []string `json:"author_roles"  validate:"omitempty"`
}

type UpdateBookRequest struct {
	Title         *string `json:"title"          validate:"omitempty,min=1,max=255"`
	Genre         *string `json:"genre"          validate:"omitempty,max=100"`
	Description   *string `json:"description"    validate:"omitempty,max=2000"`
	CoverImage    *string `json:"cover_image"    validate:"omitempty,url"`
	PublishedYear *int    `json:"published_year" validate:"omitempty,min=1000,max=2100"`
	TotalCopies   *int    `json:"total_copies"   validate:"omitempty,min=1"`
	IsDigital     *bool   `json:"is_digital"`
}

type BookResponse struct {
	ID              uint64           `json:"id"`
	Title           string           `json:"title"`
	ISBN            string           `json:"isbn"`
	Genre           string           `json:"genre"`
	Description     *string          `json:"description,omitempty"`
	CoverImage      *string          `json:"cover_image,omitempty"`
	PublishedYear   *int             `json:"published_year,omitempty"`
	TotalCopies     int              `json:"total_copies"`
	AvailableCopies int              `json:"available_copies"`
	IsDigital       bool             `json:"is_digital"`
	FileFormat      *string          `json:"file_format,omitempty"`
	Authors         []AuthorResponse `json:"authors"`
}

type AssignAuthorRequest struct {
	AuthorID uint64 `json:"author_id" validate:"required"`
	Role     string `json:"role"      validate:"required,oneof=primary co-author editor illustrator"`
}

// BookSearchParams is parsed from GET /search query params, not a JSON body
// — see BookHandler.Search. Query matches book title/description (full-text)
// OR author name (partial); Genre/Format/AuthorID/Year are exact-match
// filters. Zero values mean "no filter" for every field.
type BookSearchParams struct {
	Query    string
	Genre    string
	Format   string // "", "digital", or "physical"
	AuthorID uint64
	Year     int
}
