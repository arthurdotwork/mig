package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/arthurdotwork/mig/internal/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func createTempConfig(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	tempFile, err := os.CreateTemp("", "mig_config_*.yaml")
	require.NoError(t, err, "Failed to create temp file")
	defer tempFile.Close() //nolint:errcheck

	data, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal config to YAML")

	_, err = tempFile.Write(data)
	require.NoError(t, err, "Failed to write to temp file")

	err = tempFile.Close()
	require.NoError(t, err, "Failed to close temp file")

	return tempFile.Name()
}

func TestLoad(t *testing.T) {
	t.Run("it should return an error if the file doesn't exist", func(t *testing.T) {
		_, err := config.Load("nonexistent_file.yaml")
		require.Error(t, err)
	})

	t.Run("it should return an error if it can not parse the config file", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "mig_invalid_config_*.yaml")
		require.NoError(t, err)
		defer tempFile.Close() //nolint:errcheck

		_, err = tempFile.WriteString("invalid_yaml")
		require.NoError(t, err)

		_, err = config.Load(tempFile.Name())
		require.Error(t, err)
	})

	t.Run("it should return an error if the configuration is invalid", func(t *testing.T) {
		configPath := createTempConfig(t, map[string]interface{}{
			"database": map[string]interface{}{
				"port":     5432,
				"password": "test",
				"sslmode":  "disable",
			},
			"migrations": map[string]interface{}{
				"directory": "migrations",
			},
		})

		_, err := config.Load(configPath)
		require.Error(t, err)
	})

	t.Run("it should override values with environment variables", func(t *testing.T) {
		configPath := createTempConfig(t, map[string]interface{}{
			"database": map[string]interface{}{
				"host":     "originalhost",
				"port":     1234,
				"name":     "originaldb",
				"user":     "originaluser",
				"password": "originalpass",
				"sslmode":  "require",
			},
			"migrations": map[string]interface{}{
				"directory": "originalmigrations",
			},
		})

		t.Setenv("DATABASE_HOST", "envhost")
		t.Setenv("DATABASE_PORT", "5678")
		t.Setenv("DATABASE_NAME", "envdb")
		t.Setenv("DATABASE_USER", "envuser")
		t.Setenv("DATABASE_PASSWORD", "envpass")
		t.Setenv("DATABASE_SSLMODE", "disable")

		cfg, err := config.Load(configPath)
		require.NoError(t, err)

		require.Equal(t, "envhost", cfg.Database.Host)
		require.Equal(t, 5678, cfg.Database.Port)
		require.Equal(t, "envdb", cfg.Database.Name)
		require.Equal(t, "envuser", cfg.Database.User)
		require.Equal(t, "envpass", cfg.Database.Password)
		require.Equal(t, "disable", cfg.Database.SSLMode)
	})

	t.Run("it should skip invalid numeric port in environment variable", func(t *testing.T) {
		configPath := createTempConfig(t, map[string]interface{}{
			"database": map[string]interface{}{
				"host":     "host",
				"port":     1234,
				"name":     "db",
				"user":     "user",
				"password": "pass",
				"sslmode":  "disable",
			},
			"migrations": map[string]interface{}{
				"directory": "migrations",
			},
		})

		t.Setenv("DATABASE_PORT", "invalid")

		cfg, err := config.Load(configPath)
		require.NoError(t, err)

		require.Equal(t, 1234, cfg.Database.Port)
	})

	t.Run("it should load a valid config file", func(t *testing.T) {
		configPath := createTempConfig(t, map[string]interface{}{
			"database": map[string]interface{}{
				"host":     "testhost",
				"port":     5432,
				"name":     "testdb",
				"user":     "testuser",
				"password": "testpass",
				"sslmode":  "disable",
			},
			"migrations": map[string]interface{}{
				"directory": "testmigrations",
			},
		})

		cfg, err := config.Load(configPath)
		require.NoError(t, err)

		require.Equal(t, "testhost", cfg.Database.Host)
		require.Equal(t, 5432, cfg.Database.Port)
		require.Equal(t, "testdb", cfg.Database.Name)
		require.Equal(t, "testuser", cfg.Database.User)
		require.Equal(t, "testpass", cfg.Database.Password)
		require.Equal(t, "disable", cfg.Database.SSLMode)
		wd, err := os.Getwd()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s/testmigrations", wd), cfg.Migrations.Directory)
	})
}

func TestCreateDefault(t *testing.T) {
	t.Parallel()

	t.Run("it should return an error if it cannot write to the file", func(t *testing.T) {
		nonExistentDir := "/non/existent/directory/config.yaml"
		err := config.CreateDefault(nonExistentDir)
		require.Error(t, err)
	})

	t.Run("it should create a default config file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "mig_config_test")
		require.NoError(t, err)

		configPath := filepath.Join(tempDir, "config.yaml")

		err = config.CreateDefault(configPath)
		require.NoError(t, err)

		_, err = os.Stat(configPath)
		require.NoError(t, err)

		cfg, err := config.Load(configPath)
		require.NoError(t, err)

		require.Equal(t, "localhost", cfg.Database.Host)
		require.Equal(t, 5432, cfg.Database.Port)
		require.Equal(t, "postgres", cfg.Database.Name)
		require.Equal(t, "postgres", cfg.Database.User)
		require.Equal(t, "postgres", cfg.Database.Password)
		require.Equal(t, "disable", cfg.Database.SSLMode)
		wd, err := os.Getwd()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s/migrations", wd), cfg.Migrations.Directory)
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("it should return an error if database host is empty", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "migrations",
			},
		}
		err := config.Validate(cfg)
		require.Error(t, err)
	})

	t.Run("it should return an error if database name is empty", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "migrations",
			},
		}
		err := config.Validate(cfg)
		require.Error(t, err)
	})

	t.Run("it should return an error if database user is empty", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "migrations",
			},
		}
		err := config.Validate(cfg)
		require.Error(t, err)
	})

	t.Run("it should set default port if port is 0", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     0,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "migrations",
			},
		}
		err := config.Validate(cfg)
		require.NoError(t, err)
		require.Equal(t, 5432, cfg.Database.Port)
	})

	t.Run("it should set default SSLMode if SSLMode is empty", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "",
			},
			Migrations: config.MigrationsConfig{
				Directory: "migrations",
			},
		}
		err := config.Validate(cfg)
		require.NoError(t, err)
		require.Equal(t, "disable", cfg.Database.SSLMode)
	})

	t.Run("it should set default migrations directory if directory is empty", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "",
			},
		}
		err := config.Validate(cfg)
		require.NoError(t, err)
		wd, err := os.Getwd()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s/migrations", wd), cfg.Migrations.Directory)
	})

	t.Run("it should convert relative migrations directory to absolute path", func(t *testing.T) {
		cwd, err := os.Getwd()
		require.NoError(t, err)

		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: "relative/path",
			},
		}

		err = config.Validate(cfg)
		require.NoError(t, err)

		expected := filepath.Join(cwd, "relative/path")
		require.Equal(t, expected, cfg.Migrations.Directory)
	})

	t.Run("it should keep absolute migrations directory unchanged", func(t *testing.T) {
		absPath := filepath.Join("/", "absolute", "path")
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			Migrations: config.MigrationsConfig{
				Directory: absPath,
			},
		}
		err := config.Validate(cfg)
		require.NoError(t, err)
		require.Equal(t, absPath, cfg.Migrations.Directory)
	})
}
