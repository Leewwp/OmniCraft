package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// IPVisitHistoryHandler exposes the account-bound recent IP visit history:
// direct recording with the server receive time and an idempotent anonymous
// merge for the login cutover. All routes require authentication and live
// under /api/v1/users/me.
type IPVisitHistoryHandler struct {
	repo          *repository.IPVisitHistoryRepository
	displaySigner *service.DisplayURLSigner
}

func NewIPVisitHistoryHandler(db *gorm.DB) *IPVisitHistoryHandler {
	return &IPVisitHistoryHandler{repo: repository.NewIPVisitHistoryRepository(db)}
}

// SetDisplayURLSigner wires display URL signing for the IP cover summaries
// (B-002).
func (h *IPVisitHistoryHandler) SetDisplayURLSigner(signer *service.DisplayURLSigner) {
	h.displaySigner = signer
}

// ListRecent returns the current user's up-to-six recent visits as public IP
// summaries, ordered by most recent first.
func (h *IPVisitHistoryHandler) ListRecent(c *gin.Context) {
	userID := middleware.GetUserID(c)

	items, err := h.repo.ListRecent(userID, repository.RecentIPVisitLimit)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "IP_VISIT_LIST_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": toIPVisitResponse(h.displaySigner, items),
		"limit": repository.RecentIPVisitLimit,
	})
}

// RecordVisit records or refreshes one visit at the server receive time.
// The operation is idempotent: repeated calls return 204 without duplicating
// rows and never lower recency.
func (h *IPVisitHistoryHandler) RecordVisit(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ipID, err := strconv.ParseInt(c.Param("ipId"), 10, 64)
	if err != nil || ipID <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": "IP_NOT_FOUND", "message": "ip not found"})
		return
	}

	found, err := h.repo.RecordVisit(userID, ipID, time.Now().UTC())
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "IP_VISIT_RECORD_FAILED", err)
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": "IP_NOT_FOUND", "message": "ip not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

type ipVisitMergeInput struct {
	Visits *[]struct {
		IPID      int64  `json:"ip_id"`
		VisitedAt string `json:"visited_at"`
	} `json:"visits"`
}

// MergeVisits upserts up to six local anonymous visits into the signed-in
// account's history. The whole payload is validated before anything is
// written; duplicate ip_ids fold to their newest time, client-future times
// clamp to the server receive time, and unavailable IPs are reported in
// discarded_ip_ids instead of blocking the merge.
func (h *IPVisitHistoryHandler) MergeVisits(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var input ipVisitMergeInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Visits == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IP_VISIT_MERGE", "message": "invalid ip visit merge payload"})
		return
	}
	raw := *input.Visits
	if len(raw) > repository.RecentIPVisitLimit {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IP_VISIT_MERGE", "message": "invalid ip visit merge payload"})
		return
	}

	now := time.Now().UTC()
	latestByIP := make(map[int64]time.Time, len(raw))
	order := make([]int64, 0, len(raw))
	for _, v := range raw {
		if v.IPID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IP_VISIT_MERGE", "message": "invalid ip visit merge payload"})
			return
		}
		ts, err := time.Parse(time.RFC3339Nano, v.VisitedAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_IP_VISIT_MERGE", "message": "invalid ip visit merge payload"})
			return
		}
		if existing, ok := latestByIP[v.IPID]; !ok || ts.After(existing) {
			if !ok {
				order = append(order, v.IPID)
			}
			latestByIP[v.IPID] = ts
		}
	}

	visits := make([]model.IPVisitHistory, 0, len(order))
	for _, ipID := range order {
		ts := latestByIP[ipID]
		if ts.After(now) {
			ts = now
		}
		visits = append(visits, model.IPVisitHistory{UserID: userID, IPID: ipID, VisitedAt: ts})
	}

	accepted, discarded, items, err := h.repo.MergeVisits(userID, visits)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "IP_VISIT_MERGE_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accepted_ip_ids":  accepted,
		"discarded_ip_ids": discarded,
		"items":            toIPVisitResponse(h.displaySigner, items),
	})
}

func toIPVisitResponse(signer *service.DisplayURLSigner, items []repository.IPVisitHistoryItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"ip": model.IP{
				ID:          item.IPID,
				Name:        item.IPName,
				Slug:        item.IPSlug,
				Description: item.IPDescription,
				CoverURL:    signer.SignURL(item.IPCoverURL),
				Category:    item.IPCategory,
			},
			"visited_at": item.VisitedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return out
}
