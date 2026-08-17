package utils

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type PaginatedResponse struct {
	Items           any  `json:"items"`
	TotalCount      int  `json:"total_count"`
	Page            int  `json:"page"`
	Limit           int  `json:"limit"`
	TotalPages      int  `json:"total_pages"`
	HasNextPage     bool `json:"has_next_page"`
	HasPreviousPage bool `json:"has_previous_page"`
}

func GetPagination(r *http.Request) Pagination {
	page := 1
	limit := 10

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	return Pagination{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

func NewPaginatedResponse(items any, total, page, limit int) PaginatedResponse {
	totalPages := (total + limit - 1) / limit
	return PaginatedResponse{
		Items:           items,
		TotalCount:      total,
		Page:            page,
		Limit:           limit,
		TotalPages:      totalPages,
		HasNextPage:     page < totalPages,
		HasPreviousPage: page > 1,
	}
}
