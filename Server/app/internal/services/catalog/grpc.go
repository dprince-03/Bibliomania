package catalog

import (
	"context"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

// GRPCServer implements catalogv1.CatalogServiceServer on top of the same
// BookService the REST handlers use. Errors come back as *AppError and are
// translated to gRPC codes by internal/grpcx's interceptor.
type GRPCServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	books *BookService
}

func NewGRPCServer(books *BookService) *GRPCServer {
	return &GRPCServer{books: books}
}

func (s *GRPCServer) GetBook(ctx context.Context, req *catalogv1.GetBookRequest) (*catalogv1.Book, error) {
	book, err := s.books.GetByID(ctx, req.GetBookId())
	if err != nil {
		return nil, err
	}
	return ToProto(book), nil
}

func (s *GRPCServer) GetBooksByIds(ctx context.Context, req *catalogv1.GetBooksByIdsRequest) (*catalogv1.GetBooksByIdsResponse, error) {
	books, err := s.books.GetByIDs(ctx, req.GetBookIds())
	if err != nil {
		return nil, err
	}
	resp := &catalogv1.GetBooksByIdsResponse{Books: make([]*catalogv1.Book, len(books))}
	for i := range books {
		resp.Books[i] = ToProto(&books[i])
	}
	return resp, nil
}

func (s *GRPCServer) SearchBooks(ctx context.Context, req *catalogv1.SearchBooksRequest) (*catalogv1.SearchBooksResponse, error) {
	pg := utils.NewPagination(int(req.GetPage()), int(req.GetLimit()))
	books, total, err := s.books.SearchRaw(ctx, BookSearchParams{
		Query:    req.GetQuery(),
		Genre:    req.GetGenre(),
		Format:   req.GetFormat(),
		AuthorID: req.GetAuthorId(),
		Year:     int(req.GetYear()),
	}, pg)
	if err != nil {
		return nil, err
	}
	resp := &catalogv1.SearchBooksResponse{
		Books:      make([]*catalogv1.Book, len(books)),
		TotalCount: int32(total),
		Page:       int32(pg.Page),
		Limit:      int32(pg.Limit),
	}
	for i := range books {
		resp.Books[i] = ToProto(&books[i])
	}
	return resp, nil
}

func (s *GRPCServer) ReserveCopy(ctx context.Context, req *catalogv1.ReserveCopyRequest) (*catalogv1.ReserveCopyResponse, error) {
	reserved, err := s.books.ReserveCopy(ctx, req.GetBookId(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return &catalogv1.ReserveCopyResponse{Reserved: reserved}, nil
}

func (s *GRPCServer) ReleaseCopy(ctx context.Context, req *catalogv1.ReleaseCopyRequest) (*catalogv1.ReleaseCopyResponse, error) {
	released, err := s.books.ReleaseCopy(ctx, req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return &catalogv1.ReleaseCopyResponse{Released: released}, nil
}

// ToProto maps the REST response DTO onto the gRPC message.
func ToProto(b *BookResponse) *catalogv1.Book {
	pb := &catalogv1.Book{
		Id:              b.ID,
		Title:           b.Title,
		Isbn:            b.ISBN,
		Genre:           b.Genre,
		Description:     b.Description,
		CoverImage:      b.CoverImage,
		TotalCopies:     int32(b.TotalCopies),
		AvailableCopies: int32(b.AvailableCopies),
		IsDigital:       b.IsDigital,
		FileFormat:      b.FileFormat,
		PriceCents:      b.PriceCents,
		Currency:        b.Currency,
	}
	if b.PublishedYear != nil {
		year := int32(*b.PublishedYear)
		pb.PublishedYear = &year
	}
	for _, a := range b.Authors {
		pb.Authors = append(pb.Authors, &catalogv1.Author{
			Id:        a.ID,
			FirstName: a.FirstName,
			LastName:  a.LastName,
			Biography: a.Biography,
		})
	}
	return pb
}
