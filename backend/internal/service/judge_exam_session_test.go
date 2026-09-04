package service

// T37（FIX-36a）：考试防作弊。
// ①抽题会话绑定：GetExam 把题 ID 集→correct_key 写 Redis `judge:exam:{uid}:{category}`
//   （TTL 15min），SubmitExam 只按会话题集评分，不得重新抽题；
// ②total=服务端下发题数（会话题集长度），单题提交不再可过；
// ③会话缺失/过期返回 ErrExamSessionExpired，不得按新题集评分；
// ④已持有有效资格再考返回 ErrAlreadyQualified（连考限制）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

const t37ContentType = "article"

func setupT37ExamTest(t *testing.T) (*JudgeService, *gorm.DB, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.JudgeQuestion{}, &model.JudgeQualification{},
		&model.JudgeExamRecord{}, &model.ReputationLog{}, &model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 2, PassThreshold: 0.6}}
	svc := NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg)
	svc.SetContentOutcomeWriter(db, rdb, repository.NewOutboxRepository(db))
	return svc, db, mr, rdb
}

func seedT37User(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		ID:           id,
		Email:        fmt.Sprintf("t37-%d@example.test", id),
		Username:     fmt.Sprintf("t37_judge_%d", id),
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
	}).Error)
}

func seedT37Questions(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		data, err := json.Marshal(map[string]interface{}{
			"question":      "根据多数投票意见，以下内容是否违规？",
			"options":       map[string]string{"A": "违规（应下架）", "B": "不违规（应保留）"},
			"correct_key":   "A",
			"votes_approve": 3,
			"votes_reject":  1,
			"explanation":   "解析：多数派认定违规，正确答案 A",
		})
		require.NoError(t, err)
		require.NoError(t, db.Create(&model.JudgeQuestion{
			ContentType:  t37ContentType,
			QuestionData: data,
			IsActive:     true,
		}).Error)
	}
}

func t37SeedQualification(t *testing.T, db *gorm.DB, userID int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.JudgeQualification{
		UserID:      userID,
		ContentType: t37ContentType,
		IsActive:    true,
	}).Error)
}

func t37SessionKey(userID int64) string {
	return fmt.Sprintf("judge:exam:%d:%s", userID, t37ContentType)
}

// t37SessionAnswers 从 Redis 会话取出题集并构造全对答案（模拟会话内答题）。
func t37SessionAnswers(t *testing.T, rdb *redis.Client, userID int64) []ExamAnswer {
	t.Helper()
	raw, err := rdb.Get(context.Background(), t37SessionKey(userID)).Result()
	require.NoError(t, err)
	var m map[int64]string
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	answers := make([]ExamAnswer, 0, len(m))
	for qid, key := range m {
		answers = append(answers, ExamAnswer{QuestionID: qid, Answer: key})
	}
	return answers
}

func TestT37GetExamBindsRedisSession(t *testing.T) {
	svc, db, mr, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3701)
	seedT37Questions(t, db, examQuestionCount)

	questions, err := svc.GetExam(t37ContentType, 3701)
	require.NoError(t, err)
	require.Len(t, questions, examQuestionCount)

	raw, err := rdb.Get(context.Background(), t37SessionKey(3701)).Result()
	require.NoError(t, err, "GetExam 必须写抽题会话")
	var session map[int64]string
	require.NoError(t, json.Unmarshal([]byte(raw), &session))
	require.Len(t, session, examQuestionCount)

	ttl := mr.TTL(t37SessionKey(3701))
	require.Greater(t, ttl, time.Duration(0), "会话必须有 TTL")
	require.LessOrEqual(t, ttl, 15*time.Minute)
}

func TestT37GetExamRejectsAlreadyQualified(t *testing.T) {
	svc, db, _, _ := setupT37ExamTest(t)
	seedT37User(t, db, 3702)
	seedT37Questions(t, db, examQuestionCount)
	t37SeedQualification(t, db, 3702)

	_, err := svc.GetExam(t37ContentType, 3702)
	require.ErrorIs(t, err, ErrAlreadyQualified)
}

func TestT37SubmitExamSingleCorrectAnswerFails(t *testing.T) {
	svc, db, _, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3703)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3703)
	require.NoError(t, err)

	// 只提交 1 条正确答案：total=服务端下发题数（10），单题不再可过。
	all := t37SessionAnswers(t, rdb, 3703)
	record, passed, err := svc.SubmitExam(SubmitExamInput{
		ContentType: t37ContentType,
		Answers:     all[:1],
	}, 3703)
	require.NoError(t, err)
	require.False(t, passed)
	require.Equal(t, 1, record.Score)
	require.Equal(t, examQuestionCount, record.Total, "total 必须是服务端下发题数而非 len(answers)")
}

