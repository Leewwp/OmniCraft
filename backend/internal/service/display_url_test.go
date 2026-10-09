package service

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

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

// #861（D4，spec §9.5）对 #813 白名单的有意放宽：audio/model3d/document 整族
// 进入短 TTL 签名通道（#813 收窄时三者没有预览消费方、是纯暴露面；统一媒体
// 列使预览成为真实消费方）；text 族仅持久化 MIME 规范化后为 application/pdf
// 才准入——空 MIME、text/plain、仅文件名后缀像 .pdf 而 MIME 非 pdf 一律不新
// 签名，且拒签清掉旧 OSSURL；mod 恒不签名（无预览语义）、纯文本走下载行；
// quarantine/ 前缀在任何路径永不签名。
func TestAttachmentDisplayFamilyWhitelistAndQuarantine(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))
	before := time.Now().Unix()

	previewable := []string{"image", "video", "sheet_music", "audio", "model3d", "document"}
	for _, family := range previewable {
		attachments := []model.ContentAttachment{{FileType: family, OSSKey: "uploads/3/" + family + "/a.bin"}}
		signer.DecorateAttachments(attachments)
		requireSignedShape(t, attachments[0].OSSURL, "uploads/3/"+family+"/a.bin")
		require.Equal(t, "uploads/3/"+family+"/a.bin", attachments[0].OSSKey, "oss_key must stay canonical")
		require.InDelta(t, float64(before+300), float64(signedExpires(t, attachments[0].OSSURL)), 2,
			"family %q rides the 300s attachment channel without bucket alignment", family)
	}

	// text 族：持久化 MIME 决定一切——规范化后为 application/pdf 才放行
	//（大小写与参数容错）。
	for _, mime := range []string{"application/pdf", "APPLICATION/PDF", "application/pdf; charset=utf-8"} {
		attachments := []model.ContentAttachment{{FileType: "text", MimeType: mime, OSSKey: "uploads/3/text/doc.pdf"}}
		signer.DecorateAttachments(attachments)
		requireSignedShape(t, attachments[0].OSSURL, "uploads/3/text/doc.pdf")
	}

	// 反例：text/plain、空 MIME、仅文件名像 .pdf 而 MIME 非 pdf——不新签名，
	// 且拒签必须清掉调用方可能带入的旧 OSSURL。
	staleTxt := []model.ContentAttachment{{
		FileType: "text", MimeType: "text/plain", OSSKey: "uploads/3/text/a.txt",
		OSSURL: "https://stale.example/previous-signature",
	}}
	signer.DecorateAttachments(staleTxt)
	require.Equal(t, "", staleTxt[0].OSSURL, "text/plain must stay unsigned and drop any stale oss_url")

	nameOnly := []model.ContentAttachment{{
		FileType: "text", OriginalFileName: &pdfLikeName, OSSKey: "uploads/3/text/looks-like.pdf",
		OSSURL: "https://stale.example/previous-signature",
	}}
	signer.DecorateAttachments(nameOnly)
	require.Equal(t, "", nameOnly[0].OSSURL, "a .pdf file name without a persisted pdf MIME never qualifies")

	emptyMime := []model.ContentAttachment{{FileType: "text", OSSKey: "uploads/3/text/no-mime.txt"}}
	signer.DecorateAttachments(emptyMime)
	require.Equal(t, "", emptyMime[0].OSSURL, "text without persisted MIME never qualifies")

	for _, family := range []string{"mod", ""} {
		attachments := []model.ContentAttachment{{FileType: family, OSSKey: "uploads/3/" + family + "/a.bin"}}
		signer.DecorateAttachments(attachments)
		require.Equal(t, "", attachments[0].OSSURL, "file-delivery family %q must not receive a signed display URL", family)
	}

	// quarantine 键在任何家族（含展示族与新预览族）都不签。
	attachments := []model.ContentAttachment{
		{FileType: "image", OSSKey: "quarantine/archive-scan/9/2/job11"},
		{FileType: "mod", OSSKey: "quarantine/archive-scan/9/2/job12"},
		{FileType: "document", OSSKey: "quarantine/archive-scan/9/2/job13"},
		{FileType: "audio", OSSKey: "quarantine/archive-scan/9/2/job14"},
	}
	signer.DecorateAttachments(attachments)
	for i := range attachments {
		require.Equal(t, "", attachments[i].OSSURL, "quarantine key must never be signed")
	}
}

// pdfLikeName backs the "pdf file name without pdf MIME" negative fixture.
var pdfLikeName = "looks-like.pdf"

