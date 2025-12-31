package store

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	// Create a temporary database file
	tmpFile, err := os.CreateTemp("", "clarateach_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := InitDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to initialize database: %v", err)
	}

	store := NewSQLiteStore(db)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

// User Tests

func TestCreateAndGetUser(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	user := &User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		Name:         "Test User",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}

	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Test GetUser
	got, err := store.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetUser() returned nil")
	}

	if got.ID != user.ID {
		t.Errorf("GetUser() ID = %v, want %v", got.ID, user.ID)
	}

	if got.Email != user.Email {
		t.Errorf("GetUser() Email = %v, want %v", got.Email, user.Email)
	}

	if got.Name != user.Name {
		t.Errorf("GetUser() Name = %v, want %v", got.Name, user.Name)
	}
}

func TestGetUserByEmail(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	user := &User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		Name:         "Test User",
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}

	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Test GetUserByEmail
	got, err := store.GetUserByEmail(user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetUserByEmail() returned nil")
	}

	if got.ID != user.ID {
		t.Errorf("GetUserByEmail() ID = %v, want %v", got.ID, user.ID)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	got, err := store.GetUserByEmail("nonexistent@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}

	if got != nil {
		t.Errorf("GetUserByEmail() = %v, want nil", got)
	}
}

func TestListUsers(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	users := []*User{
		{ID: "user-1", Email: "user1@example.com", PasswordHash: "hash", Name: "User 1", Role: "instructor", CreatedAt: time.Now()},
		{ID: "user-2", Email: "user2@example.com", PasswordHash: "hash", Name: "User 2", Role: "admin", CreatedAt: time.Now()},
	}

	for _, u := range users {
		if err := store.CreateUser(u); err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}
	}

	got, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(got) != len(users) {
		t.Errorf("ListUsers() returned %d users, want %d", len(got), len(users))
	}
}

// Workshop Tests

func TestCreateAndGetWorkshop(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		OwnerID:   "user-123",
		CreatedAt: time.Now(),
	}

	err := store.CreateWorkshop(workshop)
	if err != nil {
		t.Fatalf("CreateWorkshop() error = %v", err)
	}

	// Test GetWorkshop
	got, err := store.GetWorkshop(workshop.ID)
	if err != nil {
		t.Fatalf("GetWorkshop() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetWorkshop() returned nil")
	}

	if got.ID != workshop.ID {
		t.Errorf("GetWorkshop() ID = %v, want %v", got.ID, workshop.ID)
	}

	if got.Name != workshop.Name {
		t.Errorf("GetWorkshop() Name = %v, want %v", got.Name, workshop.Name)
	}

	if got.Code != workshop.Code {
		t.Errorf("GetWorkshop() Code = %v, want %v", got.Code, workshop.Code)
	}

	// Verify OwnerID is persisted correctly
	if got.OwnerID != workshop.OwnerID {
		t.Errorf("GetWorkshop() OwnerID = %v, want %v", got.OwnerID, workshop.OwnerID)
	}
}

func TestWorkshopOwnerIDPersistence(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Test that owner_id is properly saved and retrieved
	testCases := []struct {
		name    string
		ownerID string
	}{
		{"with owner ID", "user-owner-123"},
		{"empty owner ID", ""},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workshop := &Workshop{
				ID:        "workshop-" + string(rune('A'+i)),
				Name:      "Workshop " + tc.name,
				Code:      "CODE" + string(rune('A'+i)),
				Seats:     5,
				ApiKey:    "sk-test",
				Status:    "created",
				OwnerID:   tc.ownerID,
				CreatedAt: time.Now(),
			}

			err := store.CreateWorkshop(workshop)
			if err != nil {
				t.Fatalf("CreateWorkshop() error = %v", err)
			}

			got, err := store.GetWorkshop(workshop.ID)
			if err != nil {
				t.Fatalf("GetWorkshop() error = %v", err)
			}

			if got.OwnerID != tc.ownerID {
				t.Errorf("OwnerID = %q, want %q", got.OwnerID, tc.ownerID)
			}
		})
	}
}

func TestGetWorkshopByCode(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}

	err := store.CreateWorkshop(workshop)
	if err != nil {
		t.Fatalf("CreateWorkshop() error = %v", err)
	}

	got, err := store.GetWorkshopByCode(workshop.Code)
	if err != nil {
		t.Fatalf("GetWorkshopByCode() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetWorkshopByCode() returned nil")
	}

	if got.ID != workshop.ID {
		t.Errorf("GetWorkshopByCode() ID = %v, want %v", got.ID, workshop.ID)
	}
}

