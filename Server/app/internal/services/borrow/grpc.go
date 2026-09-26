package borrow

import (
	"context"

	borrowv1 "github.com/dprince-03/Bibliomania/gen/borrow/v1"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements borrowv1.BorrowServiceServer for the gateway. The
// caller is always the JWT-verified user in ctx (internal/grpcx).
type GRPCServer struct {
	borrowv1.UnimplementedBorrowServiceServer
	service *Service
}

func NewGRPCServer(service *Service) *GRPCServer {
	return &GRPCServer{service: service}
}

func (s *GRPCServer) BorrowBook(ctx context.Context, req *borrowv1.BorrowBookRequest) (*borrowv1.BorrowRecord, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, _, err := s.service.Borrow(ctx, userID, middleware.GetUserEmail(ctx),
		BorrowRequest{BookID: req.GetBookId()}, req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return toProto(resp), nil
}

func (s *GRPCServer) ReturnBook(ctx context.Context, req *borrowv1.ReturnBookRequest) (*borrowv1.BorrowRecord, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.service.Return(ctx, userID, middleware.GetUserRole(ctx), req.GetBorrowId())
	if err != nil {
		return nil, err
	}
	return toProto(resp), nil
}

func (s *GRPCServer) ListMyBorrows(ctx context.Context, req *borrowv1.ListMyBorrowsRequest) (*borrowv1.ListMyBorrowsResponse, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.service.GetMyBorrows(ctx, userID, utils.NewPagination(int(req.GetPage()), int(req.GetLimit())))
	if err != nil {
		return nil, err
	}
	out := &borrowv1.ListMyBorrowsResponse{TotalCount: int32(total)}
	for i := range items {
		out.Records = append(out.Records, toProto(&items[i]))
	}
	return out, nil
}

func (s *GRPCServer) GetMyActiveBorrow(ctx context.Context, req *borrowv1.GetMyActiveBorrowRequest) (*borrowv1.BorrowRecord, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.service.GetMyOpenBorrow(ctx, userID, req.GetBookId())
	if err != nil {
		return nil, err
	}
	return toProto(resp), nil
}

func toProto(r *BorrowResponse) *borrowv1.BorrowRecord {
	pb := &borrowv1.BorrowRecord{
		Id:         r.ID,
		PublicId:   r.PublicID,
		BookId:     r.BookID,
		BookTitle:  r.BookTitle,
		BorrowedAt: timestamppb.New(r.BorrowedAt),
		DueAt:      timestamppb.New(r.DueAt),
		Status:     r.Status,
	}
	if r.ReturnedAt != nil {
		pb.ReturnedAt = timestamppb.New(*r.ReturnedAt)
	}
	return pb
}
