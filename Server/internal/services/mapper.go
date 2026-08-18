package services

import (
	"github.com/yourusername/bibliotheca/internal/dto"
	"github.com/yourusername/bibliotheca/internal/models"
)

func mapBookToResponse(b *models.Book, authors []dto.AuthorResponse) dto.BookResponse {
	resp := dto.BookResponse{
		ID:              b.ID,
		Title:           b.Title,
		ISBN:            b.ISBN,
		Genre:           b.Genre,
		Description:     b.Description,
		CoverImage:      b.CoverImage,
		PublishedYear:   b.PublishedYear,
		TotalCopies:     b.TotalCopies,
		AvailableCopies: b.AvailableCopies,
		IsDigital:       b.IsDigital,
		FileFormat:      b.FileFormat,
		Authors:         authors,
	}

	if resp.Authors == nil {
		resp.Authors = []dto.AuthorResponse{}
	}

	return resp
}