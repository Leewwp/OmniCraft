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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	status := c.Query("status") // open | adopted | rejected | all | "" (history)
	query := c.Query("q")       // IP 内搜索（#290）：description_change 包含匹配
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	viewerID := middleware.GetUserID(c)

	views, total, err := h.svc.ListProposals(c.Request.Context(), ipID, status, query, page, pageSize, viewerID)
	if err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid proposal id"})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	var input service.CreateIPProposalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": "invalid proposal payload"})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid proposal id"})
		return
	}
	var body struct {
		Vote string `json:"vote"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (body.Vote != "yes" && body.Vote != "no") {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": "vote must be yes or no"})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	versions, err := h.svc.ListVersions(c.Request.Context(), ipID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *IPProposalHandler) mapError(c *gin.Context, err error) {
	switch err {
	case service.ErrProposalNotFound:
		c.JSON(http.StatusNotFound, gin.H{"code": "PROPOSAL_NOT_FOUND", "message": "proposal not found"})
	case service.ErrProposalNotEligible:
		c.JSON(http.StatusForbidden, gin.H{"code": "PROPOSAL_NOT_ELIGIBLE", "message": "reputation too low, or not following the ip (voting requires following)"})
	case service.ErrProposalOpenExists:
		c.JSON(http.StatusConflict, gin.H{"code": "PROPOSAL_OPEN_EXISTS", "message": "an open proposal already exists for this ip"})
	case service.ErrProposalAlreadyVoted:
		c.JSON(http.StatusConflict, gin.H{"code": "PROPOSAL_ALREADY_VOTED", "message": "already voted on this proposal"})
	case service.ErrProposalClosed:
		c.JSON(http.StatusConflict, gin.H{"code": "PROPOSAL_CLOSED", "message": "proposal is closed"})
	case service.ErrProposalTagConflict:
		c.JSON(http.StatusBadRequest, gin.H{"code": "PROPOSAL_TAG_CONFLICT", "message": "tag change conflicts with current ip tags"})
	case service.ErrProposalEmpty:
		c.JSON(http.StatusBadRequest, gin.H{"code": "PROPOSAL_EMPTY", "message": "proposal must change at least one field"})
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}
