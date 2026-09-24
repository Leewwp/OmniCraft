package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// IPProposalHandler exposes the collaborative-governance proposal domain
// (#290). All routes hang under /ips/:id/proposals so the ip id is implicit.
type IPProposalHandler struct {
	svc *service.IPProposalService
}

func NewIPProposalHandler(svc *service.IPProposalService) *IPProposalHandler {
	return &IPProposalHandler{svc: svc}
}

func (h *IPProposalHandler) ListProposals(c *gin.Context) {
	ipID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid ip id")
		return
	}
	status := c.Query("status") // open | adopted | rejected | all | "" (history)
	query := c.Query("q")       // IP 内搜索（#290）：description_change 包含匹配
	page, pageSize := pageQuery(c, 20)
	// #668：标准列表分页迁 pageQuery（缺省/无效/越界语义见 pagination.go）。
	viewerID := middleware.GetUserID(c)

	views, total, err := h.svc.ListProposals(c.Request.Context(), ipID, status, query, page, pageSize, viewerID)
	if err != nil {
		if err == service.ErrIPNotFound {
			response.Error(c, http.StatusNotFound, "IP_NOT_FOUND", "ip not found")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	minVotes, passThreshold := h.svc.GovernanceDisplay()
	c.JSON(http.StatusOK, gin.H{
		"proposals":      views,
		"total":          total,
		"min_votes":      minVotes,
		"pass_threshold": passThreshold,
		"page":           page,
		"page_size":      pageSize,
	})
}

func (h *IPProposalHandler) GetProposal(c *gin.Context) {
	proposalID, err := strconv.ParseInt(c.Param("proposalId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid proposal id")
		return
	}
	view, err := h.svc.GetProposal(c.Request.Context(), proposalID, middleware.GetUserID(c))
	if err != nil {
		h.mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"proposal": view})
}

func (h *IPProposalHandler) CreateProposal(c *gin.Context) {
	ipID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid ip id")
		return
	}
	var input service.CreateIPProposalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid proposal payload")
		return
	}
	proposal, err := h.svc.CreateProposal(c.Request.Context(), ipID, middleware.GetUserID(c), input)
	if err != nil {
		h.mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"proposal": proposal})
}

func (h *IPProposalHandler) SubmitVote(c *gin.Context) {
	proposalID, err := strconv.ParseInt(c.Param("proposalId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid proposal id")
		return
	}
	var body struct {
		Vote string `json:"vote"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (body.Vote != "yes" && body.Vote != "no") {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "vote must be yes or no")
		return
	}
	if err := h.svc.SubmitVote(c.Request.Context(), proposalID, middleware.GetUserID(c), body.Vote); err != nil {
		h.mapError(c, err)
		return
	}
	view, err := h.svc.GetProposal(c.Request.Context(), proposalID, middleware.GetUserID(c))
	if err != nil {
		h.mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"proposal": view})
}

func (h *IPProposalHandler) ListVersions(c *gin.Context) {
	ipID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid ip id")
		return
	}
	versions, err := h.svc.ListVersions(c.Request.Context(), ipID, middleware.GetUserID(c))
	if err != nil {
		if err == service.ErrIPNotFound {
			response.Error(c, http.StatusNotFound, "IP_NOT_FOUND", "ip not found")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *IPProposalHandler) mapError(c *gin.Context, err error) {
	switch err {
	case service.ErrProposalNotFound:
		response.Error(c, http.StatusNotFound, "PROPOSAL_NOT_FOUND", "proposal not found")
	case service.ErrIPNotFound:
		// #446：提案随 IP 可见性走——所属 IP 不可见时按 IP 不存在回应。
		response.Error(c, http.StatusNotFound, "IP_NOT_FOUND", "ip not found")
	case service.ErrProposalNotEligible:
		response.Error(c, http.StatusForbidden, "PROPOSAL_NOT_ELIGIBLE", "reputation too low, or not following the ip (voting requires following)")
	case service.ErrProposalOpenExists:
		response.Error(c, http.StatusConflict, "PROPOSAL_OPEN_EXISTS", "an open proposal already exists for this ip")
	case service.ErrProposalAlreadyVoted:
		response.Error(c, http.StatusConflict, "PROPOSAL_ALREADY_VOTED", "already voted on this proposal")
	case service.ErrProposalClosed:
		response.Error(c, http.StatusConflict, "PROPOSAL_CLOSED", "proposal is closed")
	case service.ErrProposalTagConflict:
		response.Error(c, http.StatusBadRequest, "PROPOSAL_TAG_CONFLICT", "tag change conflicts with current ip tags")
	case service.ErrProposalEmpty:
		response.Error(c, http.StatusBadRequest, "PROPOSAL_EMPTY", "proposal must change at least one field")
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}
