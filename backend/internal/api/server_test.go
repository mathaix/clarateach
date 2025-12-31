package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/clarateach/backend/internal/auth"
	"github.com/clarateach/backend/internal/provisioner"
	"github.com/clarateach/backend/internal/store"
)

// MockProvisioner implements provisioner.Provisioner for testing
type MockProvisioner struct {
	CreateVMError  error
	DeleteVMError  error
	GetVMError     error
	CreatedVMs     map[string]*provisioner.VMInstance
	DeletedVMs     []string
}

func NewMockProvisioner() *MockProvisioner {
	return &MockProvisioner{
		CreatedVMs: make(map[string]*provisioner.VMInstance),
		DeletedVMs: []string{},
	}
}

func (m *MockProvisioner) CreateVM(ctx context.Context, cfg provisioner.VMConfig) (*provisioner.VMInstance, error) {
	if m.CreateVMError != nil {
		return nil, m.CreateVMError
	}
	vm := &provisioner.VMInstance{
		ID:         "test-vm-id-" + cfg.WorkshopID,
		Name:       "clarateach-" + cfg.WorkshopID,
		ExternalIP: "1.2.3.4",
		InternalIP: "10.0.0.1",
		Status:     "RUNNING",
		Zone:       "us-central1-a",
	}
	m.CreatedVMs[cfg.WorkshopID] = vm
	return vm, nil
}

func (m *MockProvisioner) DeleteVM(ctx context.Context, workshopID string) error {
	if m.DeleteVMError != nil {
		return m.DeleteVMError
	}
	m.DeletedVMs = append(m.DeletedVMs, workshopID)
	delete(m.CreatedVMs, workshopID)
	return nil
}

func (m *MockProvisioner) GetVM(ctx context.Context, workshopID string) (*provisioner.VMInstance, error) {
	if m.GetVMError != nil {
		return nil, m.GetVMError
	}
	if vm, ok := m.CreatedVMs[workshopID]; ok {
		return vm, nil
	}
	return nil, nil
}

func (m *MockProvisioner) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
	return nil
}

func (m *MockProvisioner) ListVMs(ctx context.Context, workshopID string) ([]*provisioner.VMInstance, error) {
	var vms []*provisioner.VMInstance
	for _, vm := range m.CreatedVMs {
		vms = append(vms, vm)
	}
	return vms, nil
}

func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// Create a temporary database file
	tmpFile, err := os.CreateTemp("", "clarateach_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := store.InitDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to initialize database: %v", err)
	}

	s := store.NewSQLiteStore(db)
	mockProv := NewMockProvisioner()

	// Create server with auth disabled for most tests
	server := NewServer(s, mockProv, false, true) // authDisabled = true

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return server, cleanup
}

// setupTestServerWithMock returns the server, store, mock provisioner, and cleanup func
func setupTestServerWithMock(t *testing.T, authDisabled bool) (*Server, *store.SQLiteStore, *MockProvisioner, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "clarateach_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := store.InitDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to initialize database: %v", err)
	}

	s := store.NewSQLiteStore(db)
	mockProv := NewMockProvisioner()

	server := NewServer(s, mockProv, false, authDisabled)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return server, s, mockProv, cleanup
}

func setupTestServerWithAuth(t *testing.T) (*Server, *store.SQLiteStore, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "clarateach_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := store.InitDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to initialize database: %v", err)
	}

	s := store.NewSQLiteStore(db)
	mockProv := NewMockProvisioner()

	// Create server with auth enabled
	server := NewServer(s, mockProv, false, false) // authDisabled = false

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return server, s, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Health endpoint returned %d, want %d", rr.Code, http.StatusOK)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Health status = %v, want ok", response["status"])
	}
}

func TestAuthRegister(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	body := map[string]string{
		"email":    "newuser@example.com",
		"password": "password123",
		"name":     "New User",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Register returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["token"] == nil || response["token"] == "" {
		t.Error("Register did not return a token")
	}

	if response["user"] == nil {
		t.Error("Register did not return a user")
	}
}

func TestAuthRegisterDuplicate(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	body := map[string]string{
		"email":    "duplicate@example.com",
		"password": "password123",
		"name":     "First User",
	}
	bodyBytes, _ := json.Marshal(body)

	// First registration
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("First register failed: %d", rr.Code)
	}

	// Second registration with same email
	bodyBytes, _ = json.Marshal(body)
	req = httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Duplicate register returned %d, want %d", rr.Code, http.StatusConflict)
	}
}

