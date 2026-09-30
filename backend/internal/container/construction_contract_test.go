package container

// #658 source-contract 守门测试，#672 补强为 AST 形态：三个高风险服务
// 构造器（审核 / 内容 / OSS presign）的生产构造点只允许出现在组合根
// （本包 NewContainer）；internal/testutil 是测试组合 seam，豁免。
// _test.go 一律豁免（行为测试自行组装被测面）。
//
// #672 前的字串形态有两处可绕过：① import alias——`import svc
// ".../internal/service"` 后 `svc.NewContentService(` 不含限定字串
// `service.NewContentService(`，静默漏网；② 裸构造只扫 internal/service
// 目录，dot-import 到其它包后裸调 `NewContentService(` 无人拦截。
// 现在用 go/parser 解析调用表达式 + import 路径解析：无论别名、dot
// import 还是 service 包内裸名，凡调用目标解析到
// omnicraft/backend/internal/service 的四个构造器即违规。
//
// census 勘误（#672 入档）：原始 NewReviewService 基线共 5 个调用点
// （容器 1 + handler 4），非 4 个总点；收敛后生产构造点 = 组合根 1 处
// （+ testutil seam 1 处），实现结论不变。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// guardedServiceConstructors 是受限构造器全集（service 包内定义）。
var guardedServiceConstructors = map[string]bool{
	"NewReviewService":         true,
	"NewContentService":        true,
	"NewContentServiceWithOSS": true,
	"NewOSSService":            true,
}

// constructionViolation 描述一处越界构造（文件 + 行号 + 形态）。
type constructionViolation struct {
	File string
	Line int
	Desc string
}

func (v constructionViolation) String() string {
	return filepath.ToSlash(v.File) + ":" + strconv.Itoa(v.Line) + " " + v.Desc
}

// serviceImportPath 解析受限服务包的规范导入路径（module 名读 go.mod，
// 不硬编码）。root 是 backend/ 目录。
func serviceImportPath(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod under %s: %v", root, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")) + "/internal/service"
		}
	}
	t.Fatalf("go.mod under %s has no module directive", root)
	return ""
}

// scanHighRiskConstructions 用 AST 扫描 root 下全部生产源码，返回受限
// 构造器的越界调用点。豁免：internal/container（组合根）、internal/
// testutil（测试 seam）、一切 _test.go。alias import 与 dot import 都
// 解析到规范导入路径后判定；service 包内文件再查裸名调用（定义本身是
// FuncDecl，不会被当作调用）。
func scanHighRiskConstructions(t *testing.T, root string) []constructionViolation {
	t.Helper()
	servicePkg := serviceImportPath(t, root)
	var violations []constructionViolation

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			switch relSlash {
			case "internal/container", "internal/testutil", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		// local name → 规范导入路径；dot import 单独记账。
		imports := make(map[string]string)
		dotImports := false
		for _, imp := range file.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if imp.Name != nil {
				switch imp.Name.Name {
				case ".":
					dotImports = true
					continue
				case "_":
					continue
				default:
					imports[imp.Name.Name] = impPath
				}
				continue
			}
			imports[impPath[strings.LastIndexByte(impPath, '/')+1:]] = impPath
		}
		isServicePkg := file.Name.Name == "service" &&
			strings.HasPrefix(relSlash, "internal/service/")
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if !guardedServiceConstructors[fn.Sel.Name] {
					return true
				}
				if x, ok := fn.X.(*ast.Ident); ok && imports[x.Name] == servicePkg {
					violations = append(violations, constructionViolation{
						File: rel, Line: fset.Position(call.Pos()).Line,
						Desc: "constructs " + x.Name + "." + fn.Sel.Name,
					})
				}
			case *ast.Ident:
				if !guardedServiceConstructors[fn.Name] {
					return true
				}
				// 裸名只可能是 service 包内直呼或 dot import 回潮。
				if isServicePkg || dotImports {
					violations = append(violations, constructionViolation{
						File: rel, Line: fset.Position(call.Pos()).Line,
						Desc: "bare-constructs " + fn.Name,
					})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production sources: %v", err)
	}
	return violations
}

