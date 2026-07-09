package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ChangeType describes the type of schema change.
type ChangeType string

const (
	ChangeAdd    ChangeType = "added"
	ChangeRemove ChangeType = "removed"
	ChangeModify ChangeType = "modified"
)

// SchemaChange represents a single schema change between two database schemas.
type SchemaChange struct {
	Type       ChangeType `json:"type"`
	ObjectType string     `json:"object_type"` // "table", "column", "index", "view", "trigger", "foreign_key"
	Table      string     `json:"table,omitempty"`
	Name       string     `json:"name"`
	Detail     string     `json:"detail,omitempty"`
}

// DiffResult contains the complete diff between two schemas.
type DiffResult struct {
	Changes      []SchemaChange `json:"changes"`
	AddCount     int            `json:"add_count"`
	RemoveCount  int            `json:"remove_count"`
	ModifyCount  int            `json:"modify_count"`
	HasBreaking  bool           `json:"has_breaking"`
}

// Diff computes a semantic diff between two database schemas.
func Diff(old, new *DatabaseSchema) *DiffResult {
	result := &DiffResult{}

	oldTables := make(map[string]Table)
	newTables := make(map[string]Table)
	for _, t := range old.Tables {
		oldTables[t.Name] = t
	}
	for _, t := range new.Tables {
		newTables[t.Name] = t
	}

	for _, nt := range new.Tables {
		if ot, exists := oldTables[nt.Name]; exists {
			diffTables(&ot, &nt, result)
		} else {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeAdd, ObjectType: "table", Name: nt.Name,
			})
			result.AddCount++
		}
	}
	for _, ot := range old.Tables {
		if _, exists := newTables[ot.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeRemove, ObjectType: "table", Name: ot.Name,
			})
			result.RemoveCount++
			result.HasBreaking = true
		}
	}

	oldViews := make(map[string]View)
	newViews := make(map[string]View)
	for _, v := range old.Views {
		oldViews[v.Name] = v
	}
	for _, v := range new.Views {
		newViews[v.Name] = v
	}
	for _, nv := range new.Views {
		if _, exists := oldViews[nv.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeAdd, ObjectType: "view", Name: nv.Name,
			})
			result.AddCount++
		}
	}
	for _, ov := range old.Views {
		if _, exists := newViews[ov.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeRemove, ObjectType: "view", Name: ov.Name,
			})
			result.RemoveCount++
		}
	}

	oldTriggers := make(map[string]Trigger)
	newTriggers := make(map[string]Trigger)
	for _, tr := range old.Triggers {
		oldTriggers[tr.Name] = tr
	}
	for _, tr := range new.Triggers {
		newTriggers[tr.Name] = tr
	}
	for _, nt := range new.Triggers {
		if _, exists := oldTriggers[nt.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeAdd, ObjectType: "trigger", Name: nt.Name,
			})
			result.AddCount++
		}
	}
	for _, ot := range old.Triggers {
		if _, exists := newTriggers[ot.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeRemove, ObjectType: "trigger", Name: ot.Name,
			})
			result.RemoveCount++
		}
	}

	sort.Slice(result.Changes, func(i, j int) bool {
		return result.Changes[i].ObjectType+result.Changes[i].Name <
			result.Changes[j].ObjectType+result.Changes[j].Name
	})

	return result
}

