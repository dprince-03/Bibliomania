package catalog

func mapAuthorToResponse(a *Author) AuthorResponse {
	return AuthorResponse{
		ID:          a.ID,
		FirstName:   a.FirstName,
		LastName:    a.LastName,
		MiddleName:  a.MiddleName,
		Image:       a.Image,
		DateOfBirth: a.DateOfBirth,
		Biography:   a.Biography,
		Email:       a.Email,
	}
}

func mapBookToResponse(b *Book, authors []AuthorResponse) BookResponse {
	resp := BookResponse{
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
		resp.Authors = []AuthorResponse{}
	}

	return resp
}
