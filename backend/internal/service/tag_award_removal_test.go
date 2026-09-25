package service

import (
	"os"
	"strings"
	"testing"
)

// #659 删除契约（用户裁决 B）：未接线的 tag_recognized 声誉奖励规则整体移除。
// 生产组合根从未注入 TagService 的声誉依赖（#657 勘误），规则删除不改变
// 生产奖励流。历史 reputation_logs 与前端 reason 文案保留（历史兼容显示）。
func readServiceSourceForRemoval(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func TestTagRecognizedAwardRuleIsFullyRemoved(t *testing.T) {
	tagSrc := readServiceSourceForRemoval(t, "tag_service.go")
	for _, forbidden := range []string{
		"SetReputationService",
		"reputSvc",
		"AwardTagRecognized",
	} {
		if strings.Contains(tagSrc, forbidden) {
			t.Errorf("tag_service.go 仍含已裁决移除的奖励接线 %q", forbidden)
		}
	}

	repSrc := readServiceSourceForRemoval(t, "reputation_service.go")
	if strings.Contains(repSrc, "AwardTagRecognized") {
		t.Error("reputation_service.go 仍含 AwardTagRecognized（无调用方的奖励方法）")
	}

	cfgSrc := readServiceSourceForRemoval(t, "../../config/config.go")
	if strings.Contains(cfgSrc, "ScoreTagRecognized") {
		t.Error("config.go 仍含 ScoreTagRecognized 字段（无消费方的配置项）")
	}

	yamlSrc := readServiceSourceForRemoval(t, "../../config.yaml")
	if strings.Contains(yamlSrc, "score_tag_recognized") {
		t.Error("config.yaml 仍含 score_tag_recognized 条目（失效配置）")
	}
}
