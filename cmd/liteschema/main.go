package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/EdgarOrtegaRamirez/liteschema/pkg/export"
	"github.com/EdgarOrtegaRamirez/liteschema/pkg/schema"
	"github.com/EdgarOrtegaRamirez/liteschema/pkg/sqlite"
	"github.com/EdgarOrtegaRamirez/liteschema/pkg/viz"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return nil
	}

	cmd := args[0]
	switch cmd {
	case "parse":
		return parseSchema(args[1:])
	case "diff":
		return diffSchemas(args[1:])
	case "migrate":
		return generateMigration(args[1:])
	case "analyze":
		return analyzeSchema(args[1:])
	case "validate":
		return validateSchema(args[1:])
	case "fkgraph":
		return fkGraph(args[1:])
	case "query":
		return executeQuery(args[1:])
	case "profile":
		return profileDatabase(args[1:])
	case "export":
		return exportData(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return parseSchema(args)
	}
}

func printUsage() {
	fmt.Println(`LiteSchema — SQLite Schema Analysis & Migration CLI

Usage:
  liteschema <command> [options] <args>

Schema Commands:
  parse <file>              Parse and display a SQLite schema
  diff <old> <new>          Compute schema diff between two files
  migrate <old> <new>       Generate migration SQL from diff
  analyze <file>            Analyze indexes and schema health
  validate <file>           Validate schema for common issues
  fkgraph <file>            Display foreign key dependency graph

Data Commands:
  query <database> <sql>    Execute SQL queries with formatted output
  profile <database> [table]  Show table statistics and data profiling
  export <database> <table>   Export table data to various formats

Options:
  --format text|json|markdown|sql|mermaid|dot|csv  Output format
  --show-sql                    Include CREATE statements
  --file <path>                 Read SQL from file (query)
  --output <file>               Output file (export)
  --limit <n>                   Limit rows (export)

Examples:
  liteschema parse schema.sql
  liteschema query myapp.db "SELECT * FROM users"
  liteschema query myapp.db --format json "SELECT id, name FROM users"
  liteschema profile myapp.db users
  liteschema export myapp.db users --format json --limit 10
  liteschema fkgraph myapp.db --format mermaid
  liteschema fkgraph myapp.db --format dot`)
}

func parseFlags(args []string) (files []string, format string, showSQL bool) {
	format = "text"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--show-sql":
			showSQL = true
		default:
			files = append(files, args[i])
		}
	}
	return
}

func loadSchema(path string) (*schema.DatabaseSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}

	var s *schema.DatabaseSchema

	if strings.HasSuffix(path, ".json") {
		s = &schema.DatabaseSchema{}
		if err := json.Unmarshal(data, s); err == nil {
			return s, nil
		}
	}

	p := schema.NewParser()
	s, err = p.ParseFromSQL(string(data))
	if err == nil && len(s.Tables) > 0 {
		return s, nil
	}

	if strings.HasSuffix(path, ".db") || strings.HasSuffix(path, ".sqlite") {
		s, err = p.ParseFromDB(path)
		if err == nil && len(s.Tables) > 0 {
			return s, nil
		}
	}

	if strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".txt") || strings.HasSuffix(path, ".ddl") {
		return s, nil
	}

	return nil, fmt.Errorf("could not parse %q as SQL, JSON, or SQLite database", path)
}

func parseSchema(args []string) error {
	files, format, showSQL := parseFlags(args)
	if len(files) == 0 {
		return fmt.Errorf("missing file argument")
	}

	s, err := loadSchema(files[0])
	if err != nil {
		return err
	}
	if s == nil || len(s.Tables) == 0 {
		return fmt.Errorf("no tables found in schema")
	}

	switch format {
	case "json":
		fmt.Println(schema.PrintSchemaJSON(s))
	case "markdown":
		fmt.Print(schema.PrintSchemaMarkdown(s))
	case "sql":
		fmt.Print(schema.PrintSchemaSQL(s))
	default:
		fmt.Print(schema.PrintSchema(s, showSQL))
	}
	return nil
}

func diffSchemas(args []string) error {
	files, format, _ := parseFlags(args)
	if len(files) < 2 {
		return fmt.Errorf("diff requires two schema files")
	}

	old, err := loadSchema(files[0])
	if err != nil {
		return fmt.Errorf("old schema: %w", err)
	}
	new_, err := loadSchema(files[1])
	if err != nil {
		return fmt.Errorf("new schema: %w", err)
	}

	d := schema.Diff(old, new_)
	switch format {
	case "json":
		fmt.Println(schema.FormatDiffJSON(d))
	default:
		fmt.Print(schema.FormatDiff(d))
	}
	return nil
}

