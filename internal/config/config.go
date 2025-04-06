package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultMigrationsDir is the default name of the migrations directory
	DefaultMigrationsDir = "migrations"
)

// DatabaseConfig represents the configuration for the database connection
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

// MigrationsConfig represents the configuration for migrations
type MigrationsConfig struct {
	Directory string `yaml:"directory"`
}

// Config represents the configuration for the migrator
type Config struct {
	Database   DatabaseConfig   `yaml:"database"`
	Migrations MigrationsConfig `yaml:"migrations"`
}

// Load loads the configuration from the specified file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides for sensitive fields
	if envHost := os.Getenv("DATABASE_HOST"); envHost != "" {
		config.Database.Host = envHost
	}

	if envPort := os.Getenv("DATABASE_PORT"); envPort != "" {
		var port int
		if _, err := fmt.Sscanf(envPort, "%d", &port); err == nil {
			config.Database.Port = port
		}
	}

	if envName := os.Getenv("DATABASE_NAME"); envName != "" {
		config.Database.Name = envName
	}

	if envUser := os.Getenv("DATABASE_USER"); envUser != "" {
		config.Database.User = envUser
	}

	if envPassword := os.Getenv("DATABASE_PASSWORD"); envPassword != "" {
		config.Database.Password = envPassword
	}

	if envSSLMode := os.Getenv("DATABASE_SSLMODE"); envSSLMode != "" {
		config.Database.SSLMode = envSSLMode
	}

	// Validate the configuration
	if err := Validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateDefault creates a default configuration file
func CreateDefault(path string) error {
	// Create a default configuration
	config := Config{}

	// Set default database settings
	config.Database.Host = "localhost"
	config.Database.Port = 5432
	config.Database.Name = "postgres"
	config.Database.User = "postgres"
	config.Database.Password = "postgres"
	config.Database.SSLMode = "disable"

	// Set default migrations directory
	config.Migrations.Directory = DefaultMigrationsDir

	// Marshal the configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write the configuration to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func Validate(config *Config) error {
	if config.Database.Host == "" {
		return errors.New("database host is required")
	}

	if config.Database.Port == 0 {
		config.Database.Port = 5432 // Default PostgreSQL port
	}

	if config.Database.Name == "" {
		return errors.New("database name is required")
	}

	if config.Database.User == "" {
		return errors.New("database user is required")
	}

	if config.Database.SSLMode == "" {
		config.Database.SSLMode = "disable" // Default SSL mode
	}

	if config.Migrations.Directory == "" {
		config.Migrations.Directory = DefaultMigrationsDir
	}

	// Ensure the migrations directory path is absolute
	if !filepath.IsAbs(config.Migrations.Directory) {
		absPath, err := filepath.Abs(config.Migrations.Directory)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for migrations directory: %w", err)
		}
		config.Migrations.Directory = absPath
	}

	return nil
}
