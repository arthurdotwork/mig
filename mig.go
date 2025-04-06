package mig

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	// Version is the version of the migrator
	Version = "0.1.0"

	// DefaultConfigFilename is the default name of the configuration file
	DefaultConfigFilename = "mig.yaml"

	// DefaultMigrationsDir is the default name of the migrations directory
	DefaultMigrationsDir = "migrations"
)

// Migration represents a single migration file
type Migration struct {
	ID        string    // Unique identifier (filename without extension)
	Filename  string    // Full filename
	Content   string    // SQL content
	DisableTx bool      // Whether to disable transactions
	CreatedAt time.Time // Creation time based on the filename
}

// MigrationVersion represents a record in the mig_versions table
type MigrationVersion struct {
	ID        int       `db:"id"`
	Version   string    `db:"version"`
	AppliedAt time.Time `db:"applied_at"`
}

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

// Connect establishes a connection to the PostgreSQL database
func Connect(config *Config) (*sql.DB, error) {
	// Construct the connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		config.Database.Host,
		config.Database.Port,
		config.Database.Name,
		config.Database.User,
		config.Database.Password,
		config.Database.SSLMode,
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

// Migration filename pattern: YYYY_MM_DD_HH_MM_SS_name.sql
var migrationPattern = regexp.MustCompile(`^(\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2})_([a-zA-Z0-9_]+)\.sql$`)

// LoadMigrations loads all migration files from the specified directory
func LoadMigrations(directory string) ([]Migration, error) {
	// Check if the directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory does not exist: %s", directory)
	}

	// List all .sql files in the directory
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration

	// Process each file
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Check if the filename matches the pattern
		matches := migrationPattern.FindStringSubmatch(file.Name())
		if matches == nil {
			// Skip files that don't match the pattern
			continue
		}

		// Extract the date and name
		dateStr := matches[1]
		name := matches[2]

		// Parse the date
		createdAt, err := time.Parse("2006_01_02_15_04_05", dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date format in migration filename %s: %w", file.Name(), err)
		}

		// Read the file content
		content, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
		}

		// Check for metadata
		disableTx := false
		if strings.Contains(string(content), "-- disable-tx") {
			disableTx = true
		}

		// Create the migration
		migration := Migration{
			ID:        fmt.Sprintf("%s_%s", dateStr, name),
			Filename:  file.Name(),
			Content:   string(content),
			DisableTx: disableTx,
			CreatedAt: createdAt,
		}

		migrations = append(migrations, migration)
	}

	// Sort migrations by date (and then by name for same date)
	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].CreatedAt.Equal(migrations[j].CreatedAt) {
			return migrations[i].ID < migrations[j].ID
		}
		return migrations[i].CreatedAt.Before(migrations[j].CreatedAt)
	})

	return migrations, nil
}

// CreateMigrationFile creates a new migration file
func CreateMigrationFile(directory, name string) (string, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Format the current date with time
	dateStr := time.Now().Format("2006_01_02_15_04_05")

	// Sanitize the name (replace spaces with underscores, remove special characters)
	sanitizedName := regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(strings.ReplaceAll(name, " ", "_"), "")

	// Generate the filename
	filename := fmt.Sprintf("%s_%s.sql", dateStr, sanitizedName)
	filepath := filepath.Join(directory, filename)

	// Check if the file already exists
	if _, err := os.Stat(filepath); err == nil {
		return "", fmt.Errorf("migration file already exists: %s", filename)
	}

	// Create the file with a template
	template := fmt.Sprintf(`-- Migration: %s
-- Created at: %s
-- 
-- Note: 
-- Add "-- disable-tx" anywhere in this file to disable transaction wrapping.

-- Your SQL goes here
`, sanitizedName, time.Now().Format("2006-01-02 15:04:05"))

	if err := os.WriteFile(filepath, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("failed to write migration file: %w", err)
	}

	return filename, nil
}

// GetPendingMigrations returns migrations that have not been applied yet
func GetPendingMigrations(allMigrations []Migration, appliedMigrations []MigrationVersion) []Migration {
	// Create a map of applied migrations for quick lookup
	appliedMap := make(map[string]bool)
	for _, m := range appliedMigrations {
		appliedMap[m.Version] = true
	}

	// Filter out migrations that have already been applied
	var pendingMigrations []Migration
	for _, m := range allMigrations {
		if !appliedMap[m.ID] {
			pendingMigrations = append(pendingMigrations, m)
		}
	}

	return pendingMigrations
}
