package repository

// T40（FIX-36d）：判官队列排除本人已投案件。

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func TestT40ListOpenCasesExcludesVotedByJudge(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.JudgeCase{}, &model.JudgeVote{}))
	require.NoError(t, db.Create(&model.User{ID: 4200, Email: "t40-judge@example.test", Username: "t40_judge", PasswordHash: "hash"}).Error)

	require.NoError(t, db.Create(&model.JudgeCase{ID: 4101, TargetType: "article", TargetID: 1, Status: "open"}).Error)
	require.NoError(t, db.Create(&model.JudgeCase{ID: 4102, TargetType: "article", TargetID: 2, Status: "open"}).Error)
	require.NoError(t, db.Create(&model.JudgeVote{CaseID: 4102, JudgeID: 4200, Vote: "approve"}).Error)

	repo := NewJudgeRepository(db)
	cases, total, err := repo.ListOpenCases([]string{"article"}, 9999, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "两个 open 案件对其他判官可见")

	cases, total, err = repo.ListOpenCases([]string{"article"}, 4200, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "已投案件被排除")
	require.Len(t, cases, 1)
	require.EqualValues(t, 4101, cases[0].ID, "只剩未投的 4101")
}
