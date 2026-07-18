package schema

import (
	"fmt"
	"math"
	"strings"
)

// ColumnStats holds statistics for a single column.
type ColumnStats struct {
	TableName  string
	ColumnName string
	ColumnType string
	Count      int64
	NullCount  int64
	NonNull    int64
	Min        interface{}
	Max        interface{}
	Avg        *float64
	Distinct   int64
	MinLength  *int
	MaxLength  *int
	AvgLength  *float64
	TopValues  []ValueCount
	Histogram  []HistogramBucket
}

// ValueCount represents a value and its frequency.
type ValueCount struct {
	Value interface{}
	Count int64
}

// HistogramBucket represents a range in a histogram.
type HistogramBucket struct {
	Min   float64
	Max   float64
	Count int64
	Label string
}

// TableStats holds statistics for an entire table.
type TableStats struct {
	TableName   string
	RowCount    int64
	ColumnCount int
	Columns     []ColumnStats
	SizeBytes   int64
}

// QueryExecutor is an interface for executing queries against a database.
type QueryExecutor interface {
	Query(query string, args ...interface{}) ([]map[string]interface{}, error)
	QueryScalar(query string, args ...interface{}) (interface{}, error)
}

// ProfileColumn computes statistics for a single column using SQL.
func ProfileColumn(db QueryExecutor, tableName, colName, colType string) (*ColumnStats, error) {
	stats := &ColumnStats{
		TableName:  tableName,
		ColumnName: colName,
		ColumnType: colType,
	}

	qt := quoteIdent(tableName)
	qc := quoteIdent(colName)
	quoted := fmt.Sprintf("%s.%s", qt, qc)

	// Count total rows
	total, err := db.QueryScalar(fmt.Sprintf("SELECT COUNT(*) FROM %s", qt))
	if err != nil {
		return nil, err
	}
	stats.Count = toInt64(total)

	// Count nulls
	nullCount, err := db.QueryScalar(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IS NULL", qt, qc))
	if err != nil {
		return nil, err
	}
	stats.NullCount = toInt64(nullCount)
	stats.NonNull = stats.Count - stats.NullCount

	// Count distinct
	distinct, err := db.QueryScalar(fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s", qc, qt))
	if err != nil {
		return nil, err
	}
	stats.Distinct = toInt64(distinct)

	if isNumericType(colType) {
		min, _ := db.QueryScalar(fmt.Sprintf("SELECT MIN(CAST(%s AS REAL)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if min != nil {
			stats.Min = min
		}
		max, _ := db.QueryScalar(fmt.Sprintf("SELECT MAX(CAST(%s AS REAL)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if max != nil {
			stats.Max = max
		}
		avg, _ := db.QueryScalar(fmt.Sprintf("SELECT AVG(CAST(%s AS REAL)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if avg != nil {
			f := toFloat64(avg)
			stats.Avg = &f
		}
	} else {
		min, _ := db.QueryScalar(fmt.Sprintf("SELECT MIN(%s) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if min != nil {
			stats.Min = min
		}
		max, _ := db.QueryScalar(fmt.Sprintf("SELECT MAX(%s) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if max != nil {
			stats.Max = max
		}

		minLen, _ := db.QueryScalar(fmt.Sprintf("SELECT MIN(LENGTH(%s)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if minLen != nil {
			l := int(toInt64(minLen))
			stats.MinLength = &l
		}
		maxLen, _ := db.QueryScalar(fmt.Sprintf("SELECT MAX(LENGTH(%s)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if maxLen != nil {
			l := int(toInt64(maxLen))
			stats.MaxLength = &l
		}
		avgLen, _ := db.QueryScalar(fmt.Sprintf("SELECT AVG(LENGTH(%s)) FROM %s WHERE %s IS NOT NULL", quoted, qt, qc))
		if avgLen != nil {
			f := toFloat64(avgLen)
			stats.AvgLength = &f
		}
	}

	// Top 5 most common values
	rows, err := db.Query(
		fmt.Sprintf("SELECT %s AS val, COUNT(*) AS cnt FROM %s WHERE %s IS NOT NULL GROUP BY %s ORDER BY cnt DESC LIMIT 5",
			quoted, qt, qc, quoted),
	)
	if err == nil {
		for _, row := range rows {
			vc := ValueCount{Value: row["val"], Count: toInt64(row["cnt"])}
			stats.TopValues = append(stats.TopValues, vc)
		}
	}

	// Build histogram for numeric columns
	if isNumericType(colType) && stats.NonNull > 0 && stats.Min != nil && stats.Max != nil {
		minVal := toFloat64(stats.Min)
		maxVal := toFloat64(stats.Max)
		if minVal < maxVal {
			buckets := 10
			step := (maxVal - minVal) / float64(buckets)
			if step > 0 {
				for i := 0; i < buckets; i++ {
					lo := minVal + float64(i)*step
					hi := minVal + float64(i+1)*step
					label := fmt.Sprintf("[%.1f, %.1f)", lo, hi)

					bucketCount, _ := db.QueryScalar(
						fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s >= %f AND %s < %f",
							qt, quoted, lo, quoted, hi),
					)
					stats.Histogram = append(stats.Histogram, HistogramBucket{
						Min: lo, Max: hi, Count: toInt64(bucketCount), Label: label,
					})
				}
			}
		}
	}

	return stats, nil
}

// ProfileTable computes statistics for all columns in a table.
func ProfileTable(db QueryExecutor, tableName string) (*TableStats, error) {
	tableInfo := &TableStats{TableName: tableName}

	count, err := db.QueryScalar(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(tableName)))
	if err != nil {
		return nil, err
	}
	tableInfo.RowCount = toInt64(count)

	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(tableName)))
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		colName := fmt.Sprintf("%v", row["name"])
		colType := fmt.Sprintf("%v", row["type"])
		stats, err := ProfileColumn(db, tableName, colName, colType)
		if err != nil {
			continue
		}
		tableInfo.Columns = append(tableInfo.Columns, *stats)
	}

	tableInfo.ColumnCount = len(tableInfo.Columns)
	return tableInfo, nil
}

// FormatProfileText formats table statistics as readable text.
func FormatProfileText(ts *TableStats) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Table: %s\n", ts.TableName))
	sb.WriteString(fmt.Sprintf("Rows: %d\n", ts.RowCount))
	sb.WriteString(fmt.Sprintf("Columns: %d\n\n", ts.ColumnCount))

	for _, col := range ts.Columns {
		sb.WriteString(fmt.Sprintf("  %s (%s)\n", col.ColumnName, col.ColumnType))
		sb.WriteString(fmt.Sprintf("    Count: %d  Nulls: %d  Distinct: %d\n", col.Count, col.NullCount, col.Distinct))

		if col.Min != nil {
			sb.WriteString(fmt.Sprintf("    Min: %v  Max: %v", col.Min, col.Max))
			if col.Avg != nil {
				sb.WriteString(fmt.Sprintf("  Avg: %.2f", *col.Avg))
			}
			sb.WriteString("\n")
		}

		if col.MinLength != nil {
			sb.WriteString(fmt.Sprintf("    Min Len: %d  Max Len: %d", *col.MinLength, *col.MaxLength))
			if col.AvgLength != nil {
				sb.WriteString(fmt.Sprintf("  Avg Len: %.1f", *col.AvgLength))
			}
			sb.WriteString("\n")
		}

		if len(col.TopValues) > 0 {
			sb.WriteString("    Top values:\n")
			for _, vc := range col.TopValues {
				sb.WriteString(fmt.Sprintf("      %v: %d\n", vc.Value, vc.Count))
			}
		}

		if len(col.Histogram) > 0 {
			sb.WriteString("    Histogram:\n")
			maxCount := int64(0)
			for _, b := range col.Histogram {
				if b.Count > maxCount {
					maxCount = b.Count
				}
			}
			for _, b := range col.Histogram {
				barLen := 0
				if maxCount > 0 {
					barLen = int(float64(b.Count) / float64(maxCount) * 20)
				}
				sb.WriteString(fmt.Sprintf("      %20s | %s (%d)\n", b.Label, strings.Repeat("#", barLen), b.Count))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatProfileJSON formats table statistics as JSON-serializable data.
func FormatProfileJSON(ts *TableStats) map[string]interface{} {
	result := map[string]interface{}{
		"table":        ts.TableName,
		"row_count":    ts.RowCount,
		"column_count": ts.ColumnCount,
	}

	columns := []map[string]interface{}{}
	for _, col := range ts.Columns {
		cd := map[string]interface{}{
			"name":       col.ColumnName,
			"type":       col.ColumnType,
			"count":      col.Count,
			"null_count": col.NullCount,
			"distinct":   col.Distinct,
		}
		if col.Min != nil {
			cd["min"] = col.Min
			cd["max"] = col.Max
		}
		if col.Avg != nil {
			cd["avg"] = *col.Avg
		}
		if col.MinLength != nil {
			cd["min_length"] = *col.MinLength
			cd["max_length"] = *col.MaxLength
		}
		if len(col.TopValues) > 0 {
			tops := []map[string]interface{}{}
			for _, vc := range col.TopValues {
				tops = append(tops, map[string]interface{}{"value": vc.Value, "count": vc.Count})
			}
			cd["top_values"] = tops
		}
		columns = append(columns, cd)
	}
	result["columns"] = columns
	return result
}

func quoteIdent(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func isNumericType(t string) bool {
	t = strings.ToUpper(t)
	return strings.Contains(t, "INT") ||
		strings.Contains(t, "REAL") ||
		strings.Contains(t, "FLOAT") ||
		strings.Contains(t, "DOUBLE") ||
		strings.Contains(t, "NUMERIC") ||
		strings.Contains(t, "DECIMAL")
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return math.NaN()
	}
}
