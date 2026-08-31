package service

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

// B-002: display URL signing unit contract. The signer re-issues short-lived
// GET URLs for platform OSS objects at the API serialization boundary, passes
// everything else through untouched, and fails open when OSS is not
// configured. It reuses aliyun.GetSignedURL / aliyun.ObjectKeyFromURL instead
// of reimplementing URL signing.

const (
	displayTestEndpoint = "http://127.0.0.1:9201"
	displayTestBucket   = "test-bucket"
	displayTestDomain   = "http://127.0.0.1:9201/test-bucket"
)

func displayTestConfig(ttlSec int) *config.Config {
	cfg := &config.Config{}
	cfg.OSS = config.OSSConfig{
		Endpoint:        displayTestEndpoint,
		AccessKeyID:     "test-access-key",
		AccessKeySecret: "test-access-secret",
		BucketName:      displayTestBucket,
		Domain:          displayTestDomain,
		DisplayURLTTL:   ttlSec,
	}
	return cfg
}

func signedExpires(t *testing.T, raw string) int64 {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	expires, err := strconv.ParseInt(parsed.Query().Get("Expires"), 10, 64)
	require.NoError(t, err, "Expires must be a unix timestamp")
	return expires
}

func TestDisplayURLSignerSignsPlatformURLsOnly(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))

	platform := aliyun.ObjectURL(displayTestDomain, "uploads/7/image/cover.png")
	signed := signer.SignURL(platform)
	require.NotEqual(t, platform, signed)
	requireSignedShape(t, signed, "uploads/7/image/cover.png")

	require.Equal(t, "https://external.example.net/x.png",
		signer.SignURL("https://external.example.net/x.png"),
		"external URL must pass through unsigned")
	require.Equal(t, "/seed-media/covers/local.svg",
		signer.SignURL("/seed-media/covers/local.svg"),
		"relative seed URL must pass through unsigned")
	require.Equal(t, "", signer.SignURL(""), "empty URL must stay empty")
}

func TestDisplayURLSignerUsesConfiguredTTLOrDefault(t *testing.T) {
	before := time.Now().Unix()

	configured := NewDisplayURLSigner(displayTestConfig(120)).SignURL(
		aliyun.ObjectURL(displayTestDomain, "uploads/7/image/ttl.png"))
	require.InDelta(t, float64(before+120), float64(signedExpires(t, configured)), 5,
		"Expires must reflect oss.display_url_ttl_sec")

	// Zero/negative config falls back to the architecture default of 1h,
	// which also stays above the 300s Redis display cache TTL so a cached
	// entry never outlives its re-issued signature.
	fallback := NewDisplayURLSigner(displayTestConfig(0)).SignURL(
		aliyun.ObjectURL(displayTestDomain, "uploads/7/image/ttl-default.png"))
	require.InDelta(t, float64(before+3600), float64(signedExpires(t, fallback)), 5,
		"Expires must default to 3600s")
}

func TestDisplayURLSignerFailsOpenWithoutOSS(t *testing.T) {
	var nilSigner *DisplayURLSigner
	const url = "https://cdn.example.test/uploads/7/image/x.png"
	require.Equal(t, url, nilSigner.SignURL(url), "nil signer must pass through")

	unconfigured := NewDisplayURLSigner(&config.Config{})
	require.Equal(t, url, unconfigured.SignURL(url),
		"OSS not configured must fail open to the bare URL")
}

