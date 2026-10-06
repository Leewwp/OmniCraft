package mcpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// SP-16 #449: hermetic end-to-end over the real Streamable HTTP transport —
// an SDK client connects to the handler, lists the four anonymous read-only
// tools and verifies the zero-leak surface review against the #446 exposure
// matrix baseline: non-public content must be invisible through every tool.
func newTestMCPStack(t *testing.T) (*sdkmcp.ClientSession, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{},
		&model.ContentTag{}, &model.Tag{}, &model.Category{}, &model.ContentUsageGuide{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	if err := db.Create(&model.User{ID: 301, Email: "mcp-author@example.com", Username: "mcp_author", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	seed := []struct {
		id     int64
		marker string
		status string
		isPub  bool
	}{
		{401, "sp16amcpPUB", "published", true},
		{402, "sp16amcpPRIV", "published", false},
		{403, "sp16amcpPEND", "pending", true},
		{404, "sp16amcpBAN", "banned", true},
	}
	for _, s := range seed {
		c := model.ContentItem{
			ID: s.id, Title: s.marker + " title", Description: s.marker + " description",
			AuthorID: 301, Zone: "original", Category: "game", ContentType: "sheet_music",
			Status: s.status, IsPublic: s.isPub, AllowCopy: true,
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed content %d: %v", s.id, err)
		}
		att := model.ContentAttachment{ID: 900 + s.id, ContentItemID: s.id, FileType: "file", OSSKey: s.marker + "_att", ScanStatus: "not_required"}
		if err := db.Create(&att).Error; err != nil {
			t.Fatalf("seed attachment: %v", err)
		}
	}
	if err := db.Create(&model.ContentUsageGuide{
		ContentItemID: 401, Locale: "zh",
		Requirements: `["sp16amcpPUB req"]`, Steps: `["sp16amcpPUB step"]`,
		Notes: "sp16amcpPUB notes", Source: "author",
	}).Error; err != nil {
		t.Fatalf("seed guide: %v", err)
	}

	contentRepo := repository.NewContentRepository(db)
	// #813: 假凭证 OSS 域让附件展示签名在测试中真实可算（签名是纯本地
	// 计算，无网络调用）——get_content 附件投影按家族断言签名有无。
	cfg := &config.Config{
		OSS: config.OSSConfig{
			Endpoint:        "http://127.0.0.1:9201",
			AccessKeyID:     "test-access-key",
			AccessKeySecret: "test-access-secret",
			BucketName:      "test-bucket",
			Domain:          "http://127.0.0.1:9201/test-bucket",
		},
	}
	guideSvc := service.NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo)
	handler := NewHandler(Deps{
		DB:            db,
		SearchRepo:    repository.NewSearchRepository(db),
		ContentRepo:   contentRepo,
		CategoryRepo:  repository.NewCategoryRepository(db),
		GuideSvc:      guideSvc,
		DisplaySigner: service.NewDisplayURLSigner(cfg),
	})

	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "mcp-test-client", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect streamable http: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, db
}

func callToolText(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: json.RawMessage(raw)})
	if err != nil {
		t.Fatalf("%s call: %v", name, err)
	}
	if res.IsError {
		return "", false
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text, true
}

func TestMCPServerFourReadOnlyTools(t *testing.T) {
	session, testDB := newTestMCPStack(t)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"omnicraft_search", "omnicraft_search_ips", "omnicraft_get_content", "omnicraft_get_usage_guide", "omnicraft_list_categories"} {
		if !names[want] {
			t.Errorf("tool %s missing from tools/list", want)
		}
	}
	if len(tools.Tools) != 5 {
		t.Errorf("tools/list returned %d tools, want exactly the five anonymous read-only tools", len(tools.Tools))
	}

	t.Run("search finds the public fixture only", func(t *testing.T) {
		text, ok := callToolText(t, session, "omnicraft_search", map[string]any{"query": "sp16amcp"})
		if !ok {
			t.Fatal("search errored")
		}
		if !strings.Contains(text, "sp16amcpPUB") {
			t.Errorf("search missed the public fixture: %.300s", text)
		}
		for _, leaked := range []string{"sp16amcpPRIV", "sp16amcpPEND", "sp16amcpBAN"} {
			if strings.Contains(text, leaked) {
				t.Errorf("search leaked non-public marker %s: %.300s", leaked, text)
			}
		}
	})

	t.Run("get_content serves public and 404-equivalents non-public", func(t *testing.T) {
		text, ok := callToolText(t, session, "omnicraft_get_content", map[string]any{"content_id": 401})
		if !ok || !strings.Contains(text, "sp16amcpPUB") {
			t.Fatalf("public detail missing: ok=%v %.200s", ok, text)
		}
		if !strings.Contains(text, "sp16amcpPUB_att") && !strings.Contains(text, "oss_url") {
			// attachment presence: oss_key itself is not serialized; the
			// display URL may be empty without OSS config — assert the
			// attachments array exists instead.
			if !strings.Contains(text, `"attachments"`) {
				t.Errorf("detail misses attachments metadata: %.200s", text)
			}
		}
		for _, id := range []int64{402, 403, 404} {
			if _, ok := callToolText(t, session, "omnicraft_get_content", map[string]any{"content_id": id}); ok {
				t.Errorf("non-public content %d served through get_content", id)
			}
		}
	})

	t.Run("usage guide merges author specifics on public content only", func(t *testing.T) {
		text, ok := callToolText(t, session, "omnicraft_get_usage_guide", map[string]any{"content_id": 401, "locale": "zh"})
		if !ok {
			t.Fatal("guide errored on public content")
		}
		if !strings.Contains(text, "sp16amcpPUB step") {
			t.Errorf("guide missed author specifics: %.300s", text)
		}
		if !strings.Contains(text, "safety") {
			t.Errorf("guide missed the template safety floor: %.300s", text)
		}
		for _, id := range []int64{402, 403, 404} {
			if _, ok := callToolText(t, session, "omnicraft_get_usage_guide", map[string]any{"content_id": id}); ok {
				t.Errorf("guide served for non-public content %d", id)
			}
		}
	})

	t.Run("list categories answers", func(t *testing.T) {
		if err := testDB.Create(&model.Category{Zone: "original", Level: "primary", NameI18n: model.JSONMap{"zh": "游戏", "en": "Game"}, Slug: "game", SortOrder: 1, IsActive: true}).Error; err != nil {
			t.Fatalf("seed category: %v", err)
		}
		text, ok := callToolText(t, session, "omnicraft_list_categories", map[string]any{})
		if !ok {
			t.Fatal("list categories errored")
		}
		if !strings.Contains(text, "game") {
			t.Errorf("categories missed fixture slug: %.200s", text)
		}
	})
}

