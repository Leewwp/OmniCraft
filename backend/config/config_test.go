package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateReleaseRejectsBypassCaptchaAndLoggerSMTP(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "bypass"
	cfg.SMTP.Mode = "logger"

	require.Error(t, cfg.ValidateRelease())
}

func TestValidateReleaseRejectsDefaultJWTSecret(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "aliyun_v2"
	cfg.SMTP.Mode = "smtp"
	cfg.JWT.Secret = "dev-secret-change-in-production"

	require.ErrorContains(t, cfg.ValidateRelease(), "jwt.secret")
}

func TestDefaultConfigDeclaresCreatorSupportDisabled(t *testing.T) {
	raw, err := os.ReadFile("../config.yaml")
	require.NoError(t, err)
	require.Contains(t, strings.ReplaceAll(string(raw), "\t", "  "), "creator_support_enabled: false")
}
