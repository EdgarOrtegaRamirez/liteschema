# AGENTS.md — For AI Coding Agents

## Project Overview

LiteSchema is a Go CLI tool for SQLite schema analysis, diffing, migration generation, index analysis, validation, and foreign key dependency graphing. Designed for database administrators, developers, and CI/CD pipelines.

## Key Files

| File | Purpose |
|------|---------|
| `cmd/liteschema/main.go` | CLI entry point — Cobra-free, hand-rolled subcommand dispatch |
| `pkg/schema/models.go` | Core data types: `Table`, `Column`, `Index`, `ForeignKey`, `Trigger`, `View`, `DatabaseSchema` |
| `pkg/schema/parser.go` | SQL and database schema parser (`ParseFromSQL`, `ParseFromDB`) |
| `pkg/schema/printer.go` | Output formatters (text tree, JSON, Markdown, SQL) |
| `pkg/schema/diff.go` | Semantic schema diff engine (`Diff`, `FormatDiff`) |
| `pkg/schema/analyze.go` | Index analysis, FK graph, schema validation, migration generation |
| `pkg/schema/schema_test.go` | 40+ tests |

## Data Flow

1. **Input**: SQL string (`.sql`), JSON file (`.json`), or SQLite database (`.db`/`.sqlite`)
2. **Parse**: `Parser.ParseFromSQL()` or `Parser.ParseFromDB()` → `*DatabaseSchema`
3. **Process**: Diff, analyze, validate, or visualize
4. **Output**: Text, JSON, Markdown, or SQL

## Important Patterns

- All schema types defined in `models.go` with JSON tags
- Parsing uses hand-rolled text processing (no external SQL parser)
- SQLite DB access uses `modernc.org/sqlite` (pure Go, no CGO)
- Tests use in-memory data structures (no DB files needed)
- No external config files — all options are CLI flags

## Build & Test

```bash
go build ./...
go test ./pkg/schema/ -v
go vet ./...
```

## When Modifying

1. Update models first if adding new schema object types
2. Add parser support in `parser.go`
3. Add diff logic in `diff.go`
4. Add formatters in `printer.go`
5. Add tests in `schema_test.go`
6. Add CLI command in `cmd/liteschema/main.go`

## Common Extensions

- **Add a new schema object type** (e.g., `CREATE VIRTUAL TABLE`): Add type to `models.go`, parser case in `ParseFromSQL`, diff logic in `Diff`
- **Add a new output format**: Add function to `printer.go` and format switch in `main.go`
- **Add a new validation rule**: Add check in `ValidateSchema` and test