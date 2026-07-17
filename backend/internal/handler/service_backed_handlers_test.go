package handler

import (
	"testing"

	"omnicraft/backend/internal/service"
)

func TestServiceBackedHandlerConstructorsKeepContainerDependencies(t *testing.T) {
	prService := new(service.PRService)
	if got := NewPRHandlerWithService(prService); got.prSvc != prService {
		t.Fatal("PR handler must retain the container-owned service")
	}

	socialService := new(service.SocialService)
	if got := NewSocialHandlerWithService(socialService, nil); got.socialSvc != socialService {
		t.Fatal("social handler must retain the container-owned service")
	}
}
