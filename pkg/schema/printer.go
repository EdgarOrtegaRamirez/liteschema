package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PrintSchema prints a human-readable ASCII tree of the database schema.
func PrintSchema(s *DatabaseSchema, showSQL bool) string {
	var b strings.Builder
	if len(s.Tables) == 0 && len(s.Views) == 0 && len(s.Triggers) == 0 && len(s.Indexes) == 0 {
		return "No schema objects found."
	}

	b.WriteString("📊 Database Schema\n")
	b.WriteString(strings.Repeat("━", 60))
	b.WriteString("\n\n")

	for i, t := range s.Tables {
		b.WriteString(fmt.Sprintf("┌─ TABLE: %s", t.Name))
		if t.WithoutRowID {
			b.WriteString(" (WITHOUT ROWID)")
		}
		b.WriteString("\n")

		for _, col := range t.Columns {
			b.WriteString(fmt.Sprintf("│  ├─ %s", col.Name))
			if col.Type != "" {
				b.WriteString(fmt.Sprintf("  %s", col.Type))
			}
			var constraints []string
			if col.PrimaryKey {
				constraints = append(constraints, "PK")
			}
			if col.NotNull {
				constraints = append(constraints, "NOT NULL")
			}
			if col.Unique {
				constraints = append(constraints, "UNIQUE")
			}
			if col.AutoIncr {
				constraints = append(constraints, "AUTOINCREMENT")
			}
			if col.Default != "" {
				constraints = append(constraints, "DEFAULT "+col.Default)
			}
			if col.Collate != "" {
				constraints = append(constraints, "COLLATE "+col.Collate)
			}
			if len(constraints) > 0 {
				b.WriteString(fmt.Sprintf("  [%s]", strings.Join(constraints, ", ")))
			}
			b.WriteString("\n")
		}

		for _, idx := range t.Indexes {
			b.WriteString(fmt.Sprintf("│  ├─ INDEX: %s (%s)", idx.Name, strings.Join(idx.Columns, ", ")))
			if idx.Unique {
				b.WriteString(" UNIQUE")
			}
			if idx.Where != "" {
				b.WriteString(fmt.Sprintf(" WHERE %s", idx.Where))
			}
			b.WriteString("\n")
		}

		for _, fk := range t.ForeignKeys {
			b.WriteString(fmt.Sprintf("│  ├─ FK: %s → %s(%s)", strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", ")))
			if fk.OnDelete != "" {
				b.WriteString(fmt.Sprintf(" ON DELETE %s", fk.OnDelete))
			}
			if fk.OnUpdate != "" {
				b.WriteString(fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate))
			}
			b.WriteString("\n")
		}

		if i < len(s.Tables)-1 {
			b.WriteString("│\n")
		} else {
			b.WriteString("\n")
		}

		if showSQL && t.CreateStmt != "" {
			b.WriteString(fmt.Sprintf("  SQL: %s\n\n", t.CreateStmt))
		}
	}

	if len(s.Views) > 0 {
		b.WriteString("┌─ VIEWS\n")
		for _, v := range s.Views {
			b.WriteString(fmt.Sprintf("├─ %s\n", v.Name))
		}
		b.WriteString("\n")
	}

	if len(s.Triggers) > 0 {
		b.WriteString("┌─ TRIGGERS\n")
		for _, tr := range s.Triggers {
			b.WriteString(fmt.Sprintf("├─ %s (%s %s ON %s)\n", tr.Name, tr.Time, tr.Event, tr.Table))
		}
		b.WriteString("\n")
	}

	if len(s.Indexes) > 0 {
		b.WriteString("┌─ STANDALONE INDEXES\n")
		for _, idx := range s.Indexes {
			b.WriteString(fmt.Sprintf("├─ %s (%s)", idx.Name, strings.Join(idx.Columns, ", ")))
			if idx.Unique {
				b.WriteString(" UNIQUE")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(strings.Repeat("━", 60))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Summary: %d tables, %d views, %d triggers, %d indexes\n",
		len(s.Tables), len(s.Views), len(s.Triggers), len(s.Indexes)+countTableIndexes(s.Tables)))

	return b.String()
}

func countTableIndexes(tables []Table) int {
	count := 0
	for _, t := range tables {
		count += len(t.Indexes)
	}
	return count
}

// PrintSchemaJSON returns the schema in JSON format.
func PrintSchemaJSON(s *DatabaseSchema) string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

// PrintSchemaMarkdown returns the schema in Markdown format.
func PrintSchemaMarkdown(s *DatabaseSchema) string {
	var b strings.Builder
	b.WriteString("# Database Schema\n\n")

	for _, t := range s.Tables {
		b.WriteString(fmt.Sprintf("## Table: `%s`\n\n", t.Name))
		b.WriteString("| Column | Type | Constraints |\n")
		b.WriteString("|--------|------|-------------|\n")
		for _, col := range t.Columns {
			var constraints []string
			if col.PrimaryKey {
				constraints = append(constraints, "PK")
			}
			if col.NotNull {
				constraints = append(constraints, "NOT NULL")
			}
			if col.Unique {
				constraints = append(constraints, "UNIQUE")
			}
			if col.AutoIncr {
				constraints = append(constraints, "AUTOINCREMENT")
			}
			if col.Default != "" {
				constraints = append(constraints, "DEFAULT "+
					strings.ReplaceAll(col.Default, "|", "\\|"))
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", col.Name, col.Type, strings.Join(constraints, ", ")))
		}

		if len(t.Indexes) > 0 {
			b.WriteString("\n**Indexes:**\n\n")
			for _, idx := range t.Indexes {
				cols := strings.Join(idx.Columns, ", ")
				b.WriteString(fmt.Sprintf("- `%s` on (%s)", idx.Name, cols))
				if idx.Unique {
					b.WriteString(" (UNIQUE)")
				}
				b.WriteString("\n")
			}
		}

		if len(t.ForeignKeys) > 0 {
			b.WriteString("\n**Foreign Keys:**\n\n")
			for _, fk := range t.ForeignKeys {
				b.WriteString(fmt.Sprintf("- (%s) → `%s`(%s)", strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", ")))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// PrintSchemaSQL returns the CREATE statements in SQL format.
func PrintSchemaSQL(s *DatabaseSchema) string {
	var b strings.Builder
	for _, t := range s.Tables {
		if t.CreateStmt != "" {
			b.WriteString(t.CreateStmt)
			b.WriteString(";\n\n")
		}
	}
	for _, v := range s.Views {
		if v.Content != "" {
			b.WriteString(v.Content)
			b.WriteString(";\n\n")
		}
	}
	return b.String()
}
