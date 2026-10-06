package service

import (
	"context"
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

	// #428: expiry is bucket-aligned, so the URL never expires sooner than
	// the configured ttl and lands on the deterministic aligned second
	// (within the SDK's 1s truncation slop).
	configured := NewDisplayURLSigner(displayTestConfig(120)).SignURL(
		aliyun.ObjectURL(displayTestDomain, "uploads/7/image/ttl.png"))
	configuredAligned := alignDisplayExpiry(time.Unix(before, 0), 120*time.Second, displayURLSignatureBucketSec)
	require.InDelta(t, float64(configuredAligned), float64(signedExpires(t, configured)), 2,
		"Expires must be the bucket-aligned value for oss.display_url_ttl_sec")

	// Zero/negative config falls back to the architecture default of 1h,
	// which also stays above the 300s Redis display cache TTL so a cached
	// entry never outlives its re-issued signature.
	fallback := NewDisplayURLSigner(displayTestConfig(0)).SignURL(
		aliyun.ObjectURL(displayTestDomain, "uploads/7/image/ttl-default.png"))
	fallbackAligned := alignDisplayExpiry(time.Unix(before, 0), 3600*time.Second, displayURLSignatureBucketSec)
	require.InDelta(t, float64(fallbackAligned), float64(signedExpires(t, fallback)), 2,
		"Expires must be the bucket-aligned default (3600s budget)")
}

// #428: the signed URL must be byte-identical within one bucket window so
// /_next/image (cache key = full signed URL) hits across responses, and must
// rotate once the window advances.
func TestDisplayURLSignerStableWithinBucketAndRotatesAcross(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))
	platform := aliyun.ObjectURL(displayTestDomain, "uploads/9/image/bucket.png")

	first := signer.SignURL(platform)
	second := signer.SignURL(platform)
	require.Equal(t, first, second, "two serializations inside one bucket window must produce the identical signed URL")

	now := time.Now()
	inBucket := alignDisplayExpiry(now, time.Hour, displayURLSignatureBucketSec)
	nextBucket := alignDisplayExpiry(now.Add(time.Duration(displayURLSignatureBucketSec)*time.Second), time.Hour, displayURLSignatureBucketSec)
	require.NotEqual(t, inBucket, nextBucket, "advancing the clock one bucket must rotate the aligned expiry")
	require.Equal(t, int64(0), nextBucket%displayURLSignatureBucketSec, "aligned expiry must be epoch-aligned to the bucket")
}

func TestAlignDisplayExpiryBucketMath(t *testing.T) {
	const bucket = displayURLSignatureBucketSec
	base := time.Unix(3*bucket, 0) // exactly on a bucket boundary

	for _, tc := range []struct {
		offsetSec int64
		ttl       time.Duration
	}{
		{0, time.Hour},
		{1, time.Hour},
		{bucket - 1, time.Hour},
		{7, 2 * time.Hour},
	} {
		now := base.Add(time.Duration(tc.offsetSec) * time.Second)
		got := alignDisplayExpiry(now, tc.ttl, bucket)
		want := now.Unix() + int64(tc.ttl.Seconds())
		if rem := want % bucket; rem != 0 {
			want += bucket - rem
		}
		require.Equal(t, want, got, "offset=%d ttl=%v", tc.offsetSec, tc.ttl)
		require.GreaterOrEqual(t, got, now.Unix()+int64(tc.ttl.Seconds()),
			"ceil alignment must never shorten the validity below the ttl")
		require.Equal(t, int64(0), got%bucket)
	}

	// Degenerate bucket disables alignment (pure ttl semantics).
	require.Equal(t, base.Unix()+600, alignDisplayExpiry(base, 10*time.Minute, 0))
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
		{FileType: "image", OSSKey: "uploads/3/image/gallery-01.png"},
		{FileType: "image", OSSKey: "uploads/3/image/gallery-02.png"},
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
	attachments = []model.ContentAttachment{{FileType: "image", OSSKey: "uploads/3/image/x.png"}}
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

// #813（审计 #5 + 决策①）：附件展示签名的家族白名单——只有展示/预览族
// （image / video / sheet_music，SheetMusicViewer 页内消费）获得签名 URL；
// 文件交付族（mod / text / document / audio / model3d）一律 OSSURL=""，仅
// 经下载闸交付；quarantine/ 前缀在任何路径永不签名。
func TestAttachmentDisplayFamilyWhitelistAndQuarantine(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))

	previewable := []string{"image", "video", "sheet_music"}
	for _, family := range previewable {
		attachments := []model.ContentAttachment{{FileType: family, OSSKey: "uploads/3/" + family + "/a.bin"}}
		signer.DecorateAttachments(attachments)
		requireSignedShape(t, attachments[0].OSSURL, "uploads/3/"+family+"/a.bin")
		require.Equal(t, "uploads/3/"+family+"/a.bin", attachments[0].OSSKey, "oss_key must stay canonical")
	}

	for _, family := range []string{"mod", "text", "document", "audio", "model3d", ""} {
		attachments := []model.ContentAttachment{{FileType: family, OSSKey: "uploads/3/" + family + "/a.bin"}}
		signer.DecorateAttachments(attachments)
		require.Equal(t, "", attachments[0].OSSURL, "file-delivery family %q must not receive a signed display URL", family)
	}

	// quarantine 键在任何家族（含展示族）都不签。
	attachments := []model.ContentAttachment{
		{FileType: "image", OSSKey: "quarantine/archive-scan/9/2/job11"},
		{FileType: "mod", OSSKey: "quarantine/archive-scan/9/2/job12"},
	}
	signer.DecorateAttachments(attachments)
	require.Equal(t, "", attachments[0].OSSURL, "quarantine key must never be signed")
	require.Equal(t, "", attachments[1].OSSURL, "quarantine key must never be signed")

	// AttachmentURL 单点契约与 DecorateAttachments 一致。
	require.NotEmpty(t, signer.AttachmentURL("uploads/3/image/a.png", "image"))
	require.Empty(t, signer.AttachmentURL("uploads/3/mod/a.zip", "mod"))
	require.Empty(t, signer.AttachmentURL("quarantine/x", "image"))
}

