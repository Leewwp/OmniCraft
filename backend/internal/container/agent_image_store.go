package container

import (
	"context"
	"io"
	"time"

	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/service"
)

// ossAgentImageStore adapts the aliyun OSS client to the generate_image
// storage seam: uploads land under agent-images/<conversation>/<id>.png and
// the tool only ever receives signed GET URLs on the platform's own domain.
type ossAgentImageStore struct {
	client *aliyun.OSSClient
}

func (s *ossAgentImageStore) PutAgentImage(ctx context.Context, key string, r io.Reader) error {
	return s.client.PutObject(key, r)
}

func (s *ossAgentImageStore) SignedAgentImageURL(ctx context.Context, key string) (string, error) {
	return s.client.GetSignedURL(key, "GET", 24*time.Hour)
}

var _ service.AgentImageStore = (*ossAgentImageStore)(nil)
