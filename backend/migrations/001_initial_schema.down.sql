-- Migration: 001_initial_schema (rollback)
-- WARNING: This will delete all data!

DROP INDEX IF EXISTS idx_registrations_workshop_id;
DROP INDEX IF EXISTS idx_registrations_access_code;
DROP TABLE IF EXISTS registrations;

DROP INDEX IF EXISTS idx_workshop_vms_status;
DROP INDEX IF EXISTS idx_workshop_vms_workshop_id;
DROP TABLE IF EXISTS workshop_vms;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS workshops;

DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;

DELETE FROM schema_migrations WHERE version = 1;
