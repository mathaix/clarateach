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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Seats     int       `json:"seats"`
	ApiKey    string    `json:"-"` // Never return API key in JSON
	Status    string    `json:"status"`
	OwnerID   string    `json:"owner_id"` // User ID of the instructor
	CreatedAt time.Time `json:"created_at"`
}

// WorkshopVM represents a GCP VM provisioned for a workshop
type WorkshopVM struct {
	ID                     string     `json:"id"`                       // Internal ID
	WorkshopID             string     `json:"workshop_id"`
	VMName                 string     `json:"vm_name"`                  // GCE instance name
	VMID                   string     `json:"vm_id"`                    // GCE instance ID
	Zone                   string     `json:"zone"`
	MachineType            string     `json:"machine_type"`
	ExternalIP             string     `json:"external_ip"`
	InternalIP             string     `json:"internal_ip"`
	Status                 string     `json:"status"`                   // provisioning, running, stopping, terminated, removed
	SSHPublicKey           string     `json:"ssh_public_key"`           // OpenSSH format
	SSHPrivateKey          string     `json:"-"`                        // Never return in JSON - PEM format
	SSHUser                string     `json:"ssh_user"`                 // Username for SSH
	ProvisioningStartedAt  *time.Time `json:"provisioning_started_at"`  // When provisioning began
	ProvisioningCompletedAt *time.Time `json:"provisioning_completed_at"` // When VM became ready
	ProvisioningDurationMs int64      `json:"provisioning_duration_ms"` // Time to provision VM in milliseconds
	RemovedAt              *time.Time `json:"removed_at,omitempty"`     // When VM was removed/deleted
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// AdminWorkshopView combines workshop info with VM and session data
type AdminWorkshopView struct {
	Workshop      *Workshop     `json:"workshop"`
	VM            *WorkshopVM   `json:"vm,omitempty"`
	Sessions      []*Session    `json:"sessions"`
	ActiveStudents int          `json:"active_students"`
	TotalSeats    int           `json:"total_seats"`
	SSHCommand    string        `json:"ssh_command,omitempty"`
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

// Registration represents a learner's registration for a workshop
type Registration struct {
	ID          string     `json:"id"`
	AccessCode  string     `json:"access_code"`   // User-facing code like "FZL-7X9K"
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	WorkshopID  string     `json:"workshop_id"`
	SeatID      *int       `json:"seat_id"`       // NULL until first join
	Status      string     `json:"status"`        // registered, active, expired
	CreatedAt   time.Time  `json:"created_at"`
	JoinedAt    *time.Time `json:"joined_at"`     // When they first accessed workspace
}

type Store interface {
	// User Operations
	CreateUser(u *User) error
	GetUser(id string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	ListUsers() ([]*User, error)

	// Workshop Operations
	CreateWorkshop(w *Workshop) error
	GetWorkshop(id string) (*Workshop, error)
	GetWorkshopByCode(code string) (*Workshop, error)
	ListWorkshops() ([]*Workshop, error)
	ListWorkshopsByOwner(ownerID string) ([]*Workshop, error)
	UpdateWorkshopStatus(id string, status string) error
	DeleteWorkshop(id string) error

	// Session Operations
	CreateSession(s *Session) error
	UpdateSession(s *Session) error
	GetSession(odehash string) (*Session, error)
	ListSessions(workshopID string) ([]*Session, error)
	GetSessionBySeat(workshopID string, seatID int) (*Session, error)

	// VM Operations
	CreateVM(vm *WorkshopVM) error
	GetVM(workshopID string) (*WorkshopVM, error)            // Gets active (non-removed) VM for workshop
	GetVMByID(id string) (*WorkshopVM, error)
	UpdateVM(vm *WorkshopVM) error
	MarkVMRemoved(workshopID string) error                   // Soft delete - marks VM as removed
	ListVMs() ([]*WorkshopVM, error)                         // Lists active VMs only
	ListAllVMs() ([]*WorkshopVM, error)                      // Lists all VMs including removed
	GetVMPrivateKey(workshopID string) (string, error)       // Returns SSH private key

	// Registration Operations
	CreateRegistration(r *Registration) error
	GetRegistration(accessCode string) (*Registration, error)
	GetRegistrationByEmail(workshopID, email string) (*Registration, error)
	UpdateRegistration(r *Registration) error
	CountRegistrations(workshopID string) (int, error)
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'instructor',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS workshops (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	code TEXT NOT NULL UNIQUE,
	seats INTEGER NOT NULL,
	api_key TEXT NOT NULL,
	status TEXT NOT NULL,
	owner_id TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(owner_id) REFERENCES users(id)
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
