package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

// defaultDisplayURLTTLSec is the signed display URL budget from
// architecture.md §6.2 (private bucket, 1h signed URLs). It deliberately
// stays above the 300s Redis display cache TTL so a cached row can always be
// re-signed at the serialization boundary before its previous signature
// could expire.
const defaultDisplayURLTTLSec = 3600

// defaultDisplayAttachmentTTLSec is the #813 independent minute-level budget
// for content-attachment display signing (decision ①): preview families
// (image / video / sheet_music) are signed on a channel separate from the
// generic display budget above, so an AllowCopy=false exposure window stays
// minute-level. Anonymous detail responses cache for at most 60s, so the
// cache never outlives the signature. Configurable via
// oss.display_attachment_ttl_sec (1..3600).
const defaultDisplayAttachmentTTLSec = 300

// displayAttachmentFamilies is the #813 attachment-family whitelist (audit
// #5): only preview-consumed families get signed display URLs. File-delivery
// families (mod / text / document / audio / model3d) are delivered
// exclusively through the download gate (AllowCopy + malicious-archive scan
// + PAT download scope) — their oss_url stays empty everywhere.
var displayAttachmentFamilies = map[string]bool{
	"image":       true,
	"video":       true,
	"sheet_music": true, // SheetMusicViewer in-page preview (musicxml/midi/pdf)
}

// isDisplayAttachmentFamily reports whether an attachment family belongs to
// the display/preview whitelist (#813 audit #5 + decision ①).
func isDisplayAttachmentFamily(fileType string) bool {
	return displayAttachmentFamilies[strings.TrimSpace(fileType)]
}

// isQuarantineObjectKey reports whether an OSS key lives under the quarantine
// prefix. The archive-scan gate declares quarantine objects are never a
// delivery target; #813 extends that invariant to every signing path.
func isQuarantineObjectKey(ossKey string) bool {
	return strings.HasPrefix(strings.TrimSpace(ossKey), "quarantine/")
}

// displayURLSignatureBucketSec aligns signed-URL expiry to a fixed 30-minute
// epoch-aligned window (#428). Within one window every serialization of the
// same object produces a byte-identical signed URL, so downstream caches
// (notably /_next/image, whose cache key contains the full signed URL) hit
// across responses instead of re-fetching and re-transforming per response.
// Constraint: bucket < ttl, so a URL's remaining validity stays at or above
// ttl - bucket and the exposure window never exceeds ttl + bucket.
const displayURLSignatureBucketSec = 1800

// alignDisplayExpiry returns the absolute unix expiry for a display URL
// signed at now with budget ttl: ceil((now + ttl) / bucket) * bucket. The
// ceil keeps the effective validity at or above ttl (never shorter), and the
// epoch alignment makes the value depend only on the wall-clock window, not
// on the individual request.
func alignDisplayExpiry(now time.Time, ttl time.Duration, bucketSec int64) int64 {
	ttlSec := int64(ttl.Seconds())
	if bucketSec <= 0 {
		return now.Unix() + ttlSec
	}
	expiry := now.Unix() + ttlSec
	if rem := expiry % bucketSec; rem != 0 {
		expiry += bucketSec - rem
	}
	return expiry
}

// DisplayURLSigner re-issues short-lived signed GET URLs for display media
// (IP covers, content covers, avatars, gallery attachments) at the API
// serialization boundary. The private OSS bucket rejects anonymous reads
// (B-002: every unsigned display URL 403'd), so platform object URLs are
// signed per response while the database and Redis keep canonical bare URLs.
//
// Signing rules mirror ReviewService.resolveScanURL: only URLs under the
// configured delivery domain are signed; empty, relative and external URLs
// pass through unchanged; a missing OSS configuration or a signing failure
// fails open to the original URL (local dev and CI run without OSS). All
// methods are nil-receiver safe so handlers without a signer keep today's
// behavior.
type DisplayURLSigner struct {
	client *aliyun.OSSClient
	domain string
	ttl    time.Duration
	// attachmentTTL is the #813 independent minute-level display budget for
	// content attachments (decision ①), separate from the generic ttl above.
	attachmentTTL time.Duration
}