func TestAuthLogin(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// First register a user
	regBody := map[string]string{
		"email":    "login@example.com",
		"password": "password123",
		"name":     "Login User",
	}
	regBytes, _ := json.Marshal(regBody)

	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	// Now login
	loginBody := map[string]string{
		"email":    "login@example.com",
		"password": "password123",
	}
	loginBytes, _ := json.Marshal(loginBody)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Login returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["token"] == nil || response["token"] == "" {
		t.Error("Login did not return a token")
	}
}

func TestAuthLoginInvalidCredentials(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Register a user
	regBody := map[string]string{
		"email":    "user@example.com",
		"password": "password123",
		"name":     "Test User",
	}
	regBytes, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	// Login with wrong password
	loginBody := map[string]string{
		"email":    "user@example.com",
		"password": "wrongpassword",
	}
	loginBytes, _ := json.Marshal(loginBody)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Login with wrong password returned %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMe(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Register and get token
	regBody := map[string]string{
		"email":    "me@example.com",
		"password": "password123",
		"name":     "Me User",
	}
	regBytes, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	var regResponse map[string]interface{}
	json.Unmarshal(regRr.Body.Bytes(), &regResponse)
	token := regResponse["token"].(string)

	// Call /auth/me
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Auth/me returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	user := response["user"].(map[string]interface{})
	if user["email"] != "me@example.com" {
		t.Errorf("Auth/me email = %v, want me@example.com", user["email"])
	}
}

func TestWorkshopCRUD(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create workshop
	createBody := map[string]interface{}{
		"name":    "Test Workshop",
		"seats":   5,
		"api_key": "sk-test-key",
	}
	createBytes, _ := json.Marshal(createBody)

	req := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Create workshop returned %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var createResponse map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopID := workshop["id"].(string)
	workshopCode := workshop["code"].(string)

	if workshop["name"] != "Test Workshop" {
		t.Errorf("Workshop name = %v, want Test Workshop", workshop["name"])
	}

	// Workshop creation is now async, so it returns "provisioning" status immediately
	if workshop["status"] != "provisioning" {
		t.Errorf("Workshop status = %v, want provisioning", workshop["status"])
	}

	// Wait for async provisioning to complete
	time.Sleep(100 * time.Millisecond)

	// List workshops
	listReq := httptest.NewRequest("GET", "/api/workshops", nil)
	listRr := httptest.NewRecorder()
	server.router.ServeHTTP(listRr, listReq)

	if listRr.Code != http.StatusOK {
		t.Errorf("List workshops returned %d, want %d", listRr.Code, http.StatusOK)
	}

	var listResponse map[string]interface{}
	json.Unmarshal(listRr.Body.Bytes(), &listResponse)
	workshops := listResponse["workshops"].([]interface{})

	if len(workshops) != 1 {
		t.Errorf("List workshops returned %d workshops, want 1", len(workshops))
	}

	// Get workshop by ID
	getReq := httptest.NewRequest("GET", "/api/workshops/"+workshopID, nil)
	getRr := httptest.NewRecorder()
	server.router.ServeHTTP(getRr, getReq)

	if getRr.Code != http.StatusOK {
		t.Errorf("Get workshop returned %d, want %d", getRr.Code, http.StatusOK)
	}

	// Test that code exists
	if workshopCode == "" {
		t.Error("Workshop code is empty")
	}

	// Delete workshop (async - sets status to "deleting" immediately)
	delReq := httptest.NewRequest("DELETE", "/api/workshops/"+workshopID, nil)
	delRr := httptest.NewRecorder()
	server.router.ServeHTTP(delRr, delReq)

	if delRr.Code != http.StatusOK {
		t.Errorf("Delete workshop returned %d, want %d", delRr.Code, http.StatusOK)
	}

	// Wait for async deletion to complete
	time.Sleep(100 * time.Millisecond)

	// Verify deleted (soft delete - workshop still exists but with "deleted" status)
	listReq2 := httptest.NewRequest("GET", "/api/workshops", nil)
	listRr2 := httptest.NewRecorder()
	server.router.ServeHTTP(listRr2, listReq2)

	var listResponse2 map[string]interface{}
	json.Unmarshal(listRr2.Body.Bytes(), &listResponse2)
	workshops2 := listResponse2["workshops"].([]interface{})

	// Workshop should still be in list (soft delete) but with "deleted" status
	if len(workshops2) != 1 {
		t.Errorf("Expected 1 workshop (soft deleted), got %d", len(workshops2))
	}
	if len(workshops2) > 0 {
		ws := workshops2[0].(map[string]interface{})
		if ws["status"] != "deleted" {
			t.Errorf("Workshop status = %v, want deleted", ws["status"])
		}
	}
}

func TestRegistrationFlow(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create workshop first
	createBody := map[string]interface{}{
		"name":    "Registration Test Workshop",
		"seats":   5,
		"api_key": "sk-test-key",
	}
	createBytes, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createRr := httptest.NewRecorder()
	server.router.ServeHTTP(createRr, createReq)

	var createResponse map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopCode := workshop["code"].(string)

	// Wait for async provisioning to complete
	time.Sleep(100 * time.Millisecond)

	// Register for workshop
	regBody := map[string]interface{}{
		"workshop_code": workshopCode,
		"email":         "learner@example.com",
		"name":          "Test Learner",
	}
	regBytes, _ := json.Marshal(regBody)

	regReq := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	if regRr.Code != http.StatusOK {
		t.Fatalf("Register returned %d, want %d. Body: %s", regRr.Code, http.StatusOK, regRr.Body.String())
	}

	var regResponse map[string]interface{}
	json.Unmarshal(regRr.Body.Bytes(), &regResponse)

	accessCode := regResponse["access_code"].(string)
	if accessCode == "" {
		t.Error("Registration did not return access code")
	}

	// Check that registering again returns already_registered
	regReq2 := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regBytes))
	regReq2.Header.Set("Content-Type", "application/json")
	regRr2 := httptest.NewRecorder()
	server.router.ServeHTTP(regRr2, regReq2)

	var regResponse2 map[string]interface{}
	json.Unmarshal(regRr2.Body.Bytes(), &regResponse2)

	if regResponse2["already_registered"] != true {
		t.Error("Second registration should return already_registered: true")
	}

	// Get session by access code
	sessionReq := httptest.NewRequest("GET", "/api/session/"+accessCode, nil)
	sessionRr := httptest.NewRecorder()
	server.router.ServeHTTP(sessionRr, sessionReq)

	if sessionRr.Code != http.StatusOK {
		t.Errorf("Get session returned %d, want %d. Body: %s", sessionRr.Code, http.StatusOK, sessionRr.Body.String())
	}
}

