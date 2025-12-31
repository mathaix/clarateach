package store

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Workshop struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Seats     int       `json:"seats"`
	ApiKey    string    `json:"-"` // Never return API key in JSON
	Status    string    `json:"status"`
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

	// VM Operations
	CreateVM(vm *WorkshopVM) error
	GetVM(workshopID string) (*WorkshopVM, error)            // Gets active (non-removed) VM for workshop
	GetVMByID(id string) (*WorkshopVM, error)
	UpdateVM(vm *WorkshopVM) error
	MarkVMRemoved(workshopID string) error                   // Soft delete - marks VM as removed
	ListVMs() ([]*WorkshopVM, error)                         // Lists active VMs only
	ListAllVMs() ([]*WorkshopVM, error)                      // Lists all VMs including removed
	GetVMPrivateKey(workshopID string) (string, error)       // Returns SSH private key
}

const schema = `
CREATE TABLE IF NOT EXISTS workshops (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	code TEXT NOT NULL UNIQUE,
	seats INTEGER NOT NULL,
	api_key TEXT NOT NULL,
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
`

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return db, nil
}
