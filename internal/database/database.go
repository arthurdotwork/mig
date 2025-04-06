package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/arthurdotwork/mig/internal/config"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Constants for the SQL statements to create the migration tables
const (
	CreateVersionTableSQL = `
	CREATE TABLE IF NOT EXISTS mig_versions (
		id SERIAL PRIMARY KEY,
		version VARCHAR(255) NOT NULL UNIQUE,
		applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`

	CreateHistoryTableSQL = `
	CREATE TABLE IF NOT EXISTS mig_history (
		id SERIAL PRIMARY KEY,
		version VARCHAR(255) NOT NULL,
		command TEXT NOT NULL,
		executed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`
)

// MigrationVersion represents a record in the mig_versions table
type MigrationVersion struct {
	ID        int       // Database ID
	Version   string    // Migration version (same as Migration.ID)
	AppliedAt time.Time // When the migration was applied
}

// Connect establishes a connection to the PostgreSQL database
func Connect(cfg *config.Config) (*sql.DB, error) {
	// Construct the connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.SSLMode,
	)

	// Open the database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitializeTables creates the necessary migration tables if they don't exist
func InitializeTables(db *sql.DB) error {
	// Create the mig_versions table
	if _, err := db.Exec(CreateVersionTableSQL); err != nil {
		return fmt.Errorf("failed to create mig_versions table: %w", err)
	}

	// Create the mig_history table
	if _, err := db.Exec(CreateHistoryTableSQL); err != nil {
		return fmt.Errorf("failed to create mig_history table: %w", err)
	}

	return nil
}

// GetAppliedMigrations retrieves all applied migrations
func GetAppliedMigrations(db *sql.DB) ([]MigrationVersion, error) {
	rows, err := db.Query("SELECT id, version, applied_at FROM mig_versions ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var migrations []MigrationVersion
	for rows.Next() {
		var m MigrationVersion
		if err := rows.Scan(&m.ID, &m.Version, &m.AppliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", err)
		}
		migrations = append(migrations, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over migrations: %w", err)
	}

	return migrations, nil
}

// RecordMigration records a successfully applied migration
func RecordMigration(db *sql.DB, version string, tx *sql.Tx) error {
	query := "INSERT INTO mig_versions (version) VALUES ($1)"

	var err error
	if tx != nil {
		_, err = tx.Exec(query, version)
	} else {
		_, err = db.Exec(query, version)
	}

	if err != nil {
		return fmt.Errorf("failed to record migration version: %w", err)
	}

	return nil
}

// RecordHistory records an entry in the migration history with the SQL content
func RecordHistory(db *sql.DB, version string, sqlContent string, tx *sql.Tx) error {
	query := "INSERT INTO mig_history (version, command) VALUES ($1, $2)"

	var err error
	if tx != nil {
		_, err = tx.Exec(query, version, sqlContent)
	} else {
		_, err = db.Exec(query, version, sqlContent)
	}

	if err != nil {
		return fmt.Errorf("failed to record migration history: %w", err)
	}

	return nil
}