func TestProtectedEndpointsRequireAuth(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/workshops"},
		{"POST", "/api/workshops"},
		{"GET", "/api/workshops/test-id"},
		{"DELETE", "/api/workshops/test-id"},
		{"GET", "/api/auth/me"},
	}

	for _, endpoint := range endpoints {
		req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
		rr := httptest.NewRecorder()
		server.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s returned %d without auth, want %d", endpoint.method, endpoint.path, rr.Code, http.StatusUnauthorized)
		}
	}
}

func TestAdminEndpointsRequireAdmin(t *testing.T) {
	server, s, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Create a regular instructor user
	hash, _ := auth.HashPassword("password123")
	user := &store.User{
		ID:           "instructor-user",
		Email:        "instructor@example.com",
		PasswordHash: hash,
		Name:         "Instructor",
		Role:         "instructor",
	}
	s.CreateUser(user)

	token, _ := auth.GenerateToken(user)

	adminEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/admin/overview"},
		{"GET", "/api/admin/vms"},
	}

	for _, endpoint := range adminEndpoints {
		req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		server.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s returned %d for non-admin, want %d", endpoint.method, endpoint.path, rr.Code, http.StatusForbidden)
		}
	}
}

// ================== End-to-End Auth Flow Tests ==================
// These tests verify the complete user journey with authentication

