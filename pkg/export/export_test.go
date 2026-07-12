package export

import (
	"bytes"
	"strings"
	"testing"
)

func TestExportJSON(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice", "active": true},
		{"id": int64(2), "name": "Bob", "active": false},
	}

	var buf bytes.Buffer
	err := ExportJSON(&buf, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Alice") {
		t.Error("expected output to contain 'Alice'")
	}
	if !strings.Contains(output, "Bob") {
		t.Error("expected output to contain 'Bob'")
	}
}

func TestExportCSV(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}

	var buf bytes.Buffer
	err := ExportCSV(&buf, []string{"id", "name"}, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "id,name") {
		t.Error("expected CSV header 'id,name', got:", output)
	}
	if !strings.Contains(output, "1,Alice") {
		t.Error("expected row '1,Alice', got:", output)
	}
}

func TestExportCSVEmptyRows(t *testing.T) {
	var buf bytes.Buffer
	err := ExportCSV(&buf, []string{"id", "name"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output != "id,name\n" {
		t.Errorf("expected just header, got: %q", output)
	}
}

func TestExportSQL(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice", "score": 95.5},
		{"id": int64(2), "name": "Bob", "score": nil},
	}

	var buf bytes.Buffer
	err := ExportSQL(&buf, "users", []string{"id", "name", "score"}, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INSERT INTO users") {
		t.Error("expected INSERT INTO users, got:", output)
	}
	if !strings.Contains(output, "NULL") {
		t.Error("expected NULL value, got:", output)
	}
	if !strings.Contains(output, "95.500000") {
		t.Error("expected float value, got:", output)
	}
}

func TestExportSQLEscape(t *testing.T) {
	rows := []map[string]interface{}{
		{"name": "O'Brien"},
	}

	var buf bytes.Buffer
	err := ExportSQL(&buf, "users", []string{"name"}, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "O''Brien") {
		t.Errorf("expected escaped single quote, got: %q", output)
	}
}

func TestExportJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := ExportJSON(&buf, []map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "[]") {
		t.Errorf("expected empty array, got: %q", output)
	}
}