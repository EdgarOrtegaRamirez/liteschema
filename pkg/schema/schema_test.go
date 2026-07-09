package schema

import (
	"fmt"
	"testing"
)

func TestParseCreateTable(t *testing.T) {
	sql := "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, email TEXT UNIQUE);"
	table, err := parseCreateTable(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if table.Name != "users" {
		t.Errorf("expected name 'users', got %q", table.Name)
	}
	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}
	if !table.Columns[0].PrimaryKey {
		t.Error("expected first column to be primary key")
	}
	if !table.Columns[0].AutoIncr {
		t.Error("expected first column to be autoincrement")
	}
	if !table.Columns[1].NotNull {
		t.Error("expected second column to be not null")
	}
	if !table.Columns[2].Unique {
		t.Error("expected third column to be unique")
	}
}

func TestParseCreateTableWithFK(t *testing.T) {
	sql := `CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		product TEXT NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`
	table, err := parseCreateTable(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(table.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(table.ForeignKeys))
	}
	fk := table.ForeignKeys[0]
	if fk.RefTable != "users" {
		t.Errorf("expected ref table 'users', got %q", fk.RefTable)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("expected ON DELETE CASCADE, got %q", fk.OnDelete)
	}
}

func TestParseFullSchema(t *testing.T) {
	sql := `CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX idx_users_email ON users(email);
	CREATE VIEW user_emails AS SELECT id, email FROM users;
	CREATE TRIGGER after_user_insert AFTER INSERT ON users BEGIN
		UPDATE users SET created_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
	END;`

	p := NewParser()
	s, err := p.ParseFromSQL(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(s.Tables))
	}
	if len(s.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(s.Views))
	}
	if len(s.Triggers) != 1 {
		t.Errorf("expected 1 trigger, got %d", len(s.Triggers))
	}
}

func TestDiffNoChanges(t *testing.T) {
	sql := "CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT);"
	p := NewParser()
	s1, _ := p.ParseFromSQL(sql)
	s2, _ := p.ParseFromSQL(sql)
	d := Diff(s1, s2)
	if len(d.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(d.Changes))
	}
}

func TestDiffAddTable(t *testing.T) {
	oldSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY);"
	newSQL := `CREATE TABLE t1 (id INTEGER PRIMARY KEY);
	CREATE TABLE t2 (val TEXT);`
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	if d.AddCount != 1 {
		t.Errorf("expected 1 add, got %d", d.AddCount)
	}
}

func TestDiffRemoveTable(t *testing.T) {
	oldSQL := `CREATE TABLE t1 (id INTEGER PRIMARY KEY);
	CREATE TABLE t2 (val TEXT);`
	newSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY);"
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	if d.RemoveCount != 1 {
		t.Errorf("expected 1 remove, got %d", d.RemoveCount)
	}
	if !d.HasBreaking {
		t.Error("expected breaking change for table removal")
	}
}

func TestDiffAddColumn(t *testing.T) {
	oldSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY);"
	newSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT);"
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	if d.AddCount != 1 {
		t.Errorf("expected 1 add, got %d", d.AddCount)
	}
}

func TestMigrationGeneration(t *testing.T) {
	oldSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY);"
	newSQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT NOT NULL);"
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	gen := NewMigrationGenerator()
	sql := gen.Generate(d)
	if !contains(sql, "ADD COLUMN") {
		t.Error("expected migration SQL to contain ADD COLUMN")
	}
}

func TestForeignKeyGraph(t *testing.T) {
	sql := `CREATE TABLE users (id INTEGER PRIMARY KEY);
	CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`
	p := NewParser()
	s, err := p.ParseFromSQL(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := BuildFKGraph(s)
	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0].FromTable != "orders" {
		t.Errorf("expected from 'orders', got %q", g.Edges[0].FromTable)
	}
}

func TestValidateSchema(t *testing.T) {
	sql := "CREATE TABLE t1 (id INTEGER, name TEXT);"
	p := NewParser()
	s, _ := p.ParseFromSQL(sql)
	r := ValidateSchema(s)
	if len(r.Warnings) == 0 {
		t.Error("expected warning about missing primary key")
	}
}

func TestCycleDetection(t *testing.T) {
	edges := []FKEdge{
		{FromTable: "a", ToTable: "b"},
		{FromTable: "b", ToTable: "c"},
		{FromTable: "c", ToTable: "a"},
	}
	cycles := detectCycles([]string{"a", "b", "c"}, edges)
	if len(cycles) == 0 {
		t.Error("expected cycle detection")
	}
}

func TestIndexAnalysis(t *testing.T) {
	table := Table{
		Name: "test",
		Columns: []Column{
			{Name: "id", Type: "INTEGER", PrimaryKey: true},
			{Name: "user_id", Type: "INTEGER"},
		},
		Indexes: []Index{
			{Name: "idx_a", Columns: []string{"user_id"}},
			{Name: "idx_b", Columns: []string{"user_id", "id"}},
		},
	}
	s := &DatabaseSchema{Tables: []Table{table}}
	r := AnalyzeIndexes(s)
	if len(r.RedundantIndexes) == 0 {
		t.Error("expected redundant index detection")
	}
}