func TestT37SubmitExamAllCorrectPassesAndQualifies(t *testing.T) {
	svc, db, _, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3704)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3704)
	require.NoError(t, err)

	record, passed, err := svc.SubmitExam(SubmitExamInput{
		ContentType: t37ContentType,
		Answers:     t37SessionAnswers(t, rdb, 3704),
	}, 3704)
	require.NoError(t, err)
	require.True(t, passed)
	require.Equal(t, examQuestionCount, record.Score)
	require.Equal(t, examQuestionCount, record.Total)

	quals, err := svc.judgeRepo.GetUserQualifications(3704)
	require.NoError(t, err)
	require.Len(t, quals, 1, "通过后应创建判官资格")
}

func TestT37SubmitExamWithoutSessionDoesNotResample(t *testing.T) {
	svc, db, _, _ := setupT37ExamTest(t)
	seedT37User(t, db, 3705)
	seedT37Questions(t, db, examQuestionCount)

	// 未抽题直接提交（题库有题也不得重新抽题评分）。
	_, _, err := svc.SubmitExam(SubmitExamInput{
		ContentType: t37ContentType,
		Answers:     []ExamAnswer{{QuestionID: 1, Answer: "A"}},
	}, 3705)
	require.ErrorIs(t, err, ErrExamSessionExpired)

	var recordCount, qualCount int64
	require.NoError(t, db.Model(&model.JudgeExamRecord{}).Where("user_id = ?", 3705).Count(&recordCount).Error)
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ?", 3705).Count(&qualCount).Error)
	require.Zero(t, recordCount, "无会话提交不得产生成绩")
	require.Zero(t, qualCount, "无会话提交不得授予资格")
}

func TestT37SubmitExamSessionExpiredAfterTTL(t *testing.T) {
	svc, db, mr, _ := setupT37ExamTest(t)
	seedT37User(t, db, 3706)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3706)
	require.NoError(t, err)

	mr.FastForward(15*time.Minute + time.Second)

	_, _, err = svc.SubmitExam(SubmitExamInput{
		ContentType: t37ContentType,
		Answers:     []ExamAnswer{{QuestionID: 1, Answer: "A"}},
	}, 3706)
	require.ErrorIs(t, err, ErrExamSessionExpired)

	var qualCount int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ?", 3706).Count(&qualCount).Error)
	require.Zero(t, qualCount, "过期会话不得授资格")
}

func TestT37SubmitExamRejectsAlreadyQualified(t *testing.T) {
	svc, db, _, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3707)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3707)
	require.NoError(t, err)
	t37SeedQualification(t, db, 3707)

	_, _, err = svc.SubmitExam(SubmitExamInput{
		ContentType: t37ContentType,
		Answers:     t37SessionAnswers(t, rdb, 3707),
	}, 3707)
	require.ErrorIs(t, err, ErrAlreadyQualified)
}

func TestT37SessionConsumedAfterSubmit(t *testing.T) {
	svc, db, _, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3708)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3708)
	require.NoError(t, err)

	answers := t37SessionAnswers(t, rdb, 3708)
	// 首次提交只答一半（不通过、不授资格）。
	_, passed, err := svc.SubmitExam(SubmitExamInput{ContentType: t37ContentType, Answers: answers[:len(answers)/2]}, 3708)
	require.NoError(t, err)
	require.False(t, passed)

	// 同一会话重放必须被拒：会话已消费（一次性），不得二次评分。
	_, _, err = svc.SubmitExam(SubmitExamInput{ContentType: t37ContentType, Answers: answers}, 3708)
	require.ErrorIs(t, err, ErrExamSessionExpired)

	var qualCount int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ?", 3708).Count(&qualCount).Error)
	require.Zero(t, qualCount, "未通过不得授资格")
}

func TestT37SessionConsumedByQualification(t *testing.T) {
	svc, db, _, rdb := setupT37ExamTest(t)
	seedT37User(t, db, 3710)
	seedT37Questions(t, db, examQuestionCount)

	_, err := svc.GetExam(t37ContentType, 3710)
	require.NoError(t, err)

	answers := t37SessionAnswers(t, rdb, 3710)
	_, passed, err := svc.SubmitExam(SubmitExamInput{ContentType: t37ContentType, Answers: answers}, 3710)
	require.NoError(t, err)
	require.True(t, passed)

	// 通过并授资格后，同会话再提交命中连考限制。
	_, _, err = svc.SubmitExam(SubmitExamInput{ContentType: t37ContentType, Answers: answers}, 3710)
	require.ErrorIs(t, err, ErrAlreadyQualified)
}

func TestT37InsufficientQuestionsStillRejected(t *testing.T) {
	svc, db, _, _ := setupT37ExamTest(t)
	seedT37User(t, db, 3709)
	seedT37Questions(t, db, examQuestionCount-1)

	_, err := svc.GetExam(t37ContentType, 3709)
	require.ErrorIs(t, err, ErrInsufficientQuestions)
	_ = strconv.Itoa // keep strconv import stable if assertions evolve
}