func diffTables(old, new *Table, result *DiffResult) {
	oldCols := make(map[string]Column)
	newCols := make(map[string]Column)
	for _, c := range old.Columns {
		oldCols[c.Name] = c
	}
	for _, c := range new.Columns {
		newCols[c.Name] = c
	}

	for _, nc := range new.Columns {
		if oc, exists := oldCols[nc.Name]; exists {
			change := diffColumn(old.Name, &oc, &nc)
			if change != nil {
				result.Changes = append(result.Changes, *change)
				result.ModifyCount++
			}
		} else {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeAdd, ObjectType: "column", Table: new.Name,
				Name: nc.Name, Detail: nc.Type,
			})
			result.AddCount++
		}
	}

	for _, oc := range old.Columns {
		if _, exists := newCols[oc.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeRemove, ObjectType: "column", Table: old.Name,
				Name: oc.Name, Detail: oc.Type,
			})
			result.RemoveCount++
			result.HasBreaking = true
		}
	}

	oldIdxs := make(map[string]Index)
	newIdxs := make(map[string]Index)
	for _, idx := range old.Indexes {
		oldIdxs[idx.Name] = idx
	}
	for _, idx := range new.Indexes {
		newIdxs[idx.Name] = idx
	}
	for _, ni := range new.Indexes {
		if _, exists := oldIdxs[ni.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeAdd, ObjectType: "index", Table: new.Name,
				Name: ni.Name, Detail: strings.Join(ni.Columns, ", "),
			})
			result.AddCount++
		}
	}
	for _, oi := range old.Indexes {
		if _, exists := newIdxs[oi.Name]; !exists {
			result.Changes = append(result.Changes, SchemaChange{
				Type: ChangeRemove, ObjectType: "index", Table: old.Name,
				Name: oi.Name, Detail: strings.Join(oi.Columns, ", "),
			})
			result.RemoveCount++
		}
	}

	if len(old.ForeignKeys) != len(new.ForeignKeys) {
		result.Changes = append(result.Changes, SchemaChange{
			Type: ChangeModify, ObjectType: "foreign_key", Table: old.Name,
			Name: fmt.Sprintf("%d→%d FKs", len(old.ForeignKeys), len(new.ForeignKeys)),
		})
		result.ModifyCount++
	}
}

func diffColumn(table string, old, new *Column) *SchemaChange {
	parts := make([]string, 0)
	if old.Type != new.Type {
		parts = append(parts, fmt.Sprintf("type: %s→%s", old.Type, new.Type))
	}
	if old.NotNull != new.NotNull {
		parts = append(parts, fmt.Sprintf("not_null: %v→%v", old.NotNull, new.NotNull))
	}
	if old.Default != new.Default {
		parts = append(parts, fmt.Sprintf("default: %s→%s", old.Default, new.Default))
	}
	if old.PrimaryKey != new.PrimaryKey {
		parts = append(parts, "pk changed")
	}
	if old.Unique != new.Unique {
		parts = append(parts, "unique changed")
	}
	if len(parts) == 0 {
		return nil
	}
	return &SchemaChange{
		Type: ChangeModify, ObjectType: "column", Table: table,
		Name: new.Name, Detail: strings.Join(parts, ", "),
	}
}

// FormatDiff formats the diff result in human-readable text.
func FormatDiff(d *DiffResult) string {
	var b strings.Builder
	if len(d.Changes) == 0 {
		return "No schema changes detected.\n"
	}

	b.WriteString(fmt.Sprintf("Schema Diff: %d added, %d removed, %d modified",
		d.AddCount, d.RemoveCount, d.ModifyCount))
	if d.HasBreaking {
		b.WriteString(" ⚠️ BREAKING CHANGES")
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")

	for _, c := range d.Changes {
		prefix := "  "
		location := c.Name
		if c.Table != "" {
			location = fmt.Sprintf("%s.%s", c.Table, c.Name)
		}

		switch c.Type {
		case ChangeAdd:
			prefix = "  + "
		case ChangeRemove:
			prefix = "  - "
		case ChangeModify:
			prefix = "  ~ "
		}
		b.WriteString(fmt.Sprintf("%s%s %s", prefix, c.ObjectType, location))
		if c.Detail != "" {
			b.WriteString(fmt.Sprintf(" (%s)", c.Detail))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatDiffJSON formats the diff result in JSON.
func FormatDiffJSON(d *DiffResult) string {
	data, _ := json.MarshalIndent(d, "", "  ")
	return string(data)
}