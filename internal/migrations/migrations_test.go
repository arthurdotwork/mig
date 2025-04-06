package migrations_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arthurdotwork/mig/internal/database"
	"github.com/arthurdotwork/mig/internal/migrations"
	"github.com/stretchr/testify/require"
)

func createTempDir(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "mig_migrations_test")
	require.NoError(t, err, "Failed to create temp directory")

	return tempDir
}

func createMigrationFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	filepath := filepath.Join(dir, filename)
	err := os.WriteFile(filepath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write migration file")

	return filepath
}

// createUnreadableMigrationFile creates a migration file but makes it unreadable
func createUnreadableMigrationFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := createMigrationFile(t, dir, filename, content)

	// Make the file unreadable
	err := os.Chmod(path, 0)
	require.NoError(t, err, "Failed to make file unreadable")

	return path
}

func TestLoadMigrations(t *testing.T) {
	t.Parallel()

	t.Run("it should return an error for non-existent directory", func(t *testing.T) {
		_, err := migrations.LoadMigrations("/non/existent/directory")
		require.Error(t, err)
		require.Contains(t, err.Error(), "migrations directory does not exist")
	})

	t.Run("it should return empty slice for empty directory", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		migs, err := migrations.LoadMigrations(tempDir)
		require.NoError(t, err)
		require.Empty(t, migs, "Expected empty migrations slice for empty directory")
	})

	t.Run("it should ignore non-sql files", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createMigrationFile(t, tempDir, "2023_01_01_00_00_00_test.txt", "Test content")

		migs, err := migrations.LoadMigrations(tempDir)
		require.NoError(t, err)
		require.Empty(t, migs, "Expected empty migrations slice for non-SQL files")
	})

	t.Run("it should ignore files not matching migration pattern", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createMigrationFile(t, tempDir, "invalid_migration.sql", "SELECT 1;")

		migs, err := migrations.LoadMigrations(tempDir)
		require.NoError(t, err)
		require.Empty(t, migs, "Expected empty migrations slice for invalid filenames")
	})

	t.Run("it should return an error for files with invalid date format", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createMigrationFile(t, tempDir, "2023_13_01_10_00_00_invalid_date.sql", "SELECT 1;")

		_, err := migrations.LoadMigrations(tempDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid date format in migration filename")
	})

	t.Run("it should return an error for unreadable migration files", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping test when running as root, as root can read any file")
		}

		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createUnreadableMigrationFile(t, tempDir, "2023_01_01_10_00_00_unreadable.sql", "SELECT 1;")

		_, err := migrations.LoadMigrations(tempDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read migration file")
	})

	t.Run("it should load valid migrations and sort them correctly", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createMigrationFile(t, tempDir, "2023_01_02_10_00_00_second.sql", "SELECT 2;")
		createMigrationFile(t, tempDir, "2023_01_01_10_00_00_first.sql", "SELECT 1;")
		createMigrationFile(t, tempDir, "2023_01_03_10_00_00_third.sql", "SELECT 3;")
		createMigrationFile(t, tempDir, "2023_01_04_10_00_00_fourth.sql", "-- disable-tx\nSELECT 4;")

		migs, err := migrations.LoadMigrations(tempDir)
		require.NoError(t, err)
		require.Len(t, migs, 4, "Expected 4 migrations")

		require.Equal(t, "2023_01_01_10_00_00_first", migs[0].ID)
		require.Equal(t, "2023_01_02_10_00_00_second", migs[1].ID)
		require.Equal(t, "2023_01_03_10_00_00_third", migs[2].ID)
		require.Equal(t, "2023_01_04_10_00_00_fourth", migs[3].ID)

		require.Equal(t, "first", migs[0].Name)
		require.Equal(t, "2023_01_01_10_00_00_first.sql", migs[0].Filename)
		require.Equal(t, "SELECT 1;", migs[0].Content)
		require.False(t, migs[0].DisableTx)

		require.True(t, migs[3].DisableTx)
	})

	t.Run("it should handle migrations with same timestamp", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		createMigrationFile(t, tempDir, "2023_01_01_10_00_00_beta.sql", "SELECT 2;")
		createMigrationFile(t, tempDir, "2023_01_01_10_00_00_alpha.sql", "SELECT 1;")

		migs, err := migrations.LoadMigrations(tempDir)
		require.NoError(t, err)
		require.Len(t, migs, 2, "Expected 2 migrations")

		require.Equal(t, "2023_01_01_10_00_00_alpha", migs[0].ID)
		require.Equal(t, "2023_01_01_10_00_00_beta", migs[1].ID)
	})
}