func TestListWorkshopsByOwner(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Create workshops for different owners
	workshops := []*Workshop{
		{ID: "w1", Name: "Workshop 1", Code: "CODE1", Seats: 5, ApiKey: "key", Status: "created", OwnerID: "user-1", CreatedAt: time.Now()},
		{ID: "w2", Name: "Workshop 2", Code: "CODE2", Seats: 5, ApiKey: "key", Status: "created", OwnerID: "user-1", CreatedAt: time.Now()},
		{ID: "w3", Name: "Workshop 3", Code: "CODE3", Seats: 5, ApiKey: "key", Status: "created", OwnerID: "user-2", CreatedAt: time.Now()},
	}

	for _, w := range workshops {
		if err := store.CreateWorkshop(w); err != nil {
			t.Fatalf("CreateWorkshop() error = %v", err)
		}
	}

	// Test for user-1
	got, err := store.ListWorkshopsByOwner("user-1")
	if err != nil {
		t.Fatalf("ListWorkshopsByOwner() error = %v", err)
	}

	if len(got) != 2 {
		t.Errorf("ListWorkshopsByOwner() returned %d workshops, want 2", len(got))
	}

	// Test for user-2
	got, err = store.ListWorkshopsByOwner("user-2")
	if err != nil {
		t.Fatalf("ListWorkshopsByOwner() error = %v", err)
	}

	if len(got) != 1 {
		t.Errorf("ListWorkshopsByOwner() returned %d workshops, want 1", len(got))
	}
}

func TestUpdateWorkshopStatus(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}

	err := store.CreateWorkshop(workshop)
	if err != nil {
		t.Fatalf("CreateWorkshop() error = %v", err)
	}

	err = store.UpdateWorkshopStatus(workshop.ID, "running")
	if err != nil {
		t.Fatalf("UpdateWorkshopStatus() error = %v", err)
	}

	got, err := store.GetWorkshop(workshop.ID)
	if err != nil {
		t.Fatalf("GetWorkshop() error = %v", err)
	}

	if got.Status != "running" {
		t.Errorf("UpdateWorkshopStatus() Status = %v, want running", got.Status)
	}
}

func TestDeleteWorkshop(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}

	err := store.CreateWorkshop(workshop)
	if err != nil {
		t.Fatalf("CreateWorkshop() error = %v", err)
	}

	err = store.DeleteWorkshop(workshop.ID)
	if err != nil {
		t.Fatalf("DeleteWorkshop() error = %v", err)
	}

	got, err := store.GetWorkshop(workshop.ID)
	if err != nil {
		t.Fatalf("GetWorkshop() error = %v", err)
	}

	if got != nil {
		t.Error("DeleteWorkshop() did not delete the workshop")
	}
}

// Session Tests

func TestCreateAndGetSession(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// First create a workshop
	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	session := &Session{
		OdeHash:     "odehash-123",
		WorkshopID:  workshop.ID,
		SeatID:      1,
		Name:        "Test Learner",
		Status:      "ready",
		ContainerID: "container-123",
		IP:          "192.168.1.1",
		JoinedAt:    time.Now(),
	}

	err := store.CreateSession(session)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	got, err := store.GetSession(session.OdeHash)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetSession() returned nil")
	}

	if got.OdeHash != session.OdeHash {
		t.Errorf("GetSession() OdeHash = %v, want %v", got.OdeHash, session.OdeHash)
	}

	if got.Name != session.Name {
		t.Errorf("GetSession() Name = %v, want %v", got.Name, session.Name)
	}
}

func TestGetSessionBySeat(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	session := &Session{
		OdeHash:     "odehash-123",
		WorkshopID:  workshop.ID,
		SeatID:      5,
		Name:        "Test Learner",
		Status:      "ready",
		ContainerID: "container-123",
		IP:          "192.168.1.1",
		JoinedAt:    time.Now(),
	}
	store.CreateSession(session)

	got, err := store.GetSessionBySeat(workshop.ID, 5)
	if err != nil {
		t.Fatalf("GetSessionBySeat() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetSessionBySeat() returned nil")
	}

	if got.SeatID != 5 {
		t.Errorf("GetSessionBySeat() SeatID = %v, want 5", got.SeatID)
	}
}

