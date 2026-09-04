package router

import (
	"strings"
	"testing"

	"omnicraft/backend/internal/middleware"
)

func TestInteractionPolicyArchetypes(t *testing.T) {
	standard := standardVerifiedInteractionPolicy()
	wantStandard := middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}
	if standard != wantStandard {
		t.Fatalf("standardVerifiedInteractionPolicy() = %+v, want %+v", standard, wantStandard)
	}

	publishing := publishingInteractionPolicy()
	wantPublishing := middleware.InteractionPolicy{
		RequireVerifiedEmail:   true,
		RequireReputation:      true,
		RequireNoPublishFreeze: true,
	}
	if publishing != wantPublishing {
		t.Fatalf("publishingInteractionPolicy() = %+v, want %+v", publishing, wantPublishing)
	}
}

func TestRoutePolicyAttachmentsPreserveOperationSpecificGuards(t *testing.T) {
	source := readRoutesSource(t)

	guardPolicies := map[string]string{
		"publishGuard":    "publishingInteractionPolicy()",
		"editDeleteGuard": "standardVerifiedInteractionPolicy()",
		"commentsGuard":   "standardVerifiedInteractionPolicy()",
		"reactionsGuard":  "standardVerifiedInteractionPolicy()",
		"collectionGuard": "standardVerifiedInteractionPolicy()",
		"reportsGuard":    "standardVerifiedInteractionPolicy()",
		"prGuard":         "standardVerifiedInteractionPolicy()",
		"judgeGuard":      "standardVerifiedInteractionPolicy()",
		"followsGuard":    "standardVerifiedInteractionPolicy()",
		"messagesGuard":   "standardVerifiedInteractionPolicy()",
		"downloadsGuard":  "standardVerifiedInteractionPolicy()",
		"agentGuard":      "standardVerifiedInteractionPolicy()",
	}
	for guard, policy := range guardPolicies {
		attachment := guard + " := middleware.InteractionRequired(cfg, db, rdb, " + policy + ")"
		if !strings.Contains(source, attachment) {
			t.Errorf("route composition missing guard policy attachment %q", attachment)
		}
	}

	contracts := []string{
		`contents.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)`,
		// T15 (F-103): IP creation enters the review queue and is public-facing
		// free text, so it carries the same publishing guard + upload rate
		// limit as content publishing.
		`ips.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), ipHandler.CreateIP)`,
		`contents.PATCH("/:id", authReq, editDeleteGuard, contentHandler.UpdateContent)`,
		`social.PATCH("/comments/:id", authReq, commentsGuard, middleware.CommentEditRateLimit(rdb), socialHandler.EditComment)`,
		`v1.POST("/collections", authReq, collectionGuard, collectionHandler.CreateCollection)`,
		`messages := v1.Group("/messages", authReq)`,
		`messages.POST("", messagesGuard, msgHandler.SendMessage)`,
		// Task 3: Agent generation quota is reserved inside each
		// Provider-consuming handler right before the first Provider call
		// (feature/schema/visibility rejections precede it and never consume
		// quota). The agent group mounts no quota middleware, so conversation
		// history and deletion routes stay outside every quota path.
		`agent := v1.Group("/agent", authReq, agentGuard)`,
	}
	for _, contract := range contracts {
		if !strings.Contains(source, contract) {
			t.Errorf("route composition missing operation-specific attachment %q", contract)
		}
	}

	if strings.Contains(source, "middleware.InteractionPolicy{") {
		t.Fatal("routes.go must use named policy archetypes instead of repeating anonymous policy literals")
	}
}
