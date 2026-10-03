package service

// #661 守门测试（source-contract 风格，先例 routes_test.go 禁构造 /
// judge_wiring_test.go 钉接线行）：终局四件套（落库/trace 终行/metrics/
// 终局事件）只允许发生在 finalizeTurn 一处；ChatStream 保持装配壳体量。
// 漂移病复发（新代码在别处手写终局）或函数回胖都会被本测试拦下。
//
// #673 收紧：原先的「定义所在文件」整文件豁免留了绕过口——persistPartialTurn
// 的定义文件 agent_conversation.go、终局指标的 agent_stream.go（ChatStream
// 本体）里新增调用点不会被发现。现在定义文件只允许 FuncDecl 本体出现该
// 名（AST 引用计数：调用、方法值、任何非声明引用都算违规）；调用点白名单
// 仍是 finalizeTurn 收口所在文件 + 既有登记例外（agent_service.go 的
// auxLLMTrace 是回合外辅助调用的独立 run）。反例由
// TestQuartetGuardCatchesNewCallSitesInDefinitionFiles 钉死。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// quartetRules 登记每件终局四件套的调用点白名单（owners）与定义文件
// （defHome / defName：该文件内除 FuncDecl 声明名外出现该名即违规）。
// RecordRunEnd 的 agent_service.go 豁免是 auxLLMTrace 的回合外独立 run，
// 非终局收口，维持整文件白名单（其新增调用属辅助调用面，票面未收）。
var quartetRules = []struct {
	piece   string
	marker  string
	owners  map[string]bool
	defHome string
	defName string
}{
	{"terminal trace row", "RecordRunEnd(", map[string]bool{
		"agent_turn_finalize.go": true,
		"agent_service.go":       true, // auxLLMTrace：回合外的辅助调用独立 run
	}, "", ""},
	{"run SLA metric", "recordAgentRunMetrics(", map[string]bool{
		"agent_turn_finalize.go": true,
	}, "agent_stream.go", "recordAgentRunMetrics"},
	{"partial-turn persistence", "persistPartialTurn(", map[string]bool{
		"agent_turn_finalize.go": true,
	}, "agent_conversation.go", "persistPartialTurn"},
	{"terminal done event", "Type:           AgentEventDone,", map[string]bool{
		"agent_turn_finalize.go": true,
	}, "", ""},
}

// defFileExtraReferences 解析单个定义文件，返回 defName 除 FuncDecl
// 声明名之外的引用数（调用、方法值、选择子都计入）。
func defFileExtraReferences(path, defName string) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return 0, err
	}
	exempt := make(map[*ast.Ident]bool)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == defName {
			exempt[fn.Name] = true
		}
	}
	refs := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == defName && !exempt[id] {
			refs++
		}
		return true
	})
	return refs, nil
}

// scanQuartetViolations 扫描 dir 下的生产 .go 文件（_test.go 豁免）：
// 非白名单文件按字串形态查 marker（宽网）；定义文件按 AST 引用计数
// 只放行声明本体。
func scanQuartetViolations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, entry := range entries {
		f := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(f, ".go") || strings.HasSuffix(f, "_test.go") {
			continue
		}
		path := filepath.Join(dir, f)
		for _, rule := range quartetRules {
			if rule.owners[f] {
				continue
			}
			if rule.defHome == f {
				refs, refErr := defFileExtraReferences(path, rule.defName)
				if refErr != nil {
					return nil, refErr
				}
				if refs > 0 {
					violations = append(violations,
						f+" references ["+rule.piece+"] beyond its definition ("+strconv.Itoa(refs)+" extra ref) — terminal work belongs in finalizeTurn")
				}
				continue
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, readErr
			}
			if strings.Contains(string(data), rule.marker) {
				violations = append(violations, f+" touches quartet piece ["+rule.piece+"] outside finalizeTurn")
			}
		}
	}
	return violations, nil
}

