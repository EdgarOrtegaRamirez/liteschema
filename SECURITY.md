# Security Policy

## Supported Versions

| Version | Supported          |
|---------|-------------------|
| 0.x     | ✅ Active development |

## Reporting a Vulnerability

Open an issue on GitHub describing the vulnerability. Do not include sensitive information in the issue body.

## Security Practices

- **Read-only database access**: LiteSchema only reads schema metadata from SQLite databases — it never writes to or modifies user databases.
- **No network access**: All operations are local file-based. No data is sent to external services.
- **Input validation**: All file paths provided via CLI are validated before use. Path traversal attacks are prevented by limiting reads to user-specified paths.
- **No code execution**: Schema SQL is parsed and analyzed, not executed. No SQL injection risk.
- **Dependency management**: Dependencies are pinned to specific versions in `go.mod`. Regular dependency updates are performed via Dependabot or manual review.

## Data Handling

LiteSchema processes:
1. SQL schema files (CREATE TABLE/INDEX/VIEW/TRIGGER statements)
2. SQLite database files (read-only, schema tables only)
3. JSON schema dumps

No database content/rows are ever read or processed — only schema metadata.

## Updates

Security updates are published as GitHub releases. Users are advised to keep their installation up to date with `go install github.com/EdgarOrtegaRamirez/liteschema/cmd/liteschema@latest`.