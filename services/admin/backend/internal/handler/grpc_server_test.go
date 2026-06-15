package handler_test

import (
	"context"
	"testing"

	"charity-chest/services/admin/backend/internal/cache"
	"charity-chest/services/admin/backend/internal/grpc/adminpb"
	"charity-chest/services/admin/backend/internal/handler"
	"charity-chest/services/admin/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// newGRPCSrv wires a fresh gRPC server against an isolated per-test Postgres
// database, returning both so fixtures can be seeded directly.
func newGRPCSrv(t *testing.T) (adminpb.AdminInternalServer, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	return handler.NewGRPCServer(db, cache.Disabled(), testCfg()), db
}

// grpcBg returns a plain background context (no gRPC metadata).
// Use for method-level tests where the interceptor is not in the call path.
func grpcBg() context.Context { return context.Background() }

// ctxWithKey returns an incoming gRPC context with x-service-key set.
func ctxWithKey(key string) context.Context {
	md := metadata.New(map[string]string{"x-service-key": key})
	return metadata.NewIncomingContext(context.Background(), md)
}

// grpcHashPw hashes pw with bcrypt.MinCost for test speed.
func grpcHashPw(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

// mustGRPCCode asserts err is a gRPC status error with code want.
func mustGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected gRPC error with code %s, got nil", want)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != want {
		t.Errorf("code = %s (%q), want %s", st.Code(), st.Message(), want)
	}
}

// ── ServiceKeyInterceptor ─────────────────────────────────────────────────────

func TestServiceKeyInterceptor_ValidKey_PassesThrough(t *testing.T) {
	intercept := handler.ServiceKeyInterceptor("secret")
	var handlerCalled bool
	result, err := intercept(ctxWithKey("secret"), nil, nil,
		func(_ context.Context, _ any) (any, error) {
			handlerCalled = true
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %v, want ok", result)
	}
	if !handlerCalled {
		t.Error("downstream handler was not called")
	}
}

func TestServiceKeyInterceptor_NoMetadata_Unauthenticated(t *testing.T) {
	intercept := handler.ServiceKeyInterceptor("secret")
	// Plain background context carries no gRPC metadata at all.
	_, err := intercept(context.Background(), nil, nil,
		func(_ context.Context, _ any) (any, error) {
			t.Fatal("handler must not be called")
			return nil, nil
		})
	mustGRPCCode(t, err, codes.Unauthenticated)
}

func TestServiceKeyInterceptor_EmptyMetadata_Unauthenticated(t *testing.T) {
	intercept := handler.ServiceKeyInterceptor("secret")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))
	_, err := intercept(ctx, nil, nil,
		func(_ context.Context, _ any) (any, error) {
			t.Fatal("handler must not be called")
			return nil, nil
		})
	mustGRPCCode(t, err, codes.Unauthenticated)
}

func TestServiceKeyInterceptor_WrongKey_PermissionDenied(t *testing.T) {
	intercept := handler.ServiceKeyInterceptor("secret")
	_, err := intercept(ctxWithKey("wrong"), nil, nil,
		func(_ context.Context, _ any) (any, error) {
			t.Fatal("handler must not be called")
			return nil, nil
		})
	mustGRPCCode(t, err, codes.PermissionDenied)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestGRPCLogin_Success(t *testing.T) {
	srv, db := newGRPCSrv(t)
	hash := grpcHashPw(t, "correct-pass")
	u := model.User{Email: "alice@example.com", Name: "Alice", PasswordHash: &hash}
	db.Create(&u)

	dto, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "alice@example.com", Password: "correct-pass"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if dto.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", dto.Email)
	}
	if dto.Name != "Alice" {
		t.Errorf("name = %q, want Alice", dto.Name)
	}
	if dto.Uuid == "" {
		t.Error("uuid must not be empty")
	}
	// role absent — empty string in proto3
	if dto.Role != "" {
		t.Errorf("role = %q, want empty", dto.Role)
	}
	if dto.MfaEnabled {
		t.Error("mfa_enabled should be false for a fresh user")
	}
}

