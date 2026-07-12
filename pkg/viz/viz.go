// Package viz provides relationship graph visualization in multiple formats
// (ASCII, Mermaid, Graphviz DOT) for foreign key dependencies.
package viz

import (
	"fmt"
	"strings"
)

// TableRelation represents a foreign key relationship between two tables.
type TableRelation struct {
	FromTable  string
	FromColumn string
	ToTable    string
	ToColumn   string
	OnDelete   string
	OnUpdate   string
}

// RelationshipGraph holds all table relationships for visualization.
type RelationshipGraph struct {
	Tables      []string
	Relations   []TableRelation
	PrimaryKeys map[string][]string // table -> pk columns
}

// NewRelationshipGraph creates a new graph.
func NewRelationshipGraph() *RelationshipGraph {
	return &RelationshipGraph{
		PrimaryKeys: make(map[string][]string),
	}
}

// AddRelation adds a foreign key relationship.
func (g *RelationshipGraph) AddRelation(r TableRelation) {
	g.Relations = append(g.Relations, r)
	g.addTable(r.FromTable)
	g.addTable(r.ToTable)
}

func (g *RelationshipGraph) addTable(name string) {
	for _, t := range g.Tables {
		if t == name {
			return
		}
	}
	g.Tables = append(g.Tables, name)
}

// AddPrimaryKey marks a column as primary key for a table.
func (g *RelationshipGraph) AddPrimaryKey(table, column string) {
	for _, c := range g.PrimaryKeys[table] {
		if c == column {
			return
		}
	}
	g.PrimaryKeys[table] = append(g.PrimaryKeys[table], column)
	g.addTable(table)
}

// FormatASCII renders the relationship graph as ASCII art.
func (g *RelationshipGraph) FormatASCII() string {
	if len(g.Tables) == 0 {
		return "No tables found."
	}

	var sb strings.Builder
	sb.WriteString("Table Relationships\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	bySource := make(map[string][]TableRelation)
	for _, r := range g.Relations {
		bySource[r.FromTable] = append(bySource[r.FromTable], r)
	}

	for _, table := range g.Tables {
		sb.WriteString(fmt.Sprintf("┌─ %s", table))
		if pks, ok := g.PrimaryKeys[table]; ok && len(pks) > 0 {
			sb.WriteString(fmt.Sprintf("  [PK: %s]", strings.Join(pks, ", ")))
		}
		sb.WriteString("\n")

		rels := bySource[table]
		if len(rels) == 0 {
			sb.WriteString("│  (no outgoing foreign keys)\n")
		} else {
			for _, r := range rels {
				line := fmt.Sprintf("│  %s → %s.%s", r.FromColumn, r.ToTable, r.ToColumn)
				if r.OnDelete != "" && r.OnDelete != "NO ACTION" {
					line += fmt.Sprintf(" [ON DELETE %s]", r.OnDelete)
				}
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("└" + strings.Repeat("─", 59) + "\n\n")
	}

	sb.WriteString(fmt.Sprintf("Total: %d tables, %d foreign key relationships\n", len(g.Tables), len(g.Relations)))
	return sb.String()
}

// FormatMermaid renders the relationship graph as a Mermaid ER diagram.
func (g *RelationshipGraph) FormatMermaid() string {
	var sb strings.Builder
	sb.WriteString("erDiagram\n")

	for _, table := range g.Tables {
		pks := g.PrimaryKeys[table]
		for _, pk := range pks {
			sb.WriteString(fmt.Sprintf("    %s {\n", table))
			sb.WriteString(fmt.Sprintf("        string %s PK\n", pk))
			sb.WriteString("    }\n")
			// Only generate one entry per table
			break
		}
		if len(pks) == 0 {
			sb.WriteString(fmt.Sprintf("    %s {\n", table))
			sb.WriteString("    }\n")
		}
	}

	for _, r := range g.Relations {
		sb.WriteString(fmt.Sprintf("    %s }|--|| %s : \"%s\"\n",
			r.ToTable, r.FromTable,
			fmt.Sprintf("%s.%s -> %s.%s", r.FromTable, r.FromColumn, r.ToTable, r.ToColumn)))
	}

	return sb.String()
}

// FormatDot renders the relationship graph as a Graphviz DOT diagram.
func (g *RelationshipGraph) FormatDot() string {
	var sb strings.Builder
	sb.WriteString("digraph ER {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=record];\n\n")

	for _, table := range g.Tables {
		pks := g.PrimaryKeys[table]
		sb.WriteString(fmt.Sprintf("  %s [label=\"{<f0> %s", table, table))
		if len(pks) > 0 {
			sb.WriteString(fmt.Sprintf("\\nPK: %s", strings.Join(pks, ", ")))
		}
		sb.WriteString("}\"];\n")
	}

	sb.WriteString("\n")

	for _, r := range g.Relations {
		sb.WriteString(fmt.Sprintf("  %s -> %s [label=\"%s.%s\"];\n",
			r.ToTable, r.FromTable, r.FromColumn, r.ToColumn))
	}

	sb.WriteString("}\n")
	return sb.String()
}