func TestParseFromSQLWithComments(t *testing.T) {
	sql := `-- This is a comment
	CREATE TABLE t1 (id INTEGER PRIMARY KEY);
	/* multi-line
	comment */
	CREATE TABLE t2 (val TEXT);`
	p := NewParser()
	s, err := p.ParseFromSQL(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(s.Tables))
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPrintSchema(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "test", Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}}},
		},
	}
	output := PrintSchema(s, false)
	if !contains(output, "test") {
		t.Error("expected schema output to contain table name")
	}
}

func TestPrintSchemaMarkdown(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "test", Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}}},
		},
	}
	output := PrintSchemaMarkdown(s)
	if !contains(output, "## Table:") {
		t.Error("expected markdown output to contain table header")
	}
}

func TestEmptySchema(t *testing.T) {
	s := &DatabaseSchema{}
	output := PrintSchema(s, false)
	if !contains(output, "No schema objects found") {
		t.Error("expected empty schema message")
	}
}

func TestDiffWithView(t *testing.T) {
	oldSQL := "CREATE VIEW v1 AS SELECT 1;"
	newSQL := "CREATE VIEW v2 AS SELECT 2;"
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	if d.AddCount != 1 || d.RemoveCount != 1 {
		t.Errorf("expected 1 add and 1 remove, got %d add, %d remove", d.AddCount, d.RemoveCount)
	}
}

func TestDiffWithTrigger(t *testing.T) {
	oldSQL := "CREATE TABLE t1 (id INTEGER); CREATE TRIGGER tr1 AFTER INSERT ON t1 BEGIN SELECT 1; END;"
	newSQL := "CREATE TABLE t1 (id INTEGER);"
	p := NewParser()
	old, _ := p.ParseFromSQL(oldSQL)
	new_, _ := p.ParseFromSQL(newSQL)
	d := Diff(old, new_)
	if d.RemoveCount != 1 {
		t.Errorf("expected 1 remove for trigger, got %d", d.RemoveCount)
	}
}

func TestForeignKeyGraphNoCycles(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "a", Columns: []Column{{Name: "id", Type: "INTEGER"}}},
			{Name: "b", Columns: []Column{{Name: "id", Type: "INTEGER"}}},
		},
	}
	g := BuildFKGraph(s)
	if len(g.Cycles) > 0 {
		t.Error("expected no cycles")
	}
}

func TestValidateSchemaNoIssues(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "t1", Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}}},
		},
	}
	r := ValidateSchema(s)
	if len(r.Errors) > 0 || len(r.Warnings) > 0 {
		t.Errorf("expected no issues, got %d errors, %d warnings", len(r.Errors), len(r.Warnings))
	}
}

func TestValidateSchemaOrphanedFK(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{
				Name: "t1",
				Columns: []Column{{Name: "ref_id", Type: "INTEGER"}},
				ForeignKeys: []ForeignKey{
					{Columns: []string{"ref_id"}, RefTable: "nonexistent", RefColumns: []string{"id"}},
				},
			},
		},
	}
	r := ValidateSchema(s)
	if len(r.Errors) == 0 {
		t.Error("expected error for orphaned foreign key")
	}
}