func TestGRPCLogin_WrongPassword_Unauthenticated(t *testing.T) {
	srv, db := newGRPCSrv(t)
	hash := grpcHashPw(t, "correct-pass")
	db.Create(&model.User{Email: "bob@example.com", Name: "Bob", PasswordHash: &hash})

	_, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "bob@example.com", Password: "wrong-pass"})
	mustGRPCCode(t, err, codes.Unauthenticated)
}

func TestGRPCLogin_UnknownEmail_Unauthenticated(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "ghost@example.com", Password: "pass"})
	// No user enumeration: same code as wrong password.
	mustGRPCCode(t, err, codes.Unauthenticated)
}

func TestGRPCLogin_GoogleOnlyAccount_Unauthenticated(t *testing.T) {
	srv, db := newGRPCSrv(t)
	googleID := "google-sub-123"
	// No PasswordHash — Google-only account.
	db.Create(&model.User{Email: "gonly@example.com", Name: "Google User", GoogleID: &googleID})

	_, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "gonly@example.com", Password: "anything"})
	// Same generic 401 — must not reveal account exists or login type.
	mustGRPCCode(t, err, codes.Unauthenticated)
}

func TestGRPCLogin_EmptyEmail_InvalidArgument(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "", Password: "pass"})
	mustGRPCCode(t, err, codes.InvalidArgument)
}

func TestGRPCLogin_EmptyPassword_InvalidArgument(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "a@b.com", Password: ""})
	mustGRPCCode(t, err, codes.InvalidArgument)
}

func TestGRPCLogin_ReturnsRoleWhenSet(t *testing.T) {
	srv, db := newGRPCSrv(t)
	hash := grpcHashPw(t, "pass")
	role := model.RoleSystem
	db.Create(&model.User{Email: "sys@example.com", Name: "Sys", PasswordHash: &hash, Role: &role})

	dto, err := srv.Login(grpcBg(), &adminpb.LoginRequest{Email: "sys@example.com", Password: "pass"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if dto.Role != string(model.RoleSystem) {
		t.Errorf("role = %q, want %q", dto.Role, model.RoleSystem)
	}
}

// ── GoogleAuth ───────────────────────────────────────────────────────────────

func TestGRPCGoogleAuth_CreatesNewUser(t *testing.T) {
	srv, db := newGRPCSrv(t)

	dto, err := srv.GoogleAuth(grpcBg(), &adminpb.GoogleAuthRequest{
		GoogleSub: "sub-new-001",
		Email:     "new@google.com",
		Name:      "New User",
	})
	if err != nil {
		t.Fatalf("GoogleAuth: %v", err)
	}
	if dto.Email != "new@google.com" {
		t.Errorf("email = %q, want new@google.com", dto.Email)
	}
	if dto.Name != "New User" {
		t.Errorf("name = %q, want New User", dto.Name)
	}

	// User must be persisted.
	var u model.User
	if err := db.Where("email = ?", "new@google.com").First(&u).Error; err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
}

func TestGRPCGoogleAuth_FindsExistingBySub(t *testing.T) {
	srv, db := newGRPCSrv(t)
	sub := "sub-existing-001"
	db.Create(&model.User{Email: "exist@google.com", Name: "Existing", GoogleID: &sub})

	dto, err := srv.GoogleAuth(grpcBg(), &adminpb.GoogleAuthRequest{
		GoogleSub: sub,
		Email:     "exist@google.com",
		Name:      "Existing",
	})
	if err != nil {
		t.Fatalf("GoogleAuth: %v", err)
	}
	if dto.Email != "exist@google.com" {
		t.Errorf("email = %q, want exist@google.com", dto.Email)
	}

	// Exactly one user in the DB.
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Errorf("user count = %d, want 1", count)
	}
}

func TestGRPCGoogleAuth_LinksGoogleIDToExistingEmail(t *testing.T) {
	srv, db := newGRPCSrv(t)
	hash := grpcHashPw(t, "pass")
	db.Create(&model.User{Email: "link@example.com", Name: "Link User", PasswordHash: &hash})

	// Same email — Google sub should be linked onto the existing account.
	dto, err := srv.GoogleAuth(grpcBg(), &adminpb.GoogleAuthRequest{
		GoogleSub: "sub-link-001",
		Email:     "link@example.com",
		Name:      "Link User",
	})
	if err != nil {
		t.Fatalf("GoogleAuth: %v", err)
	}

	// Still only one user.
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Errorf("user count = %d, want 1", count)
	}

	// Google ID is now set on the row.
	var u model.User
	db.Where("email = ?", "link@example.com").First(&u)
	if u.GoogleID == nil || *u.GoogleID != "sub-link-001" {
		t.Errorf("google_id = %v, want sub-link-001", u.GoogleID)
	}
	if dto.Uuid == "" {
		t.Error("returned uuid must not be empty")
	}
}

