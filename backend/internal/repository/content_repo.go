package repository

import (
	"fmt"
	"log/slog"
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ContentRepository struct {
	db *gorm.DB
}

func NewContentRepository(db *gorm.DB) *ContentRepository {
	return &ContentRepository{db: db}
}

func (r *ContentRepository) DB() *gorm.DB { return r.db }

type ListContentsFilter struct {
	Zone string
	// IncludeAllStatuses 用于作者自助列表（/users/me/contents）：列出全部
	// 状态（含 banned/pending），公开浏览路径不得开启（FIX-16/T08）。
	IncludeAllStatuses bool
	IPID               *int64
	SourceOriginalID   *int64
	SourceFanworkID    *int64
	Category           string
	ContentType        string
	ContentTypes       []string
	AuthorID           *int64
	Status             string
	Tags               []string
	Sort               string
	TimeRange          string
	// Search does a title-substring filter (IP 内搜索, #290) on the list path;
	// full-text relevance search stays on /contents/search.
	Search   string
	Page     int
	PageSize int
	// ViewerID lets source-linkage queries reuse the centralized content
	// visibility scope (published, non-deleted, author/IP not banned, and
	// is_public OR author-owned) for returned children.
	ViewerID int64
}

func (r *ContentRepository) CreateContent(content *model.ContentItem) error {
	return r.db.Create(content).Error
}

func (r *ContentRepository) Transaction(fn func(tx *ContentRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &ContentRepository{db: tx}
		return fn(txRepo)
	})
}

func (r *ContentRepository) FindByID(id int64) (*model.ContentItem, error) {
	var content model.ContentItem
	err := r.db.Preload("Author").Where("deleted_at IS NULL").First(&content, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	// #846：详情路径挂载关联 IP 摘要（GetContent 的详情缓存写入发生在本方法
	// 之后，缓存 JSON 与回源行因此携带同一 ip 形状）。
	rows := []model.ContentItem{content}
	r.hydrateIPSummaries(rows)
	content.IP = rows[0].IP
	return &content, nil
}

// FindByIDForUpdate locks the content row for UPDATE inside a transaction and
// returns it, or nil when it is missing or soft-deleted. Serializing on the
// content row is what keeps concurrent invites from over-reserving contributor
// slots for the same content.
func (r *ContentRepository) FindByIDForUpdate(id int64) (*model.ContentItem, error) {
	var content model.ContentItem
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("deleted_at IS NULL").
		First(&content, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &content, nil
}

// IsContributor reports whether the user is a confirmed contributor of the
// content item.
func (r *ContentRepository) IsContributor(contentID, userID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.ContentContributor{}).
		Where("content_item_id = ? AND user_id = ?", contentID, userID).
		Count(&count).Error
	return count > 0, err
}

// CountContributors returns the number of confirmed contributor rows.
func (r *ContentRepository) CountContributors(contentID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.ContentContributor{}).
		Where("content_item_id = ?", contentID).
		Count(&count).Error
	return count, err
}

// CountPendingInviteesNotContributors returns the number of distinct invitees
// of active pending invites for the content who are not already confirmed
// contributors. Together with CountContributors this is the authoritative
// capacity check performed under the locked content row.
func (r *ContentRepository) CountPendingInviteesNotContributors(contentID int64) (int64, error) {
	var count int64
	err := r.db.Raw(`
		SELECT COUNT(DISTINCT ci.invitee_id)
		FROM collaboration_invites AS ci
		LEFT JOIN content_contributors AS cc
		  ON cc.content_item_id = ci.content_id AND cc.user_id = ci.invitee_id
		WHERE ci.content_id = ? AND ci.status = ? AND cc.user_id IS NULL
	`, contentID, model.CollabInviteStatusPending).Scan(&count).Error
	return count, err
}

// InsertContributorIfAbsent records a collaboration-created contributor row
// with pr_count=0 and the caller-supplied first_at. ON CONFLICT DO NOTHING
// makes the accept idempotent: an existing row keeps its pr_count and first_at
// untouched. pr_count is deliberately never incremented here — only merged
// pull requests do that (PRRepository.UpsertContributor).
func (r *ContentRepository) InsertContributorIfAbsent(contentID, userID int64, now time.Time) error {
	return r.db.Exec(`
		INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at)
		VALUES (?, ?, 0, ?)
		ON CONFLICT (content_item_id, user_id) DO NOTHING
	`, contentID, userID, now).Error
}

