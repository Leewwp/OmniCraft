package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"omnicraft/backend/internal/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// AdminEvalHandler serves the evaluation trend + badcase feedback surface
// (SP-22 E5): eval_runs history for the /admin/evals trend table and the
// trace -> golden-draft feedback queue. Drafts never enter the frozen set
// from here; freezing stays a deliberate curation step (golden set seeding
// flow) because a frozen-set change shifts the dataset checksum every PR
// gate snapshot depends on.
type AdminEvalHandler struct {
	db        *gorm.DB
	evalRepo  *repository.RagEvaluationRepository
	traceRepo *repository.AgentTraceRepository
	auditSvc  AdminRAGAuditRecorder
}

func NewAdminEvalHandler(
	db *gorm.DB,
	evalRepo *repository.RagEvaluationRepository,
	traceRepo *repository.AgentTraceRepository,
	auditSvc AdminRAGAuditRecorder,
) *AdminEvalHandler {
	return &AdminEvalHandler{db: db, evalRepo: evalRepo, traceRepo: traceRepo, auditSvc: auditSvc}
}

// ListRuns returns eval_runs newest first. The trend page derives headline
// metric columns from metrics.retrieval_headline when present; older
// baseline rows pass through unchanged.
func (h *AdminEvalHandler) ListRuns(c *gin.Context) {
	runs, err := h.evalRepo.ListEvalRuns(c.Request.Context(), 100)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to load eval runs")
		return
	}
	if runs == nil {
		runs = []model.EvalRun{}
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// ListDrafts returns the golden-draft feedback queue (newest first).
func (h *AdminEvalHandler) ListDrafts(c *gin.Context) {
	drafts, err := h.evalRepo.ListGoldenDrafts(c.Request.Context(), 100)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to load golden drafts")
		return
	}
	if drafts == nil {
		drafts = []model.EvalGoldenCase{}
	}
	c.JSON(http.StatusOK, gin.H{"drafts": drafts})
}

type createDraftInput struct {
	TraceID string `json:"trace_id" binding:"required"`
}

type traceMessageRow struct {
	ID   int64   `json:"id"`
	Role string  `json:"role"`
	Body *string `json:"body"`
}

