package schema

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

// Parser handles parsing SQLite schema definitions.
type Parser struct{}

// NewParser creates a new schema parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseFromSQL parses one or more CREATE statements and returns a DatabaseSchema.
func (p *Parser) ParseFromSQL(sqlScript string) (*DatabaseSchema, error) {
	schema := &DatabaseSchema{}
	statements := splitSQLStatements(sqlScript)
	for i, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			table, err := parseCreateTable(trimmed)
			if err != nil {
				return nil, fmt.Errorf("statement %d: %w", i+1, err)
			}
			schema.Tables = append(schema.Tables, *table)
		case strings.HasPrefix(upper, "CREATE INDEX"):
			idx, err := parseCreateIndex(trimmed)
			if err != nil {
				return nil, fmt.Errorf("statement %d: %w", i+1, err)
			}
			schema.Indexes = append(schema.Indexes, *idx)
		case strings.HasPrefix(upper, "CREATE VIEW"):
			view, err := parseCreateView(trimmed)
			if err != nil {
				return nil, fmt.Errorf("statement %d: %w", i+1, err)
			}
			schema.Views = append(schema.Views, *view)
		case strings.HasPrefix(upper, "CREATE TRIGGER"):
			trigger, err := parseCreateTrigger(trimmed)
			if err != nil {
				return nil, fmt.Errorf("statement %d: %w", i+1, err)
			}
			schema.Triggers = append(schema.Triggers, *trigger)
		}
	}
	return schema, nil
}

// ParseFromDB connects to a SQLite database and extracts its schema.
func (p *Parser) ParseFromDB(dbPath string) (*DatabaseSchema, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", dbPath, err)
	}
	defer db.Close()

	schema := &DatabaseSchema{}

	// Parse tables
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("reading tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, sqlStr string
		if err := rows.Scan(&name, &sqlStr); err != nil {
			return nil, fmt.Errorf("scanning table row: %w", err)
		}
		table, err := parseCreateTable(sqlStr)
		if err != nil {
			return nil, fmt.Errorf("parsing table %q: %w", name, err)
		}
		table.CreateStmt = sqlStr
		schema.Tables = append(schema.Tables, *table)
	}

	// Parse indexes
	rows2, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("reading indexes: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var name, sqlStr string
		if err := rows2.Scan(&name, &sqlStr); err != nil {
			return nil, err
		}
		if sqlStr == "" {
			continue // auto-generated index
		}
		idx, err := parseCreateIndex(sqlStr)
		if err != nil {
			return nil, fmt.Errorf("parsing index %q: %w", name, err)
		}
		schema.Indexes = append(schema.Indexes, *idx)
	}

	// Parse views
	rows3, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='view' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("reading views: %w", err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var name, sqlStr string
		if err := rows3.Scan(&name, &sqlStr); err != nil {
			return nil, err
		}
		schema.Views = append(schema.Views, View{Name: name, Content: sqlStr})
	}

	// Parse triggers
	rows4, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='trigger' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("reading triggers: %w", err)
	}
	defer rows4.Close()
	for rows4.Next() {
		var name, sqlStr string
		if err := rows4.Scan(&name, &sqlStr); err != nil {
			return nil, err
		}
		trigger, err := parseCreateTrigger(sqlStr)
		if err != nil {
			return nil, fmt.Errorf("parsing trigger %q: %w", name, err)
		}
		schema.Triggers = append(schema.Triggers, *trigger)
	}

	return schema, nil
}

func splitSQLStatements(s string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)
	depth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			current.WriteByte(ch)
			if ch == stringChar && (i == 0 || s[i-1] != '\\') {
				inString = false
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = true
			stringChar = ch
			current.WriteByte(ch)
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ';' && depth == 0 {
			statements = append(statements, current.String())
			current.Reset()
			continue
		}
		if ch == '-' && i+1 < len(s) && s[i+1] == '-' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i < len(s) {
				if s[i] == '*' && i+1 < len(s) && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		current.WriteByte(ch)
	}
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		statements = append(statements, remaining)
	}
	return statements
}

func trimBrackets(name string) string {
	return strings.Trim(strings.TrimSpace(name), "\"`[]'")
}

