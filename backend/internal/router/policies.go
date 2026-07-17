package router

import "omnicraft/backend/internal/middleware"

func standardVerifiedInteractionPolicy() middleware.InteractionPolicy {
	return middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	}
}

func publishingInteractionPolicy() middleware.InteractionPolicy {
	policy := standardVerifiedInteractionPolicy()
	policy.RequireNoPublishFreeze = true
	return policy
}