func TestGRPCGoogleAuth_EmptySub_InvalidArgument(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.GoogleAuth(grpcBg(), &adminpb.GoogleAuthRequest{GoogleSub: "", Email: "a@b.com", Name: "A"})
	mustGRPCCode(t, err, codes.InvalidArgument)
}

func TestGRPCGoogleAuth_EmptyEmail_InvalidArgument(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.GoogleAuth(grpcBg(), &adminpb.GoogleAuthRequest{GoogleSub: "sub-001", Email: "", Name: "A"})
	mustGRPCCode(t, err, codes.InvalidArgument)
}

// ── GetUser ───────────────────────────────────────────────────────────────────

func TestGRPCGetUser_Found(t *testing.T) {
	srv, db := newGRPCSrv(t)
	u := model.User{Email: "getme@example.com", Name: "Get Me"}
	db.Create(&u)

	dto, err := srv.GetUser(grpcBg(), &adminpb.GetUserRequest{UserUuid: u.UUID.String()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if dto.Uuid != u.UUID.String() {
		t.Errorf("uuid = %q, want %q", dto.Uuid, u.UUID.String())
	}
	if dto.Email != "getme@example.com" {
		t.Errorf("email = %q, want getme@example.com", dto.Email)
	}
	if dto.Name != "Get Me" {
		t.Errorf("name = %q, want Get Me", dto.Name)
	}
}

func TestGRPCGetUser_NotFound(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.GetUser(grpcBg(), &adminpb.GetUserRequest{UserUuid: "00000000-0000-0000-0000-000000000000"})
	mustGRPCCode(t, err, codes.NotFound)
}

func TestGRPCGetUser_InvalidUUID_InvalidArgument(t *testing.T) {
	srv, _ := newGRPCSrv(t)
	_, err := srv.GetUser(grpcBg(), &adminpb.GetUserRequest{UserUuid: "not-a-uuid"})
	mustGRPCCode(t, err, codes.InvalidArgument)
}

func TestGRPCGetUser_NilRole_EmptyInDTO(t *testing.T) {
	srv, db := newGRPCSrv(t)
	// No role set — nil in DB.
	u := model.User{Email: "norole@example.com", Name: "No Role"}
	db.Create(&u)

	dto, err := srv.GetUser(grpcBg(), &adminpb.GetUserRequest{UserUuid: u.UUID.String()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if dto.Role != "" {
		t.Errorf("role = %q, want empty string for nil role", dto.Role)
	}
}

func TestGRPCGetUser_SensitiveFieldsAbsent(t *testing.T) {
	srv, db := newGRPCSrv(t)
	hash := grpcHashPw(t, "secret-pass")
	u := model.User{Email: "secret@example.com", Name: "Secret", PasswordHash: &hash}
	db.Create(&u)

	dto, err := srv.GetUser(grpcBg(), &adminpb.GetUserRequest{UserUuid: u.UUID.String()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	// The proto DTO only carries uuid/email/name/role/mfa_enabled.
	// Verify all expected safe fields are present and non-empty where applicable.
	if dto.Uuid == "" {
		t.Error("uuid must not be empty")
	}
	if dto.Email != "secret@example.com" {
		t.Errorf("email = %q, want secret@example.com", dto.Email)
	}
	// MFA not enabled by default.
	if dto.MfaEnabled {
		t.Error("mfa_enabled should be false")
	}
}