// #813（审计 #5 + 决策①）：get_content 附件投影与 REST 展示策略对齐——
// 展示/预览族（image / video / sheet_music）暴露分钟级短时效 oss_url；
// 文件交付族（mod / text 等）仅元数据（无 oss_url 字段），一律经下载闸
// （omnicraft_request_download）交付；工具描述如实描述该边界。
func TestGetContentAttachmentProjectionDisplayFamiliesOnly(t *testing.T) {
	session, testDB := newTestMCPStack(t)

	// 对已发布的 401 重建附件矩阵：清掉旧 "file" 族附件，按家族播种。
	require.NoError(t, testDB.Where("content_item_id = ?", 401).Delete(&model.ContentAttachment{}).Error)
	seeds := []struct {
		id     int64
		family string
		ossKey string
		scan   string
	}{
		{9501, "image", "uploads/301/image/a.png", "not_required"},
		{9502, "sheet_music", "uploads/301/sheet_music/a.musicxml", "not_required"},
		{9503, "mod", "uploads/301/mod/a.zip", "clean"},
		{9504, "text", "uploads/301/text/a.txt", "not_required"},
	}
	for _, s := range seeds {
		require.NoError(t, testDB.Create(&model.ContentAttachment{
			ID: s.id, ContentItemID: 401, FileType: s.family, OSSKey: s.ossKey, ScanStatus: s.scan,
		}).Error)
	}

	text, ok := callToolText(t, session, "omnicraft_get_content", map[string]any{"content_id": 401})
	if !ok {
		t.Fatal("get_content errored on public content")
	}
	var payload struct {
		Attachments []map[string]any `json:"attachments"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("get_content payload is not JSON: %v; %.200s", err, text)
	}
	got := map[string]map[string]any{}
	for _, att := range payload.Attachments {
		got[att["file_type"].(string)] = att
	}
	for _, family := range []string{"image", "sheet_music"} {
		ossURL, _ := got[family]["oss_url"].(string)
		if ossURL == "" {
			t.Errorf("%s attachment must expose a signed oss_url; got %+v", family, got[family])
		}
	}
	for _, family := range []string{"mod", "text"} {
		if _, has := got[family]["oss_url"]; has {
			t.Errorf("%s attachment must be metadata-only (no oss_url key); got %+v", family, got[family])
		}
	}

	// 工具描述如实描述家族边界与下载闸独占交付。
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tl := range tools.Tools {
		if tl.Name == "omnicraft_get_content" {
			if !strings.Contains(tl.Description, "metadata only") || !strings.Contains(tl.Description, "download") {
				t.Errorf("get_content description must state the family boundary honestly: %s", tl.Description)
			}
		}
	}
}
