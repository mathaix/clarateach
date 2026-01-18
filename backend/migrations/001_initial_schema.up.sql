-- Migration: 001_initial_schema
-- Description: Initial PostgreSQL schema for ClaraTeach
-- Converted from SQLite schema

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'instructor',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Workshops table
CREATE TABLE IF NOT EXISTS workshops (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    seats INTEGER NOT NULL,
    api_key TEXT NOT NULL,
    runtime_type TEXT NOT NULL DEFAULT 'docker',
    status TEXT NOT NULL,
    owner_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(owner_id) REFERENCES users(id)
);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    odehash TEXT PRIMARY KEY,
    workshop_id TEXT NOT NULL,
    seat_id INTEGER NOT NULL,
    name TEXT,
    status TEXT DEFAULT 'provisioning',
    container_id TEXT,
    ip TEXT,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(workshop_id) REFERENCES workshops(id),
    UNIQUE(workshop_id, seat_id)
);

-- Workshop VMs table
CREATE TABLE IF NOT EXISTS workshop_vms (
    id TEXT PRIMARY KEY,
    workshop_id TEXT NOT NULL,
    vm_name TEXT NOT NULL,
    vm_id TEXT,
    zone TEXT NOT NULL,
    machine_type TEXT NOT NULL,
    external_ip TEXT,
    internal_ip TEXT,
    tunnel_url TEXT,
    status TEXT NOT NULL DEFAULT 'provisioning',
    ssh_public_key TEXT,
    ssh_private_key TEXT,
    ssh_user TEXT DEFAULT 'clarateach',
    provisioning_started_at TIMESTAMP,
    provisioning_completed_at TIMESTAMP,
    provisioning_duration_ms INTEGER DEFAULT 0,
    removed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(workshop_id) REFERENCES workshops(id)
);

CREATE INDEX IF NOT EXISTS idx_workshop_vms_workshop_id ON workshop_vms(workshop_id);
CREATE INDEX IF NOT EXISTS idx_workshop_vms_status ON workshop_vms(status);

-- Registrations table
CREATE TABLE IF NOT EXISTS registrations (
    id TEXT PRIMARY KEY,
    access_code TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    workshop_id TEXT NOT NULL,
    seat_id INTEGER,
    status TEXT NOT NULL DEFAULT 'registered',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    joined_at TIMESTAMP,
    FOREIGN KEY(workshop_id) REFERENCES workshops(id),
    UNIQUE(workshop_id, email)
);

CREATE INDEX IF NOT EXISTS idx_registrations_access_code ON registrations(access_code);
CREATE INDEX IF NOT EXISTS idx_registrations_workshop_id ON registrations(workshop_id);

-- Schema migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Record this migration
INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT DO NOTHING;
