package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/arthurdotwork/mig"
)

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, args []string) error
}

var (
	// Global flags
	configPath  string
	logLevel    string
	showVersion bool

	// Available commands
	commands = map[string]*Command{
		"init": {
			Name:        "init",
			Description: "Initialize the migration environment",
			Execute:     cmdInit,
		},
		"create": {
			Name:        "create",
			Description: "Create a new migration",
			Execute:     cmdCreate,
		},
		"up": {
			Name:        "up",
			Description: "Apply the next pending migration",
			Execute:     cmdUp,
		},
		"up-all": {
			Name:        "up-all",
			Description: "Apply all pending migrations",
			Execute:     cmdUpAll,
		},
		"status": {
			Name:        "status",
			Description: "Show the status of migrations",
			Execute:     cmdStatus,
		},
	}
)

func init() {
	// Define global flags
	flag.StringVar(&configPath, "config", mig.DefaultConfigFilename, "Path to the configuration file")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, fatal)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse flags
	flag.Parse()

	// Configure logger based on log level
	setupLogger(logLevel)

	// Show version information if requested
	if showVersion {
		fmt.Printf("Migrator version %s\n", mig.Version)
		os.Exit(0)
	}

	// Get the command and arguments
	args := flag.Args()
	if len(args) == 0 {
		showHelp()
		os.Exit(1)
	}

	// Check if the command exists
	cmd, ok := commands[args[0]]
	if !ok {
		slog.ErrorContext(ctx, "unknown command", slog.String("command", args[0]))
		showHelp()
		os.Exit(1)
	}

	// Execute the command
	if err := cmd.Execute(ctx, args[1:]); err != nil {
		slog.ErrorContext(ctx, "failed to execute command",
			slog.String("command", args[0]),
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// setupLogger configures the slog logger with appropriate level
func setupLogger(level string) {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
}

// showHelp displays help information
func showHelp() {
	fmt.Printf("Migrator version %s\n\n", mig.Version)
	fmt.Println("Usage:")
	fmt.Println("  mig [options] <command> [arguments]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Description)
	}
}

// cmdInit initializes the migration environment
func cmdInit(ctx context.Context, args []string) error {
	// Parse command flags
	cmdFlags := flag.NewFlagSet("init", flag.ExitOnError)
	migrationsDir := cmdFlags.String("dir", mig.DefaultMigrationsDir, "Path to the migrations directory")
	cmdFlags.Parse(args) //nolint:errcheck

	// Initialize the environment
	err := mig.Initialize(configPath, *migrationsDir)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize migrations", slog.String("dir", *migrationsDir))
		return err
	}

	slog.InfoContext(ctx, "migration environment initialized",
		slog.String("config", configPath),
		slog.String("dir", *migrationsDir))
	return nil
}

// cmdCreate creates a new migration
func cmdCreate(ctx context.Context, args []string) error {
	// Parse command flags
	cmdFlags := flag.NewFlagSet("create", flag.ExitOnError)
	cmdFlags.Parse(args) //nolint:errcheck

	// Get the migration name
	if cmdFlags.NArg() == 0 {
		return fmt.Errorf("migration name is required")
	}
	name := strings.Join(cmdFlags.Args(), "_")

	// Create a new migrator
	m, err := mig.New(configPath)
	if err != nil {
		return err
	}
	defer m.Close() //nolint:errcheck

	// Create the migration
	filename, err := m.CreateMigration(name)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "migration created", slog.String("name", name), slog.String("filename", filename))
	return nil
}

// cmdUp applies the next pending migration
func cmdUp(ctx context.Context, args []string) error {
	// Parse command flags
	cmdFlags := flag.NewFlagSet("up", flag.ExitOnError)
	cmdFlags.Parse(args) //nolint:errcheck

	// Create a new migrator
	m, err := mig.New(configPath)
	if err != nil {
		return err
	}
	defer m.Close() //nolint:errcheck

	// Apply the next migration
	executed, err := m.MigrateUp()
	if err != nil {
		return err
	}

	if executed {
		slog.InfoContext(ctx, "migration up succeeded")
	} else {
		slog.WarnContext(ctx, "no migration to apply")
	}

	return nil
}

// cmdUpAll applies all pending migrations
func cmdUpAll(ctx context.Context, args []string) error {
	// Parse command flags
	cmdFlags := flag.NewFlagSet("up-all", flag.ExitOnError)
	cmdFlags.Parse(args) //nolint:errcheck

	// Create a new migrator
	m, err := mig.New(configPath)
	if err != nil {
		return err
	}
	defer m.Close() //nolint:errcheck

	// Apply all migrations
	count, err := m.MigrateUpAll()
	if err != nil {
		return err
	}

	if count > 0 {
		slog.InfoContext(ctx, "migrations up succeeded", slog.Int("count", count))
	} else {
		slog.WarnContext(ctx, "no migrations to apply")
	}

	return nil
}

// cmdStatus shows the status of migrations
func cmdStatus(ctx context.Context, args []string) error {
	// Parse command flags
	cmdFlags := flag.NewFlagSet("status", flag.ExitOnError)
	cmdFlags.Parse(args) //nolint:errcheck

	// Create a new migrator
	m, err := mig.New(configPath)
	if err != nil {
		return err
	}
	defer m.Close() //nolint:errcheck

	// Get the status
	statuses, err := m.Status()
	if err != nil {
		return err
	}

	// Display the status
	fmt.Println("Migration Status:")
	fmt.Println("=================")

	// Count applied migrations
	appliedCount := 0
	for _, status := range statuses {
		if status.Applied {
			appliedCount++
		}
	}

	fmt.Printf("Total: %d, Applied: %d, Pending: %d\n\n", len(statuses), appliedCount, len(statuses)-appliedCount)

	// Display the list of migrations
	if len(statuses) > 0 {
		fmt.Println("Migrations:")
		for _, status := range statuses {
			statusText := "PENDING"
			appliedAt := ""
			if status.Applied {
				statusText = "APPLIED"
				appliedAt = status.AppliedAt
			}
			fmt.Printf("  %-10s  %s  %s\n", statusText, appliedAt, status.ID)
		}
	} else {
		fmt.Println("No migrations found")
	}

	return nil
}
