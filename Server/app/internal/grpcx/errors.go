// Package grpcx is the shared gRPC plumbing for internal service-to-service
// calls: server/client construction, identity forwarding, and translating
// between internal/errors.AppError and gRPC status codes in both directions,
// so an error keeps its meaning (404 stays "not found") across a hop.
package grpcx

import (
	"errors"
	"net/http"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNoCopies is the sentinel catalog-service returns from ReserveCopy when
// the book exists but nothing is available. It maps to FAILED_PRECONDITION
// so borrow-service can tell it apart from NOT_FOUND without string-matching.
// HTTP 409, not 400: the request is valid, the book's state (none left) is
// what conflicts — clients should retry later, not fix their request.
var ErrNoCopies = &apperrors.AppError{Code: http.StatusConflict, Message: "no available copies for this book"}

// ToStatus converts a service-layer error into a gRPC status error.
func ToStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok && !isAppError(err) {
		return err
	}
	if errors.Is(err, ErrNoCopies) {
		return status.Error(codes.FailedPrecondition, ErrNoCopies.Message)
	}

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, "internal error")
	}

	switch appErr.Code {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case http.StatusNotFound:
		return status.Error(codes.NotFound, appErr.Message)
	case http.StatusConflict:
		return status.Error(codes.AlreadyExists, appErr.Message)
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, appErr.Message)
	case http.StatusServiceUnavailable:
		return status.Error(codes.Unavailable, appErr.Message)
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func isAppError(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr)
}

// FromStatus converts an error returned by a gRPC client call back into an
// *AppError, so a handler can pass it straight to utils.HandleError and the
// original HTTP meaning survives the hop.
func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperrors.Internal(err)
	}

	msg := st.Message()
	switch st.Code() {
	case codes.InvalidArgument:
		return apperrors.BadRequest(msg, nil)
	case codes.FailedPrecondition:
		return &apperrors.AppError{Code: http.StatusConflict, Message: msg, Err: ErrNoCopies}
	case codes.Unauthenticated:
		return apperrors.Unauthorized(msg)
	case codes.PermissionDenied:
		return apperrors.Forbidden(msg)
	case codes.NotFound:
		return &apperrors.AppError{Code: http.StatusNotFound, Message: msg}
	case codes.AlreadyExists:
		return apperrors.Conflict(msg)
	case codes.ResourceExhausted:
		return apperrors.TooManyRequests(msg)
	case codes.Unavailable, codes.DeadlineExceeded:
		return &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "a dependent service is unavailable", Err: err}
	default:
		return apperrors.Internal(err)
	}
}

// IsCode reports whether err (from a client call) carries the given code.
func IsCode(err error, code codes.Code) bool {
	return status.Code(err) == code
}
