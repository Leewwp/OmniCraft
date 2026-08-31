package service

import (
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
	client, _ := aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName)
	return &DisplayURLSigner{client: client, domain: cfg.OSS.Domain, ttl: time.Duration(ttlSec) * time.Second}
}

// SignURL returns rawURL as a short-lived signed GET URL when it is a
// platform OSS object URL, and unchanged otherwise.
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
	signed, err := s.client.GetSignedURL(key, http.MethodGet, s.ttl)
	if err != nil {
		return rawURL
	}
	return signed
}

// AttachmentURL derives the signed display URL for an attachment object key.
// Without a delivery domain there is no canonical display URL to present, so
// it returns "" and the transient oss_url field stays absent instead of
// leaking a bare oss_key into an img src.
func (s *DisplayURLSigner) AttachmentURL(ossKey string) string {
	if s == nil {
		return ""
	}
	key := strings.TrimSpace(ossKey)
	if key == "" || strings.TrimSpace(s.domain) == "" {
		return ""
	}
	return s.SignURL(aliyun.ObjectURL(s.domain, key))
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
// URL derived from the canonical OSSKey, which stays unchanged.
func (s *DisplayURLSigner) DecorateAttachments(attachments []model.ContentAttachment) {
	if s == nil {
		return
	}
	for i := range attachments {
		attachments[i].OSSURL = s.AttachmentURL(attachments[i].OSSKey)
	}
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
