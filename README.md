# Tel

Interactive TUI application for database queries with filtering and state persistence.

## Features

- **Interactive Table UI** - Browse query results in a searchable table
- **Filter Support** - Apply SQL WHERE clauses to filter data
- **State Persistence** - Remembers selected row and filter by UID
- **Multi-database** - Supports PostgreSQL, DuckDB, and SQLite
- **File-based Config** - Metadata stored as files, not in a database

## Installation

```bash
make build
```

## Usage

```bash
./tel -item <name> [-filter <expr>] [-uid <uid>] [-view r|c]
```

### Flags

| Flag | Description | Required |
|------|-------------|----------|
| `-item` | Item name (directory under `data/`) | Yes |
| `-filter` | Initial filter (SQL WHERE clause) | No |
| `-uid` | UID to restore previous session state | No |
| `-view` | View mode: `r` (row) or `c` (column/vertical) | No |

### Examples

```bash
./tel -item users
./tel -item users -filter "salary > 100000"
./tel -item products -uid <uid_from_previous_session>
./tel -item users -view c
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
├── cmd/tel/              # Main application
│   ├── main.go           # Entry point
│   └── model.go          # TUI model
├── config/               # Configuration
│   ├── config.go         # Instance/session persistence (SQLite)
│   └── fileconfig.go     # File-based config (TOML)
├── db/                   # Database layer
│   └── database.go       # DB connections
├── data/                 # Item definitions
│   ├── <item>/
│   │   ├── meta.toml     # Connection + layout settings
│   │   ├── query.sql     # SQL query
│   │   └── <sub-item>/   # Nested items (future)
│   └── ...
└── logs/                 # Application logs
```

## Item Structure

Each item is a directory under `data/` containing:

### `meta.toml`

```toml
[connection]
driver = "sqlite"          # sqlite | duckdb | postgres
dsn = "path/to/db.sqlite"  # for sqlite/duckdb

# For postgres:
# host = "localhost"
# port = 5432
# user = "user"
# password = "pass"
# dbname = "mydb"

[layout]
height = 10
view = "r"                 # r = row, c = column/vertical

[layout.widths]
ID = 6
NAME = 20

[variables]
# placeholder vars (future: inherited from parent items)
```

### `query.sql`

The SQL query to execute, with optional `:placeholder` substitution.

## Development

```bash
make build   # Build binary
make run     # Build and run with default item (users)
make clean   # Clean artifacts
make lint    # Run go vet
```
