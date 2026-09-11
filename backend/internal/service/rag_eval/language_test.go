package rageval

import (
	"testing"
)

// TestClassifyQueryLanguage pins the golden-set language rule (rag design §6):
// after stripping punctuation, a Chinese-character ratio > 50% is zh, 10%–50%
// is mixed, and a ratio below 10% (including pure ASCII) is en.
func TestClassifyQueryLanguage(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		// "Blender 插件安装教程": 6 CJK / 13 letters+digits = 46% -> mixed
		// (Chinese query with an English product term, per §6 10%-50% band).
		{"Blender 插件安装教程", "mixed"},
		{"教我如何给游戏安装模组", "zh"},
		{"如何剪辑视频并导出高清画质", "zh"},
		{"哑铃全身训练", "zh"},
		{"流浪猫救助喂养", "zh"},
		{"dumbbell full-body workout plan", "en"},
		{"macbook migration guide", "en"},
		{"5k morning run route", "en"},
		{"MacBook 开箱迁移指南", "mixed"},
		// "艾尔登法环 Boss 怎么打": 8 CJK / 12 = 67% -> zh.
		{"艾尔登法环 Boss 怎么打", "zh"},
		{"拉花 latte art 教程", "mixed"},
		{"quinoa 藜麦备餐", "mixed"},
		{"", "en"},
	}
	for _, tc := range cases {
		if got := ClassifyQueryLanguage(tc.query); got != tc.want {
			t.Errorf("ClassifyQueryLanguage(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

// TestClassifyQueryLanguagePunctuationInsensitive verifies the rule strips
// punctuation before computing ratios: "赛博朋克，2077！截图？" keeps 6 CJK /
// 10 letters+digits = 60% -> zh; "Blender (插件) 安装教程" keeps 6 CJK / 12
// = 50% -> mixed (the band boundary sits in mixed).
func TestClassifyQueryLanguagePunctuationInsensitive(t *testing.T) {
	if got := ClassifyQueryLanguage("赛博朋克，2077！截图？"); got != "zh" {
		t.Errorf("punctuation-stripped Chinese query = %q, want zh", got)
	}
	if got := ClassifyQueryLanguage("Blender (插件) 安装教程"); got != "mixed" {
		t.Errorf("parenthesised query = %q, want mixed", got)
	}
}