// CreateDraftFromTrace turns one agent trace into a golden-set draft: the
// traced turn's question, answer and validated citations land as a draft
// row (status='draft') the curator reviews before anything freezes. The
// same trace fed twice is idempotent — the first draft is kept and returned.
func (h *AdminEvalHandler) CreateDraftFromTrace(c *gin.Context) {
	var in createDraftInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ARGS", "trace_id is required")
		return
	}
	traceID := strings.TrimSpace(in.TraceID)
	if len(traceID) < 8 || len(traceID) > 64 {
		response.Error(c, http.StatusBadRequest, "INVALID_ARGS", "invalid trace id")
		return
	}

	run, err := h.traceRepo.GetRunByTraceID(c.Request.Context(), traceID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "TRACE_NOT_FOUND", "trace run not found")
		return
	}
	if run.ConversationID == nil || run.MessageID == nil {
		response.Error(c, http.StatusUnprocessableEntity, "TRACE_NOT_LINKED", "trace has no conversation/message link")
		return
	}

	// The answer row (message_id) carries the validated citations; the
	// question is the closest preceding user message in the conversation.
	var answerMsg model.AgentMessage
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND conversation_id = ?", *run.MessageID, *run.ConversationID).
		First(&answerMsg).Error; err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "TRACE_ANSWER_MISSING", "answer message not found for trace")
		return
	}
	var questionMsg model.AgentMessage
	if err := h.db.WithContext(c.Request.Context()).
		Where("conversation_id = ? AND role = ? AND id < ?", *run.ConversationID, "user", answerMsg.ID).
		Order("id DESC").First(&questionMsg).Error; err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "TRACE_QUESTION_MISSING", "no user question before the traced answer")
		return
	}
	question := strings.TrimSpace(derefString(questionMsg.Content))
	if question == "" {
		response.Error(c, http.StatusUnprocessableEntity, "TRACE_QUESTION_MISSING", "traced user message is empty")
		return
	}

	caseKey := "draft-trace-" + traceID
	existing, err := h.evalRepo.GetGoldenCaseByKey(c.Request.Context(), caseKey)
	if err == nil && existing != nil {
		c.JSON(http.StatusOK, gin.H{"case_key": existing.CaseKey, "created": false, "draft": existing})
		return
	}
	if err != nil && !errors.Is(err, repository.ErrRagEvalNotFound) {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to check existing draft")
		return
	}

	citations := make([]map[string]any, 0, len(answerMsg.Citations))
	contentIDs := map[int64]bool{}
	for _, cit := range answerMsg.Citations {
		citations = append(citations, map[string]any{
			"content_id": cit.ContentID, "content_version": cit.ContentVersion,
			"chunk_key": cit.ChunkKey, "chunk_index": cit.ChunkIndex, "title": cit.Title,
		})
		contentIDs[cit.ContentID] = true
	}
	ids := make([]int64, 0, len(contentIDs))
	for id := range contentIDs {
		ids = append(ids, id)
	}
	viewerCtx, _ := json.Marshal(map[string]string{"principal_key": "registered"})
	expectedCitations, _ := json.Marshal(citations)
	relevantIDs, _ := json.Marshal(ids)
	rubric, _ := json.Marshal(map[string]string{
		"note": "draft from trace - curate assertions/tiers before freezing",
	})
	classification, _ := json.Marshal(map[string]string{
		"primary_layer": "draft", "split": "",
		"note": "trace feedback draft (SP-22 E5); assign layer on curation",
	})

	draft := model.EvalGoldenCase{
		CaseKey:             caseKey,
		SchemaVersion:       2,
		Query:               question,
		QueryLanguage:       "zh",
		ViewerContext:       model.JSONB(viewerCtx),
		RelevantContentIDs:  model.JSONB(relevantIDs),
		ExpectedCitations:   model.JSONB(expectedCitations),
		ForbiddenContentIDs: model.JSONB("[]"),
		AnswerRubric:        model.JSONB(rubric),
		Classification:      model.JSONB(classification),
		IsActive:            true,
		Status:              "draft",
		SourceTraceID:       traceID,
	}
	if err := h.evalRepo.CreateGoldenDraft(c.Request.Context(), &draft); err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to create golden draft")
		return
	}
	adminID := middleware.GetUserID(c)
	if h.auditSvc != nil {
		_ = h.auditSvc.Record(c.Request.Context(), service.RecordAdminAuditInput{
			AdminUserID: adminID,
			Action:      "eval.golden_draft.create",
			TargetType:  "eval_golden_case",
			TargetID:    caseKey,
			Metadata:    map[string]interface{}{"trace_id": traceID, "citations": len(citations)},
			Result:      "success",
		})
	}
	created, err := h.evalRepo.GetGoldenCaseByKey(c.Request.Context(), caseKey)
	if err != nil {
		created = &draft
	}
	c.JSON(http.StatusCreated, gin.H{"case_key": caseKey, "created": true, "draft": created})
}

// DeleteDraft removes a draft from the feedback queue. Frozen cases are a
// 404 by design (they are never deletable from this surface).
func (h *AdminEvalHandler) DeleteDraft(c *gin.Context) {
	caseKey := strings.TrimSpace(c.Param("case_key"))
	if caseKey == "" || len(caseKey) > 128 {
		response.Error(c, http.StatusBadRequest, "INVALID_ARGS", "invalid case key")
		return
	}
	if err := h.evalRepo.DeleteGoldenDraft(c.Request.Context(), caseKey); err != nil {
		if errors.Is(err, repository.ErrRagEvalNotFound) {
			response.Error(c, http.StatusNotFound, "DRAFT_NOT_FOUND", "golden draft not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to delete golden draft")
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Record(c.Request.Context(), service.RecordAdminAuditInput{
			AdminUserID: middleware.GetUserID(c),
			Action:      "eval.golden_draft.delete",
			TargetType:  "eval_golden_case",
			TargetID:    caseKey,
			Result:      "success",
		})
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
