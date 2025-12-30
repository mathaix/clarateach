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
