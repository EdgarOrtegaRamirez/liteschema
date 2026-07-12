package viz

import (
	"strings"
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewRelationshipGraph()
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(g.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(g.Tables))
	}
}

func TestAddRelation(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddRelation(TableRelation{
		FromTable: "orders", FromColumn: "user_id",
		ToTable: "users", ToColumn: "id",
	})

	if len(g.Tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(g.Tables))
	}
	if len(g.Relations) != 1 {
		t.Errorf("expected 1 relation, got %d", len(g.Relations))
	}
}

func TestAddPrimaryKey(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.AddPrimaryKey("users", "email")

	pks := g.PrimaryKeys["users"]
	if len(pks) != 2 {
		t.Errorf("expected 2 PKs, got %d", len(pks))
	}
	if pks[0] != "id" || pks[1] != "email" {
		t.Errorf("unexpected PKs: %v", pks)
	}
}

func TestFormatASCII(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.AddRelation(TableRelation{
		FromTable: "orders", FromColumn: "user_id",
		ToTable: "users", ToColumn: "id",
	})

	output := g.FormatASCII()
	if !strings.Contains(output, "users") {
		t.Error("expected output to contain 'users'")
	}
	if !strings.Contains(output, "orders") {
		t.Error("expected output to contain 'orders'")
	}
	if !strings.Contains(output, "PK:") {
		t.Error("expected output to contain 'PK:'")
	}
}

func TestFormatASCIIEmpty(t *testing.T) {
	g := NewRelationshipGraph()
	output := g.FormatASCII()
	if !strings.Contains(output, "No tables found") {
		t.Errorf("expected 'No tables found', got: %q", output)
	}
}

func TestFormatMermaid(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.AddRelation(TableRelation{
		FromTable: "orders", FromColumn: "user_id",
		ToTable: "users", ToColumn: "id",
	})

	output := g.FormatMermaid()
	if !strings.HasPrefix(output, "erDiagram") {
		t.Errorf("expected 'erDiagram' prefix, got: %q", output[:min(len(output), 20)])
	}
	if !strings.Contains(output, "users") {
		t.Error("expected output to contain 'users'")
	}
	if !strings.Contains(output, "PK") {
		t.Error("expected output to contain 'PK'")
	}
}

func TestFormatDot(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.AddRelation(TableRelation{
		FromTable: "orders", FromColumn: "user_id",
		ToTable: "users", ToColumn: "id",
	})

	output := g.FormatDot()
	if !strings.HasPrefix(output, "digraph ER") {
		t.Errorf("expected 'digraph ER' prefix, got: %q", output[:min(len(output), 20)])
	}
	if !strings.Contains(output, "users") {
		t.Error("expected output to contain 'users'")
	}
	if !strings.Contains(output, "orders") {
		t.Error("expected output to contain 'orders'")
	}
	if !strings.Contains(output, "->") {
		t.Error("expected output to contain arrow '->'")
	}
}

func TestMultipleRelations(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.AddPrimaryKey("products", "id")
	g.AddRelation(TableRelation{FromTable: "orders", FromColumn: "user_id", ToTable: "users", ToColumn: "id"})
	g.AddRelation(TableRelation{FromTable: "orders", FromColumn: "product_id", ToTable: "products", ToColumn: "id"})

	if len(g.Relations) != 2 {
		t.Errorf("expected 2 relations, got %d", len(g.Relations))
	}
	if len(g.Tables) != 3 {
		t.Errorf("expected 3 tables, got %d", len(g.Tables))
	}

	ascii := g.FormatASCII()
	if !strings.Contains(ascii, "user_id → users.id") {
		t.Error("expected first FK in output")
	}
	if !strings.Contains(ascii, "product_id → products.id") {
		t.Error("expected second FK in output")
	}
}

func TestNoRelationsASCII(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddPrimaryKey("users", "id")
	g.addTable("users") // just ensure table is there

	output := g.FormatASCII()
	if !strings.Contains(output, "(no outgoing foreign keys)") {
		t.Errorf("expected 'no outgoing foreign keys', got: %q", output)
	}
}

func TestFormatDotEdgeLabel(t *testing.T) {
	g := NewRelationshipGraph()
	g.AddRelation(TableRelation{
		FromTable: "orders", FromColumn: "user_id",
		ToTable: "users", ToColumn: "id",
	})

	output := g.FormatDot()
	if !strings.Contains(output, `label="user_id.id"`) {
		t.Errorf("expected edge label, got: %q", output)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