func generateMigration(args []string) error {
	files, format, _ := parseFlags(args)
	if len(files) < 2 {
		return fmt.Errorf("migrate requires two schema files")
	}

	old, err := loadSchema(files[0])
	if err != nil {
		return fmt.Errorf("old schema: %w", err)
	}
	new_, err := loadSchema(files[1])
	if err != nil {
		return fmt.Errorf("new schema: %w", err)
	}

	d := schema.Diff(old, new_)
	gen := schema.NewMigrationGenerator()
	if format == "sql" {
		fmt.Print(gen.Generate(d))
	} else {
		fmt.Print(schema.FormatDiff(d))
		fmt.Println("\nMigration SQL (use --format sql to see SQL):")
		fmt.Print(gen.Generate(d))
	}
	return nil
}

func analyzeSchema(args []string) error {
	files, format, _ := parseFlags(args)
	if len(files) == 0 {
		return fmt.Errorf("missing file argument")
	}

	s, err := loadSchema(files[0])
	if err != nil {
		return err
	}

	result := schema.AnalyzeIndexes(s)
	switch format {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	case "markdown":
		fmt.Println("```")
		fmt.Print(schema.FormatIndexAnalysis(result))
		fmt.Println("```")
	default:
		fmt.Print(schema.FormatIndexAnalysis(result))
	}
	return nil
}

func validateSchema(args []string) error {
	files, format, _ := parseFlags(args)
	if len(files) == 0 {
		return fmt.Errorf("missing file argument")
	}

	s, err := loadSchema(files[0])
	if err != nil {
		return err
	}

	result := schema.ValidateSchema(s)
	switch format {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	default:
		fmt.Print(schema.FormatValidation(result))
	}

	if len(result.Errors) > 0 {
		os.Exit(1)
	}
	return nil
}

func fkGraph(args []string) error {
	files, format, _ := parseFlags(args)
	if len(files) == 0 {
		return fmt.Errorf("missing file argument")
	}

	s, err := loadSchema(files[0])
	if err != nil {
		return err
	}

	// Check if we have a live database to extract PK/FK info
	if len(files) > 1 {
		// Use live database mode
		dbPath := files[1]
		db, err := sqlite.Open(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		graph := viz.NewRelationshipGraph()
		for _, t := range s.Tables {
			// Get PKs from live DB
			cols, err := db.TableInfo(t.Name)
			if err == nil {
				for _, col := range cols {
					if col.PrimaryKey {
						graph.AddPrimaryKey(t.Name, col.Name)
					}
				}
			}
			// Also add PKs from schema
			for _, col := range t.Columns {
				if col.PrimaryKey {
					graph.AddPrimaryKey(t.Name, col.Name)
				}
			}
			// Add FK relations
			for _, fk := range t.ForeignKeys {
				graph.AddRelation(viz.TableRelation{
					FromTable:  t.Name,
					FromColumn: strings.Join(fk.Columns, ", "),
					ToTable:    fk.RefTable,
					ToColumn:   strings.Join(fk.RefColumns, ", "),
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				})
			}
		}

		switch format {
		case "mermaid":
			fmt.Print(graph.FormatMermaid())
		case "dot":
			fmt.Print(graph.FormatDot())
		default:
			fmt.Print(graph.FormatASCII())
		}
		return nil
	}

	// Schema-only mode (existing behavior)
	g := schema.BuildFKGraph(s)
	switch format {
	case "json":
		data, _ := json.MarshalIndent(g, "", "  ")
		fmt.Println(string(data))
	case "mermaid":
		// Convert to mermaid format
		graph := viz.NewRelationshipGraph()
		for _, t := range s.Tables {
			for _, col := range t.Columns {
				if col.PrimaryKey {
					graph.AddPrimaryKey(t.Name, col.Name)
				}
			}
			for _, fk := range t.ForeignKeys {
				graph.AddRelation(viz.TableRelation{
					FromTable:  t.Name,
					FromColumn: strings.Join(fk.Columns, ", "),
					ToTable:    fk.RefTable,
					ToColumn:   strings.Join(fk.RefColumns, ", "),
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				})
			}
		}
		fmt.Print(graph.FormatMermaid())
	case "dot":
		graph := viz.NewRelationshipGraph()
		for _, t := range s.Tables {
			for _, col := range t.Columns {
				if col.PrimaryKey {
					graph.AddPrimaryKey(t.Name, col.Name)
				}
			}
			for _, fk := range t.ForeignKeys {
				graph.AddRelation(viz.TableRelation{
					FromTable:  t.Name,
					FromColumn: strings.Join(fk.Columns, ", "),
					ToTable:    fk.RefTable,
					ToColumn:   strings.Join(fk.RefColumns, ", "),
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				})
			}
		}
		fmt.Print(graph.FormatDot())
	default:
		fmt.Print(schema.FormatFKGraph(g))
	}
	return nil
}

// --- NEW COMMANDS ---

func executeQuery(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: liteschema query <database> <sql> [--format table|json|csv] [--file <path>]")
	}

	dbPath := args[0]
	query := args[1]
	format := "table"
	file := ""

	// Parse remaining flags
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--file":
			if i+1 < len(args) {
				file = args[i+1]
				i++
			}
		}
	}

	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read query file: %w", err)
		}
		query = string(data)
	}

	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, ";")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("execute query: %w", err)
	}

	if len(rows) == 0 {
		fmt.Println("No results.")
		return nil
	}

	// Get column names from first row
	columns := make([]string, 0, len(rows[0]))
	for col := range rows[0] {
		columns = append(columns, col)
	}

	switch format {
	case "json":
		return export.ExportJSON(os.Stdout, rows)
	case "csv":
		return export.ExportCSV(os.Stdout, columns, rows)
	default: // table
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for i, col := range columns {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, strings.ToUpper(col))
		}
		fmt.Fprintln(tw)

		for i := range columns {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, strings.Repeat("-", len(strings.ToUpper(columns[i]))))
		}
		fmt.Fprintln(tw)

		for _, row := range rows {
			for i, col := range columns {
				if i > 0 {
					fmt.Fprint(tw, "\t")
				}
				fmt.Fprint(tw, row[col])
			}
			fmt.Fprintln(tw)
		}
		tw.Flush()

		fmt.Printf("\n(%d rows)\n", len(rows))
	}

	return nil
}

