package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// TableDef represents a parsed SQL table definition.
type TableDef struct {
	Name              string
	Columns           []ColumnDef
	UniqueConstraints [][]string
}

// ColumnDef represents a single column in a table.
type ColumnDef struct {
	Name       string
	Type       string
	NotNull    bool
	Default    string
	PrimaryKey bool
	Unique     bool
	References string
	Comment    string
}

// parseMigrations reads all migration SQL files and extracts table definitions.
func parseMigrations() ([]TableDef, error) {
	migDir := filepath.Join("backend", "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	createTableRe := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(`)

	var allTables []TableDef
	seen := map[string]bool{}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}

		path := filepath.Join(migDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		content := string(data)
		matches := createTableRe.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			tableName := content[match[2]:match[3]]
			if seen[tableName] {
				continue
			}
			seen[tableName] = true

			body, err := extractParenthesizedBlock(content, match[1]-1)
			if err != nil {
				continue
			}
			tableDef := parseTableBody(tableName, body)
			allTables = append(allTables, tableDef)
		}
	}

	sort.Slice(allTables, func(i, j int) bool {
		return allTables[i].Name < allTables[j].Name
	})
	return allTables, nil
}

// extractParenthesizedBlock returns text between matching parentheses starting at start.
func extractParenthesizedBlock(s string, start int) (string, error) {
	if start >= len(s) || s[start] != '(' {
		return "", fmt.Errorf("no opening paren at position %d", start)
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[start+1 : i], nil
			}
		}
	}
	return "", fmt.Errorf("unmatched parentheses starting at %d", start)
}