// #861：attachmentDisplayEligible 的判定矩阵——族 × 持久化 MIME 的纯函数
// 契约。只消费持久化元数据；text 族仅规范化 media type == application/pdf
// 准入（大小写/参数容错），mod/未知族恒拒。
func TestAttachmentDisplayEligibilityMatrix(t *testing.T) {
	cases := []struct {
		name     string
		fileType string
		mimeType string
		want     bool
	}{
		{"image", "image", "image/png", true},
		{"video", "video", "video/mp4", true},
		{"sheet_music pdf", "sheet_music", "application/pdf", true},
		{"sheet_music mscz", "sheet_music", "application/octet-stream", true},
		{"audio", "audio", "audio/mpeg", true},
		{"audio octet-stream", "audio", "application/octet-stream", true},
		{"model3d", "model3d", "model/stl", true},
		{"document docx", "document", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"document octet-stream", "document", "application/octet-stream", true},
		{"text pdf", "text", "application/pdf", true},
		{"text pdf upper", "text", "APPLICATION/PDF", true},
		{"text pdf params", "text", "application/pdf; charset=utf-8", true},
		{"text pdf padded", "text", "  application/pdf  ", true},
		{"text plain", "text", "text/plain", false},
		{"text markdown", "text", "text/markdown", false},
		{"text empty mime", "text", "", false},
		{"text octet-stream", "text", "application/octet-stream", false},
		{"text malformed mime", "text", "not a media type", false},
		{"text family case kept strict", "Text", "application/pdf", false},
		{"mod", "mod", "application/zip", false},
		{"mod any mime", "mod", "application/pdf", false},
		{"unknown family", "file", "application/pdf", false},
		{"empty family", "", "application/pdf", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, attachmentDisplayEligible(tc.fileType, tc.mimeType),
			"eligibility(%q, %q)", tc.fileType, tc.mimeType)
	}
}

