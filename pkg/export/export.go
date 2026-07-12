package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ExportJSON writes rows as formatted JSON.
func ExportJSON(w io.Writer, rows []map[string]interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// ExportSQL writes rows as SQL INSERT statements.
func ExportSQL(w io.Writer, table string, columns []string, rows []map[string]interface{}) error {
	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, col := range columns {
			v := row[col]
			switch val := v.(type) {
			case nil:
				vals[i] = "NULL"
			case string:
				vals[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
			case int64:
				vals[i] = fmt.Sprintf("%d", val)
			case float64:
				vals[i] = fmt.Sprintf("%f", val)
			default:
				vals[i] = fmt.Sprintf("'%v'", val)
			}
		}
		fmt.Fprintf(w, "INSERT INTO %s (%s) VALUES (%s);\n",
			table,
			strings.Join(columns, ", "),
			strings.Join(vals, ", "))
	}
	return nil
}

// ExportCSV writes rows as CSV with headers.
func ExportCSV(w io.Writer, columns []string, rows []map[string]interface{}) error {
	writer := csv.NewWriter(w)

	// Write header
	if err := writer.Write(columns); err != nil {
		return err
	}

	// Write rows
	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			v := row[col]
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}