func TestAuthenticatedWorkshopCreation(t *testing.T) {
	server, s, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Step 1: Register a new instructor
	regBody := map[string]string{
		"email":    "instructor@example.com",
		"password": "password123",
		"name":     "Test Instructor",
	}
	regBytes, _ := json.Marshal(regBody)

	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	if regRr.Code != http.StatusOK {
		t.Fatalf("Registration failed: %d - %s", regRr.Code, regRr.Body.String())
	}

	var regResponse map[string]interface{}
	json.Unmarshal(regRr.Body.Bytes(), &regResponse)
	token := regResponse["token"].(string)
	user := regResponse["user"].(map[string]interface{})
	userID := user["id"].(string)

	// Step 2: Create a workshop with the auth token
	createBody := map[string]interface{}{
		"name":    "Auth Test Workshop",
		"seats":   3,
		"api_key": "sk-test",
	}
	createBytes, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRr := httptest.NewRecorder()
	server.router.ServeHTTP(createRr, createReq)

	if createRr.Code != http.StatusOK {
		t.Fatalf("Workshop creation failed: %d - %s", createRr.Code, createRr.Body.String())
	}

	var createResponse map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopID := workshop["id"].(string)

	// Workshop creation is async - returns "provisioning" status immediately
	if workshop["status"] != "provisioning" {
		t.Errorf("Workshop status = %v, want provisioning", workshop["status"])
	}

	// Step 3: Verify owner_id was set correctly in the database
	dbWorkshop, err := s.GetWorkshop(workshopID)
	if err != nil {
		t.Fatalf("Failed to get workshop from DB: %v", err)
	}
	if dbWorkshop == nil {
		t.Fatal("Workshop not found in database")
	}
	if dbWorkshop.OwnerID != userID {
		t.Errorf("Workshop owner_id = %q, want %q", dbWorkshop.OwnerID, userID)
	}

	// Step 4: Verify list workshops only returns workshops owned by this user
	listReq := httptest.NewRequest("GET", "/api/workshops", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRr := httptest.NewRecorder()
	server.router.ServeHTTP(listRr, listReq)

	if listRr.Code != http.StatusOK {
		t.Fatalf("List workshops failed: %d", listRr.Code)
	}

	var listResponse map[string]interface{}
	json.Unmarshal(listRr.Body.Bytes(), &listResponse)
	workshops := listResponse["workshops"].([]interface{})

	if len(workshops) != 1 {
		t.Errorf("Expected 1 workshop, got %d", len(workshops))
	}
}

func TestWorkshopOwnershipIsolation(t *testing.T) {
	server, s, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Create two instructors
	hash, _ := auth.HashPassword("password123")
	instructor1 := &store.User{
		ID:           "instructor-1",
		Email:        "instructor1@example.com",
		PasswordHash: hash,
		Name:         "Instructor 1",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}
	instructor2 := &store.User{
		ID:           "instructor-2",
		Email:        "instructor2@example.com",
		PasswordHash: hash,
		Name:         "Instructor 2",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}
	s.CreateUser(instructor1)
	s.CreateUser(instructor2)

	token1, _ := auth.GenerateToken(instructor1)
	token2, _ := auth.GenerateToken(instructor2)

	// Instructor 1 creates a workshop
	createBody := map[string]interface{}{
		"name":    "Instructor 1 Workshop",
		"seats":   5,
		"api_key": "sk-test",
	}
	createBytes, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token1)
	createRr := httptest.NewRecorder()
	server.router.ServeHTTP(createRr, createReq)

	if createRr.Code != http.StatusOK {
		t.Fatalf("Workshop creation failed: %d - %s", createRr.Code, createRr.Body.String())
	}

	// Instructor 2 lists workshops - should see 0
	listReq := httptest.NewRequest("GET", "/api/workshops", nil)
	listReq.Header.Set("Authorization", "Bearer "+token2)
	listRr := httptest.NewRecorder()
	server.router.ServeHTTP(listRr, listReq)

	var listResponse map[string]interface{}
	json.Unmarshal(listRr.Body.Bytes(), &listResponse)
	workshops := listResponse["workshops"]

	// Should be empty or nil for instructor 2
	if workshops != nil {
		workshopList := workshops.([]interface{})
		if len(workshopList) != 0 {
			t.Errorf("Instructor 2 should see 0 workshops, got %d", len(workshopList))
		}
	}

	// Instructor 1 lists workshops - should see 1
	listReq1 := httptest.NewRequest("GET", "/api/workshops", nil)
	listReq1.Header.Set("Authorization", "Bearer "+token1)
	listRr1 := httptest.NewRecorder()
	server.router.ServeHTTP(listRr1, listReq1)

	var listResponse1 map[string]interface{}
	json.Unmarshal(listRr1.Body.Bytes(), &listResponse1)
	workshops1 := listResponse1["workshops"].([]interface{})

	if len(workshops1) != 1 {
		t.Errorf("Instructor 1 should see 1 workshop, got %d", len(workshops1))
	}
}

