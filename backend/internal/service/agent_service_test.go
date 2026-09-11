package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

type fakeAgentLLMProvider struct {
	chatContent string
	embedding   []float32
}

func (p *fakeAgentLLMProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: p.chatContent}, nil
}

func (p *fakeAgentLLMProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	return handler(llm.ChatDelta{Done: true})
}

func (p *fakeAgentLLMProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	if p.embedding != nil {
		return p.embedding, nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestUploadAssistSanitizesLLMOutput(t *testing.T) {
	rawTags := make([]string, 0, 14)
	for i := 0; i < 14; i++ {
		rawTags = append(rawTags, "tag-with-very-long-name-over-thirty-two-chars")
	}
	payload := map[string]any{
		"suggested_tags":        rawTags,
		"suggested_category":    "invalid_category",
		"suggested_title":       repeatString("T", 650),
		"suggested_description": repeatString("D", 2400),
		"unknown_field":         "ignored",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	svc := NewAgentService(
		&fakeAgentLLMProvider{chatContent: string(data)},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	result, err := svc.UploadAssist(context.Background(), 1, "original title", "original description", "file.png", "image")
	if err != nil {
		t.Fatalf("UploadAssist: %v", err)
	}

	if len(result.SuggestedTags) > 10 {
		t.Fatalf("suggested tags length = %d, want <= 10", len(result.SuggestedTags))
	}
	for _, tag := range result.SuggestedTags {
		if len(tag) > 32 {
			t.Fatalf("tag %q length = %d, want <= 32", tag, len(tag))
		}
	}
	if result.SuggestedCategory != "" {
		t.Fatalf("invalid category = %q, want blank", result.SuggestedCategory)
	}
	if len(result.SuggestedTitle) > 500 {
		t.Fatalf("title length = %d, want <= 500", len(result.SuggestedTitle))
	}
	if len(result.SuggestedDescription) > 2000 {
		t.Fatalf("description length = %d, want <= 2000", len(result.SuggestedDescription))
	}
}

func TestAgentVectorRetrievalUsesSharedContentVisibility(t *testing.T) {
	db := setupAgentVisibilityDB(t)
	viewer := createAgentUser(t, db, "viewer@example.com", "viewer", false)
	other := createAgentUser(t, db, "other@example.com", "other", false)
	bannedAuthor := createAgentUser(t, db, "banned@example.com", "banned", true)
	deletedAuthor := createAgentUser(t, db, "deleted@example.com", "deleted", false)
	if err := db.Exec("UPDATE users SET deleted_at = ? WHERE id = ?", time.Now(), deletedAuthor.ID).Error; err != nil {
		t.Fatalf("mark deleted author: %v", err)
	}
	bannedIP := model.IP{Name: "Banned IP", Slug: "banned-ip", Status: "banned"}
	if err := db.Create(&bannedIP).Error; err != nil {
		t.Fatalf("create banned ip: %v", err)
	}

	visible := createAgentContent(t, db, "visible", other.ID, nil, true)
	privateViewer := createAgentContent(t, db, "private viewer", viewer.ID, nil, false)
	privateOther := createAgentContent(t, db, "private other", other.ID, nil, false)
	bannedAuthorContent := createAgentContent(t, db, "banned author", bannedAuthor.ID, nil, true)
	deletedAuthorContent := createAgentContent(t, db, "deleted author", deletedAuthor.ID, nil, true)
	bannedIPContent := createAgentContent(t, db, "banned ip", other.ID, &bannedIP.ID, true)

	svc := NewAgentService(
		&fakeAgentLLMProvider{},
		nil,
		repository.NewContentRepository(db),
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)
	svc.vectorSearch = func(_ []float32, _ int) ([]repository.EmbeddingSearchResult, error) {
		return []repository.EmbeddingSearchResult{
			{ContentItemID: visible.ID, Score: 0.9},
			{ContentItemID: privateViewer.ID, Score: 0.8},
			{ContentItemID: privateOther.ID, Score: 0.7},
			{ContentItemID: bannedAuthorContent.ID, Score: 0.6},
			{ContentItemID: deletedAuthorContent.ID, Score: 0.5},
			{ContentItemID: bannedIPContent.ID, Score: 0.4},
		}, nil
	}

	// #309：NLSearch HTTP 面已下线；可见性收敛在共享 helper（chat 工具
	// 链向量检索同款路径），行为测试改打 helper 直测。
	contents, err := svc.listVisibleNLSearchContents([]int64{
		visible.ID, privateViewer.ID, privateOther.ID,
		bannedAuthorContent.ID, deletedAuthorContent.ID, bannedIPContent.ID,
	}, viewer.ID)
	if err != nil {
		t.Fatalf("listVisibleNLSearchContents: %v", err)
	}

	got := map[int64]bool{}
	for _, res := range contents {
		got[res.ID] = true
	}
	if !got[visible.ID] || !got[privateViewer.ID] {
		t.Fatalf("expected visible and viewer-private content, got %#v", got)
	}
	for _, hiddenID := range []int64{privateOther.ID, bannedAuthorContent.ID, deletedAuthorContent.ID, bannedIPContent.ID} {
		if got[hiddenID] {
			t.Fatalf("hidden content id %d was returned: %#v", hiddenID, got)
		}
	}
}

func setupAgentVisibilityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// :memory: databases are per-connection; pinning one connection keeps the
	// schema visible to the async auto-title goroutine and follow-up turns.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, check := range []struct {
		table  any
		column string
	}{
		{table: &model.User{}, column: "deleted_at"},
		{table: &model.ContentItem{}, column: "deleted_at"},
	} {
		if !db.Migrator().HasColumn(check.table, check.column) {
			t.Fatalf("expected AutoMigrate to own %s", check.column)
		}
	}
	return db
}

func createAgentUser(t *testing.T, db *gorm.DB, email, username string, banned bool) model.User {
	t.Helper()
	user := model.User{
		Email:        email,
		Username:     username,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     banned,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func createAgentContent(t *testing.T, db *gorm.DB, title string, authorID int64, ipID *int64, public bool) model.ContentItem {
	t.Helper()
	item := model.ContentItem{
		Title:       title,
		AuthorID:    authorID,
		IPID:        ipID,
		Zone:        "original",
		Category:    "gaming",
		ContentType: "article",
		Status:      "published",
		IsPublic:    public,
		AllowCopy:   true,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	if !public {
		if err := db.Exec("UPDATE content_items SET is_public = ? WHERE id = ?", false, item.ID).Error; err != nil {
			t.Fatalf("mark private content %s: %v", title, err)
		}
	}
	return item
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
