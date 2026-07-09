package schema

import (
	"fmt"
	"strings"
)

// MigrationGenerator generates ALTER TABLE migration SQL from a diff result.
type MigrationGenerator struct {
	IncludeDropTable bool
	IncludeDropCol   bool
}

// NewMigrationGenerator creates a new migration generator.
func NewMigrationGenerator() *MigrationGenerator {
	return &MigrationGenerator{
		IncludeDropTable: false,
		IncludeDropCol:   false,
	}
}

// Generate generates migration SQL statements from a diff result.
func (g *MigrationGenerator) Generate(d *DiffResult) string {
	var b strings.Builder
	b.WriteString("-- Generated migration SQL\n")
	b.WriteString("-- " + FormatDiff(d))
	b.WriteString("\n")

	for _, c := range d.Changes {
		switch c.Type {
		case ChangeAdd:
			switch c.ObjectType {
			case "table":
				// We can't recreate a table — just note it
				b.WriteString(fmt.Sprintf("-- TODO: CREATE TABLE %s (see schema definition)\n", c.Name))
			case "column":
				b.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;\n", c.Table, c.Name, c.Detail))
			case "index":
				b.WriteString(fmt.Sprintf("CREATE INDEX %s ON %s (%s);\n", c.Name, c.Table, c.Detail))
			}

		case ChangeRemove:
			switch c.ObjectType {
			case "table":
				if g.IncludeDropTable {
					b.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", c.Name))
				} else {
					b.WriteString(fmt.Sprintf("-- DROP TABLE IF EXISTS %s; -- manual review required\n", c.Name))
				}
			case "column":
				if g.IncludeDropCol {
					b.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;\n", c.Table, c.Name))
				} else {
					b.WriteString(fmt.Sprintf("-- ALTER TABLE %s DROP COLUMN %s; -- manual review required\n", c.Table, c.Name))
				}
			case "index":
				b.WriteString(fmt.Sprintf("DROP INDEX IF EXISTS %s;\n", c.Name))
			case "view":
				b.WriteString(fmt.Sprintf("DROP VIEW IF EXISTS %s;\n", c.Name))
			case "trigger":
				b.WriteString(fmt.Sprintf("DROP TRIGGER IF EXISTS %s;\n", c.Name))
			}

		case ChangeModify:
			switch c.ObjectType {
			case "column":
				// SQLite doesn't support ALTER COLUMN — requires table recreation
				b.WriteString(fmt.Sprintf("-- ALTER TABLE %s MODIFY COLUMN %s; -- requires table recreation\n", c.Table, c.Name))
			}
		}
	}

	return b.String()
}

// ForeignKeyGraph represents the foreign key dependency graph.
type ForeignKeyGraph struct {
	Nodes []string          `json:"nodes"`
	Edges []FKEdge          `json:"edges"`
	Cycles [][]string       `json:"cycles,omitempty"`
}

// FKEdge represents a foreign key relationship.
type FKEdge struct {
	FromTable string `json:"from_table"`
	FromCols  []string `json:"from_cols"`
	ToTable   string `json:"to_table"`
	ToCols    []string `json:"to_cols"`
}

// BuildFKGraph builds a foreign key dependency graph from a schema.
func BuildFKGraph(s *DatabaseSchema) *ForeignKeyGraph {
	graph := &ForeignKeyGraph{}
	tableSet := make(map[string]bool)

	for _, t := range s.Tables {
		tableSet[t.Name] = true
		for _, fk := range t.ForeignKeys {
			graph.Edges = append(graph.Edges, FKEdge{
				FromTable: t.Name,
				FromCols:  fk.Columns,
				ToTable:   fk.RefTable,
				ToCols:    fk.RefColumns,
			})
		}
	}

	for t := range tableSet {
		graph.Nodes = append(graph.Nodes, t)
	}

	// Detect cycles using DFS
	graph.Cycles = detectCycles(graph.Nodes, graph.Edges)

	return graph
}

