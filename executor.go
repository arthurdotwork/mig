package mig

import (
	"database/sql"
	"fmt"
)

// Executor handles the execution of migrations
type Executor struct {
	DB         *sql.DB
	Migrations []Migration
	Applied    []MigrationVersion
}

// NewExecutor creates a new migration executor
func NewExecutor(database *sql.DB) (*Executor, error) {
	// Initialize the migration tables
	if err := InitializeTables(database); err != nil {
		return nil, err
	}

	// Load the applied migrations
	applied, err := GetAppliedMigrations(database)
	if err != nil {
		return nil, err
	}

	return &Executor{
		DB:      database,
		Applied: applied,
	}, nil
}

// LoadMigrationsFromDir loads migrations from the specified directory
func (e *Executor) LoadMigrationsFromDir(directory string) error {
	migrations, err := LoadMigrations(directory)
	if err != nil {
		return err
	}

	e.Migrations = migrations
	return nil
}

// GetPendingMigrationsExecutor returns migrations that have not been applied yet
func (e *Executor) GetPendingMigrationsExecutor() []Migration {
	return GetPendingMigrations(e.Migrations, e.Applied)
}

// ExecuteMigration executes a single migration
func (e *Executor) ExecuteMigration(migration Migration) error {
	// Check if the migration uses transactions
	if migration.DisableTx {
		// Execute without a transaction
		if _, err := e.DB.Exec(migration.Content); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.ID, err)
		}

		// Record the migration
		if err := RecordMigration(e.DB, migration.ID, nil); err != nil {
			return err
		}

		// Record the history with the SQL content
		if err := RecordHistory(e.DB, migration.ID, migration.Content, nil); err != nil {
			return err
		}
	} else {
		// Begin a transaction
		tx, err := e.DB.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.ID, err)
		}

		// Execute the migration
		if _, err := tx.Exec(migration.Content); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("failed to execute migration %s: %w", migration.ID, err)
		}

		// Record the migration
		if err := RecordMigration(e.DB, migration.ID, tx); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}

		// Record the history with the SQL content
		if err := RecordHistory(e.DB, migration.ID, migration.Content, tx); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction for migration %s: %w", migration.ID, err)
		}
	}

	return nil
}

// ExecuteNextMigration executes the next pending migration
func (e *Executor) ExecuteNextMigration() (bool, error) {
	pending := e.GetPendingMigrationsExecutor()
	if len(pending) == 0 {
		return false, nil
	}

	// Execute the first pending migration
	if err := e.ExecuteMigration(pending[0]); err != nil {
		return false, err
	}

	// Refresh the list of applied migrations
	applied, err := GetAppliedMigrations(e.DB)
	if err != nil {
		return true, err
	}

	e.Applied = applied
	return true, nil
}

// ExecuteAllMigrations executes all pending migrations
func (e *Executor) ExecuteAllMigrations() (int, error) {
	count := 0
	for {
		executed, err := e.ExecuteNextMigration()
		if err != nil {
			return count, err
		}

		if !executed {
			break
		}

		count++
	}

	return count, nil
}
