package mig

import (
	"database/sql"
	"fmt"
	"os"
)

// Manager handles the migration operations
type Manager struct {
	Config   *Config
	DB       *sql.DB
	Executor *Executor
}

// NewManager creates a new migration manager
func NewManager(cfg *Config) (*Manager, error) {
	// Connect to the database
	database, err := Connect(cfg)
	if err != nil {
		return nil, err
	}

	// Create the executor
	executor, err := NewExecutor(database)
	if err != nil {
		database.Close() //nolint:errcheck
		return nil, err
	}

	// Load migrations
	if err := executor.LoadMigrationsFromDir(cfg.Migrations.Directory); err != nil {
		database.Close() //nolint:errcheck
		return nil, err
	}

	return &Manager{
		Config:   cfg,
		DB:       database,
		Executor: executor,
	}, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	return m.DB.Close()
}

// Initialize initializes the migration environment
func Initialize(configPath, migrationsDir string) error {
	// Create the config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := CreateDefaultConfig(configPath); err != nil {
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
		filename, err := CreateMigrationFile(migrationsDir, "init")
		if err != nil {
			return err
		}
		fmt.Printf("Created sample migration: %s\n", filename)
	}

	return nil
}

// CreateMigration creates a new migration file
func (m *Manager) CreateMigration(name string) (string, error) {
	filename, err := CreateMigrationFile(m.Config.Migrations.Directory, name)
	if err != nil {
		return "", err
	}

	return filename, nil
}

// MigrateUp applies the next pending migration
func (m *Manager) MigrateUp() (bool, error) {
	return m.Executor.ExecuteNextMigration()
}

// MigrateUpAll applies all pending migrations
func (m *Manager) MigrateUpAll() (int, error) {
	return m.Executor.ExecuteAllMigrations()
}

// Status returns the status of migrations
func (m *Manager) Status() ([]Migration, []MigrationVersion, error) {
	return m.Executor.Migrations, m.Executor.Applied, nil
}
