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

// -- VM Operations --

func (s *SQLiteStore) CreateVM(vm *WorkshopVM) error {
	query := `INSERT INTO workshop_vms (id, workshop_id, vm_name, vm_id, zone, machine_type, external_ip, internal_ip, status, ssh_public_key, ssh_private_key, ssh_user, provisioning_started_at, provisioning_completed_at, provisioning_duration_ms, removed_at, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, vm.ID, vm.WorkshopID, vm.VMName, vm.VMID, vm.Zone, vm.MachineType,
		vm.ExternalIP, vm.InternalIP, vm.Status, vm.SSHPublicKey, vm.SSHPrivateKey, vm.SSHUser,
		vm.ProvisioningStartedAt, vm.ProvisioningCompletedAt, vm.ProvisioningDurationMs, vm.RemovedAt,
		vm.CreatedAt, vm.UpdatedAt)
	return err
}

func (s *SQLiteStore) GetVM(workshopID string) (*WorkshopVM, error) {
	vm := &WorkshopVM{}
	query := `SELECT id, workshop_id, vm_name, vm_id, zone, machine_type, external_ip, internal_ip, status, ssh_public_key, ssh_user, provisioning_started_at, provisioning_completed_at, provisioning_duration_ms, removed_at, created_at, updated_at
			  FROM workshop_vms WHERE workshop_id = ? AND removed_at IS NULL ORDER BY created_at DESC LIMIT 1`
	err := s.db.QueryRow(query, workshopID).Scan(
		&vm.ID, &vm.WorkshopID, &vm.VMName, &vm.VMID, &vm.Zone, &vm.MachineType,
		&vm.ExternalIP, &vm.InternalIP, &vm.Status, &vm.SSHPublicKey, &vm.SSHUser,
		&vm.ProvisioningStartedAt, &vm.ProvisioningCompletedAt, &vm.ProvisioningDurationMs,
		&vm.RemovedAt, &vm.CreatedAt, &vm.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return vm, err
}

func (s *SQLiteStore) GetVMByID(id string) (*WorkshopVM, error) {
	vm := &WorkshopVM{}
	query := `SELECT id, workshop_id, vm_name, vm_id, zone, machine_type, external_ip, internal_ip, status, ssh_public_key, ssh_user, provisioning_started_at, provisioning_completed_at, provisioning_duration_ms, removed_at, created_at, updated_at
			  FROM workshop_vms WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(
		&vm.ID, &vm.WorkshopID, &vm.VMName, &vm.VMID, &vm.Zone, &vm.MachineType,
		&vm.ExternalIP, &vm.InternalIP, &vm.Status, &vm.SSHPublicKey, &vm.SSHUser,
		&vm.ProvisioningStartedAt, &vm.ProvisioningCompletedAt, &vm.ProvisioningDurationMs,
		&vm.RemovedAt, &vm.CreatedAt, &vm.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return vm, err
}

func (s *SQLiteStore) UpdateVM(vm *WorkshopVM) error {
	query := `UPDATE workshop_vms SET vm_id = ?, external_ip = ?, internal_ip = ?, status = ?, provisioning_completed_at = ?, provisioning_duration_ms = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, vm.VMID, vm.ExternalIP, vm.InternalIP, vm.Status,
		vm.ProvisioningCompletedAt, vm.ProvisioningDurationMs, vm.UpdatedAt, vm.ID)
	return err
}

func (s *SQLiteStore) MarkVMRemoved(workshopID string) error {
	query := `UPDATE workshop_vms SET status = 'removed', removed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE workshop_id = ? AND removed_at IS NULL`
	_, err := s.db.Exec(query, workshopID)
	return err
}

func (s *SQLiteStore) ListVMs() ([]*WorkshopVM, error) {
	query := `SELECT id, workshop_id, vm_name, vm_id, zone, machine_type, external_ip, internal_ip, status, ssh_public_key, ssh_user, provisioning_started_at, provisioning_completed_at, provisioning_duration_ms, removed_at, created_at, updated_at
			  FROM workshop_vms WHERE removed_at IS NULL ORDER BY created_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []*WorkshopVM
	for rows.Next() {
		vm := &WorkshopVM{}
		if err := rows.Scan(
			&vm.ID, &vm.WorkshopID, &vm.VMName, &vm.VMID, &vm.Zone, &vm.MachineType,
			&vm.ExternalIP, &vm.InternalIP, &vm.Status, &vm.SSHPublicKey, &vm.SSHUser,
			&vm.ProvisioningStartedAt, &vm.ProvisioningCompletedAt, &vm.ProvisioningDurationMs,
			&vm.RemovedAt, &vm.CreatedAt, &vm.UpdatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, nil
}

func (s *SQLiteStore) ListAllVMs() ([]*WorkshopVM, error) {
	query := `SELECT id, workshop_id, vm_name, vm_id, zone, machine_type, external_ip, internal_ip, status, ssh_public_key, ssh_user, provisioning_started_at, provisioning_completed_at, provisioning_duration_ms, removed_at, created_at, updated_at
			  FROM workshop_vms ORDER BY created_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []*WorkshopVM
	for rows.Next() {
		vm := &WorkshopVM{}
		if err := rows.Scan(
			&vm.ID, &vm.WorkshopID, &vm.VMName, &vm.VMID, &vm.Zone, &vm.MachineType,
			&vm.ExternalIP, &vm.InternalIP, &vm.Status, &vm.SSHPublicKey, &vm.SSHUser,
			&vm.ProvisioningStartedAt, &vm.ProvisioningCompletedAt, &vm.ProvisioningDurationMs,
			&vm.RemovedAt, &vm.CreatedAt, &vm.UpdatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, nil
}

func (s *SQLiteStore) GetVMPrivateKey(workshopID string) (string, error) {
	var privateKey string
	query := `SELECT ssh_private_key FROM workshop_vms WHERE workshop_id = ?`
	err := s.db.QueryRow(query, workshopID).Scan(&privateKey)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return privateKey, err
}
