package rules

import (
	"testing"
	"unicode/utf8"
)

// #800 收尾：截断必须落在 rune 边界——字节截断会把 CJK 字符砍半，在
// 生成的 config.md 里留下非法 UTF-8。
func TestConfigTableTruncationIsRuneSafe(t *testing.T) {
	long := "端点的绝对期限说明文字很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长"
	got := truncateOnRuneBoundary(long, 117)
	if len(got) > 117 {
		t.Fatalf("byte length %d exceeds budget 117", len(got))
	}
	if len(got) >= len(long) {
		t.Fatal("long input must actually be truncated")
	}
	if !utf8.ValidString(got) || !utf8.ValidString(got+"...") {
		t.Fatalf("truncated row text must stay valid UTF-8: %q", got)
	}
	if truncateOnRuneBoundary("短说明", 117) != "短说明" {
		t.Fatal("short string must pass through unchanged")
	}
	if truncateOnRuneBoundary("任何", 0) != "" {
		t.Fatal("zero budget must yield empty string")
	}
}
