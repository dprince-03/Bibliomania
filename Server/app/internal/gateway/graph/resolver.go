// Package graph is the GraphQL gateway: gqlgen-generated execution code
// (generated.go, models_gen.go — don't edit) plus the hand-written
// resolvers, which fan out to the services over gRPC. The caller's JWT
// travels with every call (internal/grpcx), so each service still decides
// for itself who the caller is.
package graph

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	borrowv1 "github.com/dprince-03/Bibliomania/gen/borrow/v1"
	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	readingv1 "github.com/dprince-03/Bibliomania/gen/reading/v1"
	userv1 "github.com/dprince-03/Bibliomania/gen/user/v1"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/gateway/graph/model"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Resolver struct {
	Catalog catalogv1.CatalogServiceClient
	Borrow  borrowv1.BorrowServiceClient
	Reading readingv1.ReadingServiceClient
	User    userv1.UserServiceClient
}

// ── Errors ────────────────────────────────────────────────

// ErrorPresenter turns *AppError (including ones translated from gRPC
// statuses) into GraphQL errors with a machine-readable extensions.code,
// so clients branch on "NOT_FOUND" rather than message text.
func ErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	gqlErr := graphql.DefaultErrorPresenter(ctx, err)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		gqlErr.Message = appErr.Message
		if gqlErr.Extensions == nil {
			gqlErr.Extensions = map[string]any{}
		}
		gqlErr.Extensions["code"] = errorCode(appErr.Code)
		gqlErr.Extensions["status"] = appErr.Code
	}
	return gqlErr
}

func errorCode(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "BAD_USER_INPUT"
	case http.StatusUnauthorized:
		return "UNAUTHENTICATED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "INTERNAL_SERVER_ERROR"
	}
}

func requireUser(ctx context.Context) error {
	if middleware.GetUserID(ctx) == 0 {
		return apperrors.Unauthorized("authentication required")
	}
	return nil
}

func isNotFound(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == http.StatusNotFound
}

func parseID(id, field string) (uint64, error) {
	v, err := strconv.ParseUint(id, 10, 64)
	if err != nil || v == 0 {
		return 0, apperrors.BadRequest("invalid "+field, nil)
	}
	return v, nil
}

// ── Mapping ───────────────────────────────────────────────

func idString(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func ts(t *timestamppb.Timestamp) string {
	return t.AsTime().UTC().Format(time.RFC3339)
}

func bookFromProto(b *catalogv1.Book) *model.Book {
	out := &model.Book{
		ID:              idString(b.GetId()),
		Title:           b.GetTitle(),
		Isbn:            b.GetIsbn(),
		Genre:           b.GetGenre(),
		Description:     b.Description,
		CoverImage:      b.CoverImage,
		IsDigital:       b.GetIsDigital(),
		FileFormat:      b.FileFormat,
		TotalCopies:     int(b.GetTotalCopies()),
		AvailableCopies: int(b.GetAvailableCopies()),
		Currency:        b.Currency,
		Authors:         []*model.Author{},
	}
	if b.PublishedYear != nil {
		y := int(b.GetPublishedYear())
		out.PublishedYear = &y
	}
	if b.PriceCents != nil {
		p := int(b.GetPriceCents())
		out.PriceCents = &p
	}
	for _, a := range b.GetAuthors() {
		out.Authors = append(out.Authors, &model.Author{
			ID:        idString(a.GetId()),
			FirstName: a.GetFirstName(),
			LastName:  a.GetLastName(),
			Biography: a.Biography,
		})
	}
	return out
}

func borrowFromProto(r *borrowv1.BorrowRecord) *model.BorrowRecord {
	out := &model.BorrowRecord{
		ID:         idString(r.GetId()),
		PublicID:   r.GetPublicId(),
		BookID:     idString(r.GetBookId()),
		BookTitle:  r.GetBookTitle(),
		BorrowedAt: ts(r.GetBorrowedAt()),
		DueAt:      ts(r.GetDueAt()),
		Status:     model.BorrowStatus(strings.ToUpper(r.GetStatus())),
	}
	if r.ReturnedAt != nil {
		s := ts(r.ReturnedAt)
		out.ReturnedAt = &s
	}
	return out
}

func libraryFromProto(e *userv1.LibraryEntry) *model.LibraryEntry {
	return &model.LibraryEntry{
		BookID:    idString(e.GetBookId()),
		Status:    model.LibraryStatus(strings.ToUpper(e.GetStatus())),
		AddedAt:   ts(e.GetAddedAt()),
		UpdatedAt: ts(e.GetUpdatedAt()),
	}
}

func historyFromProto(s *readingv1.ReadingSession) *model.ReadingHistoryEntry {
	return &model.ReadingHistoryEntry{
		BookID:      idString(s.GetBookId()),
		BookTitle:   s.GetBookTitle(),
		CurrentPage: int(s.GetCurrentPage()),
		TotalPages:  int(s.GetTotalPages()),
		ProgressPct: s.GetProgressPct(),
		IsCompleted: s.GetIsCompleted(),
		LastReadAt:  ts(s.GetLastReadAt()),
	}
}

func bookmarkFromProto(b *readingv1.Bookmark) *model.Bookmark {
	return &model.Bookmark{
		ID:        idString(b.GetId()),
		BookID:    idString(b.GetBookId()),
		Page:      int(b.GetPage()),
		Note:      b.Note,
		Highlight: b.Highlight,
		Color:     b.GetColor(),
	}
}

func pageInfo(pg utils.Pagination, total int) *model.PageInfo {
	p := utils.NewPaginatedResponse(nil, total, pg.Page, pg.Limit)
	return &model.PageInfo{
		Page:            p.Page,
		Limit:           p.Limit,
		TotalPages:      p.TotalPages,
		TotalCount:      p.TotalCount,
		HasNextPage:     p.HasNextPage,
		HasPreviousPage: p.HasPreviousPage,
	}
}

func pagination(page, limit *int) utils.Pagination {
	p, l := 0, 0
	if page != nil {
		p = *page
	}
	if limit != nil {
		l = *limit
	}
	return utils.NewPagination(p, l)
}

// getBook resolves one book live from catalog-service; nil (not an error)
// if it no longer exists.
func (r *Resolver) getBook(ctx context.Context, id string) (*model.Book, error) {
	bookID, err := parseID(id, "book id")
	if err != nil {
		return nil, err
	}
	b, err := r.Catalog.GetBook(ctx, &catalogv1.GetBookRequest{BookId: bookID})
	if err != nil {
		if err := grpcx.FromStatus(err); isNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	return bookFromProto(b), nil
}

func badInput(msg string) error {
	return apperrors.BadRequest(msg, nil)
}
