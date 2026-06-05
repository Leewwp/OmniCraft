package aliyun

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSTSTokenJSONDoesNotExposeAccessKeySecret(t *testing.T) {
	token := STSToken{
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		SecurityToken:   "security-token",
		Expiration:      "2026-06-05T00:00:00Z",
	}

	payload, err := json.Marshal(token)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "access_key_secret")
	require.NotContains(t, string(payload), "ak-secret")
}
