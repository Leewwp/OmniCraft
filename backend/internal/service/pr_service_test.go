package service

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// setupPRServiceTest wires an ephemeral Postgres with the version/PR schema
// (009 + 010) and the transactional outbox (070) so merge assertions can
// check the content.updated event row written in the same transaction.
func setupPRServiceTest(t *testing.T) (*PRService, *VersionService, *gorm.DB) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createPRBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "009_content_versions.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "010_pull_requests.sql"))
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

	svc := NewPRService(
		repository.NewPRRepository(db),
		repository.NewVersionRepository(db),
		repository.NewContentRepository(db),
	)
	vSvc := NewVersionService(
		repository.NewVersionRepository(db),
		repository.NewContentRepository(db),
	)
	return svc, vSvc, db
}

func createPRBaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			reputation INT NOT NULL DEFAULT 10,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			description TEXT,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			ip_id BIGINT,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			cover_image_url TEXT,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE reputation_logs (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			delta INT NOT NULL,
			reason VARCHAR(100) NOT NULL,
			related_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Error)
}

func seedPRUser(t *testing.T, db *gorm.DB, label string) int64 {
	t.Helper()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	var id int64
	require.NoError(t, db.Raw(`
		INSERT INTO users (email, password_hash, username)
		VALUES (?, 'x', ?) RETURNING id
	`, nonce+"@"+label+".test", label+"_"+nonce).Scan(&id).Error)
	return id
}