// parseCreateTable parses a CREATE TABLE statement.
func parseCreateTable(sql string) (*Table, error) {
	table := &Table{
		CreateStmt:  sql,
		Indexes:     make([]Index, 0),
		ForeignKeys: make([]ForeignKey, 0),
	}
	// Extract table name
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:` + "`" + `?[^"` + "`" + `\s.(]+` + "`" + `?\.)?` + "`" + `?([^"` + "`" + `\s.(]+)` + "`" + `?`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return nil, fmt.Errorf("cannot extract table name from: %s", sql[:min(len(sql), 50)])
	}
	table.Name = trimBrackets(matches[1])

	// Check WITHOUT ROWID
	if strings.Contains(strings.ToUpper(sql), "WITHOUT ROWID") || strings.Contains(strings.ToUpper(sql), "WITHOUT ROWID") {
		table.WithoutRowID = true
	}

	// Extract column definitions from parentheses
	parenStart := strings.Index(sql, "(")
	if parenStart < 0 {
		return table, nil
	}
	// Find matching closing paren
	content := extractParenContent(sql, parenStart)
	if content == "" {
		return table, nil
	}

	// Parse column definitions and constraints
	defs := splitDefinitions(content)
	for _, def := range defs {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}
		upperDef := strings.ToUpper(def)
		if strings.HasPrefix(upperDef, "PRIMARY KEY") {
			continue
		}
		if strings.HasPrefix(upperDef, "UNIQUE") {
			continue
		}
		if strings.HasPrefix(upperDef, "CHECK") {
			continue
		}
		if strings.HasPrefix(upperDef, "FOREIGN KEY") {
			fk := parseForeignKeyDef(def)
			if fk != nil {
				table.ForeignKeys = append(table.ForeignKeys, *fk)
			}
			continue
		}
		if strings.HasPrefix(upperDef, "CONSTRAINT") {
			// Handle constraint-based foreign keys
			if strings.Contains(upperDef, "FOREIGN KEY") {
				fk := parseForeignKeyDef(def)
				if fk != nil {
					table.ForeignKeys = append(table.ForeignKeys, *fk)
				}
			}
			continue
		}
		// Column definition
		col := parseColumnDef(def)
		if col != nil {
			table.Columns = append(table.Columns, *col)
		}
	}

	return table, nil
}

func extractParenContent(sql string, start int) string {
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(sql); i++ {
		if sql[i] == '(' {
			depth++
		} else if sql[i] == ')' {
			depth--
			if depth == 0 {
				return sql[start+1 : i]
			}
		}
	}
	return sql[start+1:]
}

func splitDefinitions(content string) []string {
	var defs []string
	var current strings.Builder
	depth := 0
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if inString {
			current.WriteByte(ch)
			if ch == stringChar && content[i-1] != '\\' {
				inString = false
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = true
			stringChar = ch
			current.WriteByte(ch)
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ',' && depth == 0 {
			defs = append(defs, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	rest := strings.TrimSpace(current.String())
	if rest != "" {
		defs = append(defs, rest)
	}
	return defs
}

func parseColumnDef(def string) *Column {
	def = strings.TrimSpace(def)
	parts := strings.Fields(def)
	if len(parts) == 0 {
		return nil
	}

	col := &Column{}
	col.Name = trimBrackets(parts[0])

	if len(parts) > 1 {
		// Type is everything between name and first constraint keyword
		typeEnd := 1
		for typeEnd < len(parts) {
			upper := strings.ToUpper(parts[typeEnd])
			if upper == "NOT" || upper == "PRIMARY" || upper == "UNIQUE" || upper == "CHECK" || upper == "DEFAULT" || upper == "REFERENCES" || upper == "COLLATE" || upper == "GENERATED" || upper == "AS" || upper == "CONSTRAINT" {
				break
			}
			typeEnd++
		}
		if typeEnd > 1 {
			col.Type = strings.Join(parts[1:typeEnd], " ")
		}

		// Parse constraints
		upperDef := strings.ToUpper(def)
		col.NotNull = strings.Contains(upperDef, "NOT NULL")
		col.PrimaryKey = strings.Contains(upperDef, "PRIMARY KEY")
		col.AutoIncr = strings.Contains(upperDef, "AUTOINCREMENT")
		if !col.PrimaryKey {
			col.Unique = strings.Contains(upperDef, "UNIQUE")
		}

		// Extract DEFAULT value
		if idx := strings.Index(upperDef, "DEFAULT"); idx >= 0 {
			rest := strings.TrimSpace(def[idx+7:])
			if strings.HasPrefix(rest, "(") {
				// Expression default
				closeParen := strings.Index(rest, ")")
				if closeParen >= 0 {
					col.Default = rest[:closeParen+1]
				}
			} else {
				endWords := []string{"NOT", "PRIMARY", "UNIQUE", "CHECK", "REFERENCES", "COLLATE", "GENERATED"}
				endIdx := len(rest)
				for _, w := range endWords {
					if wi := strings.Index(strings.ToUpper(rest), w); wi >= 0 && wi < endIdx {
						endIdx = wi
					}
				}
				col.Default = strings.TrimSpace(rest[:endIdx])
			}
		}

		// Extract COLLATE
		if idx := strings.Index(upperDef, "COLLATE"); idx >= 0 {
			rest := strings.Fields(def[idx+7:])
			if len(rest) > 0 {
				col.Collate = rest[0]
			}
		}

		// Extract CHECK
		if idx := strings.Index(upperDef, "CHECK"); idx >= 0 {
			checkStart := idx + 5
			checkEnd := strings.Index(def[checkStart:], ")")
			if checkEnd >= 0 {
				col.Check = def[checkStart : checkStart+checkEnd+1]
			}
		}
	}
	return col
}

func parseCreateIndex(sql string) (*Index, error) {
	idx := &Index{}
	idx.Unique = strings.Contains(strings.ToUpper(sql), "UNIQUE")

	re := regexp.MustCompile(`(?i)INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`" + `?([^\s(]+)` + "`" + `?\s+ON\s+` + "`" + `?([^\s(]+)` + "`" + `?\s*\(`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return nil, fmt.Errorf("cannot parse index from: %s", sql[:min(len(sql), 60)])
	}
	idx.Name = trimBrackets(matches[1])

	// Extract columns from parenthesized list
	parenStart := strings.Index(sql, "(")
	if parenStart < 0 {
		return idx, nil
	}
	content := extractParenContent(sql, parenStart)
	cols := strings.Split(content, ",")
	for _, c := range cols {
		c = strings.TrimSpace(c)
		// Remove optional ASC/DESC and COLLATE
		c = regexp.MustCompile(`\s+(ASC|DESC)\s*$`).ReplaceAllString(c, "")
		c = regexp.MustCompile(`\s+COLLATE\s+\S+`).ReplaceAllString(c, "")
		idx.Columns = append(idx.Columns, trimBrackets(strings.Fields(c)[0]))
	}

	// Extract WHERE clause
	if whereIdx := strings.Index(strings.ToUpper(sql), "WHERE"); whereIdx >= 0 {
		idx.Where = strings.TrimSpace(sql[whereIdx+5:])
	}

	return idx, nil
}

