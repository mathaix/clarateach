package store

import (
	"database/sql"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// -- Workshop Operations --

func (s *SQLiteStore) CreateWorkshop(w *Workshop) error {
	query := `INSERT INTO workshops (id, name, code, seats, api_key, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, w.ID, w.Name, w.Code, w.Seats, w.ApiKey, w.Status, w.CreatedAt)
	return err
}

func (s *SQLiteStore) GetWorkshop(id string) (*Workshop, error) {
	w := &Workshop{}
	query := `SELECT id, name, code, seats, api_key, status, created_at FROM workshops WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(&w.ID, &w.Name, &w.Code, &w.Seats, &w.ApiKey, &w.Status, &w.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	return w, err
}

func (s *SQLiteStore) GetWorkshopByCode(code string) (*Workshop, error) {
	w := &Workshop{}
	query := `SELECT id, name, code, seats, api_key, status, created_at FROM workshops WHERE code = ?`
	err := s.db.QueryRow(query, code).Scan(&w.ID, &w.Name, &w.Code, &w.Seats, &w.ApiKey, &w.Status, &w.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return w, err
}

func (s *SQLiteStore) ListWorkshops() ([]*Workshop, error) {
	query := `SELECT id, name, code, seats, api_key, status, created_at FROM workshops ORDER BY created_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workshops []*Workshop
	for rows.Next() {
		w := &Workshop{}
		if err := rows.Scan(&w.ID, &w.Name, &w.Code, &w.Seats, &w.ApiKey, &w.Status, &w.CreatedAt); err != nil {
			return nil, err
		}
		workshops = append(workshops, w)
	}
	return workshops, nil
}

func (s *SQLiteStore) UpdateWorkshopStatus(id string, status string) error {
	query := `UPDATE workshops SET status = ? WHERE id = ?`
	_, err := s.db.Exec(query, status, id)
	return err
}

func (s *SQLiteStore) DeleteWorkshop(id string) error {
	// First delete related sessions
	_, err := s.db.Exec(`DELETE FROM sessions WHERE workshop_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM workshops WHERE id = ?`, id)
	return err
}

// -- Session Operations --

func (s *SQLiteStore) CreateSession(session *Session) error {
	query := `INSERT INTO sessions (odehash, workshop_id, seat_id, name, status, container_id, ip, joined_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, session.OdeHash, session.WorkshopID, session.SeatID, session.Name, session.Status, session.ContainerID, session.IP, session.JoinedAt)
	return err
}

func (s *SQLiteStore) UpdateSession(session *Session) error {
	query := `UPDATE sessions SET name = ?, status = ?, container_id = ?, ip = ? WHERE odehash = ?`
	_, err := s.db.Exec(query, session.Name, session.Status, session.ContainerID, session.IP, session.OdeHash)
	return err
}

func (s *SQLiteStore) GetSession(odehash string) (*Session, error) {
	sess := &Session{}
	query := `SELECT odehash, workshop_id, seat_id, name, status, container_id, ip, joined_at FROM sessions WHERE odehash = ?`
	err := s.db.QueryRow(query, odehash).Scan(&sess.OdeHash, &sess.WorkshopID, &sess.SeatID, &sess.Name, &sess.Status, &sess.ContainerID, &sess.IP, &sess.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *SQLiteStore) GetSessionBySeat(workshopID string, seatID int) (*Session, error) {
	sess := &Session{}
	query := `SELECT odehash, workshop_id, seat_id, name, status, container_id, ip, joined_at FROM sessions WHERE workshop_id = ? AND seat_id = ?`
	err := s.db.QueryRow(query, workshopID, seatID).Scan(&sess.OdeHash, &sess.WorkshopID, &sess.SeatID, &sess.Name, &sess.Status, &sess.ContainerID, &sess.IP, &sess.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *SQLiteStore) ListSessions(workshopID string) ([]*Session, error) {
	query := `SELECT odehash, workshop_id, seat_id, name, status, container_id, ip, joined_at FROM sessions WHERE workshop_id = ?`
	rows, err := s.db.Query(query, workshopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		sess := &Session{}
		if err := rows.Scan(&sess.OdeHash, &sess.WorkshopID, &sess.SeatID, &sess.Name, &sess.Status, &sess.ContainerID, &sess.IP, &sess.JoinedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