// seedPRContent creates one published content with its initial active
// version; PRs then base themselves on that version.
func seedPRContent(t *testing.T, db *gorm.DB, authorID int64, body string) (contentID, versionID int64) {
	t.Helper()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	require.NoError(t, db.Exec(`
		INSERT INTO content_items (title, description, author_id, zone, content_type, status)
		VALUES (?, ?, ?, 'main', 'original', 'published')
	`, "PR fixture "+nonce, body, authorID).Error)
	require.NoError(t, db.Raw(`SELECT id FROM content_items WHERE title = ?`, "PR fixture "+nonce).Scan(&contentID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO content_versions (content_item_id, author_id, version_number, storage_type, storage_key, diff_summary, status, is_latest)
		VALUES (?, ?, 1, 'full', ?, 'initial version', 'active', TRUE)
	`, contentID, authorID, body).Error)
	require.NoError(t, db.Raw(`SELECT id FROM content_versions WHERE content_item_id = ? AND version_number = 1`, contentID).Scan(&versionID).Error)
	return contentID, versionID
}

// submitFixturePR submits one PR carrying new_text and returns the PR.
func submitFixturePR(t *testing.T, svc *PRService, contentID, baseVersionID, submitterID int64, newText string) *model.PullRequest {
	t.Helper()
	pr, err := svc.SubmitPR(SubmitPRInput{
		ContentItemID: contentID,
		BaseVersionID: baseVersionID,
		Message:       "fixture pr",
		NewText:       newText,
	}, submitterID)
	require.NoError(t, err)
	return pr
}

// TDD #1（FIX-21①）：SubmitPR 的 new_text 必须落库为 proposed 版本并挂到
// PR 上；proposed 版本对版本列表读者不可见、对内容作者可见。
func TestSubmitPRPersistsProposedVersion(t *testing.T) {
	svc, vSvc, db := setupPRServiceTest(t)
	author := seedPRUser(t, db, "author")
	submitter := seedPRUser(t, db, "submitter")
	reader := seedPRUser(t, db, "reader")
	contentID, baseVersionID := seedPRContent(t, db, author, "v1 body")

	pr := submitFixturePR(t, svc, contentID, baseVersionID, submitter, "proposed body")
	require.NotNil(t, pr.ProposedVersionID, "new_text must be persisted as a proposed version")

	var proposed model.ContentVersion
	require.NoError(t, db.First(&proposed, *pr.ProposedVersionID).Error)
	require.Equal(t, "proposed", proposed.Status)
	require.False(t, proposed.IsLatest, "proposed version must not be the published latest")
	require.Equal(t, "full", proposed.StorageType)
	require.Equal(t, "proposed body", proposed.StorageKey)
	require.Equal(t, submitter, proposed.AuthorID)

	// 作者视角：v1 + proposed 都可见
	authorVersions, total, err := vSvc.ListVersionsPagedForViewer(contentID, 1, 20, author)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.True(t, containsVersionStatus(authorVersions, "proposed"))

	// 读者视角：只见 active
	readerVersions, total, err := vSvc.ListVersionsPagedForViewer(contentID, 1, 20, reader)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.False(t, containsVersionStatus(readerVersions, "proposed"))

	// base 版本必须属于该内容：跨内容 id 拒绝（防两个版本链被拼接）
	otherContentID, otherVersionID := seedPRContent(t, db, seedPRUser(t, db, "other"), "other body")
	require.NotEqual(t, otherVersionID, baseVersionID)
	_, err = svc.SubmitPR(SubmitPRInput{
		ContentItemID: otherContentID,
		BaseVersionID: baseVersionID, // 属于第一个内容
		NewText:       "x",
	}, submitter)
	require.ErrorIs(t, err, ErrPRBaseInvalid)
}

func containsVersionStatus(versions []model.ContentVersion, status string) bool {
	for _, v := range versions {
		if v.Status == status {
			return true
		}
	}
	return false
}

// TDD #2（FIX-21②）：accept 之后 merge 必须放行（移除互斥）；merge 未显式
// 给 merged_text 时采用 proposed 版本正文；终态 PR 不可再 accept/merge。
func TestAcceptThenManualMergeSucceeds(t *testing.T) {
	svc, _, db := setupPRServiceTest(t)
	author := seedPRUser(t, db, "author")
	submitter := seedPRUser(t, db, "submitter")
	contentID, baseVersionID := seedPRContent(t, db, author, "v1 body")
	svc.SetMergeSupport(nil, repository.NewOutboxRepository(db), NewReputationService(db))

	pr := submitFixturePR(t, svc, contentID, baseVersionID, submitter, "merged via pr")
	require.NoError(t, svc.AcceptPR(pr.ID, author))

	merged, err := svc.ManualMerge(pr.ID, author, "")
	require.NoError(t, err, "merging an accepted PR must be allowed")
	require.True(t, merged.IsLatest)
	require.Equal(t, "merged via pr", merged.StorageKey, "empty merged_text falls back to the proposed version body")

	var after model.PullRequest
	require.NoError(t, db.First(&after, pr.ID).Error)
	require.Equal(t, "merged", after.Status)

	// 终态守卫：merged 之后不可再次 merge / accept
	_, err = svc.ManualMerge(pr.ID, author, "")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPRInvalidState)
	require.ErrorIs(t, svc.AcceptPR(pr.ID, author), ErrPRInvalidState)
}

// TDD #3（FIX-21②③）：merge 事务内同步 content_items 正文并发出
// content.updated 索引事件；merge 后给提交者 +3 信誉分（AwardPRMerged）。
func TestManualMergeSyncsBodyEventAndReputation(t *testing.T) {
	svc, _, db := setupPRServiceTest(t)
	author := seedPRUser(t, db, "author")
	submitter := seedPRUser(t, db, "submitter")
	contentID, baseVersionID := seedPRContent(t, db, author, "v1 body")
	svc.SetMergeSupport(nil, repository.NewOutboxRepository(db), NewReputationService(db))

	pr := submitFixturePR(t, svc, contentID, baseVersionID, submitter, "ignored")
	require.NoError(t, svc.AcceptPR(pr.ID, author))
	_, err := svc.ManualMerge(pr.ID, author, "manually merged text")
	require.NoError(t, err)

	var content model.ContentItem
	require.NoError(t, db.First(&content, contentID).Error)
	require.Equal(t, "manually merged text", content.Description, "merge must sync the content body")

	var event model.OutboxEvent
	require.NoError(t, db.Where("aggregate_id = ? AND event_type = ?", contentID, "content.updated").First(&event).Error)

	var reput int64
	require.NoError(t, db.Raw(`SELECT reputation FROM users WHERE id = ?`, submitter).Scan(&reput).Error)
	require.EqualValues(t, 13, reput, "PR merge awards +3 reputation to the submitter")
	var log model.ReputationLog
	require.NoError(t, db.Where("user_id = ? AND reason = 'pr_merged'", submitter).First(&log).Error)
	require.EqualValues(t, 3, log.Delta)
	require.NotNil(t, log.RelatedID)
	require.Equal(t, pr.ID, *log.RelatedID)
}

// TDD #4（FIX-21④）：版本/PR 详情端点仅参与方可读（内容作者 / PR 提交者 /
// admin），其余读者一律拒绝（F-056：banned 内容版本全文不再泄露）。
func TestParticipantAuthOnVersionAndPRDetail(t *testing.T) {
	svc, vSvc, db := setupPRServiceTest(t)
	author := seedPRUser(t, db, "author")
	submitter := seedPRUser(t, db, "submitter")
	reader := seedPRUser(t, db, "reader")
	contentID, baseVersionID := seedPRContent(t, db, author, "v1 body")

	pr := submitFixturePR(t, svc, contentID, baseVersionID, submitter, "proposed body")

	// 版本详情：读者 403，作者/提交者（proposed）/admin 放行
	_, err := vSvc.GetVersionForViewer(baseVersionID, reader, false)
	require.ErrorIs(t, err, ErrVersionForbidden)
	_, err = vSvc.GetVersionForViewer(baseVersionID, author, false)
	require.NoError(t, err)
	proposedText, err := vSvc.GetVersionForViewer(*pr.ProposedVersionID, submitter, false)
	require.NoError(t, err)
	require.Equal(t, "proposed body", proposedText)
	_, err = vSvc.GetVersionForViewer(baseVersionID, 0, true)
	require.NoError(t, err)

	// PR 详情：读者 403，作者/提交者/admin 放行
	_, err = svc.GetPRForViewer(pr.ID, reader, false)
	require.ErrorIs(t, err, ErrPRForbidden)
	_, err = svc.GetPRForViewer(pr.ID, author, false)
	require.NoError(t, err)
	_, err = svc.GetPRForViewer(pr.ID, submitter, false)
	require.NoError(t, err)
	_, err = svc.GetPRForViewer(pr.ID, 0, true)
	require.NoError(t, err)
}
