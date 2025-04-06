package executor

import (
	"database/sql"
	"fmt"

	"github.com/arthurdotwork/mig/internal/config"
	"github.com/arthurdotwork/mig/internal/database"
	"github.com/arthurdotwork/mig/internal/migrations"
)

// Executor handles the execution of migrations
type Executor struct {
	cfg        *config.Config
	db         *sql.DB
	migrations []migrations.Migration
	applied    []database.MigrationVersion
}

// New creates a new migration executor
func New(cfg *config.Config) (*Executor, error) {
	// Connect to the database
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize the migration tables
	if err := database.InitializeTables(db); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	// Load the applied migrations
	applied, err := database.GetAppliedMigrations(db)
	if err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Load migrations from directory
	migrationFiles, err := migrations.LoadMigrations(cfg.Migrations.Directory)
	if err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return &Executor{
		cfg:        cfg,
		db:         db,
		migrations: migrationFiles,
		applied:    applied,
	}, nil
}

// Config returns the configuration
func (e *Executor) Config() *config.Config {
	return e.cfg
}

// Close closes the database connection
func (e *Executor) Close() error {
	return e.db.Close()
}

// GetPendingMigrations returns migrations that have not been applied yet
func (e *Executor) GetPendingMigrations() []migrations.Migration {
	return migrations.GetPendingMigrations(e.migrations, e.applied)
}

// ExecuteMigration executes a single migration
func (e *Executor) ExecuteMigration(migration migrations.Migration) error {
	// Check if the migration uses transactions
	if migration.DisableTx {
		// Execute without a transaction
		if _, err := e.db.Exec(migration.Content); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.ID, err)
		}

		// Record the migration
		if err := database.RecordMigration(e.db, migration.ID, nil); err != nil {
			return err
		}

		// Record the history with the SQL content
		if err := database.RecordHistory(e.db, migration.ID, migration.Content, nil); err != nil {
			return err
		}
	} else {
		// Begin a transaction
		tx, err := e.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.ID, err)
		}

		// Execute the migration
		if _, err := tx.Exec(migration.Content); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("failed to execute migration %s: %w", migration.ID, err)
		}

		// Record the migration
		if err := database.RecordMigration(e.db, migration.ID, tx); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}

		// Record the history with the SQL content
		if err := database.RecordHistory(e.db, migration.ID, migration.Content, tx); err != nil {
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
	pending := e.GetPendingMigrations()
	if len(pending) == 0 {
		return false, nil
	}

	// Execute the first pending migration
	if err := e.ExecuteMigration(pending[0]); err != nil {
		return false, err
	}

	// Refresh the list of applied migrations
	applied, err := database.GetAppliedMigrations(e.db)
	if err != nil {
		return true, err
	}

	e.applied = applied
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

// Status returns the status of migrations
func (e *Executor) Status() ([]migrations.Migration, []database.MigrationVersion, error) {
	// Refresh the list of applied migrations to ensure it's up to date
	applied, err := database.GetAppliedMigrations(e.db)
	if err != nil {
		return nil, nil, err
	}

	e.applied = applied
	return e.migrations, e.applied, nil
}
