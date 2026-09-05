package router

// #379（F-A001）防再犯集成门：judge 路由必须消费容器级 JudgeService
// （SetContentOutcomeWriter/SetNotificationService 已装配的那一个）。
// 回归形态：handler 自建裸 JudgeService（rdb/db/outbox/notifSvc 全 nil）——
// 考试会话永不写入 → 全对提交恒 409 EXAM_SESSION_EXPIRED；闭案回写与
// 作者通知静默跳过。两个用例都走真 HTTP 栈（gin engine + RegisterRoutes +
// miniredis + 临时 Postgres），在服务装配层断言行为而非源码形态。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/testutil"
)

const judgeWiringContentType = "article"

func setupJudgeWiringStack(t *testing.T) (*gin.Engine, *gorm.DB, *miniredis.Miniredis, *redis.Client, *config.Config) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.JudgeQuestion{}, &model.JudgeQualification{},
		&model.JudgeExamRecord{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.ReputationLog{}, &model.Notification{}, &model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{
		JWT:        config.JWTConfig{Secret: "judge-wiring-test-secret"},
		Judge:      config.JudgeConfig{MinVotesRequired: 1, PassThreshold: 0.6},
		Reputation: config.ReputationConfig{MinScoreForInteraction: 1},
		// Queue 保持关闭：Notify 走直写 DB 路径，集成测试可直接断言通知落库。
	}
	ctr := container.NewContainer(db, rdb, cfg)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterRoutes(engine.Group("/api/v1"), cfg, ctr)
	return engine, db, mr, rdb, cfg
}

func seedJudgeWiringUser(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	verifiedAt := time.Now()
	require.NoError(t, db.Create(&model.User{
		ID:              id,
		Email:           fmt.Sprintf("judge-wiring-%d@example.test", id),
		Username:        fmt.Sprintf("judge_wiring_%d", id),
		PasswordHash:    "hash",
		Role:            "user",
		Reputation:      10,
		EmailVerifiedAt: &verifiedAt,
	}).Error)
}

func seedJudgeWiringQuestions(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		data, err := json.Marshal(map[string]interface{}{
			"prompt":      fmt.Sprintf("根据多数投票意见，以下内容是否违规？(%d)", i),
			"options":     map[string]string{"A": "违规（应下架）", "B": "不违规（应保留）", "C": "信息不足", "D": "移交人工"},
			"correct_key": "A",
		})
		require.NoError(t, err)
		require.NoError(t, db.Create(&model.JudgeQuestion{
			ContentType:  judgeWiringContentType,
			QuestionData: data,
			IsActive:     true,
		}).Error)
	}
}

func judgeWiringAuthHeader(t *testing.T, cfg *config.Config, userID int64) string {
	t.Helper()
	token, err := jwtutil.GenerateAccessToken(userID, "user", cfg.JWT.Secret, 30)
	require.NoError(t, err)
	return "Bearer " + token
}

func judgeWiringSessionAnswers(t *testing.T, rdb *redis.Client, userID int64) []map[string]interface{} {
	t.Helper()
	raw, err := rdb.Get(context.Background(), fmt.Sprintf("judge:exam:%d:%s", userID, judgeWiringContentType)).Result()
	require.NoError(t, err, "GetExam 必须通过装配好的服务写入抽题会话")
	var session map[string]string
	require.NoError(t, json.Unmarshal([]byte(raw), &session))
	answers := make([]map[string]interface{}, 0, len(session))
	for qid, key := range session {
		answers = append(answers, map[string]interface{}{"question_id": json.Number(qid), "answer": key})
	}
	return answers
}