func TestHighRiskServiceConstructorsLiveOnlyInCompositionRoot(t *testing.T) {
	root := filepath.Join("..", "..") // backend/
	violations := scanHighRiskConstructions(t, root)
	if len(violations) > 0 {
		t.Fatalf("high-risk service constructions must live only in container.NewContainer (testutil seam exempt):\n%s",
			joinViolations(violations))
	}
}

func joinViolations(violations []constructionViolation) string {
	parts := make([]string, 0, len(violations))
	for _, v := range violations {
		parts = append(parts, v.String())
	}
	return strings.Join(parts, "\n")
}

// writeFixture 在 dir 下写一个语法合法的 .go 夹具（只需可解析，不需
// 可编译）。
func writeFixture(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// TestConstructionGuardCatchesRegressionShapes 是守门的负样本证明
// （#672 验收：反例先红后绿）：import alias、dot import、service 包内
// 裸 NewContentService、以及字串形态本就覆盖的限定名调用，都必须被
// 点名；组合根 / testutil seam / _test.go 的同形调用必须放行。夹具只
// 供 AST 扫描，从不参与编译。
func TestConstructionGuardCatchesRegressionShapes(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module omnicraft/backend\n\ngo 1.26\n")
	writeFixture(t, filepath.Join(root, "internal", "handler"),
		"admin_alias_fixture.go", `package handler

import contentsvc "omnicraft/backend/internal/service"

func regressionAlias() any { return contentsvc.NewContentServiceWithOSS(nil, nil, nil, nil, nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "handler"),
		"admin_plain_fixture.go", `package handler

import "omnicraft/backend/internal/service"

func regressionQualified() any { return service.NewContentService(nil, nil, nil, nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "handler"),
		"admin_dot_fixture.go", `package handler

import . "omnicraft/backend/internal/service"

func regressionDot() any { return NewOSSService(nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "service"),
		"legacy_bare_fixture.go", `package service

func regressionBare() any { return NewContentService(nil, nil, nil, nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "container"),
		"root_ok_fixture.go", `package container

import "omnicraft/backend/internal/service"

func allowed() any { return service.NewReviewService(nil, nil, nil, nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "testutil"),
		"seam_ok_fixture.go", `package testutil

import "omnicraft/backend/internal/service"

func allowed() any { return service.NewOSSService(nil) }
`)
	writeFixture(t, filepath.Join(root, "internal", "handler"),
		"skip_behavior_test.go", `package handler

import . "omnicraft/backend/internal/service"

func allowed() any { return NewReviewService(nil, nil, nil, nil) }
`)

	violations := scanHighRiskConstructions(t, root)
	joined := joinViolations(violations)
	for _, want := range []string{
		// alias 绕过字串形态的反例：必须以别名形态点名。
		"internal/handler/admin_alias_fixture.go:5 constructs contentsvc.NewContentServiceWithOSS",
		// 限定名回潮（PR #664 已从 admin handler 移除的形态）。
		"internal/handler/admin_plain_fixture.go:5 constructs service.NewContentService",
		// dot import 裸名回潮。
		"internal/handler/admin_dot_fixture.go:5 bare-constructs NewOSSService",
		// service 包内裸构造互建回潮。
		"internal/service/legacy_bare_fixture.go:3 bare-constructs NewContentService",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("guard must catch regression shape %q; violations:\n%s", want, joined)
		}
	}
	for _, banned := range []string{
		"root_ok_fixture",    // 组合根豁免
		"seam_ok_fixture",    // testutil seam 豁免
		"skip_behavior_test", // _test.go 豁免（行为测试自行组装被测面）
	} {
		if strings.Contains(joined, banned) {
			t.Errorf("guard must exempt %s; violations:\n%s", banned, joined)
		}
	}
}
