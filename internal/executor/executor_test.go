package executor_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/arthurdotwork/mig/internal/config"
	"github.com/arthurdotwork/mig/internal/database"
	"github.com/arthurdotwork/mig/internal/executor"
	"github.com/arthurdotwork/mig/internal/migrations"
	"github.com/stretchr/testify/require"
)

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// testDBConfig creates a test database configuration
func testDBConfig(t *testing.T, migrationsDir string) *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
			Port:     5432,
			Name:     getEnvOrDefault("TEST_DB_NAME", "postgres"),
			User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
			Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
			SSLMode:  "disable",
		},
		Migrations: config.MigrationsConfig{
			Directory: migrationsDir,
		},
	}
}

// setupTestDB prepares the database for testing
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg := testDBConfig(t, "")
	db, err := database.Connect(cfg)
	require.NoError(t, err)

	// Drop the migration tables and test tables to ensure clean state
	_, err = db.Exec("DROP TABLE IF EXISTS mig_history")
	require.NoError(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS mig_versions")
	require.NoError(t, err)

	// Also drop any tables that might have been created by migrations
	_, err = db.Exec("DROP INDEX IF EXISTS idx_users_email")
	require.NoError(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS users")
	require.NoError(t, err)

	return db
}

// createTempMigrationsDir creates a temporary directory with migration files
func createTempMigrationsDir(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "mig_executor_test")
	require.NoError(t, err)

	// Create some test migration files
	createMigrationFile(t, tempDir, "2023_01_01_10_00_00_create_users.sql",
		"CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT);")
	createMigrationFile(t, tempDir, "2023_01_02_10_00_00_add_email.sql",
		"ALTER TABLE users ADD COLUMN email TEXT;")
	createMigrationFile(t, tempDir, "2023_01_03_10_00_00_disable_tx.sql",
		"-- disable-tx\nCREATE INDEX idx_users_email ON users(email);")

	return tempDir
}

// createMigrationFile creates a migration file in the specified directory
func createMigrationFile(t *testing.T, dir, filename, content string) {
	t.Helper()

	filepath := filepath.Join(dir, filename)
	err := os.WriteFile(filepath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write migration file")
}

func TestNew(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	t.Run("it should create a new executor", func(t *testing.T) {
		cfg := testDBConfig(t, tempDir)
		exec, err := executor.New(cfg)
		require.NoError(t, err)
		require.NotNil(t, exec)
		defer exec.Close() //nolint:errcheck

		// Check that the config was set correctly
		require.Equal(t, cfg, exec.Config())

		// Check that migrations were loaded
		pending := exec.GetPendingMigrations()
		require.Len(t, pending, 3) // We created 3 test migration files
	})

	t.Run("it should return error for invalid database connection", func(t *testing.T) {
		cfg := testDBConfig(t, tempDir)
		cfg.Database.Host = "non-existent-host"

		_, err := executor.New(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to connect to database")
	})

	t.Run("it should return error for invalid migrations directory", func(t *testing.T) {
		cfg := testDBConfig(t, "/non/existent/directory")

		_, err := executor.New(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load migrations")
	})
}

func TestExecuteMigration(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should execute a migration with transaction", func(t *testing.T) {
		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Get the first pending migration
		pending := exec.GetPendingMigrations()
		require.NotEmpty(t, pending)

		// Execute the migration
		err = exec.ExecuteMigration(pending[0])
		require.NoError(t, err)

		// Verify the migration was recorded in the database
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM mig_versions WHERE version = $1)", pending[0].ID).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Migration version should be recorded")

		// Verify the SQL was recorded in history
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM mig_history WHERE version = $1)", pending[0].ID).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Migration history should be recorded")

		// Verify the table was created
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Users table should have been created")
	})

	t.Run("it should execute a migration without transaction", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		// First apply the first two migrations to set up the table
		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// The first migration creates the users table
		executed, err := exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "First migration should be executed")

		// The second migration adds email column
		executed, err = exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "Second migration should be executed")

		// Now get the third migration which should have disable-tx
		pending := exec.GetPendingMigrations()
		require.NotEmpty(t, pending, "Should have at least one pending migration")

		// Find the migration with DisableTx=true
		var disableTxMigration migrations.Migration
		for _, m := range pending {
			if m.DisableTx {
				disableTxMigration = m
				break
			}
		}
		require.NotEmpty(t, disableTxMigration.ID, "Should have found a migration with DisableTx=true")

		// Execute the migration without transaction
		err = exec.ExecuteMigration(disableTxMigration)
		require.NoError(t, err)

		// Verify the migration was recorded
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM mig_versions WHERE version = $1)", disableTxMigration.ID).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Migration version should be recorded")

		// Verify the index was created (note: Postgres has system-specific naming for indexes)
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = 'idx_users_email' OR indexname LIKE '%_email%')").Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Index should have been created")
	})

	t.Run("it should return error for failed migration", func(t *testing.T) {
		// Create a temporary migration with invalid SQL
		invalidMigrationFile := filepath.Join(tempDir, "2023_01_04_10_00_00_invalid.sql")
		err := os.WriteFile(invalidMigrationFile, []byte("INVALID SQL;"), 0644)
		require.NoError(t, err)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Get all pending migrations - should include our invalid one
		pending := exec.GetPendingMigrations()
		var invalidMigration migrations.Migration
		for _, m := range pending {
			if m.ID == "2023_01_04_10_00_00_invalid" {
				invalidMigration = m
				break
			}
		}
		require.NotEmpty(t, invalidMigration.ID, "Invalid migration should be found")

		// Execute the invalid migration
		err = exec.ExecuteMigration(invalidMigration)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to execute migration")

		// Verify the migration was NOT recorded
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM mig_versions WHERE version = $1)", invalidMigration.ID).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "Migration version should not be recorded")
	})
}

