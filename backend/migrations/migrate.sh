#!/bin/bash
# ClaraTeach Database Migration Script
# Usage: ./migrate.sh [up|down|status] [DATABASE_URL]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ACTION="${1:-up}"
DATABASE_URL="${2:-$DATABASE_URL}"

if [ -z "$DATABASE_URL" ]; then
    echo "Error: DATABASE_URL is required"
    echo "Usage: ./migrate.sh [up|down|status] [DATABASE_URL]"
    echo "Or set DATABASE_URL environment variable"
    exit 1
fi

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo "Error: psql is not installed"
    exit 1
fi

run_sql() {
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 "$@"
}

run_sql_quiet() {
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -t -A "$@"
}

get_current_version() {
    # Check if schema_migrations table exists
    local table_exists=$(run_sql_quiet -c "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations');" 2>/dev/null || echo "f")
    if [ "$table_exists" = "t" ]; then
        run_sql_quiet -c "SELECT COALESCE(MAX(version), 0) FROM schema_migrations;" 2>/dev/null || echo "0"
    else
        echo "0"
    fi
}

case "$ACTION" in
    up)
        echo "Running migrations..."
        current_version=$(get_current_version)
        echo "Current schema version: $current_version"

        # Find and run all up migrations
        for migration in "$SCRIPT_DIR"/*.up.sql; do
            if [ -f "$migration" ]; then
                # Extract version number from filename (e.g., 001_initial_schema.up.sql -> 1)
                filename=$(basename "$migration")
                version=$(echo "$filename" | sed 's/^0*//' | cut -d'_' -f1)

                if [ "$version" -gt "$current_version" ]; then
                    echo "Applying migration $filename..."
                    run_sql -f "$migration"
                    echo "Migration $filename applied successfully"
                else
                    echo "Skipping $filename (already applied)"
                fi
            fi
        done

        echo "Migrations complete!"
        ;;

    down)
        echo "Rolling back last migration..."
        current_version=$(get_current_version)

        if [ "$current_version" -eq "0" ]; then
            echo "No migrations to roll back"
            exit 0
        fi

        # Find the down migration for current version
        down_file=$(printf "$SCRIPT_DIR/%03d_*.down.sql" "$current_version")
        down_file=$(ls $down_file 2>/dev/null | head -1)

        if [ -f "$down_file" ]; then
            echo "Rolling back $(basename "$down_file")..."
            run_sql -f "$down_file"
            echo "Rollback complete"
        else
            echo "Error: Down migration not found for version $current_version"
            exit 1
        fi
        ;;

    status)
        echo "Migration status:"
        current_version=$(get_current_version)
        echo "Current schema version: $current_version"
        echo ""
        echo "Available migrations:"
        for migration in "$SCRIPT_DIR"/*.up.sql; do
            if [ -f "$migration" ]; then
                filename=$(basename "$migration")
                version=$(echo "$filename" | sed 's/^0*//' | cut -d'_' -f1)
                if [ "$version" -le "$current_version" ]; then
                    echo "  [x] $filename"
                else
                    echo "  [ ] $filename"
                fi
            fi
        done
        ;;

    *)
        echo "Unknown action: $ACTION"
        echo "Usage: ./migrate.sh [up|down|status] [DATABASE_URL]"
        exit 1
        ;;
esac
