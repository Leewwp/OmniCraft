package service

// #754 D 工具名守门：当前有效模板（最新 agent_system 升级 + registry 缺席
// fallback）与 serverOwnedSystemPrompt 动态拼接的 IP 子句中出现的工具名，
// 必须属于实际 advertised 的本地工具集合；幽灵名 cited_search 只允许存在
// 于历史 v1–v5 不可变快照（不做全仓替换）。

import (
	"context"
	"regexp"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/service/promptregistry"
)

var promptToolNamePattern = regexp.MustCompile(`\b(search_content|search_ips|cited_search|get_content_detail|get_usage_guide|suggest_publish_metadata|generate_image)\b`)

func TestEffectivePromptTemplatesReferenceAdvertisedToolsOnly(t *testing.T) {
	advertised := map[string]bool{}
	for _, name := range (&AgentService{}).RegisteredToolNames() {
		advertised[name] = true
	}
	advertised[ToolGenerateImage] = true // 生图工具在配置可用时 advertised（SP-23 M1）

	// 最新 agent_system 升级 = 有效 DB 模板内容。
	var latest *promptregistry.UpgradeSeed
	for i := range promptregistry.RegistryUpgrades {
		up := &promptregistry.RegistryUpgrades[i]
		if up.SlotName == promptregistry.SlotAgentSystem.Name && (latest == nil || up.Version > latest.Version) {
			latest = up
		}
	}
	if latest == nil || latest.Version < 6 {
		t.Fatalf("agent_system v6 upgrade must be shipped (got v%d)", func() int {
			if latest == nil {
				return 0
			}
			return latest.Version
		}())
	}
	for _, match := range promptToolNamePattern.FindAllString(latest.Content, -1) {
		if !advertised[match] {
			t.Errorf("effective template references non-advertised tool %q", match)
		}
	}
	if got := promptToolNamePattern.FindAllString(latest.Content, -1); len(got) == 0 {
		t.Fatal("effective template must reference the real search tools")
	}

	// registry 缺席 fallback 同样勘正。
	fallback := promptregistry.SlotAgentSystem.Fallback
	if fallback == "" {
		t.Fatal("agent_system must define a fallback template (#754 D)")
	}
	for _, match := range promptToolNamePattern.FindAllString(fallback, -1) {
		if !advertised[match] {
			t.Errorf("fallback template references non-advertised tool %q", match)
		}
	}

	// 历史 v1–v5 快照允许保留 cited_search（不可变），但不得进入 v6/fallback。
	for i := range promptregistry.RegistryUpgrades {
		up := promptregistry.RegistryUpgrades[i]
		if up.SlotName == promptregistry.SlotAgentSystem.Name && up.Version >= 6 && regexp.MustCompile(`cited_search`).MatchString(up.Content) {
			t.Errorf("agent_system v%d must not carry the ghost name", up.Version)
		}
	}
}

// 动态 IP 分类子句（运行时代码拼接）同样只引用真实工具与浏览参数。
func TestDynamicIPClauseReferencesRealToolAndBrowse(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}, IPCategories: []string{"game", "anime"}}
	provider := &recordingStreamProvider{}
	svc := NewAgentService(provider, nil, nil, nil, nil, cfg)
	msg := svc.serverOwnedSystemPrompt(context.Background(), model.AgentChatSurfaceGlobal, nil)
	content := msg.Content

	advertised := map[string]bool{}
	for _, name := range (&AgentService{}).RegisteredToolNames() {
		advertised[name] = true
	}
	advertised[ToolGenerateImage] = true
	for _, match := range promptToolNamePattern.FindAllString(content, -1) {
		if !advertised[match] {
			t.Errorf("rendered system prompt references non-advertised tool %q", match)
		}
	}
	if !regexp.MustCompile(`sort=newest`).MatchString(content) || !regexp.MustCompile(`sort=most_contents`).MatchString(content) {
		t.Fatal("dynamic IP clause must teach the explicit browse parameters (#754 A/D)")
	}
	if regexp.MustCompile(`cited_search`).MatchString(content) {
		t.Fatal("dynamic clause must not carry the ghost tool name")
	}
}
