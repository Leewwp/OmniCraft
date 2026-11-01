package handler

// 回归门 v1（C3 #922）：handler 包内不得新增手写 sentinel→HTTP 映射链。
// go/ast 模式级识别——只拦「errors.Is 条件分支且分支体内含响应调用」
// （响应调用 = response.* 任意函数、c.JSON / c.AbortWithStatusJSON；
// c.Status 不在清单，commit+204 控制流放行）。合法形态全部放行：纯副
// 作用分支（副作用后统一 write() 发响应）、return sentinel（事务内翻
// 译）、日志、rollback、quota 谓词、状态整形（captcha 形态）。
//
// 豁免：
//   - errmap.go 永久豁免——引擎 write 实现本身就是被拦模式（errors.Is
//     分支内发响应），它是规则的执行者而非违规者。
//   - gateWhitelist = 尚未迁移到手写链以外写法的存量文件（C3 推广期临
//     时豁免，每票迁完移出自己的文件；白名单只缩不涨，条目失效时本门
//     会报 stale 逼清理）。
//
// 模式级门不需要文件级豁免来放行 T/C 合法用法：含事务内翻译/谓词的文
// 件不因合法用法整文件免检，只有真实映射链才计入违规。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gateResponseCall 返回 stmts 内第一个直接 HTTP 响应调用：response.* 任
// 意函数，或 gin context（receiver 惯用名 c）上的 c.JSON /
// c.AbortWithStatusJSON。c.Status 是状态码赋值控制流而非响应体写出，
// 刻意不在清单（agent.go commit+204 形态合法）。
func gateResponseCall(stmts []ast.Stmt) *ast.CallExpr {
	var found *ast.CallExpr
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(node ast.Node) bool {
			if found != nil {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok {
				if x.Name == "response" {
					found = call
					return false
				}
				if x.Name == "c" && (sel.Sel.Name == "JSON" || sel.Sel.Name == "AbortWithStatusJSON") {
					found = call
					return false
				}
			}
			return true
		})
		if found != nil {
			return found
		}
	}
	return found
}

// gateErrorsIsIn 报告 n 内是否出现 errors.Is(...) 调用（包名惯用
// errors，未别名）。
func gateErrorsIsIn(n ast.Node) bool {
	if n == nil {
		return false
	}
	seen := false
	ast.Inspect(n, func(node ast.Node) bool {
		if seen {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "errors" && sel.Sel.Name == "Is" {
			seen = true
			return false
		}
		return true
	})
	return seen
}

// gateViolations 解析单个源文件，返回其中全部手写映射链位置（file:line
// 形式）。模式：if 语句的条件（含 init）含 errors.Is 且 then 分支体内含
// 响应调用；或 case 子句的条件列表含 errors.Is 且分支体内含响应调用。
// else 分支与不含 errors.Is 的分支不计（else-if 链由嵌套 if 自行命中）。
func gateViolations(t *testing.T, filename, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("gate: parse %s: %v", filename, err)
	}
	var violations []string
	add := func(n ast.Node, kind string) {
		pos := fset.Position(n.Pos())
		violations = append(violations, fmt.Sprintf("%s:%d: %s", filepath.Base(pos.Filename), pos.Line, kind))
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.IfStmt:
			hasIs := gateErrorsIsIn(stmt.Cond) || gateErrorsIsIn(stmt.Init)
			if hasIs && stmt.Body != nil && gateResponseCall(stmt.Body.List) != nil {
				add(stmt, "if 分支 errors.Is + 响应调用（手写映射链）")
			}
		case *ast.CaseClause:
			hasIs := false
			for _, expr := range stmt.List {
				if gateErrorsIsIn(expr) {
					hasIs = true
					break
				}
			}
			if hasIs && gateResponseCall(stmt.Body) != nil {
				add(stmt, "case 分支 errors.Is + 响应调用（手写映射链）")
			}
		}
		return true
	})
	return violations
}

// gatePermanentExemption 永久豁免的引擎文件（理由见文件头注释）。
var gatePermanentExemption = map[string]string{
	"errmap.go": "引擎本体：write 实现即「errors.Is 分支 + 响应调用」模式，是规则的执行者",
}

// gateWhitelist v1 = C3 推广期尚未迁移的存量文件（reason 指向收编票）。
// 每票迁完移出自己的文件；条目对应的文件不再含违规模式时本门报
// stale，逼白名单只缩不涨。
var gateWhitelist = map[string]string{
	"admin_archive_scan.go": "C3.3b #929（admin 卫星四文件）",
	"admin_evals.go":        "C3.3b #929（admin 卫星四文件）",
	"admin_feedback.go":     "C3.3b #929（admin 卫星四文件）",
	"admin_prompt.go":       "C3.3b #929（admin 卫星四文件）",
	"browse_history.go":     "C3.4 #924（扫尾收容）",
	"category.go":           "C3.4 #924（扫尾收容；事务内翻译已天然放行）",
	"collection.go":         "C3.4 #924（扫尾收容）",
	"content.go":            "C3.4 #924（pilot 两端点之外的余链）",
	"discussion.go":         "C3.4 #924（扫尾收容）",
	"ip.go":                 "C3.4 #924（扫尾收容）",
	"message.go":            "C3.4 #924（扫尾收容）",
	"series.go":             "C3.4 #924（扫尾收容）",
	"tag.go":                "C3.4 #924（扫尾收容）",
	"user.go":               "C3.4 #924（扫尾收容；gorm.ErrRecordNotFound == 变体一并收编）",
}