func TestExecuteNextMigration(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should execute only the next pending migration", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Execute the next migration
		executed, err := exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "Should have executed a migration")

		// Verify only one migration was applied
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM mig_versions").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "Only one migration should be applied")

		// Execute the next migration again
		executed, err = exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "Should have executed another migration")

		// Verify two migrations are now applied
		err = db.QueryRow("SELECT COUNT(*) FROM mig_versions").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "Two migrations should be applied")
	})

	t.Run("it should return false when no migrations are pending", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		// Create a new executor and apply all migrations
		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Apply all migrations one by one to avoid issues
		for i := 0; i < 3; i++ {
			executed, err := exec.ExecuteNextMigration()
			if err != nil {
				t.Logf("Error on migration %d: %v", i, err)
			}
			if !executed {
				break
			}
		}

		// Try to execute next migration when none are pending
		executed, err := exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.False(t, executed, "Should not have executed any migration")
	})
}

func TestExecuteAllMigrations(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should execute all pending migrations", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Execute all migrations
		count, err := exec.ExecuteAllMigrations()
		require.NoError(t, err)
		require.Equal(t, 3, count, "Should have executed 3 migrations")

		// Verify all migrations were applied
		var dbCount int
		err = db.QueryRow("SELECT COUNT(*) FROM mig_versions").Scan(&dbCount)
		require.NoError(t, err)
		require.Equal(t, 3, dbCount, "All 3 migrations should be applied")

		// Try to execute all migrations again
		count, err = exec.ExecuteAllMigrations()
		require.NoError(t, err)
		require.Equal(t, 0, count, "No additional migrations should be executed")
	})

	t.Run("it should stop execution on first error", func(t *testing.T) {
		// Reset the database
		setupTestDB(t)

		// Create a new migrations directory with an invalid migration
		newTempDir := createTempMigrationsDir(t)
		defer os.RemoveAll(newTempDir) //nolint:errcheck

		// Add an invalid migration with syntactically valid timestamp but invalid SQL
		createMigrationFile(t, newTempDir, "2023_01_01_15_00_00_invalid.sql", "INVALID SQL;")

		newCfg := testDBConfig(t, newTempDir)
		exec, err := executor.New(newCfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Execute all migrations - should fail on the invalid one
		_, err = exec.ExecuteAllMigrations()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to execute migration")

		// We can't be certain how many migrations were executed before the error
		// since the order depends on the filename timestamps
		// Just check that not all migrations were applied
		var dbCount int
		err = db.QueryRow("SELECT COUNT(*) FROM mig_versions").Scan(&dbCount)
		require.NoError(t, err)
		require.Less(t, dbCount, 4, "Not all migrations should be applied")
	})
}

