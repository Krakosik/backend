package procedure

import (
	"context"
	"strings"

	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor(authService service.AuthService) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		if token == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		user, err := authService.ValidateToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		newCtx := context.WithValue(ctx, dto.UserContextKey, user)
		return handler(newCtx, req)
	}
}

func StreamAuthInterceptor(authService service.AuthService) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(stream.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		if token == authHeader[0] {
			return status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		user, err := authService.ValidateToken(stream.Context(), token)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid token")
		}

		newCtx := context.WithValue(stream.Context(), dto.UserContextKey, user)
		wrappedStream := &wrappedServerStream{
			ServerStream: stream,
			ctx:          newCtx,
		}
		return handler(srv, wrappedStream)
	}
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func GetUserFromContext(ctx context.Context) (service.User, bool) {
	user, ok := ctx.Value(dto.UserContextKey).(service.User)
	return user, ok
}
