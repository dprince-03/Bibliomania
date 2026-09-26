package reading

import (
	"context"

	readingv1 "github.com/dprince-03/Bibliomania/gen/reading/v1"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements readingv1.ReadingServiceServer for the gateway.
type GRPCServer struct {
	readingv1.UnimplementedReadingServiceServer
	service *Service
}

func NewGRPCServer(service *Service) *GRPCServer {
	return &GRPCServer{service: service}
}

func (s *GRPCServer) GetHistory(ctx context.Context, req *readingv1.GetHistoryRequest) (*readingv1.GetHistoryResponse, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.service.GetHistory(ctx, userID, utils.NewPagination(int(req.GetPage()), int(req.GetLimit())))
	if err != nil {
		return nil, err
	}
	out := &readingv1.GetHistoryResponse{TotalCount: int32(total)}
	for _, h := range items {
		out.Sessions = append(out.Sessions, &readingv1.ReadingSession{
			BookId:      h.BookID,
			BookTitle:   h.BookTitle,
			CurrentPage: h.CurrentPage,
			TotalPages:  h.TotalPages,
			ProgressPct: h.ProgressPct,
			IsCompleted: h.IsCompleted,
			LastReadAt:  timestamppb.New(h.LastReadAt),
		})
	}
	return out, nil
}

func (s *GRPCServer) UpdateProgress(ctx context.Context, req *readingv1.UpdateProgressRequest) (*readingv1.ReadingSession, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.service.UpdateProgress(ctx, userID, req.GetBookId(), ProgressUpdateRequest{
		CurrentPage:    req.GetCurrentPage(),
		TotalPages:     req.GetTotalPages(),
		CurrentChapter: req.CurrentChapter,
	})
	if err != nil {
		return nil, err
	}
	title := ""
	if sess, err := s.service.store.GetSession(ctx, userID, req.GetBookId()); err == nil {
		title = sess.BookTitle
	}
	return &readingv1.ReadingSession{
		BookId:         resp.BookID,
		BookTitle:      title,
		CurrentPage:    resp.CurrentPage,
		TotalPages:     resp.TotalPages,
		ProgressPct:    resp.ProgressPct,
		CurrentChapter: resp.CurrentChapter,
		IsCompleted:    resp.IsCompleted,
		LastReadAt:     timestamppb.New(resp.LastReadAt),
	}, nil
}

func (s *GRPCServer) ListBookmarks(ctx context.Context, req *readingv1.ListBookmarksRequest) (*readingv1.ListBookmarksResponse, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.service.GetBookmarks(ctx, userID, req.GetBookId())
	if err != nil {
		return nil, err
	}
	out := &readingv1.ListBookmarksResponse{}
	for i := range items {
		out.Bookmarks = append(out.Bookmarks, bookmarkToProto(&items[i]))
	}
	return out, nil
}

func (s *GRPCServer) CreateBookmark(ctx context.Context, req *readingv1.CreateBookmarkRequest) (*readingv1.Bookmark, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.service.CreateBookmark(ctx, userID, req.GetBookId(), BookmarkRequest{
		Page:      req.GetPage(),
		Note:      req.Note,
		Highlight: req.Highlight,
		Color:     req.GetColor(),
	})
	if err != nil {
		return nil, err
	}
	return bookmarkToProto(resp), nil
}

func (s *GRPCServer) DeleteBookmark(ctx context.Context, req *readingv1.DeleteBookmarkRequest) (*readingv1.DeleteBookmarkResponse, error) {
	userID, err := grpcx.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.service.DeleteBookmark(ctx, userID, req.GetBookId(), req.GetBookmarkId()); err != nil {
		return nil, err
	}
	return &readingv1.DeleteBookmarkResponse{}, nil
}

func bookmarkToProto(b *BookmarkResponse) *readingv1.Bookmark {
	return &readingv1.Bookmark{
		Id:        b.ID,
		BookId:    b.BookID,
		Page:      b.Page,
		Note:      b.Note,
		Highlight: b.Highlight,
		Color:     b.Color,
	}
}