func TestParseCreateIndex(t *testing.T) {
	idx, err := parseCreateIndex("CREATE INDEX idx_name ON users(name);")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx.Name != "idx_name" {
		t.Errorf("expected name 'idx_name', got %q", idx.Name)
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "name" {
		t.Errorf("expected column 'name', got %v", idx.Columns)
	}
}

func TestParseCreateView(t *testing.T) {
	view, err := parseCreateView("CREATE VIEW v1 AS SELECT id, name FROM users;")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Name != "v1" {
		t.Errorf("expected name 'v1', got %q", view.Name)
	}
}

func TestParseCreateTrigger(t *testing.T) {
	trigger, err := parseCreateTrigger("CREATE TRIGGER tr1 AFTER INSERT ON users BEGIN SELECT 1; END;")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Name != "tr1" {
		t.Errorf("expected name 'tr1', got %q", trigger.Name)
	}
	if trigger.Time != "AFTER" {
		t.Errorf("expected time AFTER, got %q", trigger.Time)
	}
	if trigger.Event != "INSERT" {
		t.Errorf("expected event INSERT, got %q", trigger.Event)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	sql := "SELECT 1; SELECT 2;"
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Errorf("expected 2 statements, got %d", len(stmts))
	}
}

func TestParseColumnDef(t *testing.T) {
	col := parseColumnDef("name TEXT NOT NULL DEFAULT 'hello'")
	if col == nil {
		t.Fatal("expected non-nil column")
	}
	if col.Name != "name" {
		t.Errorf("expected name 'name', got %q", col.Name)
	}
	if col.Type != "TEXT" {
		t.Errorf("expected type TEXT, got %q", col.Type)
	}
	if !col.NotNull {
		t.Error("expected not null")
	}
	if col.Default != "'hello'" {
		t.Errorf("expected default 'hello', got %q", col.Default)
	}
}

func TestFormatDiff(t *testing.T) {
	d := &DiffResult{
		Changes: []SchemaChange{
			{Type: ChangeAdd, ObjectType: "table", Name: "t1"},
		},
		AddCount: 1,
	}
	output := FormatDiff(d)
	if !contains(output, "t1") {
		t.Error("expected diff output to contain table name")
	}
}

func TestFormatDiffNoChanges(t *testing.T) {
	d := &DiffResult{Changes: []SchemaChange{}}
	output := FormatDiff(d)
	if !contains(output, "No schema changes") {
		t.Error("expected no changes message")
	}
}

func TestFormatIndexAnalysisNoIssues(t *testing.T) {
	r := &IndexAnalysisResult{}
	output := FormatIndexAnalysis(r)
	if !contains(output, "No issues found") {
		t.Error("expected no issues message")
	}
}

func TestFormatValidationNoIssues(t *testing.T) {
	r := &ValidationResult{}
	output := FormatValidation(r)
	if !contains(output, "No issues found") {
		t.Error("expected no issues message")
	}
}

func TestFormatFKGraphNoEdges(t *testing.T) {
	g := &ForeignKeyGraph{}
	output := FormatFKGraph(g)
	if !contains(output, "No foreign key") {
		t.Error("expected no foreign keys message")
	}
}

func TestParseForeignKeyDef(t *testing.T) {
	fk := parseForeignKeyDef("FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE")
	if fk == nil {
		t.Fatal("expected non-nil foreign key")
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Errorf("expected column user_id, got %v", fk.Columns)
	}
	if fk.RefTable != "users" {
		t.Errorf("expected ref table users, got %q", fk.RefTable)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("expected ON DELETE CASCADE, got %q", fk.OnDelete)
	}
}

func TestWithoutRowid(t *testing.T) {
	sql := "CREATE TABLE t1 (id INTEGER PRIMARY KEY) WITHOUT ROWID;"
	table, err := parseCreateTable(sql)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !table.WithoutRowID {
		t.Error("expected WITHOUT ROWID")
	}
}

func TestColumnWithCollate(t *testing.T) {
	col := parseColumnDef("name TEXT COLLATE NOCASE")
	if col == nil {
		t.Fatal("expected non-nil column")
	}
	if col.Collate != "NOCASE" {
		t.Errorf("expected COLLATE NOCASE, got %q", col.Collate)
	}
}

func TestOverIndexedTables(t *testing.T) {
	indexes := make([]Index, 6)
	for i := 0; i < 6; i++ {
		indexes[i] = Index{Name: fmt.Sprintf("idx_%d", i), Columns: []string{fmt.Sprintf("col%d", i)}}
	}
	table := Table{
		Name:    "over",
		Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}},
		Indexes: indexes,
	}
	s := &DatabaseSchema{Tables: []Table{table}}
	r := AnalyzeIndexes(s)
	if len(r.OverIndexedTables) != 1 {
		t.Errorf("expected 1 over-indexed table, got %d", len(r.OverIndexedTables))
	}
}

func TestRedundantIndexDetection(t *testing.T) {
	table := Table{
		Name:    "test",
		Columns: []Column{{Name: "a", Type: "INTEGER"}, {Name: "b", Type: "INTEGER"}},
		Indexes: []Index{
			{Name: "idx_a", Columns: []string{"a"}},
			{Name: "idx_ab", Columns: []string{"a", "b"}},
		},
	}
	s := &DatabaseSchema{Tables: []Table{table}}
	r := AnalyzeIndexes(s)
	if len(r.RedundantIndexes) == 0 {
		t.Error("expected redundant index detection for prefix")
	}
}

func TestPrintSchemaJSON(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "test", Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}}},
		},
	}
	output := PrintSchemaJSON(s)
	if !contains(output, "test") {
		t.Error("expected JSON output to contain table name")
	}
}

func TestPrintSchemaSQL(t *testing.T) {
	s := &DatabaseSchema{
		Tables: []Table{
			{Name: "test", CreateStmt: "CREATE TABLE test (id INTEGER PRIMARY KEY)", Columns: []Column{{Name: "id", Type: "INTEGER", PrimaryKey: true}}},
		},
	}
	output := PrintSchemaSQL(s)
	if !contains(output, "CREATE TABLE") {
		t.Error("expected SQL output to contain CREATE TABLE")
	}
}

func TestFormatDiffJSON(t *testing.T) {
	d := &DiffResult{
		Changes: []SchemaChange{
			{Type: ChangeAdd, ObjectType: "table", Name: "t1"},
		},
		AddCount: 1,
	}
	output := FormatDiffJSON(d)
	if !contains(output, "t1") {
		t.Error("expected JSON diff output to contain table name")
	}
}

func TestParseFromDB(t *testing.T) {
	t.Skip("ParseFromDB requires a SQLite database file")
}