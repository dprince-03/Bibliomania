package user

import (
	"context"

	userv1 "github.com/dprince-03/Bibliomania/gen/user/v1"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements userv1.UserServiceServer for the gateway.
type GRPCServer struct {
	userv1.UnimplementedUserServiceServer
	service  *Service
	validate func(any) error
}

func NewGRPCServer(service *Service, validate func(any) error) *GRPCServer {
	return &GRPCServer{service: service, validate: validate}
}

func (s *GRPCServer) GetMe(ctx context.Context, _ *userv1.GetMeRequest) (*userv1.UserProfile, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.service.GetMe(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userv1.UserProfile{
		Id:             p.ID,
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		Email:          p.Email,
		Role:           p.Role,
		IsActive:       p.IsActive,
		PhoneNumber:    p.PhoneNumber,
		Bio:            p.Bio,
		ProfilePicture: p.ProfilePicture,
		TotalBooksRead: p.TotalBooksRead,
		TotalPagesRead: p.TotalPagesRead,
	}, nil
}

func (s *GRPCServer) ListLibrary(ctx context.Context, req *userv1.ListLibraryRequest) (*userv1.ListLibraryResponse, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	entries, total, err := s.service.ListLibrary(ctx, userID, req.GetStatus(), utils.NewPagination(int(req.GetPage()), int(req.GetLimit())))
	if err != nil {
		return nil, err
	}
	out := &userv1.ListLibraryResponse{TotalCount: int32(total)}
	for _, e := range entries {
		out.Entries = append(out.Entries, entryToProto(e))
	}
	return out, nil
}

func (s *GRPCServer) GetLibraryEntry(ctx context.Context, req *userv1.GetLibraryEntryRequest) (*userv1.LibraryEntry, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	e, err := s.service.GetLibraryEntry(ctx, userID, req.GetBookId())
	if err != nil {
		return nil, err
	}
	return entryToProto(e), nil
}

func (s *GRPCServer) UpdateLibraryStatus(ctx context.Context, req *userv1.UpdateLibraryStatusRequest) (*userv1.LibraryEntry, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	body := UpdateLibraryStatusRequest{Status: req.GetStatus()}
	if err := s.validate(body); err != nil {
		return nil, invalid(err)
	}
	resp, err := s.service.UpdateLibraryStatus(ctx, userID, req.GetBookId(), body)
	if err != nil {
		return nil, err
	}
	return &userv1.LibraryEntry{
		BookId:    resp.BookID,
		Status:    resp.Status,
		AddedAt:   timestamppb.New(resp.AddedAt),
		UpdatedAt: timestamppb.New(resp.UpdatedAt),
	}, nil
}

func entryToProto(e *UserLibrary) *userv1.LibraryEntry {
	return &userv1.LibraryEntry{
		BookId:    e.BookID,
		Status:    e.Status,
		AddedAt:   timestamppb.New(e.AddedAt),
		UpdatedAt: timestamppb.New(e.UpdatedAt),
	}
}

func invalid(err error) error {
	return apperrors.UnprocessableEntity(err.Error())
}