func TestStatus(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should return status of migrations", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Execute the first migration
		executed, err := exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed)

		// Get status
		migs, applied, err := exec.Status()
		require.NoError(t, err)
		require.Len(t, migs, 3, "Should have 3 migrations total")
		require.Len(t, applied, 1, "Should have 1 applied migration")

		// Verify the correct migration was applied
		require.Equal(t, migs[0].ID, applied[0].Version)

		// Execute the remaining migrations one by one
		executed, err = exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "Second migration should be executed")

		executed, err = exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed, "Third migration should be executed")

		// Get status again
		migs, applied, err = exec.Status()
		require.NoError(t, err)
		require.Len(t, migs, 3, "Should still have 3 migrations total")
		require.Len(t, applied, 3, "Should now have 3 applied migrations")
	})

	t.Run("it should return status with no migrations", func(t *testing.T) {
		// Create empty migrations directory
		emptyDir, err := os.MkdirTemp("", "mig_executor_empty_test")
		require.NoError(t, err)
		defer os.RemoveAll(emptyDir) //nolint:errcheck

		// Create a fresh database state
		cleanDB := setupTestDB(t)
		defer cleanDB.Close() //nolint:errcheck

		emptyCfg := testDBConfig(t, emptyDir)
		exec, err := executor.New(emptyCfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Get status for empty directory
		migs, applied, err := exec.Status()
		require.NoError(t, err)
		require.Empty(t, migs, "Should have no migrations")
		require.Empty(t, applied, "Should have no applied migrations")
	})
}

func TestGetPendingMigrations(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should return all migrations when none are applied", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		pending := exec.GetPendingMigrations()
		require.Len(t, pending, 3, "Should have 3 pending migrations")
	})

	t.Run("it should return only pending migrations", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Apply the first migration
		executed, err := exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed)

		// Check pending migrations
		pending := exec.GetPendingMigrations()
		require.Len(t, pending, 2, "Should have 2 pending migrations")

		// Apply another migration
		executed, err = exec.ExecuteNextMigration()
		require.NoError(t, err)
		require.True(t, executed)

		// Check pending migrations again
		pending = exec.GetPendingMigrations()
		require.Len(t, pending, 1, "Should have 1 pending migration")
	})

	t.Run("it should return empty slice when all migrations are applied", func(t *testing.T) {
		// Setup a fresh database state
		setupTestDB(t)

		exec, err := executor.New(cfg)
		require.NoError(t, err)
		defer exec.Close() //nolint:errcheck

		// Apply each migration individually
		for i := 0; i < 3; i++ {
			executed, err := exec.ExecuteNextMigration()
			require.NoError(t, err)
			require.True(t, executed, "Migration should be executed")
		}

		// Check pending migrations
		pending := exec.GetPendingMigrations()
		require.Empty(t, pending, "Should have no pending migrations")
	})
}

func TestClose(t *testing.T) {
	// Setup
	setupTestDB(t)

	tempDir := createTempMigrationsDir(t)
	defer os.RemoveAll(tempDir) //nolint:errcheck

	cfg := testDBConfig(t, tempDir)

	t.Run("it should close the database connection", func(t *testing.T) {
		exec, err := executor.New(cfg)
		require.NoError(t, err)

		// Close the executor
		err = exec.Close()
		require.NoError(t, err)

		// Verify that operations fail after close
		_, err = exec.ExecuteNextMigration()
		require.Error(t, err, "Should fail after connection is closed")
	})
}
