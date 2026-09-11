package mcpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// SP-16 #451: PAT-scoped write tools. The tool surface degrades honestly with
// the token: anonymous sessions see exactly the four read-only tools, a
// download-scope token additionally sees omnicraft_request_download, and an
// upload-scope token sees the three publish tools. Every write tool re-checks
// the scope at execution time so a leaked session id cannot outrank the
// current request's token.

type mcpFakeSigner struct{}

func (mcpFakeSigner) GeneratePresignDownloadURL(ctx context.Context, ossKey string, ttl time.Duration) (string, error) {
	return "https://signed.example.com/" + ossKey, nil
}

type fakeSuggest struct{ called int }

func (f *fakeSuggest) suggest(ctx context.Context, title, description, filename, contentType string) (*service.UploadAssistResult, error) {
	f.called++
	return &service.UploadAssistResult{
		SuggestedTitle:       "assisted " + title,
		SuggestedDescription: "assisted description",
		SuggestedCategory:    "game",
		SuggestedTags:        []string{"tag-a", "tag-b"},
	}, nil
}

type fakeIssuer struct{ calls []service.PresignUploadRequest }

func (f *fakeIssuer) issue(ctx context.Context, req service.PresignUploadRequest, userID int64) (*service.PresignUploadResponse, error) {
	f.calls = append(f.calls, req)
	return &service.PresignUploadResponse{
		UploadURL: "https://oss.example.com/uploads/fake?sig=1",
		OSSKey:    "uploads/sp16b/fake.pdf",
		ExpiresIn: 300,
		GrantID:   "grant-sp16b-1",
	}, nil
}

func newWriteToolStack(t *testing.T, scopes []string) (*sdkmcp.ClientSession, *gorm.DB, *fakeSuggest, *fakeIssuer) {
	return newWriteToolStackScoped(t, scopes, 601)
}