func TestCreateMigrationFile(t *testing.T) {
	t.Parallel()

	t.Run("it should create directory if it doesn't exist", func(t *testing.T) {
		baseDir := createTempDir(t)
		defer os.RemoveAll(baseDir) //nolint:errcheck

		migDir := filepath.Join(baseDir, "migrations")

		_, err := os.Stat(migDir)
		require.True(t, os.IsNotExist(err))

		filename, err := migrations.CreateMigrationFile(migDir, "test_migration")
		require.NoError(t, err)
		require.NotEmpty(t, filename)

		_, err = os.Stat(migDir)
		require.NoError(t, err)
	})

	t.Run("it should generate a filename with current timestamp", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		now := time.Now()
		datePrefix := now.Format("2006_01_02")

		filename, err := migrations.CreateMigrationFile(tempDir, "test_migration")
		require.NoError(t, err)
		require.Contains(t, filename, datePrefix, "Filename should contain current date")
		require.Contains(t, filename, "test_migration.sql", "Filename should contain migration name")

		fullPath := filepath.Join(tempDir, filename)
		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)

		require.Contains(t, string(content), "-- Migration: test_migration")
		require.Contains(t, string(content), "-- Your SQL goes here")
	})

	t.Run("it should sanitize migration name", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		filename, err := migrations.CreateMigrationFile(tempDir, "test migration with spaces & special @# chars")
		require.NoError(t, err)

		require.Contains(t, filename, "test_migration_with_spaces__special__chars.sql")
		require.NotContains(t, filename, "&")
		require.NotContains(t, filename, "@#")
	})

	t.Run("it should fail if migration file already exists", func(t *testing.T) {
		tempDir := createTempDir(t)
		defer os.RemoveAll(tempDir) //nolint:errcheck

		_, err := migrations.CreateMigrationFile(tempDir, "test")
		require.NoError(t, err)

		_, err = migrations.CreateMigrationFile(tempDir, "test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "migration file already exists")
	})
}

func TestGetPendingMigrations(t *testing.T) {
	t.Parallel()

	mig1 := migrations.Migration{ID: "2023_01_01_00_00_00_first", Name: "first"}
	mig2 := migrations.Migration{ID: "2023_01_02_00_00_00_second", Name: "second"}
	mig3 := migrations.Migration{ID: "2023_01_03_00_00_00_third", Name: "third"}

	allMigrations := []migrations.Migration{mig1, mig2, mig3}

	t.Run("it should return all migrations when none are applied", func(t *testing.T) {
		var appliedMigrations []database.MigrationVersion

		pending := migrations.GetPendingMigrations(allMigrations, appliedMigrations)
		require.Len(t, pending, 3)
		require.Equal(t, mig1.ID, pending[0].ID)
		require.Equal(t, mig2.ID, pending[1].ID)
		require.Equal(t, mig3.ID, pending[2].ID)
	})

	t.Run("it should return only pending migrations", func(t *testing.T) {
		appliedMigrations := []database.MigrationVersion{
			{Version: mig1.ID},
		}

		pending := migrations.GetPendingMigrations(allMigrations, appliedMigrations)
		require.Len(t, pending, 2)
		require.Equal(t, mig2.ID, pending[0].ID)
		require.Equal(t, mig3.ID, pending[1].ID)

		appliedMigrations = append(appliedMigrations, database.MigrationVersion{Version: mig2.ID})

		pending = migrations.GetPendingMigrations(allMigrations, appliedMigrations)
		require.Len(t, pending, 1)
		require.Equal(t, mig3.ID, pending[0].ID)
	})

	t.Run("it should return empty slice when all migrations are applied", func(t *testing.T) {
		appliedMigrations := []database.MigrationVersion{
			{Version: mig1.ID},
			{Version: mig2.ID},
			{Version: mig3.ID},
		}

		pending := migrations.GetPendingMigrations(allMigrations, appliedMigrations)
		require.Empty(t, pending)
	})

	t.Run("it should handle out-of-order applied migrations", func(t *testing.T) {
		appliedMigrations := []database.MigrationVersion{
			{Version: mig3.ID},
			{Version: mig1.ID},
		}

		pending := migrations.GetPendingMigrations(allMigrations, appliedMigrations)
		require.Len(t, pending, 1)
		require.Equal(t, mig2.ID, pending[0].ID)
	})
}
