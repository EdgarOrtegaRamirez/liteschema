// Package sqlite provides a lightweight SQLite database wrapper using the
// modernc.org/sqlite driver (pure Go, no CGO required).
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection with helper methods.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens an existing SQLite database file.
func Open(path string) (*DB, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file does not exist: %s", absPath)
	}

	conn, err := sql.Open("sqlite", absPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{conn: conn, path: absPath}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Query executes a SELECT query and returns rows as a slice of maps.
func (db *DB) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return results, nil
}

// QueryScalar executes a query and returns a single scalar value.
func (db *DB) QueryScalar(query string, args ...interface{}) (interface{}, error) {
	var val interface{}
	err := db.conn.QueryRow(query, args...).Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("execute scalar query: %w", err)
	}
	if b, ok := val.([]byte); ok {
		return string(b), nil
	}
	return val, nil
}

// Exec executes a statement (INSERT, UPDATE, DELETE, DDL).
func (db *DB) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(stmt, args...)
}

// TableNames returns a list of all table names in the database.
func (db *DB) TableNames() ([]string, error) {
	rows, err := db.Query(
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, row := range rows {
		if name, ok := row["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

// TableInfo returns column information for a table.
func (db *DB) TableInfo(tableName string) ([]ColumnInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(tableName)))
	if err != nil {
		return nil, fmt.Errorf("get table info: %w", err)
	}

	var columns []ColumnInfo
	for _, row := range rows {
		col := ColumnInfo{}
		if v, ok := row["cid"].(int64); ok {
			col.Ordinal = int(v)
		}
		if v, ok := row["name"].(string); ok {
			col.Name = v
		}
		if v, ok := row["type"].(string); ok {
			col.Type = v
		}
		if v, ok := row["notnull"].(int64); ok {
			col.NotNull = v != 0
		}
		if v, ok := row["dflt_value"]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			col.Default = &s
		}
		if v, ok := row["pk"].(int64); ok {
			col.PrimaryKey = v != 0
		}
		columns = append(columns, col)
	}
	return columns, nil
}

// ForeignKeyInfo returns foreign key information for a table.
func (db *DB) ForeignKeyInfo(tableName string) ([]ForeignKeyInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(tableName)))
	if err != nil {
		return nil, fmt.Errorf("get foreign keys: %w", err)
	}

	var fks []ForeignKeyInfo
	for _, row := range rows {
		fk := ForeignKeyInfo{FromTable: tableName}
		if v, ok := row["table"].(string); ok {
			fk.ToTable = v
		}
		if v, ok := row["from"].(string); ok {
			fk.FromColumn = v
		}
		if v, ok := row["to"].(string); ok {
			fk.ToColumn = v
		}
		if v, ok := row["on_update"].(string); ok && v != "NO ACTION" {
			fk.OnUpdate = v
		}
		if v, ok := row["on_delete"].(string); ok && v != "NO ACTION" {
			fk.OnDelete = v
		}
		fks = append(fks, fk)
	}
	return fks, nil
}

// RowCount returns the number of rows in a table.
func (db *DB) RowCount(tableName string) (int64, error) {
	val, err := db.QueryScalar(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(tableName)))
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// ColumnInfo holds information about a database column.
type ColumnInfo struct {
	Ordinal    int
	Name       string
	Type       string
	NotNull    bool
	Default    *string
	PrimaryKey bool
}

// ForeignKeyInfo holds information about a foreign key constraint.
type ForeignKeyInfo struct {
	FromTable  string
	FromColumn string
	ToTable    string
	ToColumn   string
	OnUpdate   string
	OnDelete   string
}

func quoteIdent(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}