// hydrateIPSummaries 批量挂载内容行的关联 IP 摘要 {id, name, cover_url}
// （#846：/contents 列表与详情从未返回嵌套 ip，前端「基于 XX」/「IP：XX」/
// 关联 IP 卡全链路恒不渲染）。单条 IN 查询去重后一次取回，避免逐行 N+1；
// 只取摘要列——IP 行的 status/creator/description 等不随内容响应外泄，封面
// 裸 URL 由 handler 边界的 DecorateContent 签名（与 /ips 列表同一链路）。
// 口径与内容可见性一致：IP banned 的内容已被 ApplyContentVisibilityScope
// （列表/来源联动）与 contentVisibleToViewer（详情）排除，本方法不引入新
// 越权面。摘要属于展示性数据：查询失败记 WARN 后静默降级为无 ip（与缓存
// 层 Redis 故障当 miss 的降级口径一致），不阻断内容读路径；悬空 ip_id
// （IP 行已删）同样落为 nil。
func (r *ContentRepository) hydrateIPSummaries(items []model.ContentItem) {
	ids := make([]int64, 0, len(items))
	seen := make(map[int64]bool, len(items))
	for i := range items {
		if items[i].IPID != nil && *items[i].IPID > 0 && !seen[*items[i].IPID] {
			seen[*items[i].IPID] = true
			ids = append(ids, *items[i].IPID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var ips []model.IP
	if err := r.db.Select("id", "name", "cover_url").Where("id IN ?", ids).Find(&ips).Error; err != nil {
		slog.Warn("hydrate content ip summaries failed", "error", err)
		return
	}
	byID := make(map[int64]*model.IP, len(ips))
	for i := range ips {
		byID[ips[i].ID] = &ips[i]
	}
	for i := range items {
		if items[i].IPID == nil {
			continue
		}
		if ip, ok := byID[*items[i].IPID]; ok {
			items[i].IP = ip
		}
	}
}

func (r *ContentRepository) BatchGetByIDs(ids []int64) ([]model.ContentItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var contents []model.ContentItem
	err := r.db.Preload("Author").Where("id IN ? AND deleted_at IS NULL", ids).Find(&contents).Error
	if err != nil {
		return nil, err
	}
	// #846：hot 列表（getHotContents）走本方法取载荷，rank ZSET 只存 ID，
	// 每次回源都经此挂载，hot 路径与列表/详情天然同形状。
	r.hydrateIPSummaries(contents)
	return contents, nil
}

func (r *ContentRepository) ListContents(f ListContentsFilter) ([]model.ContentItem, int64, error) {
	var items []model.ContentItem
	var total int64

	q := r.db.Model(&model.ContentItem{}).Preload("Author")

	// Source-linkage queries (related fanworks / derivative works) reuse the
	// centralized content visibility scope instead of a partial
	// status='published' predicate that can drift from soft-delete,
	// author-deleted and banned rules.
	if f.SourceOriginalID != nil || f.SourceFanworkID != nil {
		q = ApplyContentVisibilityScope(q, f.ViewerID)
	} else if f.IncludeAllStatuses {
		// 作者自助列表（/users/me/contents）：全部状态，仅自身可见。
		q = q.Where("deleted_at IS NULL")
	} else if f.Status != "" {
		// 显式状态过滤（admin 终审队列等）：保留原语义。
		q = q.Where("deleted_at IS NULL")
		q = q.Where("status = ?", f.Status)
	} else {
		// 主列表统一 viewer-aware 可见性（FIX-12+43）：published + 公开 +
		// 作者未封禁/未注销 + IP 未封禁（作者本人可见自己的私密内容）。
		q = ApplyContentVisibilityScope(q, f.ViewerID)
	}

	if f.Zone != "" {
		q = q.Where("zone = ?", f.Zone)
	}
	if f.IPID != nil {
		q = q.Where("ip_id = ?", *f.IPID)
	}
	if f.SourceOriginalID != nil {
		q = q.Where("source_original_id = ?", *f.SourceOriginalID)
	}
	if f.SourceFanworkID != nil {
		q = q.Where("source_fanwork_id = ?", *f.SourceFanworkID)
	}
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if len(f.ContentTypes) > 0 {
		q = q.Where("content_type IN ?", f.ContentTypes)
	} else if f.ContentType != "" {
		q = q.Where("content_type = ?", f.ContentType)
	}
	if f.Search != "" {
		q = q.Where("title LIKE ?", "%"+f.Search+"%")
	}
	if f.AuthorID != nil {
		q = q.Where("author_id = ?", *f.AuthorID)
	}

	if f.TimeRange != "" && f.TimeRange != "all" {
		var since time.Time
		now := time.Now()
		switch f.TimeRange {
		case "week":
			since = now.AddDate(0, 0, -7)
		case "month":
			since = now.AddDate(0, -1, 0)
		case "year":
			since = now.AddDate(-1, 0, 0)
		}
		q = q.Where("created_at >= ?", since)
	}

	if len(f.Tags) > 0 {
		for _, tag := range f.Tags {
			q = q.Where("id IN (SELECT content_item_id FROM content_tags WHERE tag = ?)", tag)
		}
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	switch f.Sort {
	case "hot":
		q = q.Order("hot_score DESC NULLS LAST, (view_count + like_count * 3) DESC")
	case "most_views":
		q = q.Order("view_count DESC")
	case "best_rated":
		q = q.Where("(like_count + dislike_count) >= 5").
			Order("rating_score DESC NULLS LAST")
	default:
		q = q.Order("created_at DESC")
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if err := q.Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	// #846：主列表（含 hot 缓存未命中回源与 /ips 分享页等复用方）挂载关联
	// IP 摘要；service 层的列表缓存写入发生在本方法之后，缓存 JSON 与回源
	// 行携带同一 ip 形状。
	r.hydrateIPSummaries(items)

	return items, total, nil
}

// AuthorContentTotals aggregates lifetime view/like sums across an author's
// own contents (all statuses, soft-delete excluded — same scope as the
// /users/me/contents self-service list). Serves the studio overview stats
// cards so totals no longer reduce whatever the first page happened to load.
func (r *ContentRepository) AuthorContentTotals(authorID int64, contentType string) (views int64, likes int64, err error) {
	q := r.db.Model(&model.ContentItem{}).
		Where("author_id = ? AND deleted_at IS NULL", authorID)
	if contentType != "" {
		q = q.Where("content_type = ?", contentType)
	}
	row := struct {
		Views int64
		Likes int64
	}{}
	if err := q.Select("COALESCE(SUM(view_count), 0) AS views, COALESCE(SUM(like_count), 0) AS likes").
		Scan(&row).Error; err != nil {
		return 0, 0, err
	}
	return row.Views, row.Likes, nil
}

// CountByTypeWithinIP returns per-content_type hit counts for the IP share
// tab facet chips (#290). It mirrors the share-tab list semantics (public
// fanworks of this IP, optional title search) but ignores the active type
// filter so chip counts stay comparable across pills.
func (r *ContentRepository) CountByTypeWithinIP(ipID int64, search string) (map[string]int64, error) {
	var rows []struct {
		ContentType string
		Count       int64
	}
	q := ApplyContentVisibilityScope(r.db.Model(&model.ContentItem{}), 0).
		Where("content_items.ip_id = ?", ipID).
		Where("content_items.zone = ?", "fanwork")
	if search != "" {
		q = q.Where("content_items.title LIKE ?", "%"+search+"%")
	}
	if err := q.Select("content_type, COUNT(*) AS count").
		Group("content_type").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.ContentType] = row.Count
	}
	return counts, nil
}

func (r *ContentRepository) UpdateContent(id int64, updates map[string]interface{}) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ContentRepository) DeleteContent(id int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

func (r *ContentRepository) SoftDeleteContent(id int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

func (r *ContentRepository) RestoreContent(id int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		Update("deleted_at", nil).Error
}

func (r *ContentRepository) IncrViewCount(id int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *ContentRepository) IncrViewCountBy(id int64, delta int64) error {
	return r.db.Model(&model.ContentItem{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta)).Error
}

func (r *ContentRepository) BatchIncrViewCounts(batch map[int64]int64) error {
	if len(batch) == 0 {
		return nil
	}

	/* #400：CASE 表达式交给 UpdateColumn 的列赋值——表达式本身不得再带
	   "view_count = " 前缀，否则生成 SET view_count = view_count = CASE ...
	   （内层 = 是 boolean 比较），PostgreSQL 以 SQLSTATE 42804 拒绝
	   （bigint 列收到 boolean）；sqlite 宽松类型不报错故线上才暴露。 */
	caseStmt := "CASE id "
	var ids []int64
	for id, delta := range batch {
		caseStmt += fmt.Sprintf("WHEN %d THEN view_count + %d ", id, delta)
		ids = append(ids, id)
	}
	caseStmt += "ELSE view_count END"

	return r.db.Model(&model.ContentItem{}).Where("id IN ?", ids).
		UpdateColumn("view_count", gorm.Expr(caseStmt)).Error
}

func (r *ContentRepository) CreateAttachments(attachments []model.ContentAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	return r.db.Create(&attachments).Error
}

func (r *ContentRepository) CreateTags(tags []model.ContentTag) error {
	if len(tags) == 0 {
		return nil
	}
	return r.db.Create(&tags).Error
}

func (r *ContentRepository) GetAttachments(contentID int64) ([]model.ContentAttachment, error) {
	var attachments []model.ContentAttachment
	// Media sets are browsed in stable order: sort_order ASC NULLS LAST with
	// id ASC as the deterministic fallback for legacy rows without sort_order.
	err := r.db.Where("content_item_id = ?", contentID).
		Order("sort_order ASC NULLS LAST, id ASC").
		Find(&attachments).Error
	return attachments, err
}

func (r *ContentRepository) GetTags(contentID int64) ([]model.ContentTag, error) {
	var tags []model.ContentTag
	err := r.db.Where("content_item_id = ?", contentID).Find(&tags).Error
	return tags, err
}

func (r *ContentRepository) AddTag(contentID int64, tag string) error {
	ct := model.ContentTag{ContentItemID: contentID, Tag: tag}
	return r.db.FirstOrCreate(&ct, model.ContentTag{ContentItemID: contentID, Tag: tag}).Error
}

func (r *ContentRepository) RemoveTag(contentID int64, tag string) error {
	return r.db.Where("content_item_id = ? AND tag = ?", contentID, tag).Delete(&model.ContentTag{}).Error
}
