package rules

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ConfigField represents a flattened config field extracted from config.go.
type ConfigField struct {
	Path        string // dot-separated path, e.g. "server.port"
	GoType      string // Go type, e.g. "int", "string", "bool"
	Description string // inferred from field name + comments
}

// RuleIssue is a single check issue.
type RuleIssue struct {
	Severity string
	File     string
	Message  string
}

// extractConfigFields parses config/config.go and returns all config fields.
func extractConfigFields() ([]ConfigField, error) {
	configPath := filepath.Join("backend", "config", "config.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, configPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}

	// Collect all type declarations from the file
	typeDecls := map[string]*ast.StructType{}
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			typeDecls[typeSpec.Name.Name] = structType
		}
	}

	// Find comments associated with fields
	comments := extractFieldComments(f)

	var fields []ConfigField
	configStruct, ok := typeDecls["Config"]
	if !ok {
		return nil, fmt.Errorf("Config struct not found in %s", configPath)
	}

	for _, field := range configStruct.Fields.List {
		if len(field.Names) == 0 {
			continue // embedded field
		}
		fieldName := field.Names[0].Name
		tagValue := getMapstructureTag(field.Tag)
		if tagValue == "" {
			continue
		}

		typeName := resolveTypeName(field.Type)
		fieldDesc := buildFieldDescription(fieldName, field.Doc, comments["Config."+fieldName])

		// Check if this is a struct type defined in the file
		if _, ok := typeDecls[typeName]; ok {
			// Recurse into nested struct
			nested := flattenStruct(typeDecls, typeName, tagValue, comments)
			fields = append(fields, nested...)
		} else {
			// External type - just record the field
			fields = append(fields, ConfigField{
				Path:        tagValue,
				GoType:      typeName,
				Description: fieldDesc,
			})
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})

	return fields, nil
}

// flattenStruct recursively flattens a struct's fields into dot-separated paths.
func flattenStruct(typeDecls map[string]*ast.StructType, typeName, prefix string, comments map[string]string) []ConfigField {
	var fields []ConfigField
	structType, ok := typeDecls[typeName]
	if !ok {
		return fields
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		fieldName := field.Names[0].Name
		tagValue := getMapstructureTag(field.Tag)
		if tagValue == "" {
			continue
		}

		fullPath := prefix + "." + tagValue
		typeNameInner := resolveTypeName(field.Type)
		fieldDesc := buildFieldDescription(fieldName, field.Doc, comments[typeName+"."+fieldName])

		if _, ok := typeDecls[typeNameInner]; ok {
			nested := flattenStruct(typeDecls, typeNameInner, fullPath, comments)
			fields = append(fields, nested...)
		} else {
			fields = append(fields, ConfigField{
				Path:        fullPath,
				GoType:      typeNameInner,
				Description: fieldDesc,
			})
		}
	}

	return fields
}

// extractFieldComments extracts comments for struct fields.
// Key format: "StructName.FieldName"
func extractFieldComments(f *ast.File) map[string]string {
	result := map[string]string{}

	// Walk through declarations looking for struct types
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			structName := typeSpec.Name.Name
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				fieldName := field.Names[0].Name
				key := structName + "." + fieldName
				if field.Doc != nil {
					result[key] = field.Doc.Text()
				} else if field.Comment != nil {
					result[key] = field.Comment.Text()
				}
			}
		}
	}
	return result
}

// getMapstructureTag extracts the value of a mapstructure struct tag.
func getMapstructureTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	raw := tag.Value
	re := regexp.MustCompile(`mapstructure:"([^"]+)"`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// resolveTypeName returns the string representation of an AST expression type.
func resolveTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return resolveTypeName(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + resolveTypeName(t.Elt)
	case *ast.StarExpr:
		return "*" + resolveTypeName(t.X)
	case *ast.MapType:
		return "map[" + resolveTypeName(t.Key) + "]" + resolveTypeName(t.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// buildFieldDescription creates a human-readable description for a config field.
func buildFieldDescription(fieldName string, doc *ast.CommentGroup, commentText string) string {
	text := strings.TrimSpace(commentText)
	if text != "" {
		return text
	}
	if doc != nil {
		t := strings.TrimSpace(doc.Text())
		if t != "" {
			return t
		}
	}
	// Infer from field name
	return snakeToTitle(fieldName)
}

// snakeToTitle converts snake_case to Title Case.
func snakeToTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// generateConfigTable generates a markdown table of config fields.
func generateConfigTable(fields []ConfigField) string {
	var b strings.Builder
	b.WriteString("| 配置路径 | 类型 | 说明 |\n")
	b.WriteString("|----------|------|------|\n")
	for _, f := range fields {
		// Escape pipe characters in description
		desc := strings.ReplaceAll(f.Description, "|", "\\|")
		// Truncate long descriptions
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", f.Path, f.GoType, desc))
	}
	return b.String()
}

// CheckConfigSync compares config fields with architecture.md section 7.
func CheckConfigSync() []RuleIssue {
	// Read the current architecture.md section 7
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: archPath, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	fields, err := extractConfigFields()
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: "backend/config/config.go", Message: fmt.Sprintf("cannot extract fields: %v", err)}}
	}

	var issues []RuleIssue

	// Check each config field appears in architecture.md
	for _, f := range fields {
		if !strings.Contains(string(content), f.Path) {
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     archPath,
				Message:  fmt.Sprintf("config field %q (type %s) not documented in §7", f.Path, f.GoType),
			})
		}
	}

	return issues
}

