package handler

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"

	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/config"
	"charity-chest/services/admin/backend/internal/grpc/adminpb"
	"charity-chest/services/admin/backend/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type grpcServer struct {
	adminpb.UnimplementedAdminInternalServer
	db    *gorm.DB
	cache *cache.Cache
	cfg   *config.Config
}

// NewGRPCServer returns an AdminInternalServer that delegates to the same DB
// and cache used by the HTTP handlers.
func NewGRPCServer(db *gorm.DB, c *cache.Cache, cfg *config.Config) adminpb.AdminInternalServer {
	return &grpcServer{db: db, cache: c, cfg: cfg}
}

// ServiceKeyInterceptor is a gRPC unary server interceptor that validates the
// "x-service-key" metadata entry in constant time.
// Missing entry → UNAUTHENTICATED; mismatch → PERMISSION_DENIED.
func ServiceKeyInterceptor(expected string) grpc.UnaryServerInterceptor {
	expectedBytes := []byte(expected)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("x-service-key")
		if len(vals) == 0 || vals[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing service key")
		}
		if subtle.ConstantTimeCompare([]byte(vals[0]), expectedBytes) != 1 {
			return nil, status.Error(codes.PermissionDenied, "invalid service key")
		}
		return handler(ctx, req)
	}
}

// Login validates email + password and returns the slim user DTO.
// Every failure mode (unknown user, no password set, wrong password) returns
// UNAUTHENTICATED to avoid user enumeration — mirrors the public login.
func (s *grpcServer) Login(ctx context.Context, req *adminpb.LoginRequest) (*adminpb.UserDTO, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	var user model.User
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		log.Printf("grpc: login: db error: %v", err)
		return nil, status.Error(codes.Internal, "database error")
	}
	if user.PasswordHash == nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return toGRPCUserDTO(&user), nil
}

// GoogleAuth finds or creates a user from pre-verified Google ID token claims.
// Reuses findOrCreateGoogleUser so the browser OAuth callback and this
// service-to-service path stay in lock-step.
func (s *grpcServer) GoogleAuth(ctx context.Context, req *adminpb.GoogleAuthRequest) (*adminpb.UserDTO, error) {
	if req.GoogleSub == "" || req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "google_sub and email are required")
	}

	gUser := &googleUserInfo{ID: req.GoogleSub, Email: req.Email, Name: req.Name}
	user, err := findOrCreateGoogleUser(s.db, s.cache, gUser)
	if err != nil {
		log.Printf("grpc: google auth: %v", err)
		return nil, status.Error(codes.Internal, "failed to resolve user")
	}

	return toGRPCUserDTO(user), nil
}

// GetUser returns the slim user DTO for the given public UUID.
func (s *grpcServer) GetUser(ctx context.Context, req *adminpb.GetUserRequest) (*adminpb.UserDTO, error) {
	parsed, err := uuid.Parse(req.UserUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user UUID")
	}

	var user model.User
	if err := s.db.WithContext(ctx).Where("uuid = ?", parsed).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Printf("grpc: get user: db error: %v", err)
		return nil, status.Error(codes.Internal, "database error")
	}

	return toGRPCUserDTO(&user), nil
}

func toGRPCUserDTO(u *model.User) *adminpb.UserDTO {
	dto := &adminpb.UserDTO{
		Uuid:       u.UUID.String(),
		Email:      u.Email,
		Name:       u.Name,
		MfaEnabled: u.MFAEnabled,
	}
	if u.Role != nil {
		dto.Role = string(*u.Role)
	}
	return dto
}