// #813 决策①：附件展示签名走独立短 TTL 通道（oss.display_attachment_ttl_sec，
// 出厂 300s），与通用 3600s 展示预算分离，且不做 #428 桶对齐（Expires 差值
// 就是配置值本身）。
func TestAttachmentDisplayTTLIndependentChannel(t *testing.T) {
	before := time.Now().Unix()

	cfg := displayTestConfig(0)
	cfg.OSS.DisplayAttachmentTTLSec = 90
	signer := NewDisplayURLSigner(cfg)
	signed := signer.AttachmentURL(model.ContentAttachment{FileType: "image", OSSKey: "uploads/3/image/ttl.png"})
	requireSignedShape(t, signed, "uploads/3/image/ttl.png")
	require.InDelta(t, float64(before+90), float64(signedExpires(t, signed)), 2,
		"attachment display TTL must come from oss.display_attachment_ttl_sec, without bucket alignment")

	// #861：新预览族（audio）与 text-PDF 同走该独立通道——配置值优先、无桶对齐。
	audioSigned := signer.AttachmentURL(model.ContentAttachment{FileType: "audio", OSSKey: "uploads/3/audio/ttl.mp3"})
	requireSignedShape(t, audioSigned, "uploads/3/audio/ttl.mp3")
	require.InDelta(t, float64(before+90), float64(signedExpires(t, audioSigned)), 2,
		"new preview families ride the configured attachment TTL channel")

	textPdf := []model.ContentAttachment{{FileType: "text", MimeType: "application/pdf", OSSKey: "uploads/3/text/ttl.pdf"}}
	signer.DecorateAttachments(textPdf)
	requireSignedShape(t, textPdf[0].OSSURL, "uploads/3/text/ttl.pdf")
	require.InDelta(t, float64(before+90), float64(signedExpires(t, textPdf[0].OSSURL)), 2,
		"text-pdf rides the configured attachment TTL channel")

	// 未设置（0）回退 300s 工厂默认。
	defaulted := NewDisplayURLSigner(displayTestConfig(0)).
		AttachmentURL(model.ContentAttachment{FileType: "image", OSSKey: "uploads/3/image/ttl-default.png"})
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

// ScanAware 路径与白名单一致：gate=nil（委派 DecorateAttachments）时新白名单
// 同样生效——audio/model3d/document/text-pdf 拿到签名 URL，mod/纯文本仍被拒；
// 非 nil gate 时 mod 在触达扫描门之前即被资格判断拒签（家族/MIME 资格判断仍
// 是第一道分支，先于任何 DB 访问）。
func TestScanAwareDecorateRespectsFamilyWhitelist(t *testing.T) {
	signer := NewDisplayURLSigner(displayTestConfig(0))

	attachments := []model.ContentAttachment{
		{ID: 1, FileType: "image", OSSKey: "uploads/3/image/a.png"},
		{ID: 2, FileType: "mod", OSSKey: "uploads/3/mod/a.zip"},
		{ID: 3, FileType: "document", OSSKey: "uploads/3/document/a.docx"},
		{ID: 4, FileType: "text", MimeType: "application/pdf", OSSKey: "uploads/3/text/a.pdf"},
		{ID: 5, FileType: "text", MimeType: "text/plain", OSSKey: "uploads/3/text/a.txt"},
	}
	signer.ScanAwareDecorateAttachments(context.Background(), attachments, nil, 0)
	requireSignedShape(t, attachments[0].OSSURL, "uploads/3/image/a.png")
	require.Equal(t, "", attachments[1].OSSURL, "mod must stay unsigned through the scan-aware path")
	requireSignedShape(t, attachments[2].OSSURL, "uploads/3/document/a.docx")
	requireSignedShape(t, attachments[3].OSSURL, "uploads/3/text/a.pdf")
	require.Equal(t, "", attachments[4].OSSURL, "plain text must stay unsigned through the scan-aware path")

	// 非 nil gate：资格判断先于扫描门——mod 在触达 RequireAttachmentClean
	//（无 DB 会 panic）之前即被拒签，证明白名单是第一道分支。
	gate := NewArchiveScanGate(nil, true, nil)
	fileOnly := []model.ContentAttachment{{ID: 11, FileType: "mod", OSSKey: "uploads/3/mod/b.zip"}}
	signer.ScanAwareDecorateAttachments(context.Background(), fileOnly, gate, 0)
	require.Equal(t, "", fileOnly[0].OSSURL, "file family must be rejected before the scan gate")
}

// #861：新族过扫描门的完整矩阵（DB-backed gate，与下载闸同判）——audio/
// model3d/document 是 scannable 族：clean 放行（scan TTL，cap 300s、无桶对
// 齐），pending/blocked/failed 拒签，not_required 视为接线异常 fail-closed
// （mod 先例）；非扫描族（image / text-pdf）not_required 正常放行（attachment
// TTL 通道）；quarantine 恒拒；扫描开关关闭时除 quarantine 前缀外不做扫描
// 状态拦截（现有语义）；拒签清空旧 OSSURL。
func TestScanAwareDecorateNewFamiliesScanGateMatrix(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}))

	signer := NewDisplayURLSigner(displayTestConfig(0))
	before := time.Now().Unix()
	gate := NewArchiveScanGate(db, true, nil)

	rows := []model.ContentAttachment{
		{FileType: "audio", OSSKey: "uploads/3/audio/clean.mp3", ScanStatus: model.ScanStatusClean, ScanRequired: true},
		{FileType: "model3d", OSSKey: "uploads/3/model3d/clean.stl", ScanStatus: model.ScanStatusClean, ScanRequired: true},
		{FileType: "document", OSSKey: "uploads/3/document/clean.docx", ScanStatus: model.ScanStatusClean, ScanRequired: true},
		{FileType: "audio", OSSKey: "uploads/3/audio/pending.mp3", ScanStatus: model.ScanStatusPending, ScanRequired: true},
		{FileType: "model3d", OSSKey: "uploads/3/model3d/blocked.stl", ScanStatus: model.ScanStatusBlocked, ScanRequired: true},
		{FileType: "document", OSSKey: "uploads/3/document/failed.docx", ScanStatus: model.ScanStatusFailed, ScanRequired: true},
		{FileType: "audio", OSSKey: "uploads/3/audio/not-required.mp3", ScanStatus: model.ScanStatusNotRequired},
		{FileType: "image", OSSKey: "uploads/3/image/not-required.png", ScanStatus: model.ScanStatusNotRequired},
		{FileType: "text", MimeType: "application/pdf", OSSKey: "uploads/3/text/not-required.pdf", ScanStatus: model.ScanStatusNotRequired},
		{FileType: "document", OSSKey: "quarantine/archive-scan/3/1/job9", ScanStatus: model.ScanStatusClean, ScanRequired: true},
	}
	require.NoError(t, db.Create(&rows).Error)

	signer.ScanAwareDecorateAttachments(context.Background(), rows, gate, 0)

	for _, i := range []int{0, 1, 2} {
		requireSignedShape(t, rows[i].OSSURL, rows[i].OSSKey)
		require.InDelta(t, float64(before+300), float64(signedExpires(t, rows[i].OSSURL)), 2,
			"scannable clean rows ride the scan-aware TTL (cap 300s, no bucket alignment)")
	}
	for _, i := range []int{3, 4, 5, 6, 9} {
		require.Equal(t, "", rows[i].OSSURL, "row %d (%s/%s) must stay unsigned", i, rows[i].FileType, rows[i].ScanStatus)
	}
	requireSignedShape(t, rows[7].OSSURL, rows[7].OSSKey)
	requireSignedShape(t, rows[8].OSSURL, rows[8].OSSKey)
	require.InDelta(t, float64(before+300), float64(signedExpires(t, rows[8].OSSURL)), 2,
		"text-pdf rides the attachment TTL channel")

	// scanTTL 配置值低于 cap 时 scannable 族按配置值签（archive_scan.url_ttl_sec）。
	rows[0].OSSURL = ""
	signer.ScanAwareDecorateAttachments(context.Background(), rows[0:1], gate, 120)
	require.InDelta(t, float64(before+120), float64(signedExpires(t, rows[0].OSSURL)), 2,
		"scan TTL below the cap comes from the configured archive_scan.url_ttl_sec")

	// 扫描开关关闭：除 quarantine 前缀外不做扫描状态拦截（现有语义）。
	disabled := NewArchiveScanGate(db, false, nil)
	pending := rows[3:4]
	signer.ScanAwareDecorateAttachments(context.Background(), pending, disabled, 0)
	requireSignedShape(t, pending[0].OSSURL, "uploads/3/audio/pending.mp3")
}
