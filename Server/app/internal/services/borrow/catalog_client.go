package borrow

import (
	"context"
	"errors"
	"net/http"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
)

// grpcCatalog is the production Catalog: catalog-service over gRPC.
type grpcCatalog struct {
	client catalogv1.CatalogServiceClient
}

func NewCatalogClient(client catalogv1.CatalogServiceClient) Catalog {
	return &grpcCatalog{client: client}
}

func (c *grpcCatalog) BookTitle(ctx context.Context, bookID uint64) (string, error) {
	book, err := c.client.GetBook(ctx, &catalogv1.GetBookRequest{BookId: bookID})
	if err != nil {
		return "", grpcx.FromStatus(err)
	}
	return book.GetTitle(), nil
}

func (c *grpcCatalog) ReserveCopy(ctx context.Context, bookID uint64, key string) error {
	resp, err := c.client.ReserveCopy(ctx, &catalogv1.ReserveCopyRequest{BookId: bookID, IdempotencyKey: key})
	if err != nil {
		return grpcx.FromStatus(err)
	}
	if !resp.GetReserved() {
		// Only possible on a replayed key whose reservation was already
		// released — treat as "no copy held".
		return grpcx.ErrNoCopies
	}
	return nil
}

func (c *grpcCatalog) ReleaseCopy(ctx context.Context, bookID uint64, key string) error {
	_, err := c.client.ReleaseCopy(ctx, &catalogv1.ReleaseCopyRequest{BookId: bookID, IdempotencyKey: key})
	return grpcx.FromStatus(err)
}

// isUnavailable reports whether err means catalog-service couldn't be
// reached (so a ReserveCopy's outcome is unknown).
func isUnavailable(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == http.StatusServiceUnavailable
}