// #813 决策①：附件展示签名走独立短 TTL 通道（oss.display_attachment_ttl_sec，
// 出厂 300s），与通用 3600s 展示预算分离，且不做 #428 桶对齐（Expires 差值
// 就是配置值本身）。
func TestAttachmentDisplayTTLIndependentChannel(t *testing.T) {
	before := time.Now().Unix()

	cfg := displayTestConfig(0)
	cfg.OSS.DisplayAttachmentTTLSec = 90
	signer := NewDisplayURLSigner(cfg)
	signed := signer.AttachmentURL("uploads/3/image/ttl.png", "image")
	requireSignedShape(t, signed, "uploads/3/image/ttl.png")
	require.InDelta(t, float64(before+90), float64(signedExpires(t, signed)), 2,
		"attachment display TTL must come from oss.display_attachment_ttl_sec, without bucket alignment")

	// 未设置（0）回退 300s 工厂默认。
	defaulted := NewDisplayURLSigner(displayTestConfig(0)).AttachmentURL("uploads/3/image/ttl-default.png", "image")
	require.InDelta(t, float64(before+300), float64(signedExpires(t, defaulted)), 2,
		"unset display_attachment_ttl_sec falls back to the 300s factory default")

	// 通用展示通道（封面/头像）不受新键影响，仍是 3600s + 桶对齐。
	generic := NewDisplayURLSigner(displayTestConfig(0)).SignURL(
		aliyun.ObjectURL(displayTestDomain, "uploads/7/image/generic.png"))
	genericAligned := alignDisplayExpiry(time.Unix(before, 0), 3600*time.Second, displayURLSignatureBucketSec)
	require.InDelta(t, float64(genericAligned), float64(signedExpires(t, generic)), 2,
		"generic display channel keeps the 3600s bucket-aligned budget")
}

// quarantine 前缀在任何签名路径不可达：通用 SignURL 通道（封面/头像）对
// quarantine 平台对象同样拒绝签名（原样返回未签名 URL，私桶匿名读取 403）。
func TestSignURLNeverSignsQuarantineKeys(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))
	quarantine := aliyun.ObjectURL(displayTestDomain, "quarantine/archive-scan/9/2/job11")
	require.Equal(t, quarantine, signer.SignURL(quarantine), "quarantine platform URL must pass through unsigned")
}

// ScanAware 路径与白名单一致：gate=nil（委派 DecorateAttachments）时文件
// 族仍不得拿到签名 URL，展示族照常签名——家族白名单先于一切签名路径。
func TestScanAwareDecorateRespectsFamilyWhitelist(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))

	attachments := []model.ContentAttachment{
		{ID: 1, FileType: "image", OSSKey: "uploads/3/image/a.png"},
		{ID: 2, FileType: "mod", OSSKey: "uploads/3/mod/a.zip"},
		{ID: 3, FileType: "document", OSSKey: "uploads/3/document/a.docx"},
	}
	signer.ScanAwareDecorateAttachments(context.Background(), attachments, nil, 0)
	requireSignedShape(t, attachments[0].OSSURL, "uploads/3/image/a.png")
	require.Equal(t, "", attachments[1].OSSURL, "mod must stay unsigned through the scan-aware path")
	require.Equal(t, "", attachments[2].OSSURL, "document must stay unsigned through the scan-aware path")

	// 非 nil gate：家族白名单先于扫描门——文件族在触达 RequireAttachmentClean
	//（无 DB 会 panic）之前即被拒签，证明白名单是第一道分支。
	gate := NewArchiveScanGate(nil, true, nil)
	fileOnly := []model.ContentAttachment{{ID: 11, FileType: "mod", OSSKey: "uploads/3/mod/b.zip"}}
	signer.ScanAwareDecorateAttachments(context.Background(), fileOnly, gate, 0)
	require.Equal(t, "", fileOnly[0].OSSURL, "file family must be rejected before the scan gate")
}