func parseCreateView(sql string) (*View, error) {
	re := regexp.MustCompile(`(?i)CREATE\s+VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`" + `?([^\s(]+)` + "`" + `?`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return nil, fmt.Errorf("cannot parse view from: %s", sql[:min(len(sql), 50)])
	}
	return &View{Name: trimBrackets(matches[1]), Content: sql}, nil
}

func parseCreateTrigger(sql string) (*Trigger, error) {
	trigger := &Trigger{}

	re := regexp.MustCompile(`(?i)CREATE\s+TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`" + `?([^\s(]+)` + "`" + `?\s+(BEFORE|AFTER|INSTEAD\s+OF)\s+(INSERT|UPDATE|DELETE)\s+ON\s+` + "`" + `?([^\s(]+)` + "`" + `?`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 5 {
		return nil, fmt.Errorf("cannot parse trigger from: %s", sql[:min(len(sql), 60)])
	}
	trigger.Name = trimBrackets(matches[1])
	trigger.Time = matches[2]
	trigger.Event = matches[3]
	trigger.Table = trimBrackets(matches[4])

	if strings.Contains(strings.ToUpper(sql), "FOR EACH ROW") {
		trigger.ForEach = "ROW"
	} else if strings.Contains(strings.ToUpper(sql), "FOR EACH STATEMENT") {
		trigger.ForEach = "STATEMENT"
	}

	// Body is everything after BEGIN
	if beginIdx := strings.Index(strings.ToUpper(sql), "BEGIN"); beginIdx >= 0 {
		trigger.Body = strings.TrimSpace(sql[beginIdx+5:])
		if endIdx := strings.LastIndex(strings.ToUpper(trigger.Body), "END"); endIdx >= 0 {
			trigger.Body = strings.TrimSpace(trigger.Body[:endIdx])
		}
	}

	return trigger, nil
}

func parseForeignKeyDef(def string) *ForeignKey {
	fk := &ForeignKey{}

	// Extract column list
	re1 := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(([^)]+)\)`)
	m1 := re1.FindStringSubmatch(def)
	if len(m1) >= 2 {
		for _, c := range strings.Split(m1[1], ",") {
			fk.Columns = append(fk.Columns, trimBrackets(strings.TrimSpace(c)))
		}
	}

	// Extract REFERENCES
	re2 := regexp.MustCompile(`(?i)REFERENCES\s+` + "`" + `?([^\s(]+)` + "`" + `?\s*\(([^)]+)\)`)
	m2 := re2.FindStringSubmatch(def)
	if len(m2) >= 3 {
		fk.RefTable = trimBrackets(m2[1])
		for _, c := range strings.Split(m2[2], ",") {
			fk.RefColumns = append(fk.RefColumns, trimBrackets(strings.TrimSpace(c)))
		}
	}

	// Extract ON DELETE / ON UPDATE
	if idx := strings.Index(strings.ToUpper(def), "ON DELETE"); idx >= 0 {
		rest := strings.Fields(def[idx+9:])
		if len(rest) > 0 {
			fk.OnDelete = strings.ToUpper(rest[0])
		}
	}
	if idx := strings.Index(strings.ToUpper(def), "ON UPDATE"); idx >= 0 {
		rest := strings.Fields(def[idx+9:])
		if len(rest) > 0 {
			fk.OnUpdate = strings.ToUpper(rest[0])
		}
	}

	if fk.RefTable == "" {
		return nil
	}
	return fk
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
