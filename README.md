# LiteSchema — SQLite Schema Analysis & Migration CLI

A comprehensive Go CLI tool for parsing, diffing, migrating, analyzing, and validating SQLite database schemas.

## Features

- **Parse** — Extract full schema from SQL files, JSON dumps, or live `.db`/`.sqlite` files
- **Diff** — Semantic comparison of two schemas (added/removed/modified tables, columns, indexes, views, triggers)
- **Migrate** — Auto-generate ALTER TABLE migration SQL from schema diffs
- **Analyze** — Index health analysis (redundant indexes, missing FK indexes, over-indexed tables)
- **Validate** — Check for common schema issues (missing PKs, orphaned FKs, naming issues)
- **FK Graph** — Visualize foreign key dependency chains with cycle detection
- **Output formats** — Text (ASCII tree), JSON, Markdown, SQL

## Installation

### From source

```bash
git clone https://github.com/EdgarOrtegaRamirez/liteschema.git
cd liteschema
go build -o liteschema ./cmd/liteschema
```

### Go install

```bash
go install github.com/EdgarOrtegaRamirez/liteschema/cmd/liteschema@latest
```

## Quick Start

```bash
# Parse a SQL schema file
liteschema parse schema.sql

# Parse with CREATE statements visible
liteschema parse schema.sql --show-sql

# Parse a live SQLite database
liteschema parse data.db

# Diff two schema versions
liteschema diff schema_v1.sql schema_v2.sql

# Generate migration SQL
liteschema migrate schema_v1.sql schema_v2.sql --format sql

# Analyze indexes
liteschema analyze schema.sql

# Validate schema
liteschema validate data.db

# View foreign key dependency graph
liteschema fkgraph schema.sql --format json
```

## Commands

| Command | Description |
|---------|-------------|
| `parse` | Parse and display a SQLite schema from SQL, JSON, or database file |
| `diff` | Compute semantic schema diff between two files |
| `migrate` | Generate ALTER TABLE migration SQL from schema diff |
| `analyze` | Analyze indexes for redundancy and missing coverage |
| `validate` | Validate schema for common issues |
| `fkgraph` | Display foreign key dependency graph with cycle detection |
| `help` | Show usage information |

## Options

| Option | Description |
|--------|-------------|
| `--format text\|json\|markdown\|sql` | Output format (default: text) |
| `--show-sql` | Include CREATE statements in schema view |

## Example

```sql
-- v1.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);
```

```sql
-- v2.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    age INTEGER
);
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

```bash
$ liteschema diff v1.sql v2.sql
# Schema Diff: 2 added, 0 removed, 0 modified
# ────────────────────────────────────────────────────────────
#   + column users.age (INTEGER)
#   + table posts
```

```bash
$ liteschema migrate v1.sql v2.sql --format sql
# -- Generated migration SQL
# -- Schema Diff: 2 added, 0 removed, 0 modified
#
# ALTER TABLE users ADD COLUMN age INTEGER;
# -- TODO: CREATE TABLE posts (see schema definition)
```

## Output Formats

### Text (default)
```
┌─ TABLE: users
│  ├─ id  INTEGER  [PK, NOT NULL, AUTOINCREMENT]
│  ├─ name  TEXT  [NOT NULL]
│  ├─ email  TEXT  [UNIQUE]
┌─ TABLE: posts
│  ├─ id  INTEGER  [PK]
│  ├─ user_id  INTEGER  [NOT NULL]
│  ├─ title  TEXT  [NOT NULL]
│  ├─ FK: user_id → users(id)
```

### JSON
```json
{
  "tables": [
    {
      "name": "users",
      "columns": [{"name": "id", "type": "INTEGER", "primary_key": true}]
    }
  ]
}
```

### Markdown
```markdown
## Table: `users`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | `INTEGER` | PK, NOT NULL, AUTOINCREMENT |
| `name` | `TEXT` | NOT NULL |
| `email` | `TEXT` | UNIQUE |
```

## Architecture

```
cmd/liteschema/        # CLI entry point with subcommands
pkg/schema/            # Core library
├── models.go          # Data types (Table, Column, Index, etc.)
├── parser.go          # SQL and database schema parser
├── printer.go         # Text/JSON/Markdown/SQL output formatters
├── diff.go            # Semantic schema diff engine
├── analyze.go         # Index analysis, FK graph, validation
└── schema_test.go     # Comprehensive test suite
```

## Testing

```bash
go test ./pkg/schema/ -v
```

40+ tests covering: SQL parsing, schema diffing, migration generation, foreign key graph construction, cycle detection, index analysis, schema validation, output formatting.

## Security

- No hardcoded secrets
- Read-only database access (no writes to user databases)
- SQL injection-resistant (only reads metadata, never executes user SQL)
- Input validation for all CLI arguments (path sanitization)

## License

MIT