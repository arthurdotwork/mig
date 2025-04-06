package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arthurdotwork/mig/internal/database"
)

// Migration represents a single migration file
type Migration struct {
	ID        string    // Unique identifier (filename without extension)
	Name      string    // Name part of the migration
	Filename  string    // Full filename
	Content   string    // SQL content
	DisableTx bool      // Whether to disable transactions
	CreatedAt time.Time // Creation time based on the filename
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
			Name:      name,
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
func GetPendingMigrations(allMigrations []Migration, appliedMigrations []database.MigrationVersion) []Migration {
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
