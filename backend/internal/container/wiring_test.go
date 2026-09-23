package container

// #658 ValidateWiring 启动期断言单测：缺 REQUIRED 依赖时报错并列出完整
// 清单（不只报第一个）。先例：配置校验测试族的「全清单」语义。

import (
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
)

// 零值容器 = 全部 REQUIRED 依赖缺失：错误信息必须一次点名全部，
// 覆盖三类代表（输入 / 仓库 / 服务 + 观测 / 共享设施），
// 且数量级与清单规模一致（不是首错短路）。
func TestValidateWiringListsAllMissingRequiredDependencies(t *testing.T) {
	c := &ServiceContainer{}
	err := c.ValidateWiring()
	if err == nil {
		t.Fatal("zero-value container must fail ValidateWiring")
	}
	msg := err.Error()
	for _, want := range []string{
		// inputs
		"db", "redis", "config", "queue.producer",
		// repositories
		"repo.user", "repo.outbox", "repo.embedding", "repo.archive_scan",
		// services
		"service.review", "service.content", "service.studio_content",
		"service.agent", "service.notification", "service.prompt_registry",
		// shared facilities
		"upload.grants", "display.signer", "mcp.handler",
		"hybrid.retriever", "rag.projection", "agenttrace.writer",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing-dependency list lacks %q: %s", want, msg)
		}
	}
	// 完整清单而非首错：comma-separated 项数 ≥ 60（当前 REQUIRED 全集）。
	if got := len(strings.Split(msg, ", ")); got < 60 {
		t.Errorf("missing list has %d entries, want the complete list (>= 60): %s", got, msg)
	}
}

// OPTIONAL 依赖（队列 broker、OSS、归档扫描设施）缺席不进清单：
// 容器在最小合法输入上必须构造成功并通过断言。
func TestNewContainerPassesWiringAssertionOnMinimalInputs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctr, err := NewContainer(db, rdb, &config.Config{})
	if err != nil {
		t.Fatalf("minimal-input container must pass ValidateWiring: %v", err)
	}
	// OPTIONAL 语义不因断言存在而收紧：队列关闭 → broker 可为 nil。
	if ctr.QueueBroker != nil {
		t.Error("queue disabled must keep QueueBroker nil (OPTIONAL)")
	}
	if err := ctr.ValidateWiring(); err != nil {
		t.Fatalf("second ValidateWiring pass must stay clean: %v", err)
	}
}
