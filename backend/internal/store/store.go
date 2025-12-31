package store

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// User represents an instructor or admin
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never return in JSON
	Name         string    `json:"name"`
	Role         string    `json:"role"` // "instructor" or "admin"
	CreatedAt    time.Time `json:"created_at"`
}

type Workshop struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Seats       int       `json:"seats"`
	ApiKey      string    `json:"-"`
	RuntimeType string    `json:"runtime_type"` // "docker" or "firecracker"
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	OdeHash     string    `json:"odehash"`
	WorkshopID  string    `json:"workshop_id"`
	SeatID      int       `json:"seat_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`       // "provisioning", "ready", "occupied"
	ContainerID string    `json:"container_id"` // Docker ID
	IP          string    `json:"ip"`
	JoinedAt    time.Time `json:"joined_at"`
}

type Store interface {
	// Workshop Operations
	CreateWorkshop(w *Workshop) error
	GetWorkshop(id string) (*Workshop, error)
	GetWorkshopByCode(code string) (*Workshop, error)
	ListWorkshops() ([]*Workshop, error)
	UpdateWorkshopStatus(id string, status string) error
	DeleteWorkshop(id string) error

	// Session Operations
	CreateSession(s *Session) error
	UpdateSession(s *Session) error
	GetSession(odehash string) (*Session, error)
	ListSessions(workshopID string) ([]*Session, error)
	GetSessionBySeat(workshopID string, seatID int) (*Session, error)
}

const schema = `
CREATE TABLE IF NOT EXISTS workshops (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	code TEXT NOT NULL UNIQUE,
	seats INTEGER NOT NULL,
	api_key TEXT NOT NULL,
	runtime_type TEXT DEFAULT 'docker',
	status TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	odehash TEXT PRIMARY KEY,
	workshop_id TEXT NOT NULL,
	seat_id INTEGER NOT NULL,
	name TEXT,
	status TEXT DEFAULT 'provisioning',
	container_id TEXT,
	ip TEXT,
	joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(workshop_id) REFERENCES workshops(id),
	UNIQUE(workshop_id, seat_id)
);

CREATE TABLE IF NOT EXISTS workshop_vms (
	id TEXT PRIMARY KEY,
	workshop_id TEXT NOT NULL,
	vm_name TEXT NOT NULL,
	vm_id TEXT,
	zone TEXT NOT NULL,
	machine_type TEXT NOT NULL,
	external_ip TEXT,
	internal_ip TEXT,
	status TEXT NOT NULL DEFAULT 'provisioning',
	ssh_public_key TEXT,
	ssh_private_key TEXT,
	ssh_user TEXT DEFAULT 'clarateach',
	provisioning_started_at DATETIME,
	provisioning_completed_at DATETIME,
	provisioning_duration_ms INTEGER DEFAULT 0,
	removed_at DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(workshop_id) REFERENCES workshops(id)
);

CREATE INDEX IF NOT EXISTS idx_workshop_vms_workshop_id ON workshop_vms(workshop_id);
CREATE INDEX IF NOT EXISTS idx_workshop_vms_status ON workshop_vms(status);

CREATE TABLE IF NOT EXISTS registrations (
	id TEXT PRIMARY KEY,
	access_code TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL,
	name TEXT NOT NULL,
	workshop_id TEXT NOT NULL,
	seat_id INTEGER,
	status TEXT NOT NULL DEFAULT 'registered',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	joined_at DATETIME,
	FOREIGN KEY(workshop_id) REFERENCES workshops(id),
	UNIQUE(workshop_id, email)
);

CREATE INDEX IF NOT EXISTS idx_registrations_access_code ON registrations(access_code);
CREATE INDEX IF NOT EXISTS idx_registrations_workshop_id ON registrations(workshop_id);
`

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	// Run migrations for existing databases
	migrations := []string{
		// Add owner_id column to workshops if it doesn't exist
		`ALTER TABLE workshops ADD COLUMN owner_id TEXT`,
	}

	for _, migration := range migrations {
		// Ignore errors for migrations (column may already exist)
		db.Exec(migration)
	}

	return db, nil
}