func TestAdminSeesAllWorkshops(t *testing.T) {
	server, s, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	hash, _ := auth.HashPassword("password123")

	// Create an admin
	admin := &store.User{
		ID:           "admin-user",
		Email:        "admin@example.com",
		PasswordHash: hash,
		Name:         "Admin",
		Role:         "admin",
		CreatedAt:    time.Now(),
	}
	s.CreateUser(admin)
	adminToken, _ := auth.GenerateToken(admin)

	// Create two instructors with workshops
	instructor1 := &store.User{
		ID:           "instructor-1",
		Email:        "instructor1@example.com",
		PasswordHash: hash,
		Name:         "Instructor 1",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}
	s.CreateUser(instructor1)
	token1, _ := auth.GenerateToken(instructor1)

	instructor2 := &store.User{
		ID:           "instructor-2",
		Email:        "instructor2@example.com",
		PasswordHash: hash,
		Name:         "Instructor 2",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}
	s.CreateUser(instructor2)
	token2, _ := auth.GenerateToken(instructor2)

	// Each instructor creates a workshop
	for i, token := range []string{token1, token2} {
		createBody := map[string]interface{}{
			"name":    "Workshop " + string(rune('A'+i)),
			"seats":   3,
			"api_key": "sk-test",
		}
		createBytes, _ := json.Marshal(createBody)

		createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Authorization", "Bearer "+token)
		createRr := httptest.NewRecorder()
		server.router.ServeHTTP(createRr, createReq)

		if createRr.Code != http.StatusOK {
			t.Fatalf("Workshop creation failed: %d", createRr.Code)
		}
	}

	// Admin lists workshops - should see all 2
	listReq := httptest.NewRequest("GET", "/api/workshops", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listRr := httptest.NewRecorder()
	server.router.ServeHTTP(listRr, listReq)

	var listResponse map[string]interface{}
	json.Unmarshal(listRr.Body.Bytes(), &listResponse)
	workshops := listResponse["workshops"].([]interface{})

	if len(workshops) != 2 {
		t.Errorf("Admin should see 2 workshops, got %d", len(workshops))
	}
}

func TestFullLearnerJourneyWithAuth(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Step 1: Instructor registers
	regBody := map[string]string{
		"email":    "instructor@example.com",
		"password": "password123",
		"name":     "Test Instructor",
	}
	regBytes, _ := json.Marshal(regBody)

	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRr := httptest.NewRecorder()
	server.router.ServeHTTP(regRr, regReq)

	if regRr.Code != http.StatusOK {
		t.Fatalf("Registration failed: %d", regRr.Code)
	}

	var regResponse map[string]interface{}
	json.Unmarshal(regRr.Body.Bytes(), &regResponse)
	token := regResponse["token"].(string)

	// Step 2: Instructor creates a workshop
	createBody := map[string]interface{}{
		"name":    "Learning Session",
		"seats":   5,
		"api_key": "sk-test",
	}
	createBytes, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRr := httptest.NewRecorder()
	server.router.ServeHTTP(createRr, createReq)

	if createRr.Code != http.StatusOK {
		t.Fatalf("Workshop creation failed: %d - %s", createRr.Code, createRr.Body.String())
	}

	var createResponse map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopCode := workshop["code"].(string)

	// Workshop creation is async - wait for provisioning to complete
	time.Sleep(100 * time.Millisecond)

	// Step 3: Learner registers for the workshop (public endpoint)
	learnerRegBody := map[string]interface{}{
		"workshop_code": workshopCode,
		"email":         "learner@example.com",
		"name":          "Test Learner",
	}
	learnerRegBytes, _ := json.Marshal(learnerRegBody)

	learnerRegReq := httptest.NewRequest("POST", "/api/register", bytes.NewReader(learnerRegBytes))
	learnerRegReq.Header.Set("Content-Type", "application/json")
	learnerRegRr := httptest.NewRecorder()
	server.router.ServeHTTP(learnerRegRr, learnerRegReq)

	if learnerRegRr.Code != http.StatusOK {
		t.Fatalf("Learner registration failed: %d - %s", learnerRegRr.Code, learnerRegRr.Body.String())
	}

	var learnerRegResponse map[string]interface{}
	json.Unmarshal(learnerRegRr.Body.Bytes(), &learnerRegResponse)
	accessCode := learnerRegResponse["access_code"].(string)

	if accessCode == "" {
		t.Error("Learner registration did not return access code")
	}

	// Step 4: Learner accesses session (public endpoint)
	sessionReq := httptest.NewRequest("GET", "/api/session/"+accessCode, nil)
	sessionRr := httptest.NewRecorder()
	server.router.ServeHTTP(sessionRr, sessionReq)

	if sessionRr.Code != http.StatusOK {
		t.Fatalf("Session access failed: %d - %s", sessionRr.Code, sessionRr.Body.String())
	}

	var sessionResponse map[string]interface{}
	json.Unmarshal(sessionRr.Body.Bytes(), &sessionResponse)

	// Should return ready status with endpoint (after async provisioning completes)
	status := sessionResponse["status"].(string)
	if status != "ready" {
		t.Errorf("Session status = %q, want %q", status, "ready")
	}
}

// ================== Provisioner Error Tests ==================
// Tests for error handling when provisioner fails

func TestWorkshopCreationProvisionerFailure(t *testing.T) {
	server, s, mockProv, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Simulate provisioner failure
	mockProv.CreateVMError = errors.New("GCP quota exceeded")

	createBody := map[string]interface{}{
		"name":    "Failing Workshop",
		"seats":   5,
		"api_key": "sk-test",
	}
	createBytes, _ := json.Marshal(createBody)

	req := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	// With async provisioning, the HTTP response returns 200 with "provisioning" status
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 (async provisioning), got %d", rr.Code)
	}

	var createResponse map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopID := workshop["id"].(string)

	// Initial status should be "provisioning"
	if workshop["status"] != "provisioning" {
		t.Errorf("Initial status should be provisioning, got %v", workshop["status"])
	}

	// Wait for async provisioning to fail
	time.Sleep(100 * time.Millisecond)

	// Verify workshop status is now "error" after async failure
	dbWorkshop, _ := s.GetWorkshop(workshopID)
	if dbWorkshop == nil {
		t.Fatal("Workshop not found in database")
	}
	if dbWorkshop.Status != "error" {
		t.Errorf("Workshop status should be 'error' after provisioner failure, got '%s'", dbWorkshop.Status)
	}
}

