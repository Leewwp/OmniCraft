package middleware

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// #927 守门（source-contract，镜像 handler 包 envelope_gate_test.go / #669）：
// middleware 非测试源码中的错误信封只许经 internal/pkg/response 发送——
// 出现裸 gin.H{"code" 错误构造即失败。鉴权 / 限流 / CSRF / panic 恢复面
// 的错误 wire 全部走 response.Error / response.ErrorWithDetails，语义
// （status / headers / abort）由助手统一承担。
// 成功 ack（gin.H{"message" 起头）合法，不拦。
var bareErrorEnvelopePattern = regexp.MustCompile(`gin\.H\{\s*"code"`)

// envelopeGateExemptions 是逐处注明白名单：键 = "file.go:line"（信封
// 调用点起始行），值 = 必须写明理由。当前为零：middleware 面无豁免。
var envelopeGateExemptions = map[string]string{}

func TestMiddlewareHasNoBareErrorEnvelope(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read middleware dir: %v", err)
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
			// 同行直接命中；跨行命中（gin.H{ 行尾 + 下一行以 "code" 起头）同记。
			if strings.Contains(line, `gin.H{"code"`) {
				violations = append(violations, name+":"+strconv.Itoa(i+1))
				continue
			}
			if strings.HasSuffix(strings.TrimRight(line, " \t"), "gin.H{") {
				for j := i + 1; j < len(lines) && j <= i+2; j++ {
					trimmed := strings.TrimLeft(lines[j], " \t")
					if strings.HasPrefix(trimmed, `"code"`) {
						violations = append(violations, name+":"+strconv.Itoa(i+1))
					}
					break
				}
			}
		}
	}
	unexempted := []string{}
	for _, v := range violations {
		if _, ok := envelopeGateExemptions[v]; !ok {
			unexempted = append(unexempted, v)
		}
	}
	if len(unexempted) > 0 {
		t.Fatalf("middleware 非测试源码存在裸错误信封（#927 后只许走 internal/pkg/response）：%d 处\n%s\n错误响应请用 response.Error / response.ErrorWithDetails",
			len(unexempted), strings.Join(unexempted, "\n"))
	}
}

// 反例：守门谓词必须拦住新增裸信封、且不误伤成功 ack 与 response 包调用。
func TestMiddlewareBareErrorEnvelopePatternSemantics(t *testing.T) {
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
