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

func TestObjectKeyFromURL(t *testing.T) {
	const domain = "https://cdn.example.test"

	key, ok := ObjectKeyFromURL(domain, "https://cdn.example.test/probe/a b.png")
	require.True(t, ok)
	require.Equal(t, "probe/a b.png", key)

	key, ok = ObjectKeyFromURL(domain, "https://cdn.example.test/probe/x.png?Signature=abc&Expires=1")
	require.True(t, ok)
	require.Equal(t, "probe/x.png", key)

	for _, url := range []string{
		"https://evil.example.test/probe/x.png",
		"https://cdn.example.test/",
		"https://cdn.example.test",
		"",
	} {
		_, ok := ObjectKeyFromURL(domain, url)
		require.False(t, ok, "expected %q to be rejected", url)
	}

	_, ok = ObjectKeyFromURL("", "https://cdn.example.test/probe/x.png")
	require.False(t, ok)
}
