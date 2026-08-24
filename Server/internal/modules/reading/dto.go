package reading

import "time"

type UpdateProgressRequest struct {
	CurrentPage     uint32     `json:"current_page"     validate:"required,min=0"`
	TotalPages      uint32     `json:"total_pages"      validate:"required,min=1"`
	CurrentChapter  *string    `json:"current_chapter"  validate:"omitempty,max=255"`
	ClientUpdatedAt *time.Time `json:"client_updated_at"`
}

type ReadingSessionResponse struct {
	BookID         uint64    `json:"book_id"`
	CurrentPage    uint32    `json:"current_page"`
	TotalPages     uint32    `json:"total_pages"`
	ProgressPct    float64   `json:"progress_pct"`
	CurrentChapter *string   `json:"current_chapter,omitempty"`
	IsCompleted    bool      `json:"is_completed"`
	LastReadAt     time.Time `json:"last_read_at"`
}

type UpdateLibraryStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=wishlist to_read reading completed dropped"`
}

type BookmarkRequest struct {
	Page      uint32  `json:"page"      validate:"required,min=1"`
	Note      *string `json:"note"      validate:"omitempty,max=500"`
	Highlight *string `json:"highlight" validate:"omitempty,max=1000"`
	Color     string  `json:"color"     validate:"omitempty,oneof=yellow green blue pink purple"`
}

type BookmarkResponse struct {
	ID        uint64  `json:"id"`
	BookID    uint64  `json:"book_id"`
	Page      uint32  `json:"page"`
	Note      *string `json:"note,omitempty"`
	Highlight *string `json:"highlight,omitempty"`
	Color     string  `json:"color"`
}