// NewDisplayURLSigner builds a signer from the runtime config. It returns nil
// when cfg is nil; an incomplete OSS config yields a signer that passes every
// URL through (fail open).
func NewDisplayURLSigner(cfg *config.Config) *DisplayURLSigner {
	if cfg == nil {
		return nil
	}
	ttlSec := cfg.OSS.DisplayURLTTL
	if ttlSec <= 0 {
		ttlSec = defaultDisplayURLTTLSec
	}
	attachmentTTLSec := cfg.OSS.DisplayAttachmentTTLSec
	if attachmentTTLSec <= 0 {
		attachmentTTLSec = defaultDisplayAttachmentTTLSec
	}
	client, _ := aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName)
	return &DisplayURLSigner{
		client:        client,
		domain:        cfg.OSS.Domain,
		ttl:           time.Duration(ttlSec) * time.Second,
		attachmentTTL: time.Duration(attachmentTTLSec) * time.Second,
	}
}

// SignURL returns rawURL as a short-lived signed GET URL when it is a
// platform OSS object URL, and unchanged otherwise. Quarantine platform
// objects are never signed on any path (#813): the URL passes through
// unsigned and the private bucket rejects the anonymous read.
func (s *DisplayURLSigner) SignURL(rawURL string) string {
	if s == nil {
		return rawURL
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || s.client == nil {
		return rawURL
	}
	key, ok := aliyun.ObjectKeyFromURL(s.domain, rawURL)
	if !ok {
		return rawURL
	}
	if isQuarantineObjectKey(key) {
		return rawURL
	}
	// #428: hand the client the distance to the bucket-aligned absolute
	// expiry instead of a plain ttl, so every call inside the same window
	// signs the same Expires second (the SDK derives the URL timestamp from
	// its own clock plus this duration).
	remaining := time.Until(time.Unix(alignDisplayExpiry(time.Now(), s.ttl, displayURLSignatureBucketSec), 0))
	signed, err := s.client.GetSignedURL(key, http.MethodGet, remaining)
	if err != nil {
		return rawURL
	}
	return signed
}

// AttachmentURL derives the signed display URL for an attachment object key
// on the #813 independent short-TTL channel (decision ①). Only the
// display/preview families (image / video / sheet_music) are signed — file
// delivery families and quarantine keys return "" so the transient oss_url
// field stays absent and delivery stays exclusive to the download gate.
// Without a delivery domain there is no canonical display URL to present, so
// it also returns "" instead of leaking a bare oss_key into an img src.
func (s *DisplayURLSigner) AttachmentURL(ossKey, fileType string) string {
	if s == nil {
		return ""
	}
	if !isDisplayAttachmentFamily(fileType) {
		return ""
	}
	return s.signAttachmentURL(ossKey, int(s.attachmentTTL.Seconds()))
}

// DecorateIP signs the IP cover and the nested creator avatar in place. The
// value must not be written back to the database afterwards.
func (s *DisplayURLSigner) DecorateIP(ip *model.IP) {
	if s == nil || ip == nil {
		return
	}
	ip.CoverURL = s.SignURL(ip.CoverURL)
	if ip.Creator != nil {
		s.DecorateUser(ip.Creator)
	}
}

func (s *DisplayURLSigner) DecorateIPs(ips []model.IP) {
	if s == nil {
		return
	}
	for i := range ips {
		s.DecorateIP(&ips[i])
	}
}

// DecorateContent signs the content cover, the author avatar, the linked IP
// cover and the linked source covers in place. Source chains are acyclic by
// construction (a source must exist before its fanwork), so recursion
// terminates at the first unloaded link.
func (s *DisplayURLSigner) DecorateContent(item *model.ContentItem) {
	if s == nil || item == nil {
		return
	}
	item.CoverImageURL = s.SignURL(item.CoverImageURL)
	s.DecorateUser(&item.Author)
	if item.IP != nil {
		s.DecorateIP(item.IP)
	}
	if item.SourceOriginal != nil {
		s.DecorateContent(item.SourceOriginal)
	}
	if item.SourceFanwork != nil {
		s.DecorateContent(item.SourceFanwork)
	}
}

func (s *DisplayURLSigner) DecorateContents(items []model.ContentItem) {
	if s == nil {
		return
	}
	for i := range items {
		s.DecorateContent(&items[i])
	}
}

// DecorateAttachments fills the transient OSSURL field with a signed display
// URL derived from the canonical OSSKey, which stays unchanged. #813 audit
// #5: only the display/preview families are signed — file-delivery families
// keep an empty OSSURL (the client renders the scan-state card and delivers
// them through the download endpoint) and quarantine keys are never signed.
func (s *DisplayURLSigner) DecorateAttachments(attachments []model.ContentAttachment) {
	if s == nil {
		return
	}
	for i := range attachments {
		attachments[i].OSSURL = s.AttachmentURL(attachments[i].OSSKey, attachments[i].FileType)
	}
}

// ScanAwareDecorateAttachments is the #688 AttachmentScanGate preview
// closure: the REST detail and MCP read paths used to sign every attachment
// unconditionally, bypassing the scan gate that only publish review and the
// download endpoint enforced. Semantics (fail-closed, same judgment as the
// download gate):
//
//   - gate == nil → legacy unconditional decorate (wirings without a gate);
//   - #813 audit #5: the family whitelist runs FIRST — file-delivery
//     families (mod / text / document / audio / model3d) and quarantine keys
//     never receive a signed URL on any branch;
//   - every surviving attachment passes RequireAttachmentClean (disabled
//     flag → only the eternal quarantine-prefix rejection applies);
//   - rejected attachments keep an empty OSSURL — the row's scan_status
//     rides the DTO and the client renders the scan-state card;
//   - admitted scannable-family attachments sign with the scan-aware short
//     TTL (cap 300s) and NO bucket alignment, so a re-scan that turns an
//     attachment blocked leaves at most the scan TTL of exposure (v2.2 #1);
//   - non-scannable display families sign on the #813 independent
//     minute-level attachment channel (decision ①).
func (s *DisplayURLSigner) ScanAwareDecorateAttachments(ctx context.Context, attachments []model.ContentAttachment, gate *ArchiveScanGate, scanTTLSec int) {
	if s == nil {
		return
	}
	if gate == nil {
		s.DecorateAttachments(attachments)
		return
	}
	for i := range attachments {
		if !isDisplayAttachmentFamily(attachments[i].FileType) {
			attachments[i].OSSURL = ""
			continue
		}
		if err := gate.RequireAttachmentClean(ctx, attachments[i].ID); err != nil {
			attachments[i].OSSURL = ""
			continue
		}
		if gate.IsScannableFamily(attachments[i].FileType) {
			attachments[i].OSSURL = s.signAttachmentURL(attachments[i].OSSKey, gate.ScannablePreviewTTLSec(scanTTLSec))
			continue
		}
		attachments[i].OSSURL = s.AttachmentURL(attachments[i].OSSKey, attachments[i].FileType)
	}
}

// signAttachmentURL signs an attachment object on the short-TTL channel
// without the #428 bucket alignment: the minute-level preview budget must
// never stretch to ttl + bucket, and quarantine keys are never signed here
// either (#813).
func (s *DisplayURLSigner) signAttachmentURL(ossKey string, ttlSec int) string {
	if s == nil {
		return ""
	}
	key := strings.TrimSpace(ossKey)
	if key == "" || strings.TrimSpace(s.domain) == "" || s.client == nil {
		return ""
	}
	if isQuarantineObjectKey(key) {
		return ""
	}
	signed, err := s.client.GetSignedURL(key, http.MethodGet, time.Duration(ttlSec)*time.Second)
	if err != nil {
		return ""
	}
	return signed
}

// DecorateUser signs the avatar URL in place.
func (s *DisplayURLSigner) DecorateUser(user *model.User) {
	if s == nil || user == nil {
		return
	}
	user.AvatarURL = s.SignURL(user.AvatarURL)
}

func (s *DisplayURLSigner) DecorateUsers(users []model.User) {
	if s == nil {
		return
	}
	for i := range users {
		s.DecorateUser(&users[i])
	}
}

// DecorateComment signs the comment author avatar in place.
func (s *DisplayURLSigner) DecorateComment(comment *model.Comment) {
	if s == nil || comment == nil {
		return
	}
	s.DecorateUser(&comment.Author)
}

func (s *DisplayURLSigner) DecorateComments(comments []model.Comment) {
	if s == nil {
		return
	}
	for i := range comments {
		s.DecorateComment(&comments[i])
	}
}

// DecorateDiscussion signs the discussion author avatar and the linked IP
// cover in place.
func (s *DisplayURLSigner) DecorateDiscussion(discussion *model.Discussion) {
	if s == nil || discussion == nil {
		return
	}
	s.DecorateUser(&discussion.Author)
	if discussion.IP != nil {
		s.DecorateIP(discussion.IP)
	}
}

func (s *DisplayURLSigner) DecorateDiscussions(discussions []model.Discussion) {
	if s == nil {
		return
	}
	for i := range discussions {
		s.DecorateDiscussion(&discussions[i])
	}
}
