package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/internal/service"
)

// Identity is the PAT identity the gin adapter (routes.go) resolves through
// the shared auth middleware and injects into the MCP handler's request
// context. Anonymous and JWT sessions carry no Identity and keep the
// read-only surface.
type Identity struct {
	UserID int64
	Scopes []string
}

type identityKeyType struct{}

var identityKeyValue identityKeyType

// WithIdentity attaches the PAT identity to a request context.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKeyValue, &id)
}

// IdentityFromContext returns the PAT identity or nil.
func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKeyValue).(*Identity)
	return id
}

// withIdentity wraps an http.Handler injecting a fixed identity. Production
// wiring composes this in front of the streamable handler from the gin layer;
// tests use it to simulate PAT scopes.
func withIdentity(next http.Handler, id *Identity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id != nil {
			r = r.WithContext(WithIdentity(r.Context(), *id))
		}
		next.ServeHTTP(w, r)
	})
}

func identityHasScope(id *Identity, scope string) bool {
	if id == nil {
		return false
	}
	for _, s := range id.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// requireScope re-validates the PAT scope at tool execution time. Tool
// registration is fixed at session creation, so this check is the defense in
// depth that keeps a leaked session id from outranking the current request's
// token (e.g. a download-only token replaying a full-scope session).
func requireScope(ctx context.Context, scope string) (int64, error) {
	id := IdentityFromContext(ctx)
	if id == nil || id.UserID == 0 {
		return 0, errors.New("this tool requires a personal access token: Authorization: Bearer oc_pat_...")
	}
	if !identityHasScope(id, scope) {
		return 0, fmt.Errorf("token lacks required scope: %s", scope)
	}
	return id.UserID, nil
}

// requireVerifiedUser enforces the same interaction policy as the REST
// guards (verified email + reputation floor; the publish freeze is enforced
// inside ContentService.PublishContentWithContext).
func requireVerifiedUser(ctx context.Context, deps Deps, uid int64) error {
	status, err := service.ResolveRuntimeUserStatus(ctx, deps.DB, service.NewRuntimeStatusCache(nil, deps.Cfg), uid)
	if err != nil {
		return errors.New("account status is temporarily unavailable, retry later")
	}
	decision := service.EvaluateInteractionAccess(status, deps.Cfg, true, true)
	if !decision.Allowed {
		return fmt.Errorf("action denied: %s", decision.DenialReason)
	}
	return nil
}

// serverCache lazily builds one server per capability shape so every HTTP
// request resolves to a stable instance (the SDK binds sessions to the
// server returned at initialize time).
type serverCache struct {
	mu      sync.Mutex
	servers map[[2]bool]*sdkmcp.Server
	deps    Deps
}

func newServerCache(deps Deps) *serverCache {
	return &serverCache{servers: map[[2]bool]*sdkmcp.Server{}, deps: deps}
}

func (c *serverCache) get(download, upload bool) *sdkmcp.Server {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := [2]bool{download, upload}
	if s, ok := c.servers[key]; ok {
		return s
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "omnicraft", Version: "v1"}, nil)
	addSearchTool(server, c.deps)
	addGetContentTool(server, c.deps)
	addGetUsageGuideTool(server, c.deps)
	addListCategoriesTool(server, c.deps)
	if download && c.deps.ContentSvc != nil {
		addRequestDownloadTool(server, c.deps)
	}
	if upload && c.deps.uploadReady() {
		addSuggestPublishMetadataTool(server, c.deps)
		addCreateContentTool(server, c.deps)
		addRequestUploadURLTool(server, c.deps)
	}
	c.servers[key] = server
	return server
}

func (d Deps) uploadReady() bool {
	return d.SuggestPublishMetadata != nil && d.IssueUploadURL != nil && d.ConsumeUploadQuota != nil
}

// --- omnicraft_request_download ---------------------------------------------

type RequestDownloadInput struct {
	ContentID    int64 `json:"content_id" jsonschema:"numeric content id from search results"`
	AttachmentID int64 `json:"attachment_id,omitempty" jsonschema:"optional attachment id when a content item has several downloadable files"`
}

func addRequestDownloadTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_request_download",
		Description: "Request a short-lived signed download URL for one published content item " +
			"(requires a personal access token with the download scope). Returns the URL plus file metadata " +
			"and the usage guide summary; the URL itself is pre-authorized, fetch it directly with GET.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in RequestDownloadInput) (*sdkmcp.CallToolResult, any, error) {
		if in.ContentID <= 0 {
			return nil, nil, errors.New("content_id must be positive")
		}
		uid, err := requireScope(ctx, "download")
		if err != nil {
			return nil, nil, err
		}
		if err := requireVerifiedUser(ctx, deps, uid); err != nil {
			return nil, nil, err
		}
		attachmentIDStr := ""
		if in.AttachmentID > 0 {
			attachmentIDStr = strconv.FormatInt(in.AttachmentID, 10)
		}
		res, err := deps.ContentSvc.RequestDownload(ctx, uid, in.ContentID, attachmentIDStr)
		if err != nil {
			return nil, nil, errors.New(downloadErrorText(err))
		}
		out := map[string]any{
			"url":          res.URL,
			"expires_in":   res.ExpiresIn,
			"content_type": res.ContentType,
			"filename":     path.Base(res.Attachment.OSSKey),
			"mime_type":    res.Attachment.MimeType,
		}
		if res.Attachment.FileSize != nil {
			out["size"] = *res.Attachment.FileSize
		} else {
			out["size"] = 0
		}
		if guide, gerr := deps.GuideSvc.GetMergedView(ctx, in.ContentID, "zh"); gerr == nil && guide != nil {
			out["usage_guide"] = guide
		} else {
			out["usage_guide"] = nil
		}
		return textResult(out)
	})
}

func downloadErrorText(err error) string {
	switch {
	case errors.Is(err, service.ErrDownloadUnauthorized):
		return "login required"
	case errors.Is(err, service.ErrContentNotFound):
		return "content not found"
	case errors.Is(err, service.ErrDownloadNotPublished):
		return "content is not published and cannot be downloaded"
	case errors.Is(err, service.ErrDownloadUnavailable):
		return "content is unavailable (visibility scope)"
	case errors.Is(err, service.ErrDownloadNotAllowed):
		return "the author disabled downloads for this content"
	case errors.Is(err, service.ErrNoAttachments):
		return "content has no downloadable files"
	case errors.Is(err, service.ErrInvalidAttachmentID):
		return "invalid attachment_id"
	case errors.Is(err, service.ErrAttachmentMismatch):
		return "attachment does not belong to this content"
	case errors.Is(err, service.ErrAmbiguousAttachment):
		return "content has several files: pass attachment_id (list them via omnicraft_get_content)"
	case errors.Is(err, service.ErrArchiveNotClean):
		return "the archive failed the malware scan and cannot be delivered"
	case errors.Is(err, service.ErrOSSNotConfigured):
		return "file storage is not configured on the server"
	default:
		return "download temporarily unavailable"
	}
}

// --- omnicraft_suggest_publish_metadata --------------------------------------

type SuggestPublishMetadataInput struct {
	FileName    string `json:"file_name" jsonschema:"name of the local file to publish, e.g. song.pdf"`
	ContentType string `json:"content_type,omitempty" jsonschema:"content type if known: mod, sheet_music, template, audio, video, image, article, prompt, other"`
	Title       string `json:"title,omitempty" jsonschema:"working title, if the user gave one"`
	Description string `json:"description,omitempty" jsonschema:"short description of the work, if any"`
}

func addSuggestPublishMetadataTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_suggest_publish_metadata",
		Description: "Suggest publishing metadata (title, description, category, tags) for a file the user wants " +
			"to publish (requires a personal access token with the upload scope). Call before omnicraft_create_content; " +
			"always show the resulting draft to the user for confirmation.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in SuggestPublishMetadataInput) (*sdkmcp.CallToolResult, any, error) {
		uid, err := requireScope(ctx, "upload")
		if err != nil {
			return nil, nil, err
		}
		_ = uid
		result, err := deps.SuggestPublishMetadata(ctx, in.Title, in.Description, in.FileName, in.ContentType)
		if err != nil {
			return nil, nil, errors.New("metadata suggestion unavailable: " + err.Error())
		}
		return textResult(map[string]any{
			"suggested_title":       result.SuggestedTitle,
			"suggested_description": result.SuggestedDescription,
			"suggested_category":    result.SuggestedCategory,
			"suggested_tags":        result.SuggestedTags,
		})
	})
}