func newWriteToolStackScoped(t *testing.T, scopes []string, userID int64) (*sdkmcp.ClientSession, *gorm.DB, *fakeSuggest, *fakeIssuer) {
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
	if err := db.Create(&model.User{ID: 601, Email: "sp16b-owner@example.com", Username: "sp16b_owner", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	// 602: unverified owner to prove the interaction gate is enforced.
	if err := db.Create(&model.User{ID: 602, Email: "sp16b-unverified@example.com", Username: "sp16b_unverified", PasswordHash: "x", Reputation: 10, Role: "user"}).Error; err != nil {
		t.Fatalf("seed unverified: %v", err)
	}
	pub := model.ContentItem{
		ID: 701, Title: "sp16b PUB title", AuthorID: 601, Zone: "original", Category: "game",
		ContentType: "sheet_music", Status: "published", IsPublic: true, AllowCopy: true,
	}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatalf("seed pub: %v", err)
	}
	primary := true
	size := int64(2048)
	if err := db.Create(&model.ContentAttachment{
		ContentItemID: 701, FileType: "file", MimeType: "application/pdf",
		OSSKey: "uploads/sp16b/pub.pdf", FileSize: &size, ScanStatus: "not_required", IsPrimary: &primary,
	}).Error; err != nil {
		t.Fatalf("seed attachment: %v", err)
	}

	contentRepo := repository.NewContentRepository(db)
	cfg := &config.Config{}
	cfg.Reputation.MinScoreForInteraction = 1
	contentSvc := service.NewContentServiceWithDeps(contentRepo, nil, nil).WithDownloadSigner(mcpFakeSigner{})
	guideSvc := service.NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo)

	suggest := &fakeSuggest{}
	issuer := &fakeIssuer{}
	deps := Deps{
		DB:          db,
		SearchRepo:  repository.NewSearchRepository(db),
		ContentRepo: contentRepo,
		CategoryRepo: repository.NewCategoryRepository(db),
		GuideSvc:    guideSvc,
		Cfg:         cfg,
		ContentSvc:  contentSvc,
		SuggestPublishMetadata: suggest.suggest,
		IssueUploadURL:         issuer.issue,
		ConsumeUploadQuota:     func(ctx context.Context, userID int64) error { return nil },
	}

	handler := NewHandler(deps)
	if scopes != nil {
		id := Identity{UserID: userID, Scopes: scopes}
		handler = withIdentity(handler, &id)
	}
	httpSrv := httptest.NewServer(handler)
	t.Cleanup(httpSrv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "sp16b-write-test", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, db, suggest, issuer
}

func toolNames(t *testing.T, session *sdkmcp.ClientSession) map[string]bool {
	t.Helper()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	return names
}

func TestScopedToolLists(t *testing.T) {
	readonly := []string{"omnicraft_search", "omnicraft_get_content", "omnicraft_get_usage_guide", "omnicraft_list_categories"}

	anon, _, _, _ := newWriteToolStack(t, nil)
	anonNames := toolNames(t, anon)
	if len(anonNames) != len(readonly) {
		t.Errorf("anonymous session exposes %d tools, want exactly the %d read-only tools", len(anonNames), len(readonly))
	}

	dl, _, _, _ := newWriteToolStack(t, []string{"download"})
	dlNames := toolNames(t, dl)
	if len(dlNames) != 5 || !dlNames["omnicraft_request_download"] {
		t.Errorf("download-scope session tool set wrong: %v", dlNames)
	}
	for _, w := range []string{"omnicraft_create_content", "omnicraft_request_upload_url", "omnicraft_suggest_publish_metadata"} {
		if dlNames[w] {
			t.Errorf("download-scope session must NOT expose %s", w)
		}
	}

	ul, _, _, _ := newWriteToolStack(t, []string{"upload"})
	ulNames := toolNames(t, ul)
	if len(ulNames) != 7 {
		t.Errorf("upload-scope session exposes %d tools, want 7: %v", len(ulNames), ulNames)
	}
	if ulNames["omnicraft_request_download"] {
		t.Errorf("upload-scope session must NOT expose the download tool")
	}

	both, _, _, _ := newWriteToolStack(t, []string{"download", "upload"})
	bothNames := toolNames(t, both)
	if len(bothNames) != 8 {
		t.Errorf("full-scope session exposes %d tools, want 8: %v", len(bothNames), bothNames)
	}
}

func TestRequestDownloadTool(t *testing.T) {
	session, _, _, _ := newWriteToolStack(t, []string{"download"})

	text, ok := callToolText(t, session, "omnicraft_request_download", map[string]any{"content_id": 701})
	if !ok {
		t.Fatal("request_download errored on a public fixture")
	}
	for _, want := range []string{`"url"`, `"filename"`, `"size"`, `"content_type"`, `"expires_in"`} {
		if !strings.Contains(text, want) {
			t.Errorf("download result missing %s: %.300s", want, text)
		}
	}
	if !strings.Contains(text, "uploads/sp16b/pub.pdf") && !strings.Contains(text, "pub.pdf") {
		t.Errorf("signed url not derived from the attachment key: %.300s", text)
	}

	if _, ok := callToolText(t, session, "omnicraft_request_download", map[string]any{"content_id": 99999}); ok {
		t.Error("unknown content must error, not return a url")
	}
}

// callToolRaw invokes a tool and returns its text whether or not the tool
// reported an error result (for asserting failure payloads).
func callToolRaw(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: json.RawMessage(raw)})
	if err != nil {
		return err.Error(), false
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text, !res.IsError
}

func TestWriteToolsRequireScopeAtExecutionTime(t *testing.T) {
	// A session created with the download scope must refuse write tools even
	// if somehow invoked (defense in depth against session-id leakage).
	session, _, _, _ := newWriteToolStack(t, []string{"download"})
	if text, ok := callToolRaw(t, session, "omnicraft_suggest_publish_metadata", map[string]any{
		"file_name": "x.pdf", "content_type": "sheet_music", "title": "t",
	}); ok {
		t.Errorf("suggest tool succeeded on a download-scope session: %s", text)
	}
}

