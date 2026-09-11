// Package mcpserver exposes OmniCraft's public surface over the Model
// Context Protocol (SP-16 #449/#451). The Streamable HTTP handler mounts at
// POST/GET/DELETE /api/v1/mcp. Anonymous sessions get the four read-only
// tools (PG keyword search, visibility-scoped detail, merged usage guide,
// category taxonomy — viewer=0 by construction). A PAT injected into the
// request context by the gin adapter additionally registers the scoped write
// tools: download scope adds omnicraft_request_download, upload scope adds
// the three publish tools. Every write tool re-checks its scope at execution
// time so session reuse can never outrank the current token.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type Deps struct {
	DB            *gorm.DB
	SearchRepo    *repository.SearchRepository
	ContentRepo   *repository.ContentRepository
	CategoryRepo  *repository.CategoryRepository
	GuideSvc      *service.UsageGuideService
	DisplaySigner *service.DisplayURLSigner
	// Write-channel dependencies (SP-16 #451). Optional: when absent the
	// corresponding tools are simply not registered.
	Cfg        *config.Config
	ContentSvc *service.ContentService
	// SuggestPublishMetadata reuses the in-chat upload-assist LLM logic
	// (container binds AgentService.UploadAssist).
	SuggestPublishMetadata func(ctx context.Context, title, description, filename, contentType string) (*service.UploadAssistResult, error)
	// IssueUploadURL binds the OSS presign + upload grant issuance shared
	// with POST /contents/oss-token.
	IssueUploadURL func(ctx context.Context, req service.PresignUploadRequest, userID int64) (*service.PresignUploadResponse, error)
	// ConsumeUploadQuota binds the shared per-user hourly upload window
	// (middleware.ConsumeUploadQuota).
	ConsumeUploadQuota func(ctx context.Context, userID int64) error
}

// NewHandler builds the Streamable HTTP handler. The per-request callback
// resolves the PAT identity from the request context (injected by the gin
// adapter after the shared auth middleware) and returns the capability-shaped
// server; the SDK binds the session to whichever server served initialize.
func NewHandler(deps Deps) http.Handler {
	cache := newServerCache(deps)
	return sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		id := IdentityFromContext(r.Context())
		return cache.get(identityHasScope(id, "download") && deps.ContentSvc != nil,
			identityHasScope(id, "upload") && deps.uploadReady())
	}, nil)
}

func textResult(v any) (*sdkmcp.CallToolResult, any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(raw)}},
	}, nil, nil
}

// --- omnicraft_search -------------------------------------------------------

type SearchInput struct {
	Query       string `json:"query" jsonschema:"keyword query matched against title and description"`
	ContentType string `json:"content_type,omitempty" jsonschema:"optional content type filter: mod, sheet_music, template, audio, video, image, article, prompt, other"`
	Category    string `json:"category,omitempty" jsonschema:"optional category slug"`
	Page        int    `json:"page,omitempty" jsonschema:"result page, 1-based"`
	PageSize    int    `json:"page_size,omitempty" jsonschema:"results per page, 1-20"`
}

func addSearchTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_search",
		Description: "Search OmniCraft's public content (sheet music, mods, articles, …). " +
			"Only published, public content is ever returned. Anonymous keyword search, same semantics as the public REST /contents/search.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in SearchInput) (*sdkmcp.CallToolResult, any, error) {
		query := strings.TrimSpace(in.Query)
		if query == "" || len([]rune(query)) > 100 {
			return nil, nil, errors.New("query must be 1-100 characters")
		}
		page, pageSize := in.Page, in.PageSize
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 20 {
			pageSize = 10
		}
		results, total, err := deps.SearchRepo.SearchContents(
			query, "", in.Category, in.ContentType, nil, "", "", page, pageSize, 0,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("search failed")
		}
		items := make([]map[string]any, 0, len(results))
		for _, r := range results {
			items = append(items, map[string]any{
				"id":           r.ID,
				"title":        r.Title,
				"content_type": r.ContentType,
				"zone":         r.Zone,
				"category":     r.Category,
				"description":  truncateRunes(r.Description, 200),
				"created_at":   r.CreatedAt,
			})
		}
		return textResult(map[string]any{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	})
}

// --- omnicraft_get_content --------------------------------------------------

type ContentIDInput struct {
	ContentID int64 `json:"content_id" jsonschema:"numeric content id from search results"`
}

func addGetContentTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_get_content",
		Description: "Fetch one public content item with attachment metadata and short-lived signed display URLs " +
			"(covers/previews). Download URLs require authentication and are intentionally not returned here.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in ContentIDInput) (*sdkmcp.CallToolResult, any, error) {
		if in.ContentID <= 0 {
			return nil, nil, errors.New("content_id must be positive")
		}
		var content model.ContentItem
		err := repository.ApplyContentVisibilityScope(deps.DB.WithContext(ctx).Model(&model.ContentItem{}), 0).
			Where("content_items.id = ?", in.ContentID).
			Preload("Author").
			First(&content).Error
		if err != nil {
			return nil, nil, errors.New("content not found")
		}

		var attachments []model.ContentAttachment
		if err := deps.DB.WithContext(ctx).Where("content_item_id = ?", content.ID).
			Order("sort_order ASC").Find(&attachments).Error; err != nil {
			return nil, nil, fmt.Errorf("content lookup failed")
		}
		if deps.DisplaySigner != nil {
			deps.DisplaySigner.DecorateAttachments(attachments)
		}
		attViews := make([]map[string]any, 0, len(attachments))
		for _, a := range attachments {
			attViews = append(attViews, map[string]any{
				"file_type": a.FileType,
				"mime_type": a.MimeType,
				"file_size": a.FileSize,
				"width":     a.Width,
				"height":    a.Height,
				"oss_url":   a.OSSURL,
			})
		}

		var tagRows []model.ContentTag
		if err := deps.DB.WithContext(ctx).Where("content_item_id = ?", content.ID).Find(&tagRows).Error; err == nil {
			_ = tagRows
		}
		tags := make([]string, 0, len(tagRows))
		for _, t := range tagRows {
			tags = append(tags, t.Tag)
		}

		author := map[string]any{"id": content.AuthorID}
		if content.Author.ID != 0 {
			author["username"] = content.Author.Username
		}
		return textResult(map[string]any{
			"id":           content.ID,
			"title":        content.Title,
			"description":  content.Description,
			"content_type": content.ContentType,
			"zone":         content.Zone,
			"category":     content.Category,
			"author":       author,
			"tags":         tags,
			"attachments":  attViews,
			"created_at":   content.CreatedAt,
			"updated_at":   content.UpdatedAt,
			"usage_guide":  fmt.Sprintf("call omnicraft_get_usage_guide with content_id=%d", content.ID),
		})
	})
}

// --- omnicraft_get_usage_guide ----------------------------------------------

type GuideInput struct {
	ContentID int64  `json:"content_id" jsonschema:"numeric content id"`
	Locale    string `json:"locale,omitempty" jsonschema:"response locale: zh (default) or en"`
}

func addGetUsageGuideTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "omnicraft_get_usage_guide",
		Description: "Get the merged usage guide for one public content item: platform safety notes plus " +
			"author-confirmed requirements, steps and notes (falls back to the system template when the author filled none).",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in GuideInput) (*sdkmcp.CallToolResult, any, error) {
		if in.ContentID <= 0 {
			return nil, nil, errors.New("content_id must be positive")
		}
		// Same visibility gate as the REST guide endpoint: the guide is
		// content-derived and must 404-equivalent for non-public content.
		var count int64
		if err := repository.ApplyContentVisibilityScope(deps.DB.WithContext(ctx).Model(&model.ContentItem{}), 0).
			Where("content_items.id = ?", in.ContentID).
			Count(&count).Error; err != nil || count == 0 {
			return nil, nil, errors.New("content not found")
		}
		view, err := deps.GuideSvc.GetMergedView(ctx, in.ContentID, in.Locale)
		if err != nil {
			return nil, nil, errors.New("usage guide unavailable")
		}
		return textResult(view)
	})
}

// --- omnicraft_list_categories ----------------------------------------------

type CategoriesInput struct {
	Zone string `json:"zone,omitempty" jsonschema:"optional zone filter: original or fanwork"`
}

func addListCategoriesTool(server *sdkmcp.Server, deps Deps) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "omnicraft_list_categories",
		Description: "List the active content category taxonomy (id, slug, localized names) for building filters.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in CategoriesInput) (*sdkmcp.CallToolResult, any, error) {
		cats, err := deps.CategoryRepo.ListByZoneAndLevel(in.Zone, "", nil)
		if err != nil {
			return nil, nil, fmt.Errorf("category listing failed")
		}
		views := make([]map[string]any, 0, len(cats))
		for _, c := range cats {
			views = append(views, map[string]any{
				"id":        c.ID,
				"slug":      c.Slug,
				"zone":      c.Zone,
				"name_i18n": c.NameI18n,
			})
		}
		return textResult(map[string]any{"categories": views})
	})
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return strings.TrimSpace(s)
	}
	return string(runes[:max]) + "…"
}