// TestErrMapGate_MatcherSemantics 在合成片段上锁定识别口径：违规形态必
// 须命中，合法形态（纯副作用分支 / return sentinel / 日志 / rollback /
// quota 谓词 / commit+204 / 状态整形）一个不误报。
func TestErrMapGate_MatcherSemantics(t *testing.T) {
	const prelude = `package handler
import (
	"errors"
	"log/slog"
	"net/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"
)
var e = service.ErrX
func f(c *gin.Context, err error, tx *gorm.DB) {
`
	cases := []struct {
		name string
		body string
		want bool
	}{
		{name: "if chain with response.Error", want: true,
			body: `if errors.Is(err, e) { response.Error(c, 400, "X", "m"); return }`},
		{name: "switch case with response.Error", want: true,
			body: `switch { case errors.Is(err, e): response.Error(c, 400, "X", "m") }`},
		{name: "if branch with c.JSON", want: true,
			body: `if errors.Is(err, e) { c.JSON(400, gin.H{"code": "X"}) }`},
		{name: "if branch with c.AbortWithStatusJSON", want: true,
			body: `if errors.Is(err, e) { c.AbortWithStatusJSON(400, gin.H{"code": "X"}) }`},
		{name: "nested errors.Is inside default with response", want: true,
			body: `switch { default: if errors.Is(err, e) { response.Error(c, 400, "X", "m") } }`},
		{name: "errors.Is in if init", want: true,
			body: `if w := fmt.Errorf("wrap: %w", err); errors.Is(w, e) { response.SafeErrorResponse(c, 400, "X", err) }`},
		{name: "negated sentinel fallback with response (inverted D-form)", want: true,
			body: `if err != nil && !errors.Is(err, e) { response.Error(c, 500, "DB_ERROR", "boom"); return }`},
		{name: "pure side effect then unified write", want: false,
			body: `if errors.Is(err, e) { h.auditFailed(c, err); return }; xErrMap.write(c, err)`},
		{name: "side effect only, no response in branch", want: false,
			body: `if errors.Is(err, e) { h.recordLoginFailure(c, "a@b.c"); return }`},
		{name: "return sentinel translation (T-form)", want: false,
			body: `if errors.Is(err, gorm.ErrRecordNotFound) { return service.ErrCategoryNotFound }`},
		{name: "log only branch", want: false,
			body: `if errors.Is(err, e) { slog.Error("boom", "error", err); return }`},
		{name: "rollback predicate", want: false,
			body: `if errors.Is(err, e) { tx.Rollback(); return err }`},
		{name: "quota predicate boolean", want: false,
			body: `if err != nil && errors.Is(err, e) { return false }`},
		{name: "commit plus 204 via c.Status", want: false,
			body: `if errors.Is(err, gorm.ErrRecordNotFound) { tx.Commit(); c.Status(http.StatusNoContent); return }`},
		{name: "status shaping without response in branch", want: false,
			body: `status := 503; if !errors.Is(err, e) { status = 500 }; response.Error(c, status, "X", "m")`},
		{name: "response outside errors.Is branch", want: false,
			body: `if err != nil { response.Error(c, 500, "DB_ERROR", "boom") }`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := prelude + tt.body + "\n}"
			got := gateViolations(t, "snippet.go", src)
			if tt.want && len(got) == 0 {
				t.Fatalf("matcher missed violation in:\n%s", tt.body)
			}
			if !tt.want && len(got) > 0 {
				t.Fatalf("matcher false positive %v on legal form:\n%s", got, tt.body)
			}
		})
	}
}

// TestErrMapGate_NoNewHandwrittenMappingChains 扫描 handler 包全部生产
// 文件：白名单外出现手写映射链即失败（新增文件不允许带链合入）；白名
// 单条目失效（文件已无违规）也失败，保证白名单只缩不涨。
func TestErrMapGate_NoNewHandwrittenMappingChains(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("gate: read handler dir: %v", err)
	}
	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		goFiles = append(goFiles, entry.Name())
	}
	if len(goFiles) == 0 {
		t.Fatal("gate: no production files found — test must run inside backend/internal/handler")
	}

	violationsByFile := make(map[string][]string)
	for _, name := range goFiles {
		if _, exempt := gatePermanentExemption[name]; exempt {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("gate: read %s: %v", name, err)
		}
		violationsByFile[name] = gateViolations(t, name, string(src))
	}

	var names []string
	for name := range gateWhitelist {
		names = append(names, name)
	}
	for name := range violationsByFile {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		violations, whitelisted := gateWhitelist[name]
		switch {
		case len(violationsByFile[name]) > 0 && !whitelisted:
			t.Errorf("gate: %s 含手写映射链（迁移到 endpointErrMap 声明或加入白名单并注明收编票）:\n  %s",
				name, strings.Join(violationsByFile[name], "\n  "))
		case len(violationsByFile[name]) == 0 && whitelisted:
			t.Errorf("gate: 白名单条目 %s 已无违规模式（stale，%s）——迁移完成后必须移出自己的文件", name, violations)
		}
	}
}