// 全对提交 → 200 passed=true + judge_qualifications 授予（资格 API 契约为
// 200+record，票面「201」指资格授予这一事实）。
func TestJudgeWiringExamFullPassGrantsQualification(t *testing.T) {
	engine, db, _, rdb, cfg := setupJudgeWiringStack(t)
	const userID int64 = 3910
	seedJudgeWiringUser(t, db, userID)
	seedJudgeWiringQuestions(t, db, 10)

	// 1) 抽题：200 + 会话落 Redis（回归形态下这里是 0 键）。
	req := httptest.NewRequest(http.MethodGet, "/api/v1/judge/exam/"+judgeWiringContentType, nil)
	req.Header.Set("Authorization", judgeWiringAuthHeader(t, cfg, userID))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "GET exam: %s", rec.Body.String())

	var exam struct {
		Questions []map[string]interface{} `json:"questions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exam))
	require.Len(t, exam.Questions, 10)
	for _, q := range exam.Questions {
		question := q["question"].(map[string]interface{})
		require.Contains(t, question, "prompt")
		require.Contains(t, question, "options")
		raw, err := json.Marshal(q)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "correct_key", "allowlist 不得下发答案")
		require.NotContains(t, string(raw), "explanation", "allowlist 不得下发解析")
	}

	// 2) 全对提交：回归形态下这里恒 409 EXAM_SESSION_EXPIRED。
	answers := judgeWiringSessionAnswers(t, rdb, userID)
	body, err := json.Marshal(map[string]interface{}{
		"content_type": judgeWiringContentType,
		"answers":      answers,
	})
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/judge/exam/submit", strings.NewReader(string(body)))
	req.Header.Set("Authorization", judgeWiringAuthHeader(t, cfg, userID))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "全对提交应通过: %s", rec.Body.String())

	var submit struct {
		Passed bool `json:"passed"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &submit))
	require.True(t, submit.Passed)

	var qualCount int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).
		Where("user_id = ? AND content_type = ?", userID, judgeWiringContentType).
		Count(&qualCount).Error)
	require.Equal(t, int64(1), qualCount, "通过后应授予判官资格")
}

// min_votes=1 单票 approve → 内容回写 published + 作者通知落库 + 索引事件。
func TestJudgeWiringClosedCaseWritesBackAndNotifies(t *testing.T) {
	engine, db, mr, _, cfg := setupJudgeWiringStack(t)
	const authorID int64 = 3920
	const judgeID int64 = 3921
	seedJudgeWiringUser(t, db, authorID)
	seedJudgeWiringUser(t, db, judgeID)

	content := model.ContentItem{
		Title:       "judge wiring closed case content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: judgeWiringContentType,
		Status:      "under_review",
		IsPublic:    true,
	}
	require.NoError(t, db.Create(&content).Error)
	judgeCase := model.JudgeCase{
		TargetType: judgeWiringContentType,
		TargetID:   content.ID,
		Status:     "open",
		MinVotes:   1,
	}
	require.NoError(t, db.Create(&judgeCase).Error)

	// 回归形态下 nil notifSvc/rdb 静默跳过——先埋缓存键验证失效路径。
	cacheKey := fmt.Sprintf("cache:content:%d", content.ID)
	require.NoError(t, redisclient.SetJSON(context.Background(), cacheKey, nil, time.Minute))
	require.True(t, mr.Exists(cacheKey))

	body, err := json.Marshal(map[string]interface{}{"case_id": judgeCase.ID, "vote": "approve"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/judge/vote", strings.NewReader(string(body)))
	req.Header.Set("Authorization", judgeWiringAuthHeader(t, cfg, judgeID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "投票应成功: %s", rec.Body.String())

	var updated model.ContentItem
	require.NoError(t, db.First(&updated, content.ID).Error)
	require.Equal(t, "published", updated.Status, "closed_approve 应回写内容状态")

	var judgeCaseAfter model.JudgeCase
	require.NoError(t, db.First(&judgeCaseAfter, judgeCase.ID).Error)
	require.Equal(t, "closed_approve", judgeCaseAfter.Status)

	var publishedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).
		Where("event_type = ?", "content.published").Count(&publishedEvents).Error)
	require.Equal(t, int64(1), publishedEvents, "恢复展示应重发索引事件")

	require.Eventually(t, func() bool {
		var notifyCount int64
		require.NoError(t, db.Model(&model.Notification{}).
			Where("user_id = ? AND target_id = ?", authorID, content.ID).
			Count(&notifyCount).Error)
		return notifyCount >= 1
	}, 3*time.Second, 50*time.Millisecond, "闭案应通知作者")

	require.False(t, mr.Exists(cacheKey), "闭案回写应失效内容缓存")
}

// 源码契约：routes.go 的 judge handler 必须消费容器装配的 JudgeService，
// 不得退回 handler 内自建实例（F-A001 回归形态）。
func TestJudgeRoutesConsumeContainerWiredService(t *testing.T) {
	source := readRoutesSource(t)
	contract := `judgeHandler := handler.NewJudgeHandler(db, ctr.JudgeService, ctr.AdminAuditService)`
	if !strings.Contains(source, contract) {
		t.Fatalf("router source missing container-wired judge service contract %q", contract)
	}
}
