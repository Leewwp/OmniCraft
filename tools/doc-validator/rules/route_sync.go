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

const (
	routeSourcePath        = "backend/internal/router/routes.go"
	routeStartMarker       = "<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/router/routes.go | DO NOT EDIT MANUALLY -->"
	legacyRouteStartMarker = "<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/handler/routes.go | DO NOT EDIT MANUALLY -->"
	routeEndMarker         = "<!-- END AUTO-GENERATED: §3.2 -->"
)

// Route represents a parsed API route.
type Route struct {
	Method  string // GET, POST, PUT, PATCH, DELETE
	Path    string
	Handler string // Handler name
}

// routerGroup tracks a gin router group variable and its full prefix.
type routerGroup struct {
	varName string // e.g., "v1", "auth", "users", "contents"
	prefix  string // e.g., "/auth", "/users/:id"
	parent  string // parent varName
}

// extractRoutes parses the router composition root and extracts all API routes.
func extractRoutes() ([]Route, error) {
	routePath := filepath.FromSlash(routeSourcePath)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, routePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", routePath, err)
	}

	// Walk the AST for method calls on gin router groups
	var routes []Route
	knownMethods := map[string]string{
		"GET":    "GET",
		"POST":   "POST",
		"PUT":    "PUT",
		"PATCH":  "PATCH",
		"DELETE": "DELETE",
	}

	var groups []routerGroup
	// The root v1 group
	groups = append(groups, routerGroup{varName: "v1", prefix: "/api/v1"})

	// We'll process function body statements
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != "RegisterRoutes" {
			continue
		}

		// Walk the function body
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.AssignStmt:
				// Look for router assignments: `auth := v1.Group("/auth")`
				if len(stmt.Lhs) == 1 && len(stmt.Rhs) == 1 {
					ident, ok := stmt.Lhs[0].(*ast.Ident)
					if !ok {
						return true
					}
					callExpr, ok := stmt.Rhs[0].(*ast.CallExpr)
					if !ok {
						return true
					}

					// Check if it's a .Group() call
					selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
					if !ok || selExpr.Sel.Name != "Group" {
						return true
					}

					// Get the parent router
					parentIdent, ok := selExpr.X.(*ast.Ident)
					if !ok {
						return true
					}

					// Get the group path
					if len(callExpr.Args) < 1 {
						return true
					}
					pathLit, ok := callExpr.Args[0].(*ast.BasicLit)
					if !ok {
						return true
					}
					path := strings.Trim(pathLit.Value, "\"")

					// Find parent group's prefix
					parentPrefix := findGroupPrefix(groups, parentIdent.Name)
					fullPrefix := parentPrefix + path

					groups = append(groups, routerGroup{
						varName: ident.Name,
						prefix:  fullPrefix,
						parent:  parentIdent.Name,
					})
				}

			case *ast.ExprStmt:
				// Look for method calls: `v1.GET("/path", handler)`
				callExpr, ok := stmt.X.(*ast.CallExpr)
				if !ok {
					return true
				}
				selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				method, isKnown := knownMethods[selExpr.Sel.Name]
				if !isKnown {
					return true
				}

				// Get the router
				routerIdent, ok := selExpr.X.(*ast.Ident)
				if !ok {
					return true
				}

				// Get the path
				if len(callExpr.Args) < 1 {
					return true
				}
				pathLit, ok := callExpr.Args[0].(*ast.BasicLit)
				if !ok {
					return true
				}
				path := strings.Trim(pathLit.Value, "\"")

				// Get the handler name (last argument, could be a selector like handler.GetUser)
				handlerName := ""
				if len(callExpr.Args) >= 2 {
					lastArg := callExpr.Args[len(callExpr.Args)-1]
					handlerName = extractHandlerName(lastArg)
				}

				// Build full path
				prefix := findGroupPrefix(groups, routerIdent.Name)
				fullPath := prefix + path

				// Normalize path
				fullPath = normalizeRoutePath(fullPath)

				routes = append(routes, Route{
					Method:  method,
					Path:    fullPath,
					Handler: handlerName,
				})
			}

			return true
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Method != routes[j].Method {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	return routes, nil
}

