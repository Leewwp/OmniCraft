package service

// #661 守门测试（source-contract 风格，先例 routes_test.go 禁构造 /
// judge_wiring_test.go 钉接线行）：终局四件套（落库/trace 终行/metrics/
// 终局事件）只允许发生在 finalizeTurn 一处；ChatStream 保持装配壳体量。
// 漂移病复发（新代码在别处手写终局）或函数回胖都会被本测试拦下。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// quartetOwners lists the production files allowed to touch each terminal
// quartet piece. agent_service.go hosts the auxiliary-call trace wrapper
// (auxLLMTrace) which legitimately records its own non-turn runs.
var quartetRules = []struct {
	piece  string
	marker string
	owners map[string]bool
}{
	{"terminal trace row", "RecordRunEnd(", map[string]bool{
		"agent_turn_finalize.go": true,
		"agent_service.go":       true, // auxLLMTrace：回合外的辅助调用独立 run
	}},
	{"run SLA metric", "recordAgentRunMetrics(", map[string]bool{
		"agent_turn_finalize.go": true,
		"agent_stream.go":        true, // 定义所在文件
	}},
	{"partial-turn persistence", "persistPartialTurn(", map[string]bool{
		"agent_turn_finalize.go": true,
		"agent_conversation.go":  true, // 定义所在文件
	}},
	{"terminal done event", "Type:           AgentEventDone,", map[string]bool{
		"agent_turn_finalize.go": true,
	}},
}

func TestTerminalQuartetLivesOnlyInFinalizeTurn(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob service sources: %v (run from internal/service)", err)
	}
	var violations []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, rule := range quartetRules {
			if rule.owners[f] {
				continue
			}
			if strings.Contains(string(data), rule.marker) {
				violations = append(violations, f+" touches quartet piece ["+rule.piece+"] outside finalizeTurn")
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("terminal quartet must stay inside finalizeTurn (agent_turn_finalize.go):\n%s",
			strings.Join(violations, "\n"))
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