func TestSuggestPublishMetadataTool(t *testing.T) {
	session, _, suggest, _ := newWriteToolStack(t, []string{"upload"})

	text, ok := callToolText(t, session, "omnicraft_suggest_publish_metadata", map[string]any{
		"file_name": "song.pdf", "content_type": "sheet_music", "title": "My Song", "description": "A tune",
	})
	if !ok {
		t.Fatal("suggest tool errored")
	}
	if suggest.called != 1 {
		t.Errorf("suggest backend called %d times, want 1", suggest.called)
	}
	for _, want := range []string{"assisted My Song", "tag-a", "game"} {
		if !strings.Contains(text, want) {
			t.Errorf("suggest result missing %q: %.300s", want, text)
		}
	}
}

func TestCreateContentToolPublishesPendingDraft(t *testing.T) {
	session, db, _, _ := newWriteToolStack(t, []string{"upload"})

	text, ok := callToolRaw(t, session, "omnicraft_create_content", map[string]any{
		"title": "agent draft", "description": "from mcp", "zone": "original",
		"content_type": "article", "category": "game", "is_public": true, "allow_copy": true,
		"tags": []string{"mcp"},
		"usage_guide": map[string]any{
			"locale": "zh", "requirements": []string{"浏览器"}, "steps": []string{"打开阅读"}, "notes": "无",
		},
	})
	if !ok {
		t.Fatalf("create_content errored: %.500s", text)
	}
	var res struct {
		ContentID int64  `json:"content_id"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		t.Fatalf("decode result %q: %v", text, err)
	}
	if res.ContentID == 0 || res.Status != "pending" {
		t.Fatalf("result = %+v, want pending draft with id", res)
	}

	var item model.ContentItem
	if err := db.First(&item, res.ContentID).Error; err != nil {
		t.Fatalf("draft not persisted: %v", err)
	}
	if item.Status != "pending" || item.AuthorID != 601 {
		t.Errorf("draft row = status %s author %d, want pending/601 (external uploads enter review, no exemption)", item.Status, item.AuthorID)
	}
	var guide model.ContentUsageGuide
	if err := db.Where("content_id = ?", res.ContentID).First(&guide).Error; err != nil {
		t.Fatalf("usage guide specifics not persisted: %v", err)
	}
	if guide.Locale != "zh" || !strings.Contains(guide.Requirements, "浏览器") {
		t.Errorf("guide row = %+v", guide)
	}
}

func TestCreateContentToolEnforcesInteractionGate(t *testing.T) {
	// Pin the identity to the unverified user 602: the publish interaction
	// gate must reject exactly like the REST publish route.
	session, _, _, _ := newWriteToolStackScoped(t, []string{"upload"}, 602)
	if text, ok := callToolRaw(t, session, "omnicraft_create_content", map[string]any{
		"title": "unverified", "zone": "original", "content_type": "article",
	}); ok {
		t.Fatalf("unverified owner must be rejected by the interaction gate, got: %s", text)
	}
}

func TestRequestUploadURLTool(t *testing.T) {
	session, _, _, issuer := newWriteToolStack(t, []string{"upload"})

	text, ok := callToolText(t, session, "omnicraft_request_upload_url", map[string]any{
		"file_name": "song.pdf", "file_type": "file", "mime_type": "application/pdf", "file_size": 2048,
	})
	if !ok {
		t.Fatal("request_upload_url errored")
	}
	for _, want := range []string{"upload_url", "oss_key", "grant_id", "expires_in"} {
		if !strings.Contains(text, want) {
			t.Errorf("upload url result missing %s: %.300s", want, text)
		}
	}
	if len(issuer.calls) != 1 || issuer.calls[0].FileSize != 2048 {
		t.Errorf("issuer calls = %+v", issuer.calls)
	}
}
