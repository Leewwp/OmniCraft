package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/service"
)

type ReviewWorker struct {
	reviewSvc *service.ReviewService
	rdb       interface {
		IdempotentCheck(ctx context.Context, topic, msgID string) (bool, error)
	}
}

func NewReviewWorker(reviewSvc *service.ReviewService) *ReviewWorker {
	return &ReviewWorker{
		reviewSvc: reviewSvc,
	}
}

func (w *ReviewWorker) Handle(ctx context.Context, msg queue.Message) error {
	var payload struct {
		TargetType    string                 `json:"target_type"`
		TargetID      int64                  `json:"target_id"`
		ContentType   string                 `json:"content_type,omitempty"`
		Title         string                 `json:"title,omitempty"`
		Description   string                 `json:"description,omitempty"`
		AuthorID      int64                  `json:"author_id,omitempty"`
		CoverImageURL string                 `json:"cover_image_url,omitempty"`
		Result        string                 `json:"result,omitempty"`
		RawResponse   map[string]interface{} `json:"raw_response,omitempty"`
		Action        string                 `json:"action"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Error("review_worker: failed to unmarshal payload", "msg_id", msg.ID, "error", err)
		return err
	}

	slog.Info("review_worker: processing message",
		"msg_id", msg.ID, "action", payload.Action,
		"target_type", payload.TargetType, "target_id", payload.TargetID)

	switch payload.Action {
	case "submit_ai_review":
		input := service.SubmitReviewInput{
			TargetType:    payload.TargetType,
			TargetID:      payload.TargetID,
			ContentType:   payload.ContentType,
			Title:         payload.Title,
			Description:   payload.Description,
			AuthorID:      payload.AuthorID,
			CoverImageURL: payload.CoverImageURL,
		}
		if err := w.reviewSvc.SubmitForAIReview(ctx, input); err != nil {
			slog.Error("review_worker: SubmitForAIReview failed",
				"target_type", payload.TargetType, "target_id", payload.TargetID, "error", err)
			return err
		}
		slog.Info("review_worker: SubmitForAIReview completed",
			"target_type", payload.TargetType, "target_id", payload.TargetID)

	case "process_ai_callback":
		input := service.AICallbackInput{
			TargetType:  payload.TargetType,
			TargetID:    payload.TargetID,
			Result:      payload.Result,
			RawResponse: payload.RawResponse,
		}
		if err := w.reviewSvc.ProcessAICallback(ctx, input); err != nil {
			slog.Error("review_worker: ProcessAICallback failed",
				"target_type", payload.TargetType, "target_id", payload.TargetID, "error", err)
			return err
		}
		slog.Info("review_worker: ProcessAICallback completed",
			"target_type", payload.TargetType, "target_id", payload.TargetID)

	default:
		slog.Warn("review_worker: unknown action", "action", payload.Action, "msg_id", msg.ID)
	}

	return nil
}
