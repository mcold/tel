# Tel

Interactive TUI application for database queries with filtering and state persistence.

## Features

- **Interactive Table UI** - Browse query results in a searchable table
- **Filter Support** - Apply SQL WHERE clauses to filter data
- **State Persistence** - Remembers selected row and filter by UID
- **Multi-database** - Supports PostgreSQL, DuckDB, and SQLite
- **Configurable Layouts** - Column widths and aliases saved per query

## Installation

```bash
make build
```

## Usage

```bash
./tel -item <item> -sql <query_name> -db <database>
```

### Flags

| Flag | Description | Required |
|------|-------------|----------|
| `-item` | Item name for config | Yes |
| `-sql` | SQL query name from queries table | Yes |
| `-db` | Database name from dbs table | Yes |
| `-filter` | Initial filter (SQL WHERE clause) | No |
| `-args` | JSON file with placeholder args | No |
| `-uid` | UID to restore previous session state | No |

### Examples

```bash
./tel -item users -sql active_users -db analytics
```

With filter:
```bash
./tel -item users -sql active_users -db analytics -filter "status = 'active'"
```

Restore previous session:
```bash
./tel -item users -sql active_users -db analytics -uid <uid_from_previous_session>
```

## Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Apply filter / Save current row and filter |
| `Tab` | Switch focus between table and filter input |
| `Esc` | Toggle focus |
| `Ctrl+C` | Quit |

## Project Structure

```
tel/
├── cmd/tel/          # Main application
│   ├── main.go       # Entry point
│   └── model.go      # TUI model
├── config/           # Configuration & DB
│   └── config.go     # Config management
├── db/               # Database layer
│   └── database.go   # DB connections
├── zel/              # Layouts
├── args/             # Query args
└── logs/             # Application logs
```

## Database Schema

### Main Tables

- **dbs** - Database connections
- **items** - Named items linked to databases
- **queries** - SQL queries with configs
- **config** - Per-user column configurations
- **instance** - Session state (row hash, filter, UID)

## Development

```bash
make build   # Build binary
make run     # Build and run
make clean   # Clean artifacts
make lint    # Run linters
```
