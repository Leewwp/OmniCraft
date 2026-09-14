package repository

import (
	"strconv"
	"strings"

	"omnicraft/backend/internal/model"
)

// SP-18 #509：通知列表装饰——GET /notifications 每条补充 sender（触发者资料，
// 批量 join 免前端 N+1）与 target_summary（原内容引用块的 kind/title/url）。
// 响应只增不改：DecoratedNotification 内嵌原 model.Notification，老字段原样；
// url 映射与 frontend lib/notification-url.ts 同源（discussion 类在后端直接
// 批量解析出 ip_id，免前端逐条二跳）。

type NotificationSender struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio,omitempty"`
}

type NotificationTargetSummary struct {
	Kind  string `json:"kind"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

type DecoratedNotification struct {
	model.Notification
	Sender        *NotificationSender        `json:"sender,omitempty"`
	TargetSummary *NotificationTargetSummary `json:"target_summary,omitempty"`
}

func (r *NotificationRepository) ListDecorated(userID int64, channel string, page, pageSize int) ([]DecoratedNotification, int64, error) {
	notifications, total, err := r.List(userID, channel, page, pageSize)
	if err != nil {
		return nil, total, err
	}
	items := make([]DecoratedNotification, len(notifications))
	for i, n := range notifications {
		items[i] = DecoratedNotification{Notification: n}
	}
	if len(items) > 0 {
		r.decorate(items)
	}
	return items, total, nil
}

func (r *NotificationRepository) decorate(items []DecoratedNotification) {
	senders := r.sendersByIDs(collectSenderIDs(items))
	contentTitles := r.contentTitlesByIDs(collectTargetIDs(items, "content", "comment"))
	discussions := r.discussionsByIDs(collectTargetIDs(items, "discussion"))
	ipNames := r.ipNamesByIDs(collectTargetIDs(items, "ip"))
	prContentTitles := r.prContentTitlesByIDs(collectTargetIDs(items, "pr"))

	for i := range items {
		n := &items[i]
		if n.SenderID != nil {
			if sender, ok := senders[*n.SenderID]; ok {
				n.Sender = &sender
			}
		}
		if n.TargetType == nil || n.TargetID == nil || *n.TargetID <= 0 {
			continue
		}
		targetID := *n.TargetID
		switch kind := *n.TargetType; kind {
		case "content", "comment":
			if title, ok := contentTitles[targetID]; ok {
				n.TargetSummary = &NotificationTargetSummary{Kind: "content", Title: title, URL: "/content/" + strconv.FormatInt(targetID, 10)}
			}
		case "discussion":
			if d, ok := discussions[targetID]; ok {
				summary := &NotificationTargetSummary{Kind: "discussion", Title: d.Title}
				if d.IPID != nil && *d.IPID > 0 {
					summary.URL = "/ip/" + strconv.FormatInt(*d.IPID, 10) + "?tab=discussions&d=" + strconv.FormatInt(targetID, 10)
				}
				n.TargetSummary = summary
			}
		case "pr":
			if title, ok := prContentTitles[targetID]; ok {
				n.TargetSummary = &NotificationTargetSummary{Kind: "pr", Title: title, URL: "/studio/pr-requests"}
			}
		case "user":
			n.TargetSummary = &NotificationTargetSummary{Kind: "user", URL: "/user/" + strconv.FormatInt(targetID, 10)}
		case "ip":
			if name, ok := ipNames[targetID]; ok {
				url := "/ip/" + strconv.FormatInt(targetID, 10)
				if strings.HasPrefix(n.Type, "ip_proposal_") {
					url += "?tab=proposals"
				}
				n.TargetSummary = &NotificationTargetSummary{Kind: "ip", Title: name, URL: url}
			}
		case "appeal":
			n.TargetSummary = &NotificationTargetSummary{Kind: "appeal", URL: "/appeals"}
		case "report":
			n.TargetSummary = &NotificationTargetSummary{Kind: "report", URL: "/appeals?tab=reports"}
		case "feedback_ticket":
			n.TargetSummary = &NotificationTargetSummary{Kind: "feedback_ticket", URL: "/feedback/mine"}
		case "message":
			n.TargetSummary = &NotificationTargetSummary{Kind: "message", URL: "/messages?channel=dm"}
		}
	}
}

func collectSenderIDs(items []DecoratedNotification) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(items))
	for i := range items {
		if id := items[i].SenderID; id != nil && *id > 0 && !seen[*id] {
			seen[*id] = true
			ids = append(ids, *id)
		}
	}
	return ids
}

func collectTargetIDs(items []DecoratedNotification, kinds ...string) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(items))
	for i := range items {
		if items[i].TargetType == nil || items[i].TargetID == nil {
			continue
		}
		matched := false
		for _, kind := range kinds {
			if *items[i].TargetType == kind {
				matched = true
				break
			}
		}
		if matched && *items[i].TargetID > 0 && !seen[*items[i].TargetID] {
			seen[*items[i].TargetID] = true
			ids = append(ids, *items[i].TargetID)
		}
	}
	return ids
}

func (r *NotificationRepository) sendersByIDs(ids []int64) map[int64]NotificationSender {
	result := map[int64]NotificationSender{}
	if len(ids) == 0 {
		return result
	}
	var rows []NotificationSender
	if err := r.db.Model(&model.User{}).
		Select("id, username, avatar_url, bio").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result
}

func (r *NotificationRepository) contentTitlesByIDs(ids []int64) map[int64]string {
	result := map[int64]string{}
	if len(ids) == 0 {
		return result
	}
	var rows []struct {
		ID    int64
		Title string
	}
	if err := r.db.Model(&model.ContentItem{}).
		Select("id, title").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = row.Title
	}
	return result
}

func (r *NotificationRepository) discussionsByIDs(ids []int64) map[int64]struct {
	Title string
	IPID  *int64
} {
	result := map[int64]struct {
		Title string
		IPID  *int64
	}{}
	if len(ids) == 0 {
		return result
	}
	var rows []struct {
		ID    int64
		Title string
		IPID  *int64
	}
	if err := r.db.Model(&model.Discussion{}).
		Select("id, title, ip_id").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = struct {
			Title string
			IPID  *int64
		}{Title: row.Title, IPID: row.IPID}
	}
	return result
}

func (r *NotificationRepository) ipNamesByIDs(ids []int64) map[int64]string {
	result := map[int64]string{}
	if len(ids) == 0 {
		return result
	}
	var rows []struct {
		ID   int64
		Name string
	}
	if err := r.db.Model(&model.IP{}).
		Select("id, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result
}

// prContentTitlesByIDs：PR 引用块标题 = 所属内容标题（prs → content_items 一次 join）。
func (r *NotificationRepository) prContentTitlesByIDs(ids []int64) map[int64]string {
	result := map[int64]string{}
	if len(ids) == 0 {
		return result
	}
	var rows []struct {
		ID    int64
		Title string
	}
	if err := r.db.Table("pull_requests").
		Select("pull_requests.id, content_items.title").
		Joins("JOIN content_items ON content_items.id = pull_requests.content_item_id").
		Where("pull_requests.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = row.Title
	}
	return result
}