// findGroupPrefix finds the full prefix for a router variable name.
func findGroupPrefix(groups []routerGroup, varName string) string {
	for _, g := range groups {
		if g.varName == varName {
			return g.prefix
		}
	}
	return "/api/v1"
}

// extractHandlerName extracts a human-readable name from an expression.
func extractHandlerName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// e.g., authHandler.Register
		x := extractHandlerName(e.X)
		if x != "" {
			return x + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.CallExpr:
		// e.g., middleware.CredentialRateLimit(rdb, &cfg.RateLimit)
		return extractHandlerName(e.Fun) + "(...)"
	case *ast.FuncLit:
		return "inline handler"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// normalizeRoutePath cleans up a route path.
func normalizeRoutePath(path string) string {
	// Remove double slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return path
}

// generateRouteTable generates markdown for the route list.
func generateRouteTable(routes []Route) string {
	var b strings.Builder
	b.WriteString("| 方法 | 路径 | 处理器 |\n")
	b.WriteString("|------|------|--------|\n")
	for _, r := range routes {
		handler := r.Handler
		if handler == "" {
			handler = "-"
		}
		handler = strings.ReplaceAll(handler, "|", "\\|")
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", r.Method, r.Path, handler))
	}
	return b.String()
}

// CheckRouteSync compares routes with architecture.md section 3.2.
func CheckRouteSync() []RuleIssue {
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: archPath, Message: fmt.Sprintf("cannot read: %v", err)}}
	}

	routes, err := extractRoutes()
	if err != nil {
		return []RuleIssue{{Severity: "ERROR", File: routeSourcePath, Message: fmt.Sprintf("cannot extract: %v", err)}}
	}

	var issues []RuleIssue
	text := string(content)

	// Build a map of routes in the doc: search both old format (code block) and new format (table)
	docRoutes := map[string]bool{}

	// Old format: lines like "GET    /api/v1/..."
	oldRe := regexp.MustCompile(`(?m)^(GET|POST|PUT|PATCH|DELETE)\s+(/\S+)`)
	for _, m := range oldRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			key := m[1] + " " + strings.TrimSpace(m[2])
			docRoutes[key] = true
		}
	}

	// New format (auto-generated table): rows like "| `GET` | `/api/v1/...` |"
	newRe := regexp.MustCompile("(?m)^\\|[\\s]`(GET|POST|PUT|PATCH|DELETE)`[\\s]\\|[\\s]`(/[^`]+)`[\\s]\\|")
	for _, m := range newRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			key := m[1] + " " + strings.TrimSpace(m[2])
			docRoutes[key] = true
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if !docRoutes[key] {
			issues = append(issues, RuleIssue{
				Severity: "WARNING",
				File:     archPath,
				Message:  fmt.Sprintf("route %s %s not documented in §3.2 (handler: %s)", r.Method, r.Path, r.Handler),
			})
		}
	}

	return issues
}

// SyncRouteList generates route documentation and inserts into architecture.md.
func SyncRouteList() error {
	archPath := "architecture.md"
	content, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", archPath, err)
	}

	routes, err := extractRoutes()
	if err != nil {
		return fmt.Errorf("extract routes: %w", err)
	}

	table := generateRouteTable(routes)

	text := string(content)
	if !strings.Contains(text, routeStartMarker) && strings.Contains(text, legacyRouteStartMarker) {
		text = strings.Replace(text, legacyRouteStartMarker, routeStartMarker, 1)
	}

	newContent, replaced := replaceBetweenMarkers(text, routeStartMarker, routeEndMarker, table)
	if !replaced {
		newContent, _ = addAutoGeneratedMarkers(text, routeStartMarker, routeEndMarker, "完整 API 路由清单", table)
	}

	if newContent == string(content) {
		fmt.Println("  route sync: no changes needed")
		return nil
	}

	return os.WriteFile(archPath, []byte(newContent), 0644)
}