func TestDisplayURLSignerDecoratesModels(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))

	ip := model.IP{
		Name:     "ip",
		CoverURL: aliyun.ObjectURL(displayTestDomain, "uploads/1/image/ip.png"),
		Creator: &model.User{
			AvatarURL: aliyun.ObjectURL(displayTestDomain, "uploads/1/avatar/u.png"),
		},
	}
	signer.DecorateIP(&ip)
	requireSignedShape(t, ip.CoverURL, "uploads/1/image/ip.png")
	requireSignedShape(t, ip.Creator.AvatarURL, "uploads/1/avatar/u.png")

	ips := []model.IP{{CoverURL: aliyun.ObjectURL(displayTestDomain, "uploads/1/image/ip2.png")}}
	signer.DecorateIPs(ips)
	requireSignedShape(t, ips[0].CoverURL, "uploads/1/image/ip2.png")

	user := model.User{AvatarURL: aliyun.ObjectURL(displayTestDomain, "uploads/1/avatar/v.png")}
	signer.DecorateUser(&user)
	requireSignedShape(t, user.AvatarURL, "uploads/1/avatar/v.png")

	content := model.ContentItem{
		CoverImageURL: aliyun.ObjectURL(displayTestDomain, "uploads/2/image/c.png"),
		Author: model.User{
			AvatarURL: aliyun.ObjectURL(displayTestDomain, "uploads/2/avatar/a.png"),
		},
		IP: &model.IP{
			CoverURL: aliyun.ObjectURL(displayTestDomain, "uploads/2/image/ip.png"),
		},
		SourceOriginal: &model.ContentItem{
			CoverImageURL: aliyun.ObjectURL(displayTestDomain, "uploads/2/image/src.png"),
		},
	}
	signer.DecorateContent(&content)
	requireSignedShape(t, content.CoverImageURL, "uploads/2/image/c.png")
	requireSignedShape(t, content.Author.AvatarURL, "uploads/2/avatar/a.png")
	requireSignedShape(t, content.IP.CoverURL, "uploads/2/image/ip.png")
	requireSignedShape(t, content.SourceOriginal.CoverImageURL, "uploads/2/image/src.png")

	contents := []model.ContentItem{{CoverImageURL: aliyun.ObjectURL(displayTestDomain, "uploads/2/image/c2.png")}}
	signer.DecorateContents(contents)
	requireSignedShape(t, contents[0].CoverImageURL, "uploads/2/image/c2.png")
}

func TestDisplayURLSignerDecoratesAttachmentsWithSignedOSSURL(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))
	attachments := []model.ContentAttachment{
		{OSSKey: "uploads/3/image/gallery-01.png"},
		{OSSKey: "uploads/3/image/gallery-02.png"},
	}
	signer.DecorateAttachments(attachments)

	require.Equal(t, "uploads/3/image/gallery-01.png", attachments[0].OSSKey, "oss_key must stay canonical")
	requireSignedShape(t, attachments[0].OSSURL, "uploads/3/image/gallery-01.png")
	requireSignedShape(t, attachments[1].OSSURL, "uploads/3/image/gallery-02.png")

	// Without a delivery domain there is no canonical display URL to sign, so
	// the transient oss_url stays absent instead of leaking a bare oss_key.
	cfg := displayTestConfig(0)
	cfg.OSS.Domain = ""
	domainless := NewDisplayURLSigner(cfg)
	attachments = []model.ContentAttachment{{OSSKey: "uploads/3/image/x.png"}}
	domainless.DecorateAttachments(attachments)
	require.Equal(t, "", attachments[0].OSSURL)
	require.Equal(t, "uploads/3/image/x.png", attachments[0].OSSKey)
}

func TestDisplayURLSignerDecorationKeepsExternalAndEmptyURLsUntouched(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))
	const external = "https://external.example.net/a.png"

	content := model.ContentItem{CoverImageURL: external}
	signer.DecorateContent(&content)
	require.Equal(t, external, content.CoverImageURL)

	ip := model.IP{CoverURL: ""}
	signer.DecorateIP(&ip)
	require.Equal(t, "", ip.CoverURL)

	user := model.User{AvatarURL: "/seed-media/avatars/default.svg"}
	signer.DecorateUser(&user)
	require.Equal(t, "/seed-media/avatars/default.svg", user.AvatarURL)

	// Nil-signer decoration must be a safe no-op for every model shape.
	var nilSigner *DisplayURLSigner
	nilSigner.DecorateContent(&content)
	nilSigner.DecorateIP(&ip)
	nilSigner.DecorateUser(&user)
	nilSigner.DecorateAttachments(nil)
	nilSigner.DecorateContents(nil)
	nilSigner.DecorateIPs(nil)
	require.Equal(t, external, content.CoverImageURL)
}

func requireSignedShape(t *testing.T, raw, wantKey string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	query := parsed.Query()
	require.NotEmpty(t, query.Get("Signature"), "missing Signature")
	require.NotEmpty(t, query.Get("OSSAccessKeyId"), "missing OSSAccessKeyId")
	require.NotEmpty(t, query.Get("Expires"), "missing Expires")
	// The IP-shaped test endpoint makes the SDK sign path-style URLs
	// (/bucket/key); the object key must survive signing untouched.
	require.Equal(t, "/"+displayTestBucket+"/"+wantKey, parsed.Path, "object key drifted")
}
