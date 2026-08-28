package reading

import "time"

type UpdateProgressRequest struct {
	// CurrentPage has no "required" tag deliberately — 0 is the valid
	// starting value (page 0 / not started yet), and go-playground/validator's
	// "required" rejects the zero value for numeric types.
	CurrentPage     uint32     `json:"current_page"     validate:"min=0"`
	TotalPages      uint32     `json:"total_pages"      validate:"required,min=1"`
	CurrentChapter  *string    `json:"current_chapter"  validate:"omitempty,max=255"`
	ClientUpdatedAt *time.Time `json:"client_updated_at" validate:"required"`
}

// ProgressUpdateRequest is for PATCH /reading/{bookId}/progress — the
// plain, always-online progress update. Unlike UpdateProgressRequest
// (used by Sync), there's no client_updated_at: the server just stamps
// "now" and always wins, since there's no offline conflict to resolve here.
type ProgressUpdateRequest struct {
	CurrentPage    uint32  `json:"current_page"    validate:"min=0"`
	TotalPages     uint32  `json:"total_pages"     validate:"required,min=1"`
	CurrentChapter *string `json:"current_chapter" validate:"omitempty,max=255"`
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
