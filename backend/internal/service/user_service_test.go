package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/hash"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock UserRepository ---

type mockUserRepo struct {
	users map[string]*model.User // keyed by email
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*model.User)}
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) error {
	if _, exists := m.users[user.Email]; exists {
		return apierror.NewConflict("email already registered")
	}
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	if user, ok := m.users[email]; ok {
		return user, nil
	}
	return nil, apierror.NewNotFound("user not found")
}

func (m *mockUserRepo) FindByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, apierror.NewNotFound("user not found")
}

func (m *mockUserRepo) Update(_ context.Context, user *model.User) error {
	if _, ok := m.users[user.Email]; !ok {
		return apierror.NewNotFound("user not found")
	}
	user.UpdatedAt = time.Now()
	m.users[user.Email] = user
	return nil
}

// --- Mock TransactionManager ---

type mockTxManager struct{}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Helper ---

func newTestUserService() (UserService, *mockUserRepo) {
	repo := newMockUserRepo()
	txm := &mockTxManager{}
	svc := NewUserService(repo, txm, "test-jwt-secret-key-for-testing", 24)
	return svc, repo
}

// --- Tests ---

func TestRegister_Success(t *testing.T) {
	svc, repo := newTestUserService()

	req := &dto.RegisterRequest{
		Email:    "dev@example.com",
		Password: "securepassword",
		FullName: "Dev User",
	}

	resp, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Email != req.Email {
		t.Errorf("expected email %s, got %s", req.Email, resp.Email)
	}
	if resp.FullName != req.FullName {
		t.Errorf("expected full_name %s, got %s", req.FullName, resp.FullName)
	}
	if resp.Role != "developer" {
		t.Errorf("expected role developer, got %s", resp.Role)
	}
	if !resp.IsActive {
		t.Error("expected is_active true")
	}

	// Verify password was hashed (not stored as plaintext).
	stored := repo.users[req.Email]
	if stored.PasswordHash == req.Password {
		t.Error("password should be hashed, not stored as plaintext")
	}
	if err := hash.CheckPassword(req.Password, stored.PasswordHash); err != nil {
		t.Error("stored hash should match the original password")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := newTestUserService()

	req := &dto.RegisterRequest{
		Email:    "dup@example.com",
		Password: "securepassword",
		FullName: "First User",
	}

	_, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("first register should succeed, got %v", err)
	}

	_, err = svc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT code, got %s", apiErr.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc, _ := newTestUserService()

	req := &dto.RegisterRequest{
		Email:    "short@example.com",
		Password: "1234567", // 7 chars, less than 8
		FullName: "Short Pass",
	}

	_, err := svc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newTestUserService()

	// Register first.
	regReq := &dto.RegisterRequest{
		Email:    "login@example.com",
		Password: "securepassword",
		FullName: "Login User",
	}
	_, err := svc.Register(context.Background(), regReq)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Login.
	loginReq := &dto.LoginRequest{
		Email:    "login@example.com",
		Password: "securepassword",
	}
	resp, err := svc.Login(context.Background(), loginReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.ExpiresIn != 24*3600 {
		t.Errorf("expected expires_in %d, got %d", 24*3600, resp.ExpiresIn)
	}
}

func TestLogin_InvalidEmail(t *testing.T) {
	svc, _ := newTestUserService()

	loginReq := &dto.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "anypassword1",
	}
	_, err := svc.Login(context.Background(), loginReq)
	if err == nil {
		t.Fatal("expected error for invalid email, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %s", apiErr.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newTestUserService()

	// Register first.
	regReq := &dto.RegisterRequest{
		Email:    "wrongpw@example.com",
		Password: "correctpassword",
		FullName: "Wrong PW User",
	}
	_, err := svc.Register(context.Background(), regReq)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Login with wrong password.
	loginReq := &dto.LoginRequest{
		Email:    "wrongpw@example.com",
		Password: "wrongpassword1",
	}
	_, err = svc.Login(context.Background(), loginReq)
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %s", apiErr.Code)
	}
}

func TestGetProfile_Success(t *testing.T) {
	svc, _ := newTestUserService()

	// Register first.
	regReq := &dto.RegisterRequest{
		Email:    "profile@example.com",
		Password: "securepassword",
		FullName: "Profile User",
	}
	regResp, err := svc.Register(context.Background(), regReq)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	userID, _ := uuid.Parse(regResp.ID)
	resp, err := svc.GetProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Email != regReq.Email {
		t.Errorf("expected email %s, got %s", regReq.Email, resp.Email)
	}
	if resp.FullName != regReq.FullName {
		t.Errorf("expected full_name %s, got %s", regReq.FullName, resp.FullName)
	}
	if resp.Role != "developer" {
		t.Errorf("expected role developer, got %s", resp.Role)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	svc, _ := newTestUserService()

	_, err := svc.GetProfile(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}
