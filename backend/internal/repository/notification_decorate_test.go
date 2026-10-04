package repository

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/testutil"
)

func setupNotificationDecorateRepo(t *testing.T) (*NotificationRepository, *gorm.DB) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	for _, migration := range []string{
		"001_users.sql",
		"005_ips.sql",
		"006_content_items.sql",
		"023_notifications.sql",
	} {
		testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", migration))
	}
	return NewNotificationRepository(db), db
}

// #786：content/comment 通知深链按 content_items.zone 分流——原创内容落
// /original/{id}（/content/{id} 对 zone=original 有意 notFound），二创维持
// /content/{id}；zone 同时透出在 target_summary.zone 供前端兜底。
func TestDecorateContentTargetURLsSplitByZone(t *testing.T) {
	repo, db := setupNotificationDecorateRepo(t)

	require.NoError(t, db.Exec(`INSERT INTO users (id, email, password_hash, username) VALUES
		(1, 'author@example.test', 'hash', 'author'),
		(2, 'reader@example.test', 'hash', 'reader')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO content_items (id, title, author_id, zone, content_type) VALUES
		(101, 'original work', 1, 'original', 'article'),
		(102, 'fanwork', 1, 'fanwork', 'article')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO notifications (user_id, type, channel, title, target_type, target_id, sender_id) VALUES
		(1, 'content_like', 'like', 'liked', 'content', 101, 2),
		(1, 'comment_reply', 'reply', 'replied', 'content', 102, 2)`).Error)

	items, _, err := repo.ListDecorated(1, "", 1, 20)
	require.NoError(t, err)
	require.Len(t, items, 2)

	byTarget := map[int64]*NotificationTargetSummary{}
	for _, item := range items {
		require.NotNil(t, item.TargetSummary, "content target must decorate")
		byTarget[*item.TargetID] = item.TargetSummary
	}

	original := byTarget[101]
	require.Equal(t, "/original/101", original.URL)
	require.Equal(t, "original", original.Zone)
	require.Equal(t, "original work", original.Title)

	fanwork := byTarget[102]
	require.Equal(t, "/content/102", fanwork.URL)
	require.Equal(t, "fanwork", fanwork.Zone)
}