func profileDatabase(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: liteschema profile <database> [table] [--format text|json]")
	}

	dbPath := args[0]
	format := "text"
	tableName := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		default:
			if tableName == "" {
				tableName = args[i]
			}
		}
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	executor := &dbQueryExecutor{db: db}

	if tableName != "" {
		ts, err := schema.ProfileTable(executor, tableName)
		if err != nil {
			return fmt.Errorf("profile table: %w", err)
		}
		switch format {
		case "json":
			data := schema.FormatProfileJSON(ts)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(data)
		default:
			fmt.Print(schema.FormatProfileText(ts))
		}
	} else {
		names, err := db.TableNames()
		if err != nil {
			return err
		}
		for _, name := range names {
			ts, err := schema.ProfileTable(executor, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not profile %q: %v\n", name, err)
				continue
			}
			switch format {
			case "json":
				data := schema.FormatProfileJSON(ts)
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(data)
			default:
				fmt.Print(schema.FormatProfileText(ts))
			}
		}
	}

	return nil
}

func exportData(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: liteschema export <database> <table> [--format csv|json|sql] [--output <file>] [--limit <n>]")
	}

	dbPath := args[0]
	tableName := args[1]
	format := "csv"
	outputFile := ""
	limit := 0

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputFile = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		}
	}

	query := fmt.Sprintf("SELECT * FROM %s", tableName)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("query table: %w", err)
	}

	if len(rows) == 0 {
		fmt.Println("No data to export.")
		return nil
	}

	columns := make([]string, 0, len(rows[0]))
	for col := range rows[0] {
		columns = append(columns, col)
	}

	// Write to file or stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()

		switch format {
		case "json":
			return export.ExportJSON(f, rows)
		case "sql":
			return export.ExportSQL(f, tableName, columns, rows)
		default:
			return export.ExportCSV(f, columns, rows)
		}
	} else {
		switch format {
		case "json":
			return export.ExportJSON(os.Stdout, rows)
		case "sql":
			return export.ExportSQL(os.Stdout, tableName, columns, rows)
		default:
			return export.ExportCSV(os.Stdout, columns, rows)
		}
	}
}

// dbQueryExecutor wraps sqlite.DB to implement schema.QueryExecutor
type dbQueryExecutor struct {
	db *sqlite.DB
}

func (e *dbQueryExecutor) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	return e.db.Query(query, args...)
}

func (e *dbQueryExecutor) QueryScalar(query string, args ...interface{}) (interface{}, error) {
	return e.db.QueryScalar(query, args...)
}
