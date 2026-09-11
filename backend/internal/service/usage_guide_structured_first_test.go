package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

// SP-16 #447: the in-site usage guide reads persisted specifics first; a
// failing provider proves the LLM is never called on the structured path,
// and is still reached when no specifics exist (draft/legacy behaviour).
type usageGuideFailingProvider struct{}

func (usageGuideFailingProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, errors.New("provider must not be called")
}

func (usageGuideFailingProvider) ChatStream(context.Context, llm.ChatRequest, func(llm.ChatDelta) error) error {
	return errors.New("provider must not be called")
}

func (usageGuideFailingProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, errors.New("provider must not be called")
}

func setupStructuredUsageGuideStack(t *testing.T) (*AgentService, *UsageGuideService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentUsageGuide{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.WebAgentEnabled = true

	contentRepo := repository.NewContentRepository(db)
	guideSvc := NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo)
	svc := NewAgentService(usageGuideFailingProvider{}, nil, contentRepo, nil, db, cfg)
	svc.SetUsageGuideService(guideSvc)
	return svc, guideSvc, db
}

func TestUsageGuideStructuredFirst(t *testing.T) {
	ctx := context.Background()
	svc, guideSvc, db := setupStructuredUsageGuideStack(t)

	seed := model.ContentItem{ID: 801, Title: "structured guide target", AuthorID: 901, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("no specifics still reaches the LLM", func(t *testing.T) {
		if _, err := svc.UsageGuide(ctx, 0, 801, false); err == nil {
			t.Fatal("failing provider must surface its error when no specifics exist")
		}
	})

	t.Run("persisted specifics render without any LLM call", func(t *testing.T) {
		if err := guideSvc.SaveSpecifics(ctx, 901, 801, UsageGuideInput{
			Locale:       "zh",
			Requirements: []string{"Minecraft 1.20.1+"},
			Steps:        []string{"备份存档", "放入 mods 目录"},
			Notes:        "冲突先移除旧版。",
			Source:       "author",
		}); err != nil {
			t.Fatalf("seed specifics: %v", err)
		}

		if !svc.HasStructuredGuide(ctx, 801) {
			t.Fatal("HasStructuredGuide must be true after specifics are saved")
		}

		result, err := svc.UsageGuide(ctx, 0, 801, false)
		if err != nil {
			t.Fatalf("structured read hit the LLM: %v", err)
		}
		if !result.Structured {
			t.Fatal("result must be flagged structured")
		}
		if !strings.Contains(result.Guide, "备份存档") || !strings.Contains(result.Guide, "安全提示") {
			t.Fatalf("rendered guide misses specifics or safety floor: %q", result.Guide)
		}

		var deltas []string
		err = svc.UsageGuideStream(ctx, 0, 801, false, func(delta string, done bool) error {
			if delta != "" {
				deltas = append(deltas, delta)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("structured stream hit the LLM: %v", err)
		}
		if len(deltas) != 1 || !strings.Contains(deltas[0], "备份存档") {
			t.Fatalf("stream deltas = %#v, want one merged delta", deltas)
		}
	})

	t.Run("draft=true forces the LLM path", func(t *testing.T) {
		if _, err := svc.UsageGuide(ctx, 0, 801, true); err == nil {
			t.Fatal("draft=true must bypass specifics and reach the (failing) provider")
		}
	})
}