func TestTerminalQuartetLivesOnlyInFinalizeTurn(t *testing.T) {
	violations, err := scanQuartetViolations(".")
	if err != nil {
		t.Fatalf("scan service sources: %v (run from internal/service)", err)
	}
	if len(violations) > 0 {
		t.Fatalf("terminal quartet must stay inside finalizeTurn (agent_turn_finalize.go):\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestQuartetGuardCatchesNewCallSitesInDefinitionFiles 是收紧后的反例
// 证明（#673 验收：守门反例在增强前失败、增强后通过）：定义文件里新增
// persistPartialTurn / recordAgentRunMetrics 调用点（如 ChatStream 手写
// 部分落库或终局指标）、非白名单文件触碰 marker，都必须被点名；定义
// 本体与 finalizeTurn 收口文件的合法调用必须放行。夹具只供扫描，从不
// 参与编译。
func TestQuartetGuardCatchesNewCallSitesInDefinitionFiles(t *testing.T) {
	dir := t.TempDir()
	writeGuardFixture(t, dir, "agent_conversation.go", `package service

func (s *AgentService) persistPartialTurn(id int64, partial string) {}

func drift(s *AgentService) { s.persistPartialTurn(1, "hand-rolled") }
`)
	writeGuardFixture(t, dir, "agent_stream.go", `package service

func recordAgentRunMetrics(status string) {}

func driftShell() { recordAgentRunMetrics("error") }
`)
	writeGuardFixture(t, dir, "agent_tools.go", `package service

func driftTool(s *AgentService) { s.persistPartialTurn(9, "") }
`)
	writeGuardFixture(t, dir, "agent_turn_finalize.go", `package service

func ok1(s *AgentService) { s.persistPartialTurn(1, "finalize") }
func ok2() { recordAgentRunMetrics("error") }
func ok3(rec recordy) { rec.RecordRunEnd() }
const okDone = "Type:           AgentEventDone,"
`)
	writeGuardFixture(t, dir, "agent_service.go", `package service

func aux(r recordy) { r.RecordRunEnd() }
`)

	violations, err := scanQuartetViolations(dir)
	if err != nil {
		t.Fatalf("scan fixtures: %v", err)
	}
	joined := strings.Join(violations, "\n")
	for _, want := range []string{
		// 定义文件新增调用点：必须点名（增强前的漏洞形态）。
		"agent_conversation.go references [partial-turn persistence] beyond its definition",
		"agent_stream.go references [run SLA metric] beyond its definition",
		// 非白名单文件的字串形态：原有宽网照旧。
		"agent_tools.go touches quartet piece [partial-turn persistence] outside finalizeTurn",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("guard must catch drift %q; violations:\n%s", want, joined)
		}
	}
	for _, banned := range []string{
		"agent_turn_finalize.go", // 收口文件的合法调用放行
		"agent_service.go",       // auxLLMTrace 例外放行
	} {
		if strings.Contains(joined, banned) {
			t.Errorf("guard must exempt %s; violations:\n%s", banned, joined)
		}
	}
}

func writeGuardFixture(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// chatStreamBodyLimit pins the assembly shell's size: the pre-#661 function
// was 608 lines; the deep-module split must keep ChatStream a thin shell.
const chatStreamBodyLimit = 220

func TestChatStreamStaysAnAssemblyShell(t *testing.T) {
	data, err := os.ReadFile("agent_stream.go")
	if err != nil {
		t.Fatalf("read agent_stream.go: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	depth := 0
	for i, line := range lines {
		if start < 0 {
			if strings.HasPrefix(line, "func (s *AgentService) ChatStream(") {
				start = i
			}
			continue
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 && !strings.HasSuffix(line, "func (s *AgentService) ChatStream(ctx context.Context, userID int64, turn ChatTurnInput, resolved *ResolvedChatContext, handler func(ev AgentStreamEvent) error) error {") {
			if i > start {
				body := i - start + 1
				if body > chatStreamBodyLimit {
					t.Fatalf("ChatStream body = %d lines, exceeds the assembly-shell limit %d — move new logic into the turnRunner/finalizeTurn modules", body, chatStreamBodyLimit)
				}
				return
			}
		}
	}
	t.Fatal("could not locate the end of ChatStream")
}
