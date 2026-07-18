package schema

// Column represents a single column in a SQLite table.
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // INTEGER, TEXT, REAL, BLOB, or custom
	NotNull    bool   `json:"not_null"`
	Default    string `json:"default,omitempty"`
	PrimaryKey bool   `json:"primary_key"`
	AutoIncr   bool   `json:"auto_increment,omitempty"`
	Unique     bool   `json:"unique,omitempty"`
	Check      string `json:"check,omitempty"`
	Collate    string `json:"collate,omitempty"`
}

// Index represents a database index.
type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
	Where   string   `json:"where,omitempty"`
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
	OnDelete   string   `json:"on_delete,omitempty"` // CASCADE, SET NULL, SET DEFAULT, RESTRICT, NO ACTION
	OnUpdate   string   `json:"on_update,omitempty"`
}

// Trigger represents a SQLite trigger.
type Trigger struct {
	Name    string `json:"name"`
	Time    string `json:"time"`  // BEFORE, AFTER, INSTEAD OF
	Event   string `json:"event"` // INSERT, UPDATE, DELETE
	Table   string `json:"table"`
	ForEach string `json:"for_each"` // ROW, STATEMENT
	Body    string `json:"body"`
}

// View represents a SQL view.
type View struct {
	Name    string `json:"name"`
	Content string `json:"content"` // CREATE VIEW statement
}

// Table represents a SQLite table with its columns, indexes, and constraints.
type Table struct {
	Name         string       `json:"name"`
	Columns      []Column     `json:"columns"`
	Indexes      []Index      `json:"indexes,omitempty"`
	ForeignKeys  []ForeignKey `json:"foreign_keys,omitempty"`
	WithoutRowID bool         `json:"without_rowid,omitempty"`
	CreateStmt   string       `json:"create_stmt,omitempty"`
}

// DatabaseSchema represents the complete schema of a SQLite database.
type DatabaseSchema struct {
	Tables   []Table   `json:"tables"`
	Indexes  []Index   `json:"indexes,omitempty"`
	Views    []View    `json:"views,omitempty"`
	Triggers []Trigger `json:"triggers,omitempty"`
}
