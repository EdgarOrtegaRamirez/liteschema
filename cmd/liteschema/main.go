package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/EdgarOrtegaRamirez/liteschema/pkg/schema"
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
	case "help":
		printUsage()
		return nil
	default:
		// Try as file path — parse a .db file or .sql file
		return parseSchema(args)
	}
}

func printUsage() {
	fmt.Println(`LiteSchema — SQLite Schema Analysis & Migration CLI

Usage:
  liteschema <command> [options] <file>

Commands:
  parse <file.sql|file.db>   Parse and display a SQLite schema
  diff <old> <new>           Compute schema diff between two files
  migrate <old> <new>        Generate migration SQL from diff
  analyze <file>             Analyze indexes and schema health
  validate <file>            Validate schema for common issues
  fkgraph <file>             Display foreign key dependency graph
  help                       Show this help message

Options:
  --format text|json|markdown|sql  Output format (default: text)
  --show-sql                       Include CREATE statements in schema view

Examples:
  liteschema parse schema.sql
  liteschema parse database.db
  liteschema diff v1.sql v2.sql
  liteschema migrate v1.sql v2.sql --format sql
  liteschema analyze database.db --format json
  liteschema validate schema.sql`)
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

	// Try JSON first
	if strings.HasSuffix(path, ".json") {
		s = &schema.DatabaseSchema{}
		if err := json.Unmarshal(data, s); err == nil {
			return s, nil
		}
	}

	// Try SQL parsing
	p := schema.NewParser()
	s, err = p.ParseFromSQL(string(data))
	if err == nil && len(s.Tables) > 0 {
		return s, nil
	}

	// Try as database file
	if strings.HasSuffix(path, ".db") || strings.HasSuffix(path, ".sqlite") {
		s, err = p.ParseFromDB(path)
		if err == nil && len(s.Tables) > 0 {
			return s, nil
		}
	}

	// If SQL didn't find tables, maybe the file has a different format
	if strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".txt") {
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

	g := schema.BuildFKGraph(s)

	switch format {
	case "json":
		data, _ := json.MarshalIndent(g, "", "  ")
		fmt.Println(string(data))
	default:
		fmt.Print(schema.FormatFKGraph(g))
	}
	return nil
}