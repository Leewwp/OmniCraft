package aliyun

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

// #813 (run-1 审计 #2): the principal-bound gate is the authorization-grade
// check for user-submitted platform object URLs that get persisted and
// re-signed for every viewer — the object key must live under the subject's
// own uploads namespace and must never be a quarantine object. The
// domain-only IsPlatformObjectURL stays reserved for non-authorization
// semantics (e.g. the avatar audit tool scanning legacy rows).
func TestIsPlatformObjectURLForPrincipal(t *testing.T) {
	const domain = "https://cdn.example.test"

	// Own namespace passes.
	require.True(t, IsPlatformObjectURLForPrincipal(domain,
		"https://cdn.example.test/uploads/7/avatar/2026/08/13/a.png", 7))
	require.True(t, IsPlatformObjectURLForPrincipal(domain,
		"https://cdn.example.test/uploads/7/image/cover.png?x=1", 7))

	// Cross-user namespace is rejected.
	require.False(t, IsPlatformObjectURLForPrincipal(domain,
		"https://cdn.example.test/uploads/8/avatar/a.png", 7), "cross-user uploads key must be rejected")

	// Sibling prefix is not the caller's namespace: uploads/71 must not
	// satisfy uploads/7 (the trailing slash in the required prefix is
	// load-bearing).
	require.False(t, IsPlatformObjectURLForPrincipal(domain,
		"https://cdn.example.test/uploads/71/avatar/a.png", 7))

	// Quarantine objects are never a delivery target, for any principal.
	require.False(t, IsPlatformObjectURLForPrincipal(domain,
		"https://cdn.example.test/quarantine/archive-scan/9/2/job11", 7))

	// External domain / no domain / bare key forms are rejected.
	require.False(t, IsPlatformObjectURLForPrincipal(domain,
		"https://evil.example.test/uploads/7/a.png", 7))
	require.False(t, IsPlatformObjectURLForPrincipal("",
		"https://cdn.example.test/uploads/7/a.png", 7))
	require.False(t, IsPlatformObjectURLForPrincipal(domain, "uploads/7/a.png", 7))
}
