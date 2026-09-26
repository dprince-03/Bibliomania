package grpcx

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/resilience"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/pkg/jwt"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	mdAuthorization = "authorization"
	mdRequestID     = "x-request-id"
)

// NewServer builds a gRPC server with tracing, metrics, panic recovery,
// identity extraction and AppError→status translation. Registers the
// standard health service; reflection only outside production.
//
// Transport is plaintext: gRPC ports are only reachable on the internal
// Compose network (never published to the host). mTLS between services is
// a known follow-up — see Server/app/docs/plan.md.
func NewServer(jwtManager *jwt.Manager, isProduction bool) *grpc.Server {
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recoverInterceptor,
			telemetry.GRPCMetrics,
			identityInterceptor(jwtManager),
			errorInterceptor,
		),
	)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	if !isProduction {
		reflection.Register(srv)
	}
	return srv
}

func recoverInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "panic recovered in gRPC handler",
				"method", info.FullMethod, "error", r, "stack", string(debug.Stack()))
			err = status.Error(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}

// identityInterceptor verifies a forwarded bearer token (if any) and puts
// the caller into ctx under the same keys AuthGuard uses for HTTP, so a
// gRPC handler reads identity via middleware.GetUserID like any other.
// Calls without a token pass through anonymous — RPCs that need a user
// check with RequireUser.
func identityInterceptor(jwtManager *jwt.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)

		if ids := md.Get(mdRequestID); len(ids) > 0 {
			ctx = middleware.WithRequestID(ctx, ids[0])
		}

		if vals := md.Get(mdAuthorization); len(vals) > 0 {
			token := strings.TrimSpace(strings.TrimPrefix(vals[0], "Bearer "))
			claims, err := jwtManager.ParseAccessToken(token)
			if err != nil {
				return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
			}
			ctx = middleware.WithIdentity(ctx, claims.UserID, claims.Role, claims.Email, token)
		}
		return handler(ctx, req)
	}
}

func errorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		st := ToStatus(err)
		if status.Code(st) == codes.Internal {
			slog.ErrorContext(ctx, "gRPC handler failed", "method", info.FullMethod, "error", err)
		}
		return nil, st
	}
	return resp, nil
}

// RequireUser returns the authenticated caller's ID or UNAUTHENTICATED.
func RequireUser(ctx context.Context) (uint64, error) {
	id := middleware.GetUserID(ctx)
	if id == 0 {
		return 0, status.Error(codes.Unauthenticated, "authentication required")
	}
	return id, nil
}

// retryPolicy retries only UNAVAILABLE — the server was never reached, so a
// retry can't double-apply a write. Every mutating RPC in this system is
// also idempotent by key (ReserveCopy/ReleaseCopy, BorrowBook), so this is
// belt and braces, not the safety mechanism.
//
// round_robin spreads calls over every address the target resolves to: on
// Kubernetes the *-grpc headless Services resolve to one address per pod
// (see infra/k8s), which pick_first (the default) would pin to a single
// pod for the connection's lifetime. With one address (Compose) it's a
// no-op.
const retryPolicy = `{
  "loadBalancingConfig": [{"round_robin": {}}],
  "methodConfig": [{
    "name": [{}],
    "timeout": "5s",
    "retryPolicy": {
      "maxAttempts": 3,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`

// Dial opens a client connection to another service. Connections are lazy:
// the target doesn't need to be up yet, so services can start in any order.
// Every call goes through a circuit breaker for this target (see
// internal/resilience) — it wraps the retries, so one "failure" is a call
// that failed even after retrying.
func Dial(addr string) (*grpc.ClientConn, error) {
	breaker := resilience.NewBreaker("grpc:"+addr, isInfraFailure)
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultServiceConfig(retryPolicy),
		grpc.WithChainUnaryInterceptor(breakerInterceptor(breaker), forwardInterceptor),
	)
}

// isInfraFailure: only "the dependency is unwell" codes trip the breaker —
// NotFound, InvalidArgument, FailedPrecondition etc. are healthy answers.
func isInfraFailure(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.Unknown, codes.ResourceExhausted:
		return true
	}
	return false
}

func breakerInterceptor(b *resilience.Breaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := b.Do(func() error { return invoker(ctx, method, req, reply, cc, opts...) })
		if errors.Is(err, resilience.ErrOpen) {
			// Same code a down service gives, so callers handle both alike
			// (e.g. borrow's Saga treats it as "catalog unavailable").
			return status.Error(codes.Unavailable, err.Error())
		}
		return err
	}
}

// forwardInterceptor copies the caller's bearer token and request ID from
// ctx into outgoing metadata, so the next service sees the same identity
// and the same request ID for log correlation.
func forwardInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var pairs []string
	if token := middleware.GetToken(ctx); token != "" {
		pairs = append(pairs, mdAuthorization, "Bearer "+token)
	}
	if id := middleware.GetRequestID(ctx); id != "" {
		pairs = append(pairs, mdRequestID, id)
	}
	if len(pairs) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
