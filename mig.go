package mig

import (
	"fmt"
	"os"

	"github.com/arthurdotwork/mig/internal/config"
	"github.com/arthurdotwork/mig/internal/executor"
	"github.com/arthurdotwork/mig/internal/migrations"
)

const (
	// Version is the version of the migrator
	Version = "0.1.0"

	// DefaultConfigFilename is the default name of the configuration file
	DefaultConfigFilename = "mig.yaml"

	// DefaultMigrationsDir is the default name of the migrations directory
	DefaultMigrationsDir = config.DefaultMigrationsDir
)

// Migrator is the main struct for migration management
type Migrator struct {
	executor *executor.Executor
}

// MigrationStatus represents a migration's current status
type MigrationStatus struct {
	ID        string // Migration ID
	Name      string // Migration Name
	Filename  string // Migration Filename
	Applied   bool   // Whether the migration has been applied
	AppliedAt string // When the migration was applied (empty if not applied)
}

// New creates a new Migrator instance
func New(configPath string) (*Migrator, error) {
	// Load the configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	// Create the executor
	exec, err := executor.New(cfg)
	if err != nil {
		return nil, err
	}

	return &Migrator{
		executor: exec,
	}, nil
}

// Initialize sets up the migration environment
func Initialize(configPath, migrationsDir string) error {
	// Create the config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := config.CreateDefault(configPath); err != nil {
			return err
		}
		fmt.Printf("Created configuration file: %s\n", configPath)
	}

	// Create the migrations directory if it doesn't exist
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}
		fmt.Printf("Created migrations directory: %s\n", migrationsDir)

		// Create a sample migration
		filename, err := migrations.CreateMigrationFile(migrationsDir, "init")
		if err != nil {
			return err
		}
		fmt.Printf("Created sample migration: %s\n", filename)
	}

	return nil
}

// CreateMigration creates a new migration file
func (m *Migrator) CreateMigration(name string) (string, error) {
	return migrations.CreateMigrationFile(m.executor.Config().Migrations.Directory, name)
}

// MigrateUp applies the next pending migration
func (m *Migrator) MigrateUp() (bool, error) {
	return m.executor.ExecuteNextMigration()
}

// MigrateUpAll applies all pending migrations
func (m *Migrator) MigrateUpAll() (int, error) {
	return m.executor.ExecuteAllMigrations()
}

// Status returns the status of migrations
func (m *Migrator) Status() ([]MigrationStatus, error) {
	migrations, applied, err := m.executor.Status()
	if err != nil {
		return nil, err
	}

	// Create a map of applied migrations for quick lookup
	appliedMap := make(map[string]string)
	for _, m := range applied {
		appliedMap[m.Version] = m.AppliedAt.Format("2006-01-02 15:04:05")
	}

	// Convert to MigrationStatus
	statuses := make([]MigrationStatus, len(migrations))
	for i, m := range migrations {
		appliedAt, isApplied := appliedMap[m.ID]
		statuses[i] = MigrationStatus{
			ID:        m.ID,
			Name:      m.Name,
			Filename:  m.Filename,
			Applied:   isApplied,
			AppliedAt: appliedAt,
		}
	}

	return statuses, nil
}

// Close closes the database connection
func (m *Migrator) Close() error {
	return m.executor.Close()
}