// --- omnicraft_create_content -------------------------------------------------

type UsageGuideSpecInput struct {
	Locale       string   `json:"locale,omitempty" jsonschema:"zh (default) or en"`
	Requirements []string `json:"requirements,omitempty" jsonschema:"environment requirements, e.g. Minecraft 1.20.1+"`
	Steps        []string `json:"steps,omitempty" jsonschema:"ordered install/usage steps"`
	Notes        string   `json:"notes,omitempty" jsonschema:"free-form notes"`
}

type CreateAttachmentInput struct {
	GrantID     string `json:"grant_id" jsonschema:"grant id returned by omnicraft_request_upload_url"`
	FileType    string `json:"file_type" jsonschema:"file category matching the grant, e.g. file, mod, sheet_music_pdf, image"`
	MimeType    string `json:"mime_type,omitempty" jsonschema:"IANA mime type"`
	FileSize    int64  `json:"file_size,omitempty" jsonschema:"size in bytes"`
	SortOrder   *int   `json:"sort_order,omitempty" jsonschema:"zero-based position; required for image/video sets"`
	IsPrimary   bool   `json:"is_primary,omitempty" jsonschema:"marks the primary file when several are attached"`
}

// CreateContentInput deliberately mirrors the publishable subset of
// service.PublishContentInput instead of embedding it: the embedded struct's
// pointer fields would become required by the generated JSON schema, and the
// MCP surface should stay minimal (no cover/poster internals).
type CreateContentInput struct {
	Title            string                  `json:"title"`
	Description      string                  `json:"description,omitempty"`
	Zone             string                  `json:"zone" jsonschema:"original or fanwork"`
	ContentType      string                  `json:"content_type" jsonschema:"mod, sheet_music, template, audio, video, image, article, prompt, other"`
	Category         string                  `json:"category,omitempty"`
	IPID             *int64                  `json:"ip_id,omitempty" jsonschema:"bound original IP id for fanworks"`
	SourceOriginalID *int64                  `json:"source_original_id,omitempty" jsonschema:"source original content id (fanwork)"`
	SourceFanworkID  *int64                  `json:"source_fanwork_id,omitempty" jsonschema:"source fanwork id (fanwork of a fanwork)"`
	IsPublic         bool                    `json:"is_public,omitempty"`
	AllowCopy        bool                    `json:"allow_copy,omitempty"`
	Tags             []string                `json:"tags,omitempty"`
	Attachments      []CreateAttachmentInput `json:"attachments,omitempty"`
	UsageGuide       *UsageGuideSpecInput    `json:"usage_guide,omitempty" jsonschema:"author usage guide specifics persisted with the draft"`
}

func (in CreateContentInput) toPublishInput() service.PublishContentInput {
	atts := make([]service.AttachmentInput, 0, len(in.Attachments))
	for _, a := range in.Attachments {
		var size *int64
		if a.FileSize > 0 {
			s := a.FileSize
			size = &s
		}
		atts = append(atts, service.AttachmentInput{
			GrantID:     a.GrantID,
			FileType:    a.FileType,
			MimeType:    a.MimeType,
			FileSize:    size,
			SortOrder:   a.SortOrder,
			IsPrimary:   a.IsPrimary,
		})
	}
	return service.PublishContentInput{
		Title:            in.Title,
		Description:      in.Description,
		Zone:             in.Zone,
		IPID:             in.IPID,
		SourceOriginalID: in.SourceOriginalID,
		SourceFanworkID:  in.SourceFanworkID,
		Category:         in.Category,
		ContentType:      in.ContentType,
		IsPublic:         in.IsPublic,
		AllowCopy:        in.AllowCopy,
		Tags:             in.Tags,
		Attachments:      atts,
	}
}

func addCreateContentTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_create_content",
		Description: "Create a draft content item from confirmed metadata and upload grants " +
			"(requires a personal access token with the upload scope). The draft enters the standard review pipeline " +
			"immediately — external agents get no exemption. ONLY call after the user explicitly confirmed the full draft " +
			"(title, description, category, tags, file list, usage guide, license).",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in CreateContentInput) (*sdkmcp.CallToolResult, any, error) {
		uid, err := requireScope(ctx, "upload")
		if err != nil {
			return nil, nil, err
		}
		if err := requireVerifiedUser(ctx, deps, uid); err != nil {
			return nil, nil, err
		}
		content, err := deps.ContentSvc.PublishContentWithContext(ctx, in.toPublishInput(), uid)
		if err != nil {
			return nil, nil, errors.New(publishErrorText(err))
		}
		if in.UsageGuide != nil {
			gi := service.UsageGuideInput{
				Locale:       in.UsageGuide.Locale,
				Requirements: in.UsageGuide.Requirements,
				Steps:        in.UsageGuide.Steps,
				Notes:        in.UsageGuide.Notes,
				Source:       "llm_assisted",
			}
			if serr := deps.GuideSvc.SaveSpecifics(ctx, uid, content.ID, gi); serr != nil {
				return nil, nil, errors.New("content created but the usage guide was rejected: " + serr.Error())
			}
		}
		return textResult(map[string]any{
			"content_id": content.ID,
			"status":     content.Status,
			"review":     "the draft entered the standard review pipeline (same rules as the web studio; no exemption for external agents)",
		})
	})
}

func publishErrorText(err error) string {
	switch {
	case errors.Is(err, service.ErrPublishFrozen):
		return "publishing is temporarily frozen for this account due to recent violations"
	case errors.Is(err, service.ErrSourceNotAllowedForOriginal):
		return "original content cannot declare a source work"
	case errors.Is(err, service.ErrFanworkSourceRequired):
		return "fanwork content must declare its source (source_original_id or source_fanwork_id)"
	case errors.Is(err, service.ErrUploadGrantInvalid), errors.Is(err, service.ErrUploadGrantUnavailable):
		return "upload grant invalid or unavailable: request a fresh URL via omnicraft_request_upload_url, PUT the file, then retry"
	case errors.Is(err, service.ErrMediaSetInvalid):
		return "the media set is invalid (count/order/MIME mismatch)"
	case errors.Is(err, service.ErrArchiveScanUnavailable), errors.Is(err, service.ErrArchiveScanFailed),
		errors.Is(err, service.ErrArchiveScanPending):
		return "the archive must pass the malware scan before publishing"
	default:
		return "publishing failed: " + err.Error()
	}
}

// --- omnicraft_request_upload_url ---------------------------------------------

type RequestUploadURLInput struct {
	FileName    string `json:"file_name" jsonschema:"local file name, keeps its extension"`
	FileType    string `json:"file_type" jsonschema:"file category, e.g. file, mod, sheet_music_pdf, image"`
	MimeType    string `json:"mime_type" jsonschema:"IANA mime type of the file"`
	FileSize    int64  `json:"file_size" jsonschema:"file size in bytes"`
	DurationSec *int   `json:"duration_sec,omitempty" jsonschema:"duration for audio/video files"`
}

func addRequestUploadURLTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_request_upload_url",
		Description: "Get a pre-signed PUT URL to upload one file (requires a personal access token with the upload scope). " +
			"The grant_id it returns must be passed to omnicraft_create_content. Uploads share the same type whitelist, " +
			"size cap and per-user quota as web uploads.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in RequestUploadURLInput) (*sdkmcp.CallToolResult, any, error) {
		uid, err := requireScope(ctx, "upload")
		if err != nil {
			return nil, nil, err
		}
		if err := deps.ConsumeUploadQuota(ctx, uid); err != nil {
			return nil, nil, errors.New("upload quota exceeded, retry in the next hour: " + err.Error())
		}
		resp, err := deps.IssueUploadURL(ctx, service.PresignUploadRequest{
			FileName:    in.FileName,
			FileType:    in.FileType,
			MimeType:    in.MimeType,
			FileSize:    in.FileSize,
			DurationSec: in.DurationSec,
		}, uid)
		if err != nil {
			return nil, nil, errors.New("upload URL rejected: " + err.Error())
		}
		return textResult(map[string]any{
			"upload_url":  resp.UploadURL,
			"oss_key":     resp.OSSKey,
			"expires_in":  resp.ExpiresIn,
			"grant_id":    resp.GrantID,
			"next_step":   "PUT the file bytes to upload_url, then pass grant_id in omnicraft_create_content attachments",
		})
	})
}
