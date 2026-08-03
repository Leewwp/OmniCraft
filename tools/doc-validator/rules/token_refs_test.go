package rules

import (
	"strings"
	"testing"
)

func TestDefinedTokensInRecognizesProseAndTableForms(t *testing.T) {
	content := strings.Join([]string{
		"| Token | 值 | 用途 |",
		"| `--header-h` | `52px` | 顶部导航高度 |",
		"| `--elevation-1` | `0 1px 2px` | 静置卡片 |",
		"`--canvas-default` 是 `#FFFFFF`",
		"prose definition: --max-w: 1440px",
	}, "\n")

	defined := definedTokensIn(content)
	for _, token := range []string{"--header-h", "--elevation-1", "--canvas-default", "--max-w"} {
		if !defined[token] {
			t.Errorf("definedTokensIn(%q) missing table/prose-defined token %s", content, token)
		}
	}
	if defined["--not-present"] {
		t.Errorf("definedTokensIn(%q) must not include unreferenced tokens", content)
	}
}