func detectCycles(nodes []string, edges []FKEdge) [][]string {
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.FromTable] = append(adj[e.FromTable], e.ToTable)
	}

	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				dfs(neighbor)
			} else if recStack[neighbor] {
				// Found a cycle
				cycle := make([]string, 0)
				started := false
				for _, p := range path {
					if p == neighbor {
						started = true
					}
					if started {
						cycle = append(cycle, p)
					}
				}
				if len(cycle) > 1 {
					cycles = append(cycles, cycle)
				}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
	}

	for _, node := range nodes {
		if !visited[node] {
			dfs(node)
		}
	}

	return cycles
}

// FormatFKGraph formats the foreign key graph in human-readable text.
func FormatFKGraph(g *ForeignKeyGraph) string {
	var b strings.Builder
	b.WriteString("Foreign Key Dependency Graph\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")

	if len(g.Edges) == 0 {
		b.WriteString("No foreign key relationships.\n")
		return b.String()
	}

	for _, e := range g.Edges {
		b.WriteString(fmt.Sprintf("  %s(%s) → %s(%s)\n",
			e.FromTable, strings.Join(e.FromCols, ", "),
			e.ToTable, strings.Join(e.ToCols, ", ")))
	}

	if len(g.Cycles) > 0 {
		b.WriteString("\n⚠️ Circular Dependencies Detected:\n")
		for _, cycle := range g.Cycles {
			b.WriteString(fmt.Sprintf("  %s\n", strings.Join(cycle, " → ")))
		}
	}

	b.WriteString(fmt.Sprintf("\n%d nodes, %d edges\n", len(g.Nodes), len(g.Edges)))
	return b.String()
}

// IndexAnalysisResult contains analysis of indexes in a schema.
type IndexAnalysisResult struct {
	RedundantIndexes []RedundantIndex `json:"redundant_indexes"`
	MissingIndexes   []MissingIndex   `json:"missing_indexes"`
	UnusedIndexes    []string         `json:"unused_indexes"`
	OverIndexedTables []string        `json:"over_indexed_tables"`
}

// RedundantIndex describes an index that is redundant with another.
type RedundantIndex struct {
	Index1    string `json:"index1"`
	Index2    string `json:"index2"`
	Table     string `json:"table"`
	Reason    string `json:"reason"`
}

// MissingIndex describes a potentially missing index.
type MissingIndex struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
	Reason  string   `json:"reason"`
}

// AnalyzeIndexes analyzes indexes in a schema for potential issues.
func AnalyzeIndexes(s *DatabaseSchema) *IndexAnalysisResult {
	result := &IndexAnalysisResult{}

	for _, t := range s.Tables {
		// Check for redundant indexes
		for i := 0; i < len(t.Indexes); i++ {
			for j := i + 1; j < len(t.Indexes); j++ {
				idx1, idx2 := t.Indexes[i], t.Indexes[j]
				if isPrefix(idx1.Columns, idx2.Columns) {
					result.RedundantIndexes = append(result.RedundantIndexes, RedundantIndex{
						Index1: idx1.Name, Index2: idx2.Name, Table: t.Name,
						Reason: fmt.Sprintf("%s is a prefix of %s", strings.Join(idx1.Columns, ","), strings.Join(idx2.Columns, ",")),
					})
				} else if isPrefix(idx2.Columns, idx1.Columns) {
					result.RedundantIndexes = append(result.RedundantIndexes, RedundantIndex{
						Index1: idx2.Name, Index2: idx1.Name, Table: t.Name,
						Reason: fmt.Sprintf("%s is a prefix of %s", strings.Join(idx2.Columns, ","), strings.Join(idx1.Columns, ",")),
					})
				}
			}
		}

		// Check for missing indexes on foreign key columns
		for _, fk := range t.ForeignKeys {
			for _, col := range fk.Columns {
				if !hasIndexOnColumn(t.Indexes, col) {
					result.MissingIndexes = append(result.MissingIndexes, MissingIndex{
						Table: t.Name, Columns: fk.Columns,
						Reason: fmt.Sprintf("foreign key column %s referenced by %s", col, fk.RefTable),
					})
				}
			}
		}

		// Over-indexed tables
		if len(t.Indexes) > 5 {
			result.OverIndexedTables = append(result.OverIndexedTables, t.Name)
		}
	}

	return result
}