// parseTableBody parses the content inside CREATE TABLE parentheses.
func parseTableBody(tableName, body string) TableDef {
	td := TableDef{Name: tableName}
	lines := splitSQLBody(body)

	colRe := regexp.MustCompile(`^\s*(\w+)\s+(\S+)`)
	primaryKeyRe := regexp.MustCompile(`PRIMARY\s+KEY\s*\((\w+)\)`)
	uniqueRe := regexp.MustCompile(`UNIQUE\s*\((\w+(?:\s*,\s*\w+)*)\)`)
	fkRe := regexp.MustCompile(`REFERENCES\s+(\w+(?:\.\w+)?)\s*\((\w+)\)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if m := primaryKeyRe.FindStringSubmatch(trimmed); m != nil {
			colName := m[1]
			for i, col := range td.Columns {
				if col.Name == colName {
					td.Columns[i].PrimaryKey = true
				}
			}
			continue
		}
		if m := uniqueRe.FindStringSubmatch(trimmed); m != nil {
			cols := strings.Split(m[1], ",")
			for i := range cols {
				cols[i] = strings.TrimSpace(cols[i])
			}
			if len(cols) > 1 {
				td.UniqueConstraints = append(td.UniqueConstraints, cols)
				continue
			}
			for _, c := range cols {
				for i, col := range td.Columns {
					if col.Name == c {
						td.Columns[i].Unique = true
					}
				}
			}
			continue
		}
		if isTableConstraint(trimmed) {
			continue
		}

		if m := colRe.FindStringSubmatch(trimmed); m != nil {
			colName := m[1]
			rest := strings.TrimSpace(trimmed[len(m[0]):])
			col := ColumnDef{Name: colName, Type: m[2]}

			upper := strings.ToUpper(rest)
			if strings.Contains(upper, "NOT NULL") {
				col.NotNull = true
			}
			if strings.Contains(upper, "PRIMARY KEY") {
				col.PrimaryKey = true
			}
			if strings.Contains(upper, "UNIQUE") {
				col.Unique = true
			}
			defaultRe := regexp.MustCompile(`(?i)DEFAULT\s+(\S+)`)
			if dm := defaultRe.FindStringSubmatch(rest); dm != nil {
				val := strings.TrimRight(dm[1], ",")
				col.Default = val
			}
			if fm := fkRe.FindStringSubmatch(rest); fm != nil {
				col.References = fm[1] + "." + fm[2]
			}
			commentRe := regexp.MustCompile(`--\s*(.+)`)
			if cm := commentRe.FindStringSubmatch(rest); cm != nil {
				col.Comment = strings.TrimSpace(cm[1])
			}
			td.Columns = append(td.Columns, col)
		}
	}
	return td
}

func splitSQLBody(body string) []string {
	lines := strings.Split(body, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			trimmed = strings.TrimSuffix(trimmed, ",")
			result = append(result, trimmed)
		}
	}
	return result
}

func isTableConstraint(line string) bool {
	upper := strings.ToUpper(strings.TrimSpace(line))
	if strings.HasPrefix(upper, "PRIMARY KEY") ||
		strings.HasPrefix(upper, "UNIQUE") ||
		strings.HasPrefix(upper, "FOREIGN KEY") ||
		strings.HasPrefix(upper, "CHECK") ||
		strings.HasPrefix(upper, "CONSTRAINT") ||
		strings.HasPrefix(upper, "INDEX") ||
		strings.HasPrefix(upper, "EXCLUDE") {
		return true
	}
	return false
}

func generateSchemaTable(tables []TableDef) string {
	var b strings.Builder
	for _, td := range tables {
		b.WriteString(fmt.Sprintf("### %s\n\n", td.Name))
		b.WriteString("| 列名 | 类型 | 约束 | 说明 |\n")
		b.WriteString("|------|------|------|------|\n")
		for _, col := range td.Columns {
			constraints := buildConstraints(col)
			comment := col.Comment
			if comment == "" {
				comment = col.Name
			}
			comment = strings.ReplaceAll(comment, "|", "\\|")
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", col.Name, col.Type, constraints, comment))
		}
		for _, columns := range td.UniqueConstraints {
			quoted := make([]string, 0, len(columns))
			for _, column := range columns {
				quoted = append(quoted, "`"+column+"`")
			}
			b.WriteString(fmt.Sprintf("| — | — | UNIQUE (%s) | table constraint |\n", strings.Join(quoted, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildConstraints(col ColumnDef) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if col.NotNull {
		parts = append(parts, "NOT NULL")
	}
	if col.Unique {
		parts = append(parts, "UNIQUE")
	}
	if col.Default != "" {
		parts = append(parts, "DEFAULT "+col.Default)
	}
	if col.References != "" {
		parts = append(parts, "-> "+col.References)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

// CheckSchemaSync compares migration tables with architecture.md section 4.
func CheckSchemaSync() []RuleIssue {
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: archPath, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	tables, err := parseMigrations()
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: "backend/migrations/", Message: fmt.Sprintf("cannot parse: %v", err)}}
	}

	var issues []RuleIssue
	text := string(content)

	// Check both old format (CREATE TABLE / CREATE TABLE IF NOT EXISTS) and new format (### table_name)
	for _, td := range tables {
		foundInOld := strings.Contains(text, "CREATE TABLE "+td.Name) ||
			strings.Contains(text, "CREATE TABLE IF NOT EXISTS "+td.Name)
		foundInNew := strings.Contains(text, "### "+td.Name)
		if !foundInOld && !foundInNew {
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     archPath,
				Message:  fmt.Sprintf("table %q not documented in §4", td.Name),
			})
		}
	}
	return issues
}

// SyncSchemaDocs generates schema documentation and inserts into architecture.md.
func SyncSchemaDocs() error {
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", archPath, err)
	}

	tables, err := parseMigrations()
	if err != nil {
		return fmt.Errorf("parse migrations: %w", err)
	}

	table := generateSchemaTable(tables)

	startMarker := "<!-- AUTO-GENERATED: §4 数据库 Schema | source: backend/migrations/ | DO NOT EDIT MANUALLY -->"
	endMarker := "<!-- END AUTO-GENERATED: §4 -->"

	newContent, replaced := replaceBetweenMarkers(string(content), startMarker, endMarker, table)
	if !replaced {
		newContent, _ = addAutoGeneratedMarkers(string(content), startMarker, endMarker, "## 4. 数据库", table)
	}

	if newContent == string(content) {
		fmt.Println("  schema sync: no changes needed")
		return nil
	}

	return os.WriteFile(archPath, []byte(newContent), 0644)
}
