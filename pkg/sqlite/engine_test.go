package sqlite

import (
	"os"
	"testing"
)

func TestOpenDatabase(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	if db.Path() == "" {
		t.Error("expected non-empty path")
	}
}

func TestTableNames(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	names, err := db.TableNames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one table")
	}

	found := false
	for _, n := range names {
		if n == "test_users" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find test_users table, got %v", names)
	}
}

func TestQuery(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	rows, err := db.Query("SELECT * FROM test_users ORDER BY id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	if rows[0]["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got %v", rows[0]["name"])
	}
}

func TestQueryScalar(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	val, err := db.QueryScalar("SELECT COUNT(*) FROM test_users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := toInt64(val)
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestRowCount(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	count, err := db.RowCount("test_users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestTableInfo(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	cols, err := db.TableInfo("test_users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}

	if !cols[0].PrimaryKey {
		t.Error("expected first column to be primary key")
	}
	if cols[0].Name != "id" {
		t.Errorf("expected name 'id', got %q", cols[0].Name)
	}
}

func TestForeignKeyInfo(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	fks, err := db.ForeignKeyInfo("test_orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fks) == 0 {
		t.Fatal("expected at least one foreign key")
	}
	if fks[0].ToTable != "test_users" {
		t.Errorf("expected ToTable 'test_users', got %q", fks[0].ToTable)
	}
}

func TestExec(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	_, err := db.Exec("INSERT INTO test_users (name, email) VALUES (?, ?)", "Charlie", "charlie@test.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, _ := db.RowCount("test_users")
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

// Helper: create a temporary in-memory test database
func createTestDB(t *testing.T) *DB {
	t.Helper()

	f, err := os.CreateTemp("", "liteschema-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()

	db, err := Open(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatalf("open test db: %v", err)
	}

	schema := `
		CREATE TABLE test_users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL
		);
		CREATE TABLE test_orders (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			product TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
		);
		INSERT INTO test_users (id, name, email) VALUES (1, 'Alice', 'alice@test.com');
		INSERT INTO test_users (id, name, email) VALUES (2, 'Bob', 'bob@test.com');
		INSERT INTO test_orders (id, user_id, product) VALUES (1, 1, 'Widget');
	`

	_, err = db.Exec(schema, []interface{}{}...)
	if err != nil {
		db.Close()
		os.Remove(f.Name())
		t.Fatalf("create test schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(f.Name())
	})

	return db
}
