package config

import (
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
