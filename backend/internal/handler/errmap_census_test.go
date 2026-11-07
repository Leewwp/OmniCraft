package handler

// errmap census 终测（C3 #924）：以回归门同一 go/ast 口径复测 handler
// 包全部生产文件的 errors.Is 用法，验收口径是 D 型映射链（errors.Is 条
// 件分支且分支体内含响应调用）归零，不是原始 errors.Is 处数——T 型（事
// 务内 sentinel 翻译）、C 型（commit+204 控制流）、谓词/整形与纯副作用
// 分支是合法控制流，逐文件带理由钉在 censusResiduals 清单里。新增任何
// errors.Is 用法都会在这里显形（清单计数失配即失败），迫使作者为每处
// 新用法写明合法形态理由；D 型回潮则直接失败。`==` 直比映射链不在本
// census 口径内（C3 基线只数 errors.Is；残留文件见 #924 PR 描述的观察
// 清单）。

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

// censusEntry 钉一个文件内合法保留的 errors.Is 用法：count = 该文件预
// 期保留处数，reason = 逐处理由（T=事务内翻 sentinel / C=commit+204
// 控制流 / P=纯谓词或状态整形 / G=负向守卫分流统一 map / S=纯副作用分
// 支）。
type censusEntry struct {
	count  int
	reason string
}

// censusResiduals = C3.4 终态残余清单（2026-11-01 口径，共 13 处）。
// 不在清单内的文件 errors.Is 计数必须为 0；清单内计数漂移（新增或删
// 除）都会失败——改动时同步更新 reason 里的行号与形态说明。
var censusResiduals = map[string]censusEntry{
	"admin.go":              {count: 3, reason: "T 型事务内 sentinel 翻译 ×2（gorm.ErrRecordNotFound → return sentinel，事务闭包内无响应调用）+ S 型纯副作用分支 ×1（DLQ replay 非 NotFound 失败先写按分支失败审计不发响应，随后统一 dlqReplayErrMap.write）"},
	"admin_archive_scan.go": {count: 2, reason: "T 型事务内 sentinel 翻译 ×2（归档扫描事务闭包内 NotFound 翻 sentinel）"},
	"admin_evals.go":        {count: 1, reason: "G 型负向守卫 ×1（非 NotFound 才分流 evalDraftPrecheckErrMap.write，NotFound 走正常草稿创建流）"},
	"admin_prompt.go":       {count: 1, reason: "G 型负向守卫 ×1（非 NotFound 才分流 promptNextVersionErrMap.write，NotFound 走 next=1 冷启动流）"},
	"agent.go":              {count: 2, reason: "P 型 rollback 谓词 ×1（ErrInvalidTransaction 忽略判定，分支只打日志）+ C 型 commit+204 控制流 ×1（notFound 谓词前置，c.Status 不在门响应清单）"},
	"agent_guest.go":        {count: 1, reason: "G 型负向守卫 ×1（非配额态丢失才分流 guestQuotaStatusErrMap.write，丢失态 fail-open 走默认配额）"},
	"auth.go":               {count: 1, reason: "S 型纯副作用分支 ×1（ErrInvalidCredentials 命中先 recordLoginFailure 记账不发响应，随后统一 loginErrMap.write）"},
	"captcha.go":            {count: 1, reason: "P 型状态整形 ×1（ErrTicketStoreUnavailable 判 503/500，响应在分支外统一发出）"},
	"category.go":           {count: 1, reason: "T 型事务内 sentinel 翻译 ×1（admin 更新事务闭包内 NotFound 翻 service.ErrCategoryNotFound）"},
}

// censusErrorsIsCount 解析单个源文件，返回其中 errors.Is(...) 调用处数
// （AST 级计数，注释不计）与 D 型映射链处数（复用回归门 gateViolations
// 口径）。
func censusErrorsIsCount(t *testing.T, filename, src string) (total, dForm int) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("census: parse %s: %v", filename, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "errors" && sel.Sel.Name == "Is" {
			total++
		}
		return true
	})
	dForm = len(gateViolations(t, filename, src))
	return total, dForm
}

// TestErrMapCensus_DFormZero 锁终态验收指标：handler 包生产文件（引擎
// errmap.go 永久豁免除外）内 D 型映射链归零——与回归门空白名单互为独
// 立复测（门按文件白名单机制拦新增，本测试按口径直扫全量）。
func TestErrMapCensus_DFormZero(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("census: read handler dir: %v", err)
	}
	var dTotal int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, exempt := gatePermanentExemption[name]; exempt {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("census: read %s: %v", name, err)
		}
		total, dForm := censusErrorsIsCount(t, name, string(src))
		if dForm > 0 {
			t.Errorf("census: %s 含 %d 处 D 型映射链（errors.Is 分支 + 响应调用），须迁 endpointErrMap 声明", name, dForm)
		}
		dTotal += dForm
		if total == 0 {
			continue
		}
		t.Logf("census: %s errors.Is=%d（合法保留，见 censusResiduals）", name, total)
	}
	if dTotal != 0 {
		t.Fatalf("census: D 型映射链终态应为 0，实得 %d", dTotal)
	}
}

// TestErrMapCensus_ResidualInventory 钉合法残余清单：每文件 errors.Is
// 总数必须与 censusResiduals 的 count 逐文件相等（不在清单 = 0）。新增
// errors.Is（哪怕合法形态）或删除既有用法都会失败，逼清单与理由同步。
func TestErrMapCensus_ResidualInventory(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("census: read handler dir: %v", err)
	}
	observed := make(map[string]int)
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, exempt := gatePermanentExemption[name]; exempt {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("census: read %s: %v", name, err)
		}
		total, dForm := censusErrorsIsCount(t, name, string(src))
		if dForm > 0 {
			t.Fatalf("census: %s 含 D 型映射链，先迁 endpointErrMap 再谈清单（%d 处）", name, dForm)
		}
		if total > 0 {
			observed[name] = total
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		want, listed := censusResiduals[name]
		if !listed {
			t.Errorf("census: %s 出现 %d 处 errors.Is 但不在残余清单——合法形态须在 censusResiduals 注明理由，D 型须迁 endpointErrMap", name, observed[name])
			continue
		}
		if observed[name] != want.count {
			t.Errorf("census: %s errors.Is 计数 = %d，清单钉 %d（理由：%s）——增删用法须同步更新清单与理由", name, observed[name], want.count, want.reason)
		}
	}
	for name, want := range censusResiduals {
		if got := observed[name]; got == 0 {
			t.Errorf("census: 清单条目 %s 已无 errors.Is（stale：%s）——移出清单", name, want.reason)
		}
	}
}

// TestErrMapCensus_Report 输出终态逐文件清单（file:line 级），供 PR 描述
// 与后续复核直接引用；行号取当前工作树，仅诊断用途不断言。
func TestErrMapCensus_Report(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("census: read handler dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("census: read %s: %v", name, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, string(src), 0)
		if err != nil {
			t.Fatalf("census: parse %s: %v", name, err)
		}
		var sites []string
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "errors" && sel.Sel.Name == "Is" {
				pos := fset.Position(node.Pos())
				sites = append(sites, fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line))
			}
			return true
		})
		if len(sites) > 0 {
			if _, exempt := gatePermanentExemption[name]; exempt {
				t.Logf("census: %s = 引擎永久豁免（%d 处 errors.Is）", name, len(sites))
				continue
			}
			t.Logf("census: %s = %s", name, strings.Join(sites, ", "))
		}
	}
}