// SyncConfigFields generates config field table and inserts into architecture.md.
func SyncConfigFields() error {
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", archPath, err)
	}

	fields, err := extractConfigFields()
	if err != nil {
		return fmt.Errorf("extract config fields: %w", err)
	}

	table := generateConfigTable(fields)

	startMarker := "<!-- AUTO-GENERATED: §7 配置字段注册表 | source: backend/config/config.go | DO NOT EDIT MANUALLY -->"
	endMarker := "<!-- END AUTO-GENERATED: §7 -->"

	newContent, replaced := replaceBetweenMarkers(string(content), startMarker, endMarker, table)
	if !replaced {
		// Add markers around existing content. Find section 7 and wrap it.
		newContent, _ = addAutoGeneratedMarkers(string(content), startMarker, endMarker, "## 7. 配置化开关与参数", table)
	}

	if newContent == string(content) {
		fmt.Println("  config sync: no changes needed")
		return nil
	}

	return os.WriteFile(archPath, []byte(newContent), 0644)
}

// replaceBetweenMarkers replaces content between start and end markers.
// Returns the new content and whether the markers were found.
func replaceBetweenMarkers(content, startMarker, endMarker, newSection string) (string, bool) {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return content, false
	}
	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return content, false
	}

	// Find the end of the start marker line
	startLineEnd := strings.Index(content[startIdx:], "\n")
	if startLineEnd == -1 {
		startLineEnd = len(content) - startIdx
	}
	afterStart := startIdx + startLineEnd + 1

	// Keep everything before the start marker line, the marker, then the new content, then end marker, then everything after end marker line
	before := content[:afterStart]
	after := content[endIdx:]

	return before + "\n" + newSection + "\n" + after, true
}

// addAutoGeneratedMarkers wraps a section's content with AUTO-GENERATED markers.
func addAutoGeneratedMarkers(content, startMarker, endMarker, sectionHeading, newSection string) (string, bool) {
	// Find section heading
	sectionIdx := strings.Index(content, sectionHeading)
	if sectionIdx == -1 {
		fmt.Printf("  warning: section %q not found in architecture.md, skipping\n", sectionHeading)
		return content, false
	}

	// Find end of code block (```) after section heading - find the second ``` as the code block end
	afterSection := content[sectionIdx:]
	yamlBlockEnd := findYAMLEnd(afterSection)
	if yamlBlockEnd == -1 {
		fmt.Printf("  warning: could not find code block end in section %q\n", sectionHeading)
		return content, false
	}

	// Build new content: everything before section heading + start marker + new table + end marker + everything after
	beforeSection := content[:sectionIdx]
	afterBlock := afterSection[yamlBlockEnd:]

	var b strings.Builder
	b.WriteString(strings.TrimRight(beforeSection, "\n"))
	b.WriteString("\n\n")
	b.WriteString(startMarker)
	b.WriteString("\n\n")
	b.WriteString(newSection)
	b.WriteString("\n")
	b.WriteString(endMarker)
	b.WriteString("\n\n")
	// Keep the rest after the code block
	b.WriteString(strings.TrimLeft(afterBlock, "\n"))

	return b.String(), true
}

// findYAMLEnd finds the end of the YAML code block (```) starting after the section heading.
// It looks for the second ``` that closes the fenced code block.
func findYAMLEnd(content string) int {
	first := strings.Index(content, "```")
	if first == -1 {
		return -1
	}
	afterFirst := content[first+3:]
	second := strings.Index(afterFirst, "```")
	if second == -1 {
		return -1
	}
	return first + 3 + second + 3
}