func TestListSessions(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	sessions := []*Session{
		{OdeHash: "ode1", WorkshopID: workshop.ID, SeatID: 1, Name: "Learner 1", Status: "ready", ContainerID: "c1", IP: "1.1.1.1", JoinedAt: time.Now()},
		{OdeHash: "ode2", WorkshopID: workshop.ID, SeatID: 2, Name: "Learner 2", Status: "ready", ContainerID: "c2", IP: "1.1.1.2", JoinedAt: time.Now()},
	}

	for _, s := range sessions {
		store.CreateSession(s)
	}

	got, err := store.ListSessions(workshop.ID)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(got) != 2 {
		t.Errorf("ListSessions() returned %d sessions, want 2", len(got))
	}
}

// Registration Tests

func TestCreateAndGetRegistration(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	reg := &Registration{
		ID:         "reg-123",
		AccessCode: "FZL-7X9K",
		Email:      "learner@example.com",
		Name:       "Test Learner",
		WorkshopID: workshop.ID,
		Status:     "registered",
		CreatedAt:  time.Now(),
	}

	err := store.CreateRegistration(reg)
	if err != nil {
		t.Fatalf("CreateRegistration() error = %v", err)
	}

	got, err := store.GetRegistration(reg.AccessCode)
	if err != nil {
		t.Fatalf("GetRegistration() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetRegistration() returned nil")
	}

	if got.AccessCode != reg.AccessCode {
		t.Errorf("GetRegistration() AccessCode = %v, want %v", got.AccessCode, reg.AccessCode)
	}

	if got.Email != reg.Email {
		t.Errorf("GetRegistration() Email = %v, want %v", got.Email, reg.Email)
	}
}

func TestGetRegistrationByEmail(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	reg := &Registration{
		ID:         "reg-123",
		AccessCode: "FZL-7X9K",
		Email:      "learner@example.com",
		Name:       "Test Learner",
		WorkshopID: workshop.ID,
		Status:     "registered",
		CreatedAt:  time.Now(),
	}
	store.CreateRegistration(reg)

	got, err := store.GetRegistrationByEmail(workshop.ID, reg.Email)
	if err != nil {
		t.Fatalf("GetRegistrationByEmail() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetRegistrationByEmail() returned nil")
	}

	if got.Email != reg.Email {
		t.Errorf("GetRegistrationByEmail() Email = %v, want %v", got.Email, reg.Email)
	}
}

func TestCountRegistrations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	regs := []*Registration{
		{ID: "reg-1", AccessCode: "CODE-1", Email: "a@example.com", Name: "A", WorkshopID: workshop.ID, Status: "registered", CreatedAt: time.Now()},
		{ID: "reg-2", AccessCode: "CODE-2", Email: "b@example.com", Name: "B", WorkshopID: workshop.ID, Status: "registered", CreatedAt: time.Now()},
		{ID: "reg-3", AccessCode: "CODE-3", Email: "c@example.com", Name: "C", WorkshopID: workshop.ID, Status: "registered", CreatedAt: time.Now()},
	}

	for _, r := range regs {
		store.CreateRegistration(r)
	}

	count, err := store.CountRegistrations(workshop.ID)
	if err != nil {
		t.Fatalf("CountRegistrations() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountRegistrations() = %d, want 3", count)
	}
}

func TestUpdateRegistration(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	workshop := &Workshop{
		ID:        "workshop-123",
		Name:      "Test Workshop",
		Code:      "ABC123",
		Seats:     10,
		ApiKey:    "sk-test-key",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	store.CreateWorkshop(workshop)

	reg := &Registration{
		ID:         "reg-123",
		AccessCode: "FZL-7X9K",
		Email:      "learner@example.com",
		Name:       "Test Learner",
		WorkshopID: workshop.ID,
		Status:     "registered",
		CreatedAt:  time.Now(),
	}
	store.CreateRegistration(reg)

	// Update registration
	seatID := 5
	now := time.Now()
	reg.SeatID = &seatID
	reg.Status = "active"
	reg.JoinedAt = &now

	err := store.UpdateRegistration(reg)
	if err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}

	got, _ := store.GetRegistration(reg.AccessCode)
	if got.Status != "active" {
		t.Errorf("UpdateRegistration() Status = %v, want active", got.Status)
	}
	if got.SeatID == nil || *got.SeatID != 5 {
		t.Errorf("UpdateRegistration() SeatID = %v, want 5", got.SeatID)
	}
}
