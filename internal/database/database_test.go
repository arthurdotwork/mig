package database_test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/arthurdotwork/mig/internal/config"
	"github.com/arthurdotwork/mig/internal/database"
	"github.com/stretchr/testify/require"
)

// Test database connection parameters
var testDBConfig = &config.Config{
	Database: config.DatabaseConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     5432,
		Name:     getEnvOrDefault("TEST_DB_NAME", "postgres"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
		SSLMode:  "disable",
	},
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// setupTest prepares the database for testing
func setupTest(t *testing.T) *sql.DB {
	db, err := database.Connect(testDBConfig)
	require.NoError(t, err)

	// Drop the tables if they exist to ensure clean state
	_, err = db.Exec("DROP TABLE IF EXISTS mig_history")
	require.NoError(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS mig_versions")
	require.NoError(t, err)

	return db
}

func TestConnect(t *testing.T) {
	t.Run("it should connect to a valid database", func(t *testing.T) {
		db, err := database.Connect(testDBConfig)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer db.Close() //nolint:errcheck

		err = db.Ping()
		require.NoError(t, err)
	})

	t.Run("it should return error for invalid credentials", func(t *testing.T) {
		invalidConfig := &config.Config{
			Database: config.DatabaseConfig{
				Host:     testDBConfig.Database.Host,
				Port:     testDBConfig.Database.Port,
				Name:     testDBConfig.Database.Name,
				User:     "invalid_user",
				Password: "invalid_password",
				SSLMode:  "disable",
			},
		}

		db, err := database.Connect(invalidConfig)
		require.Error(t, err)
		require.Nil(t, db)
	})
}

func TestInitializeTables(t *testing.T) {
	db := setupTest(t)
	defer db.Close() //nolint:errcheck

	t.Run("it should create migration tables if they don't exist", func(t *testing.T) {
		err := database.InitializeTables(db)
		require.NoError(t, err)

		// Verify tables were created
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mig_versions'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mig_history'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("it should not fail if tables already exist", func(t *testing.T) {
		// First initialization should already be done
		err := database.InitializeTables(db)
		require.NoError(t, err)
	})
}

func TestGetAppliedMigrations(t *testing.T) {
	db := setupTest(t)
	defer db.Close() //nolint:errcheck

	// Initialize tables for the test
	err := database.InitializeTables(db)
	require.NoError(t, err)

	t.Run("it should return empty slice when no migrations are applied", func(t *testing.T) {
		migrations, err := database.GetAppliedMigrations(db)
		require.NoError(t, err)
		require.Empty(t, migrations)
	})

	t.Run("it should return applied migrations in correct order", func(t *testing.T) {
		// Insert test migrations
		_, err := db.Exec("INSERT INTO mig_versions (version, applied_at) VALUES ('001', $1)", time.Now().Add(-2*time.Hour))
		require.NoError(t, err)

		_, err = db.Exec("INSERT INTO mig_versions (version, applied_at) VALUES ('002', $1)", time.Now().Add(-1*time.Hour))
		require.NoError(t, err)

		migrations, err := database.GetAppliedMigrations(db)
		require.NoError(t, err)
		require.Len(t, migrations, 2)
		require.Equal(t, "001", migrations[0].Version)
		require.Equal(t, "002", migrations[1].Version)
	})
}

func TestRecordMigration(t *testing.T) {
	db := setupTest(t)
	defer db.Close() //nolint:errcheck

	// Initialize tables for the test
	err := database.InitializeTables(db)
	require.NoError(t, err)

	t.Run("it should record migration without transaction", func(t *testing.T) {
		err := database.RecordMigration(db, "001", nil)
		require.NoError(t, err)

		// Verify the migration was recorded
		var version string
		err = db.QueryRow("SELECT version FROM mig_versions WHERE version = '001'").Scan(&version)
		require.NoError(t, err)
		require.Equal(t, "001", version)
	})

	t.Run("it should record migration with transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		err = database.RecordMigration(db, "002", tx)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify the migration was recorded
		var version string
		err = db.QueryRow("SELECT version FROM mig_versions WHERE version = '002'").Scan(&version)
		require.NoError(t, err)
		require.Equal(t, "002", version)
	})

	t.Run("it should rollback migration recording on transaction rollback", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		err = database.RecordMigration(db, "003", tx)
		require.NoError(t, err)

		err = tx.Rollback()
		require.NoError(t, err)

		// Verify the migration was not recorded
		var version string
		err = db.QueryRow("SELECT version FROM mig_versions WHERE version = '003'").Scan(&version)
		require.Error(t, err)
	})
}

func TestRecordHistory(t *testing.T) {
	db := setupTest(t)
	defer db.Close() //nolint:errcheck

	// Initialize tables for the test
	err := database.InitializeTables(db)
	require.NoError(t, err)

	t.Run("it should record migration history without transaction", func(t *testing.T) {
		err := database.RecordHistory(db, "001", "CREATE TABLE test (id INT)", nil)
		require.NoError(t, err)

		// Verify the history was recorded
		var version, command string
		err = db.QueryRow("SELECT version, command FROM mig_history WHERE version = '001'").Scan(&version, &command)
		require.NoError(t, err)
		require.Equal(t, "001", version)
		require.Equal(t, "CREATE TABLE test (id INT)", command)
	})

	t.Run("it should record migration history with transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		err = database.RecordHistory(db, "002", "ALTER TABLE test ADD COLUMN name TEXT", tx)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify the history was recorded
		var version, command string
		err = db.QueryRow("SELECT version, command FROM mig_history WHERE version = '002'").Scan(&version, &command)
		require.NoError(t, err)
		require.Equal(t, "002", version)
		require.Equal(t, "ALTER TABLE test ADD COLUMN name TEXT", command)
	})

	t.Run("it should rollback history recording on transaction rollback", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		err = database.RecordHistory(db, "003", "DROP TABLE test", tx)
		require.NoError(t, err)

		err = tx.Rollback()
		require.NoError(t, err)

		// Verify the history was not recorded
		var version, command string
		err = db.QueryRow("SELECT version, command FROM mig_history WHERE version = '003'").Scan(&version, &command)
		require.Error(t, err, "Query should fail because history should not exist after rollback")
	})
}