func TestWorkshopStartProvisionerFailure(t *testing.T) {
	server, s, mockProv, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a workshop without starting it (manually insert)
	workshop := &store.Workshop{
		ID:        "ws-test-start",
		Name:      "Test Start Workshop",
		Code:      "START-1234",
		Seats:     5,
		ApiKey:    "sk-test",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	// Simulate provisioner failure for start
	mockProv.CreateVMError = errors.New("VM creation failed")

	req := httptest.NewRequest("POST", "/api/workshops/ws-test-start/start", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when start provisioner fails, got %d", rr.Code)
	}
}

func TestWorkshopDeleteProvisionerFailureContinues(t *testing.T) {
	server, s, mockProv, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create and start a workshop
	createBody := map[string]interface{}{
		"name":    "Delete Test Workshop",
		"seats":   3,
		"api_key": "sk-test",
	}
	createBytes, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest("POST", "/api/workshops", bytes.NewReader(createBytes))
	createReq.Header.Set("Content-Type", "application/json")
	createRr := httptest.NewRecorder()
	server.router.ServeHTTP(createRr, createReq)

	var createResponse map[string]interface{}
	json.Unmarshal(createRr.Body.Bytes(), &createResponse)
	workshop := createResponse["workshop"].(map[string]interface{})
	workshopID := workshop["id"].(string)

	// Wait for async provisioning to complete
	time.Sleep(100 * time.Millisecond)

	// Simulate provisioner delete failure
	mockProv.DeleteVMError = errors.New("VM not found")

	// Delete should still succeed (returns 200 with "deleting" status)
	delReq := httptest.NewRequest("DELETE", "/api/workshops/"+workshopID, nil)
	delRr := httptest.NewRecorder()
	server.router.ServeHTTP(delRr, delReq)

	if delRr.Code != http.StatusOK {
		t.Errorf("Delete should succeed even if VM delete fails, got %d", delRr.Code)
	}

	// Wait for async deletion to complete (even with error, status should be "deleted")
	time.Sleep(100 * time.Millisecond)

	// Verify workshop is soft-deleted (status = "deleted")
	ws, _ := s.GetWorkshop(workshopID)
	if ws == nil {
		t.Error("Workshop should still exist in database (soft delete)")
	} else if ws.Status != "deleted" {
		t.Errorf("Workshop status should be 'deleted', got '%s'", ws.Status)
	}
}

// ================== Start Workshop Tests ==================

func TestStartWorkshopSuccess(t *testing.T) {
	server, s, mockProv, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a workshop in "created" status
	workshop := &store.Workshop{
		ID:        "ws-start-test",
		Name:      "Start Test Workshop",
		Code:      "START-TEST",
		Seats:     5,
		ApiKey:    "sk-test",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	req := httptest.NewRequest("POST", "/api/workshops/ws-start-test/start", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Start workshop failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["success"] != true {
		t.Error("Start should return success: true")
	}

	vm := response["vm"].(map[string]interface{})
	if vm["external_ip"] != "1.2.3.4" {
		t.Errorf("VM external_ip = %v, want 1.2.3.4", vm["external_ip"])
	}

	// Verify mock was called
	if len(mockProv.CreatedVMs) != 1 {
		t.Errorf("Expected 1 VM created, got %d", len(mockProv.CreatedVMs))
	}

	// Verify VM was stored in database
	dbVM, _ := s.GetVM("ws-start-test")
	if dbVM == nil {
		t.Error("VM should be stored in database")
	}
}

func TestStartWorkshopNotFound(t *testing.T) {
	server, _, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/workshops/nonexistent/start", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent workshop, got %d", rr.Code)
	}
}

// ================== Join Workshop Tests ==================

func TestJoinWorkshopSuccess(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a running workshop with VM
	workshop := &store.Workshop{
		ID:        "ws-join-test",
		Name:      "Join Test Workshop",
		Code:      "JOIN-TEST",
		Seats:     3,
		ApiKey:    "sk-test",
		Status:    "running",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	// Create sessions for the workshop
	for i := 1; i <= 3; i++ {
		session := &store.Session{
			OdeHash:    "ode-" + string(rune('a'+i)),
			WorkshopID: workshop.ID,
			SeatID:     i,
			Status:     "ready",
			JoinedAt:   time.Now(),
		}
		s.CreateSession(session)
	}

	// Create VM record
	vm := &store.WorkshopVM{
		ID:         "vm-join-test",
		WorkshopID: workshop.ID,
		VMName:     "test-vm",
		ExternalIP: "1.2.3.4",
		Status:     "RUNNING",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.CreateVM(vm)

	// Join the workshop
	joinBody := map[string]interface{}{
		"code": "JOIN-TEST",
		"name": "Test Learner",
	}
	joinBytes, _ := json.Marshal(joinBody)

	req := httptest.NewRequest("POST", "/api/join", bytes.NewReader(joinBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Join workshop failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["workshop_id"] != "ws-join-test" {
		t.Errorf("workshop_id = %v, want ws-join-test", response["workshop_id"])
	}

	if response["endpoint"] != "http://1.2.3.4:8080" {
		t.Errorf("endpoint = %v, want http://1.2.3.4:8080", response["endpoint"])
	}

	seat := response["seat"].(float64)
	if seat < 1 || seat > 3 {
		t.Errorf("seat = %v, want 1-3", seat)
	}
}

func TestJoinWorkshopNotFound(t *testing.T) {
	server, _, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	joinBody := map[string]interface{}{
		"code": "INVALID-CODE",
		"name": "Test Learner",
	}
	joinBytes, _ := json.Marshal(joinBody)

	req := httptest.NewRequest("POST", "/api/join", bytes.NewReader(joinBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for invalid code, got %d", rr.Code)
	}
}

func TestJoinWorkshopFull(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a workshop with all seats taken
	workshop := &store.Workshop{
		ID:        "ws-full",
		Name:      "Full Workshop",
		Code:      "FULL-TEST",
		Seats:     2,
		ApiKey:    "sk-test",
		Status:    "running",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	// Create VM
	vm := &store.WorkshopVM{
		ID:         "vm-full",
		WorkshopID: workshop.ID,
		VMName:     "test-vm",
		ExternalIP: "1.2.3.4",
		Status:     "RUNNING",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.CreateVM(vm)

	// Create sessions that are all occupied
	for i := 1; i <= 2; i++ {
		session := &store.Session{
			OdeHash:    "ode-full-" + string(rune('a'+i)),
			WorkshopID: workshop.ID,
			SeatID:     i,
			Name:       "Existing Learner " + string(rune('0'+i)),
			Status:     "occupied",
			JoinedAt:   time.Now(),
		}
		s.CreateSession(session)
	}

	// Try to join
	joinBody := map[string]interface{}{
		"code": "FULL-TEST",
		"name": "New Learner",
	}
	joinBytes, _ := json.Marshal(joinBody)

	req := httptest.NewRequest("POST", "/api/join", bytes.NewReader(joinBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409 when workshop is full, got %d", rr.Code)
	}
}

// ================== Admin Endpoints Tests ==================

func TestAdminOverview(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true) // auth disabled for simplicity
	defer cleanup()

	// Create some workshops
	for i := 0; i < 3; i++ {
		workshop := &store.Workshop{
			ID:        "ws-admin-" + string(rune('a'+i)),
			Name:      "Admin Workshop " + string(rune('A'+i)),
			Code:      "ADMIN-" + string(rune('A'+i)),
			Seats:     5,
			ApiKey:    "sk-test",
			Status:    "running",
			CreatedAt: time.Now(),
		}
		s.CreateWorkshop(workshop)
	}

	req := httptest.NewRequest("GET", "/api/admin/overview", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Admin overview failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	total := response["total"].(float64)
	if total != 3 {
		t.Errorf("total = %v, want 3", total)
	}

	workshops := response["workshops"].([]interface{})
	if len(workshops) != 3 {
		t.Errorf("Expected 3 workshops, got %d", len(workshops))
	}
}

func TestAdminListVMs(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a workshop with VM
	workshop := &store.Workshop{
		ID:        "ws-vm-list",
		Name:      "VM List Workshop",
		Code:      "VM-LIST",
		Seats:     5,
		ApiKey:    "sk-test",
		Status:    "running",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	vm := &store.WorkshopVM{
		ID:         "vm-list-test",
		WorkshopID: workshop.ID,
		VMName:     "clarateach-ws-vm-list",
		VMID:       "12345",
		Zone:       "us-central1-a",
		MachineType: "e2-standard-4",
		ExternalIP: "1.2.3.4",
		InternalIP: "10.0.0.1",
		Status:     "RUNNING",
		SSHUser:    "clarateach",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.CreateVM(vm)

	req := httptest.NewRequest("GET", "/api/admin/vms", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("List VMs failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	total := response["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}

	vms := response["vms"].([]interface{})
	if len(vms) != 1 {
		t.Errorf("Expected 1 VM, got %d", len(vms))
	}

	vmData := vms[0].(map[string]interface{})
	if vmData["workshop_name"] != "VM List Workshop" {
		t.Errorf("workshop_name = %v, want VM List Workshop", vmData["workshop_name"])
	}
}

func TestAdminGetVMDetails(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	workshop := &store.Workshop{
		ID:        "ws-vm-details",
		Name:      "VM Details Workshop",
		Code:      "VM-DETAILS",
		Seats:     5,
		ApiKey:    "sk-test",
		Status:    "running",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	vm := &store.WorkshopVM{
		ID:         "vm-details-test",
		WorkshopID: workshop.ID,
		VMName:     "clarateach-ws-vm-details",
		ExternalIP: "1.2.3.4",
		Status:     "RUNNING",
		SSHUser:    "clarateach",
		Zone:       "us-central1-a",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.CreateVM(vm)

	req := httptest.NewRequest("GET", "/api/admin/vms/ws-vm-details", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Get VM details failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	vmResp := response["vm"].(map[string]interface{})
	if vmResp["external_ip"] != "1.2.3.4" {
		t.Errorf("external_ip = %v, want 1.2.3.4", vmResp["external_ip"])
	}

	access := response["access"].(map[string]interface{})
	if access["ssh_command"] == nil {
		t.Error("Should return ssh_command in access")
	}
}

func TestAdminGetVMDetailsNotFound(t *testing.T) {
	server, _, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/admin/vms/nonexistent", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent VM, got %d", rr.Code)
	}
}

func TestAdminListUsers(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create some users
	hash, _ := auth.HashPassword("password123")
	for i := 0; i < 2; i++ {
		user := &store.User{
			ID:           "user-list-" + string(rune('a'+i)),
			Email:        "user" + string(rune('a'+i)) + "@example.com",
			PasswordHash: hash,
			Name:         "User " + string(rune('A'+i)),
			Role:         "instructor",
			CreatedAt:    time.Now(),
		}
		s.CreateUser(user)
	}

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("List users failed: %d - %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	users := response["users"].([]interface{})
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

// ================== Registration Edge Cases ==================

func TestRegistrationWorkshopFull(t *testing.T) {
	server, s, _, cleanup := setupTestServerWithMock(t, true)
	defer cleanup()

	// Create a workshop with 1 seat
	workshop := &store.Workshop{
		ID:        "ws-reg-full",
		Name:      "Full Registration Workshop",
		Code:      "REG-FULL",
		Seats:     1,
		ApiKey:    "sk-test",
		Status:    "running",
		CreatedAt: time.Now(),
	}
	s.CreateWorkshop(workshop)

	// First registration should succeed
	regBody1 := map[string]interface{}{
		"workshop_code": "REG-FULL",
		"email":         "first@example.com",
		"name":          "First User",
	}
	regBytes1, _ := json.Marshal(regBody1)

	req1 := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regBytes1))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	server.router.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First registration failed: %d", rr1.Code)
	}

	// Second registration should fail - workshop is full
	regBody2 := map[string]interface{}{
		"workshop_code": "REG-FULL",
		"email":         "second@example.com",
		"name":          "Second User",
	}
	regBytes2, _ := json.Marshal(regBody2)

	req2 := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regBytes2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	server.router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("Expected 409 when workshop is full, got %d", rr2.Code)
	}
}

func TestAuthLogout(t *testing.T) {
	server, _, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Logout returned %d, want %d", rr.Code, http.StatusOK)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["success"] != true {
		t.Error("Logout should return success: true")
	}
}
