# Mig

Mig is a very opinionated tool to run migrations on a database. It is designed to be simple and easy to use. It is
written in Go and uses the `database/sql` package to interact with the database.

## Design

- Can be used both through a simple binary and both through code.
- Migrations are written in SQL.
- Migrations are run in a transaction.
- Only up migrations are supported.
