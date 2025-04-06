# Mig

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8.svg)](https://go.dev/)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI Status](https://github.com/arthurdotwork/mig/actions/workflows/mig.yaml/badge.svg)](https://github.com/arthurdotwork/mig/actions)

Mig is a lightweight, flexible database migration tool for PostgreSQL, designed to help you manage your database schema changes with ease.

## ✨ Features

- **Simple Configuration**: YAML-based setup for quick deployment
- **Transactional Migrations**: Safe execution with transaction support (and optional opt-out)
- **Flexible CLI**: Intuitive commands for creating and managing migrations
- **Docker Support**: Ready-to-use with Docker Compose for local development
- **Migration History**: Tracks all executed migrations with timestamps
- **Organized Structure**: Clear file-naming convention for easy management

## 🚀 Getting Started

### Prerequisites

- Go 1.24 or later (for development)
- PostgreSQL database
- Docker (optional, for containerized development)

### Installation

#### Using Go

```bash
# Clone the repository
git clone https://github.com/arthurdotwork/mig.git
cd mig

# Build the binary
go build -o mig ./cmd/mig
```

#### Using Docker

```bash
# Start PostgreSQL with Docker Compose
docker-compose up -d
```

### Initialize Your Environment

Create a new migrations environment in your project:

```bash
./mig init
```

This will create:
- A default `mig.yaml` configuration file
- A `migrations` directory
- An initial sample migration

### Configuration

The default `mig.yaml` looks like this:

```yaml
database:
  host: localhost
  port: 5432
  name: postgres
  user: postgres
  password: postgres
  sslmode: disable

migrations:
  directory: migrations
```

You can override database configuration using environment variables:
- `DATABASE_HOST`
- `DATABASE_PORT`
- `DATABASE_NAME`
- `DATABASE_USER`
- `DATABASE_PASSWORD`
- `DATABASE_SSLMODE`

### Creating Migrations

Create a new migration file:

```bash
./mig create add_users_table
```

This creates a timestamped SQL file in your migrations directory:

```sql
-- Migration: add_users_table
-- Created at: 2025-04-06 14:30:00
-- 
-- Note: 
-- Add "-- disable-tx" anywhere in this file to disable transaction wrapping.

-- Your SQL goes here
```

### Running Migrations

Apply the next pending migration:

```bash
./mig up
```

Apply all pending migrations:

```bash
./mig up-all
```

Check migration status:

```bash
./mig status
```

## 🧩 Migration Files

Migration files follow a specific naming convention:

```
YYYY_MM_DD_HH_MM_SS_name.sql
```

For example:
```
2025_04_06_14_30_00_add_users_table.sql
```

### Transaction Control

By default, migrations run inside a transaction. If you need to execute statements that can't run in a transaction (like creating an index concurrently), add this comment at the top of your migration file:

```sql
-- disable-tx
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

## 📖 Command Reference

```
Migrator version 0.1.0

Usage:
  mig [options] <command> [arguments]

Options:
  -config string
        Path to the configuration file (default "mig.yaml")
  -log-level string
        Log level (debug, info, warn, error, fatal) (default "info")
  -version
        Show version information

Commands:
  init       Initialize the migration environment
  create     Create a new migration
  up         Apply the next pending migration
  up-all     Apply all pending migrations
  status     Show the status of migrations
```

### Command Options

#### `init`
```
mig init [-dir migrations]
```
- `-dir`: Path to the migrations directory (default: `migrations`)

#### `create`
```
mig create migration_name
```

#### `status`
```
mig status
```
Shows information about applied and pending migrations.

## 🧪 Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with code coverage
go test -cover ./...
```

### CI/CD Integration

Mig includes GitHub Actions workflows for:
- Linting with golangci-lint
- Running tests against PostgreSQL
- Automatic tag creation based on CHANGELOG updates

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Update the CHANGELOG.md when making changes
2. Add tests for new functionality
3. Ensure all tests pass before submitting PRs

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
