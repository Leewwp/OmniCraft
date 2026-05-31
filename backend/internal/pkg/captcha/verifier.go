package captcha

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"omnicraft/backend/config"
)

type CaptchaVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

type BypassVerifier struct{}

func NewBypassVerifier() *BypassVerifier {
	return &BypassVerifier{}
}

func (v *BypassVerifier) Verify(_ context.Context, _, _ string) error {
	return nil
}

type AliyunV2Verifier struct {
	accessKeyID     string
	accessKeySecret string
	sceneID         string
	prefix          string
	region          string
	httpClient      *http.Client
}

func NewAliyunV2Verifier(cfg config.CaptchaConfig) *AliyunV2Verifier {
	region := cfg.Region
	if region == "" {
		region = "cn"
	}
	return &AliyunV2Verifier{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		sceneID:         cfg.SceneID,
		prefix:          cfg.Prefix,
		region:          region,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *AliyunV2Verifier) Verify(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return fmt.Errorf("captcha token is required")
	}

	endpoint := fmt.Sprintf("https://captcha.%s.aliyuncs.com/api/captcha/verify", v.region)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create captcha verify request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-action", "VerifyCaptcha")

	q := req.URL.Query()
	q.Set("CaptchaType", "slide")
	q.Set("Scene", v.sceneID)
	q.Set("Token", token)
	if remoteIP != "" {
		q.Set("RemoteIp", remoteIP)
	}
	req.URL.RawQuery = q.Encode()

	slog.Debug("captcha verify request", "scene", v.sceneID, "remote_ip", remoteIP)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("captcha verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("captcha verify returned status %d", resp.StatusCode)
	}

	return nil
}

func NewCaptchaVerifier(cfg config.CaptchaConfig) CaptchaVerifier {
	switch cfg.Provider {
	case "aliyun_v2":
		return NewAliyunV2Verifier(cfg)
	case "bypass", "":
		return NewBypassVerifier()
	default:
		slog.Warn("unknown captcha provider, falling back to bypass", "provider", cfg.Provider)
		return NewBypassVerifier()
	}
}
