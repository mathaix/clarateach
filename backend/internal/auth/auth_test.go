package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clarateach/backend/internal/store"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}

	if hash == password {
		t.Error("HashPassword() returned unhashed password")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "incorrect password",
			password: "wrongpassword",
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPassword(tt.password, tt.hash); got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	user := &store.User{
		ID:    "user-123",
		Email: "test@example.com",
		Role:  "instructor",
	}

	token, err := GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}
}

func TestValidateToken(t *testing.T) {
	user := &store.User{
		ID:    "user-123",
		Email: "test@example.com",
		Role:  "instructor",
	}

	token, err := GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("ValidateToken() UserID = %v, want %v", claims.UserID, user.ID)
	}

	if claims.Email != user.Email {
		t.Errorf("ValidateToken() Email = %v, want %v", claims.Email, user.Email)
	}

	if claims.Role != user.Role {
		t.Errorf("ValidateToken() Role = %v, want %v", claims.Role, user.Role)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "invalid token",
			token: "invalid.token.here",
		},
		{
			name:  "malformed token",
			token: "notavalidjwt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateToken(tt.token)
			if err == nil {
				t.Error("ValidateToken() expected error for invalid token")
			}
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantNil bool
	}{
		{
			name:    "no user in context",
			ctx:     context.Background(),
			wantNil: true,
		},
		{
			name: "user in context",
			ctx: context.WithValue(context.Background(), UserContextKey, &store.User{
				ID:    "user-123",
				Email: "test@example.com",
			}),
			wantNil: false,
		},
		{
			name:    "wrong type in context",
			ctx:     context.WithValue(context.Background(), UserContextKey, "not a user"),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUserFromContext(tt.ctx)
			if tt.wantNil && got != nil {
				t.Errorf("GetUserFromContext() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("GetUserFromContext() = nil, want user")
			}
		})
	}
}

func TestAdminMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		user           *store.User
		wantStatusCode int
	}{
		{
			name:           "no user",
			user:           nil,
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "non-admin user",
			user: &store.User{
				ID:   "user-123",
				Role: "instructor",
			},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name: "admin user",
			user: &store.User{
				ID:   "user-123",
				Role: "admin",
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/admin", nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), UserContextKey, tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("AdminMiddleware() status = %v, want %v", rr.Code, tt.wantStatusCode)
			}
		})
	}
}

// MockStore implements store.Store for testing
type MockStore struct {
	users map[string]*store.User
}

func NewMockStore() *MockStore {
	return &MockStore{
		users: make(map[string]*store.User),
	}
}

func (m *MockStore) CreateUser(u *store.User) error              { m.users[u.ID] = u; return nil }
func (m *MockStore) GetUser(id string) (*store.User, error)      { return m.users[id], nil }
func (m *MockStore) GetUserByEmail(email string) (*store.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}
func (m *MockStore) ListUsers() ([]*store.User, error) { return nil, nil }

// Workshop operations (minimal implementation for tests)
func (m *MockStore) CreateWorkshop(w *store.Workshop) error                     { return nil }
func (m *MockStore) GetWorkshop(id string) (*store.Workshop, error)             { return nil, nil }
func (m *MockStore) GetWorkshopByCode(code string) (*store.Workshop, error)     { return nil, nil }
func (m *MockStore) ListWorkshops() ([]*store.Workshop, error)                  { return nil, nil }
func (m *MockStore) ListWorkshopsByOwner(ownerID string) ([]*store.Workshop, error) { return nil, nil }
func (m *MockStore) UpdateWorkshopStatus(id string, status string) error        { return nil }
func (m *MockStore) DeleteWorkshop(id string) error                             { return nil }

// Session operations
func (m *MockStore) CreateSession(s *store.Session) error                       { return nil }
func (m *MockStore) UpdateSession(s *store.Session) error                       { return nil }
func (m *MockStore) GetSession(odehash string) (*store.Session, error)          { return nil, nil }
func (m *MockStore) ListSessions(workshopID string) ([]*store.Session, error)   { return nil, nil }
func (m *MockStore) GetSessionBySeat(workshopID string, seatID int) (*store.Session, error) { return nil, nil }

// VM operations
func (m *MockStore) CreateVM(vm *store.WorkshopVM) error                        { return nil }
func (m *MockStore) GetVM(workshopID string) (*store.WorkshopVM, error)         { return nil, nil }
func (m *MockStore) GetVMByID(id string) (*store.WorkshopVM, error)             { return nil, nil }
func (m *MockStore) UpdateVM(vm *store.WorkshopVM) error                        { return nil }
func (m *MockStore) MarkVMRemoved(workshopID string) error                      { return nil }
func (m *MockStore) ListVMs() ([]*store.WorkshopVM, error)                      { return nil, nil }
func (m *MockStore) ListAllVMs() ([]*store.WorkshopVM, error)                   { return nil, nil }
func (m *MockStore) GetVMPrivateKey(workshopID string) (string, error)          { return "", nil }

// Registration operations
func (m *MockStore) CreateRegistration(r *store.Registration) error             { return nil }
func (m *MockStore) GetRegistration(accessCode string) (*store.Registration, error) { return nil, nil }
func (m *MockStore) GetRegistrationByEmail(workshopID, email string) (*store.Registration, error) { return nil, nil }
func (m *MockStore) UpdateRegistration(r *store.Registration) error             { return nil }
func (m *MockStore) CountRegistrations(workshopID string) (int, error)          { return 0, nil }

func TestAuthMiddleware(t *testing.T) {
	mockStore := NewMockStore()
	user := &store.User{
		ID:        "user-123",
		Email:     "test@example.com",
		Role:      "instructor",
		CreatedAt: time.Now(),
	}
	mockStore.CreateUser(user)

	token, _ := GenerateToken(user)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid auth format",
			authHeader:     "InvalidFormat token",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalidtoken",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + token,
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := AuthMiddleware(mockStore)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("AuthMiddleware() status = %v, want %v", rr.Code, tt.wantStatusCode)
			}
		})
	}
}
