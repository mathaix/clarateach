# ClaraTeach Database Migrations

This directory contains database migrations for the ClaraTeach PostgreSQL database.

## Prerequisites

- `psql` (PostgreSQL client) must be installed
- A valid PostgreSQL connection string

## Usage

### Run migrations

```bash
# Using environment variable
export DATABASE_URL="postgresql://user:pass@host/db?sslmode=require"
./migrate.sh up

# Or pass directly
./migrate.sh up "postgresql://user:pass@host/db?sslmode=require"
```

### Check status

```bash
./migrate.sh status
```

### Rollback last migration

```bash
./migrate.sh down
```

## Neon Database

For the Neon database (clarateach project):

```bash
export DATABASE_URL="postgresql://neondb_owner:npg_fuRoV8LF5HhX@ep-muddy-butterfly-aep0p6ai-pooler.c-2.us-east-2.aws.neon.tech/neondb?sslmode=require"
./migrate.sh up
```

## Migration Files

Each migration has two files:
- `XXX_name.up.sql` - Apply the migration
- `XXX_name.down.sql` - Rollback the migration

### Current Migrations

| Version | Name | Description |
|---------|------|-------------|
| 001 | initial_schema | Initial PostgreSQL schema (users, workshops, sessions, workshop_vms, registrations) |

## Creating New Migrations

1. Create two files with the next version number:
   - `002_description.up.sql`
   - `002_description.down.sql`

2. In the up migration, add your changes and record the version:
   ```sql
   -- Your DDL statements here
   INSERT INTO schema_migrations (version) VALUES (2) ON CONFLICT DO NOTHING;
   ```

3. In the down migration, reverse the changes and remove the version:
   ```sql
   -- Reverse DDL statements
   DELETE FROM schema_migrations WHERE version = 2;
   ```

## SQLite to PostgreSQL Differences

Key differences handled in the migration:
- `DATETIME` → `TIMESTAMP`
- `AUTOINCREMENT` → `SERIAL` (not used here, using TEXT PKs)
- Syntax for `IF NOT EXISTS` on indexes
- Connection string format
