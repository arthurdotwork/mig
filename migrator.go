package mig

// Migrator is the main struct for the migrator package
type Migrator struct {
	manager *Manager
}

// New creates a new Migrator instance
func New(configPath string) (*Migrator, error) {
	// Load the configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Create the manager
	manager, err := NewManager(cfg)
	if err != nil {
		return nil, err
	}

	return &Migrator{
		manager: manager,
	}, nil
}

// CreateMigration creates a new migration file
func (m *Migrator) CreateMigration(name string) (string, error) {
	return m.manager.CreateMigration(name)
}

// MigrateUp applies the next pending migration
func (m *Migrator) MigrateUp() (bool, error) {
	return m.manager.MigrateUp()
}

// MigrateUpAll applies all pending migrations
func (m *Migrator) MigrateUpAll() (int, error) {
	return m.manager.MigrateUpAll()
}

// Status returns the status of migrations
func (m *Migrator) Status() ([]Migration, []MigrationVersion, error) {
	return m.manager.Status()
}

// Close closes the database connection
func (m *Migrator) Close() error {
	return m.manager.Close()
}
