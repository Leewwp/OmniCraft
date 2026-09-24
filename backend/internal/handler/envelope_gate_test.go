package handler

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// #669 守门（source-contract，先例 pagination_gate_test.go / routes_test.go）：
// handler 非测试源码中的错误信封只许经 internal/pkg/response 发送——
// 出现裸 gin.H{"code" 错误构造即失败。范围 = handler 包（router 包内联
// FEATURE_DISABLED 两点为 routes_test.go 逐字契约、票面范围外）。
// 成功 ack（gin.H{"message" 起头）合法，不拦。
var bareErrorEnvelopePattern = regexp.MustCompile(`gin\.H\{\s*"code"`)

func TestHandlerHasNoBareErrorEnvelope(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handler dir: %v", err)
	}
	violations := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			// 同行直接命中；跨行命中（gin.H{ 行尾 + 下一非空行以 "code" 起头）同记。
			if strings.Contains(line, `gin.H{"code"`) {
				violations = append(violations, name+":"+itoa(i+1))
				continue
			}
			if strings.HasSuffix(strings.TrimRight(line, " \t"), "gin.H{") {
				for j := i + 1; j < len(lines) && j <= i+2; j++ {
					trimmed := strings.TrimLeft(lines[j], " \t")
					if strings.HasPrefix(trimmed, `"code"`) {
						violations = append(violations, name+":"+itoa(i+1))
					}
					break
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("handler 非测试源码存在裸错误信封（#669 后只许走 internal/pkg/response）：%d 处\n%s\n错误响应请用 response.Error / response.CodeOnly / response.CaptchaError",
			len(violations), strings.Join(violations, "\n"))
	}
}

// 反例：守门谓词必须拦住新增裸信封、且不误伤成功 ack 与 response 包调用。
func TestBareErrorEnvelopePatternSemantics(t *testing.T) {
	if !bareErrorEnvelopePattern.MatchString(`c.JSON(400, gin.H{"code": "X", "message": "y"})`) {
		t.Fatal("pattern must match a bare error envelope")
	}
	if !bareErrorEnvelopePattern.MatchString(`c.JSON(status, gin.H{` + "\n" + `"code": "X"})`) {
		t.Fatal("pattern must match a cross-line bare envelope")
	}
	if bareErrorEnvelopePattern.MatchString(`c.JSON(200, gin.H{"message": "ok"})`) {
		t.Fatal("pattern must not flag success acks")
	}
	if bareErrorEnvelopePattern.MatchString(`response.Error(c, 400, "X", "y")`) {
		t.Fatal("pattern must not flag response helper usage")
	}
}