func isPrefix(a, b []string) bool {
	if len(a) >= len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasIndexOnColumn(indexes []Index, col string) bool {
	for _, idx := range indexes {
		for _, c := range idx.Columns {
			if c == col {
				return true
			}
		}
	}
	return false
}

// FormatIndexAnalysis formats the index analysis in human-readable text.
func FormatIndexAnalysis(r *IndexAnalysisResult) string {
	var b strings.Builder
	b.WriteString("Index Analysis\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")

	if len(r.RedundantIndexes) == 0 && len(r.MissingIndexes) == 0 && len(r.OverIndexedTables) == 0 {
		b.WriteString("No issues found.\n")
		return b.String()
	}

	if len(r.RedundantIndexes) > 0 {
		b.WriteString("Redundant Indexes:\n")
		for _, ri := range r.RedundantIndexes {
			b.WriteString(fmt.Sprintf("  ⚠ %s.%s — %s\n", ri.Table, ri.Index1, ri.Reason))
		}
		b.WriteString("\n")
	}

	if len(r.MissingIndexes) > 0 {
		b.WriteString("Potentially Missing Indexes:\n")
		for _, mi := range r.MissingIndexes {
			b.WriteString(fmt.Sprintf("  ⚠ %s(%s) — %s\n", mi.Table, strings.Join(mi.Columns, ", "), mi.Reason))
		}
		b.WriteString("\n")
	}

	if len(r.OverIndexedTables) > 0 {
		b.WriteString("Over-Indexed Tables (>5 indexes):\n")
		for _, t := range r.OverIndexedTables {
			b.WriteString(fmt.Sprintf("  ⚠ %s\n", t))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ValidationResult contains schema validation results.
type ValidationResult struct {
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// ValidationIssue represents a single schema validation issue.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" or "warning"
	Table    string `json:"table,omitempty"`
	Column   string `json:"column,omitempty"`
	Message  string `json:"message"`
}

// ValidateSchema validates a schema for common issues.
func ValidateSchema(s *DatabaseSchema) *ValidationResult {
	r := &ValidationResult{}

	for _, t := range s.Tables {
		// Check for missing primary key
		hasPK := false
		for _, col := range t.Columns {
			if col.PrimaryKey {
				hasPK = true
				break
			}
		}
		if !hasPK && !t.WithoutRowID {
			r.Warnings = append(r.Warnings, ValidationIssue{
				Severity: "warning", Table: t.Name,
				Message: "table has no explicit primary key (uses implicit rowid)",
			})
		}

		// Check column naming conventions
		for _, col := range t.Columns {
			if strings.Contains(col.Name, " ") {
				r.Warnings = append(r.Warnings, ValidationIssue{
					Severity: "warning", Table: t.Name, Column: col.Name,
					Message: "column name contains spaces (requires quoting)",
				})
			}
		}

		// Check for orphaned foreign keys
		for _, fk := range t.ForeignKeys {
			found := false
			for _, ref := range s.Tables {
				if ref.Name == fk.RefTable {
					found = true
					break
				}
			}
			if !found {
				r.Errors = append(r.Errors, ValidationIssue{
					Severity: "error", Table: t.Name,
					Message: fmt.Sprintf("foreign key references non-existent table %q", fk.RefTable),
				})
			}
		}
	}

	return r
}

// FormatValidation formats the validation result in human-readable text.
func FormatValidation(r *ValidationResult) string {
	var b strings.Builder
	b.WriteString("Schema Validation\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")

	if len(r.Errors) == 0 && len(r.Warnings) == 0 {
		b.WriteString("✅ No issues found.\n")
		return b.String()
	}

	for _, err := range r.Errors {
		loc := err.Table
		if err.Column != "" {
			loc += "." + err.Column
		}
		b.WriteString(fmt.Sprintf("  ❌ %s — %s\n", loc, err.Message))
	}
	for _, warn := range r.Warnings {
		loc := warn.Table
		if warn.Column != "" {
			loc += "." + warn.Column
		}
		b.WriteString(fmt.Sprintf("  ⚠ %s — %s\n", loc, warn.Message))
	}

	b.WriteString(fmt.Sprintf("\n%d errors, %d warnings\n", len(r.Errors), len(r.Warnings)))
	return b.String()
}