// Package mcpdocserver hosts the document-editing MCP server (SP-23 M2,
// #567): draft_read / draft_suggest / draft_apply_edit over the agent
// workspace draft store, stdio transport first (in-process tests, MCP
// inspector, and the M3 client bridge all speak stdio).
//
// Naming note: the ticket wrote `draft.read` style names; the MCP tool-name
// charset allows [a-zA-Z0-9_-] only, so the tools are draft_read /
// draft_suggest / draft_apply_edit.
package mcpdocserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/internal/model"
)

// DraftStore is the persistence seam; production wires the repository, tests
// an in-memory fake. Every call re-checks ownership (userID is the
// server-bound identity from the environment, never caller-supplied).
type DraftStore interface {
	GetForUser(ctx context.Context, userID, draftID int64) (*model.AgentDraft, error)
	Update(ctx context.Context, draft *model.AgentDraft) error
}

// Limits are constants of the tool contract (Key Rule 6 applies to config
// surface; these bound tool payloads, like the existing agent tool limits).
const (
	maxEditBatch     = 50
	maxBodyRunes     = 100_000
	maxTitleRunes    = 200
	longParagraphAt  = 300
	confirmTTLHint   = "re-read the draft to refresh it"
	maxSuggestTags   = 5
	metadataDescrMax = 120
)

var (
	errDraftNotFound  = errors.New("draft not found (unknown id or not yours)")
	errConfirmMissing = errors.New("confirmation token required: read the draft, show the planned edit to the user, then pass the returned confirm_token")
	errConfirmStale   = errors.New("confirmation token stale: the draft changed since it was issued, " + confirmTTLHint)
	errEditInvalid    = errors.New("invalid edit batch")
	// errEmptySecret is the explicit mint/verify-time failure for a missing
	// HMAC key (audit #9 / #814): with an empty secret the confirm gate
	// would be forgeable, so it must fail loudly instead of silently
	// signing with a trivially known key.
	errEmptySecret = errors.New("confirm secret must not be empty")
)

// Options binds the server identity: the user whose drafts are editable and
// the HMAC secret for confirmation tokens.
type Options struct {
	UserID        int64
	ConfirmSecret []byte
}

// NewServer assembles the MCP server with the three draft tools.
func NewServer(store DraftStore, opts Options) *sdkmcp.Server {
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "omnicraft-docserver", Version: "v1"}, nil)
	addDraftRead(s, store, opts)
	addDraftSuggest(s, store, opts)
	addDraftApplyEdit(s, store, opts)
	return s
}

// --- confirmation tokens ------------------------------------------------------

// ConfirmToken mints an HMAC over the draft's current state stamp AND the
// edit batch it authorizes (audit #9 / #814): any write (ours or a
// concurrent one) changes updated_at and invalidates the token — optimistic
// concurrency for the write gate — and a token minted for one edit batch
// never verifies for another. editsHash is the SHA256 of the canonicalized
// batch ("" for the draft-level token draft_read/draft_suggest hand out).
func ConfirmToken(secret []byte, draftID, userID, updatedAtUnix int64, editsHash string) (string, error) {
	if len(secret) == 0 {
		return "", errEmptySecret
	}
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintf(mac, "draft:%d:%d:%d:%s", draftID, userID, updatedAtUnix, editsHash)
	return hex.EncodeToString(mac.Sum(nil))[:24], nil
}

// batchHash canonicalizes an edit batch into a stable SHA256: struct field
// order and JSON encoding are deterministic, so the same logical batch always
// hashes identically and any substitution (different op, index, text, tags)
// changes the hash.
func batchHash(edits []DraftEdit) string {
	raw, _ := json.Marshal(edits)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// tokenKind classifies a presented confirm token against the freshly loaded
// draft row.
type tokenKind int

const (
	tokenMissing tokenKind = iota
	tokenStale
	// tokenDraft is the draft-level token from draft_read/draft_suggest: it
	// proves the caller read the current version and unlocks the preview,
	// never the write itself.
	tokenDraft
	// tokenBatch authorizes applying exactly the batch it was minted over
	// (handed out by the stage-1 preview of that batch).
	tokenBatch
)

// classifyConfirmToken maps the presented token onto tokenKind. An empty
// HMAC secret is an explicit error (never a silent downgrade).
func classifyConfirmToken(secret []byte, token string, draft *model.AgentDraft, edits []DraftEdit) (tokenKind, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return tokenMissing, errConfirmMissing
	}
	draftToken, err := ConfirmToken(secret, draft.ID, draft.UserID, draft.UpdatedAt.Unix(), "")
	if err != nil {
		return tokenStale, err
	}
	batchToken, err := ConfirmToken(secret, draft.ID, draft.UserID, draft.UpdatedAt.Unix(), batchHash(edits))
	if err != nil {
		return tokenStale, err
	}
	switch {
	case hmac.Equal([]byte(batchToken), []byte(trimmed)):
		return tokenBatch, nil
	case hmac.Equal([]byte(draftToken), []byte(trimmed)):
		return tokenDraft, nil
	default:
		return tokenStale, errConfirmStale
	}
}

// --- draft_read ---------------------------------------------------------------

type DraftReadInput struct {
	DraftID int64 `json:"draft_id" jsonschema:"id of the draft to read"`
}

func addDraftRead(s *sdkmcp.Server, store DraftStore, opts Options) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name: "draft_read",
		Description: "Read one of the bound user's drafts: title, tags, and the paragraph list. " +
			"The response carries a confirm_token that draft_apply_edit needs to obtain a diff preview; " +
			"the preview response then carries the batch token that authorizes the write.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in DraftReadInput) (*sdkmcp.CallToolResult, any, error) {
		draft, err := store.GetForUser(ctx, opts.UserID, in.DraftID)
		if err != nil {
			return nil, nil, errDraftNotFound
		}
		token, err := ConfirmToken(opts.ConfirmSecret, draft.ID, draft.UserID, draft.UpdatedAt.Unix(), "")
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"draft_id":      draft.ID,
			"title":         draft.Title,
			"content_type":  draft.ContentType,
			"tags":          draft.Tags,
			"paragraphs":    paragraphList(draft.Body),
			"version":       draft.UpdatedAt.Unix(),
			"confirm_token": token,
		})
	})
}

// --- draft_suggest ------------------------------------------------------------

type DraftSuggestInput struct {
	DraftID int64  `json:"draft_id" jsonschema:"id of the draft to analyse"`
	Focus   string `json:"focus,omitempty" jsonschema:"optional focus note, e.g. tighter wording or structure"`
}

func addDraftSuggest(s *sdkmcp.Server, store DraftStore, opts Options) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name: "draft_suggest",
		Description: "Deterministic draft analysis feeding the assistant's own suggestions: flags overlong " +
			"paragraphs, empty/short drafts, and derives suggested metadata (title/description/tags) from the body. " +
			"The response carries the confirm_token a subsequent draft_apply_edit needs.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in DraftSuggestInput) (*sdkmcp.CallToolResult, any, error) {
		draft, err := store.GetForUser(ctx, opts.UserID, in.DraftID)
		if err != nil {
			return nil, nil, errDraftNotFound
		}
		paragraphs := paragraphList(draft.Body)
		long := []map[string]any{}
		for i, p := range paragraphs {
			if utf8.RuneCountInString(p) > longParagraphAt {
				long = append(long, map[string]any{
					"index": i, "runes": utf8.RuneCountInString(p),
					"opens_with": truncateRunes(p, 24),
					"hint":       "consider splitting or tightening this paragraph",
				})
			}
		}
		title, description := suggestMetadata(paragraphs)
		token, err := ConfirmToken(opts.ConfirmSecret, draft.ID, draft.UserID, draft.UpdatedAt.Unix(), "")
		if err != nil {
			return nil, nil, err
		}
		out := map[string]any{
			"draft_id":        draft.ID,
			"paragraph_count": len(paragraphs),
			"long_paragraphs": long,
			"suggested_metadata": map[string]any{
				"title":       title,
				"description": description,
				"tags":        suggestTags(draft.Title, paragraphs),
			},
			"confirm_token": token,
			"version":       draft.UpdatedAt.Unix(),
		}
		if strings.TrimSpace(in.Focus) != "" {
			out["focus_ack"] = in.Focus
		}
		return textResult(out)
	})
}

// --- draft_apply_edit ---------------------------------------------------------

type DraftEdit struct {
	Op    string   `json:"op" jsonschema:"one of replace_paragraph, insert_after, delete_paragraph, update_title, update_tags"`
	Index int      `json:"index,omitempty" jsonschema:"paragraph index (paragraph ops only)"`
	Text  string   `json:"text,omitempty" jsonschema:"new paragraph text, or the new title for update_title"`
	Tags  []string `json:"tags,omitempty" jsonschema:"full replacement tag list for update_tags"`
}

type DraftApplyEditInput struct {
	DraftID int64 `json:"draft_id" jsonschema:"id of the draft to edit"`
	// ConfirmToken is handler-validated (not schema-required) so a missing
	// token gets explicit guidance instead of a bare schema error.
	ConfirmToken string `json:"confirm_token,omitempty" jsonschema:"token from draft_read/draft_suggest for the CURRENT draft version; obtain user approval first"`
	// UserConfirmed is the two-stage write gate (SP-23 M4 piece 5): the
	// first call (false/omitted) returns a diff preview and writes nothing;
	// only a follow-up with true after the user approved writes.
	UserConfirmed bool        `json:"user_confirmed,omitempty" jsonschema:"set true only after the user explicitly approved the diff preview shown to them"`
	Edits         []DraftEdit `json:"edits" jsonschema:"ordered batch of structured edits, applied atomically"`
}

func addDraftApplyEdit(s *sdkmcp.Server, store DraftStore, opts Options) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name: "draft_apply_edit",
		Description: "Apply a batch of structured edits to one of the bound user's drafts. Write gate: " +
			"the confirm_token from draft_read/draft_suggest first returns a diff preview (stage 1); the preview " +
			"response carries a batch-specific confirm_token, and only a follow-up carrying THAT token with " +
			"user_confirmed=true writes. Tokens are bound to the draft version AND the exact edit batch — " +
			"any concurrent edit or batch substitution invalidates them. The whole batch applies atomically.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in DraftApplyEditInput) (*sdkmcp.CallToolResult, any, error) {
		// Per-call permission re-check: ownership is re-verified inside
		// GetForUser, and the token is verified against the freshly loaded
		// row (never trusted from the request).
		draft, err := store.GetForUser(ctx, opts.UserID, in.DraftID)
		if err != nil {
			return nil, nil, errDraftNotFound
		}
		if len(in.Edits) == 0 || len(in.Edits) > maxEditBatch {
			return nil, nil, fmt.Errorf("%w: batch size must be 1..%d", errEditInvalid, maxEditBatch)
		}
		kind, err := classifyConfirmToken(opts.ConfirmSecret, in.ConfirmToken, draft, in.Edits)
		if err != nil {
			return nil, nil, err
		}
		// Stage 2 of the write gate (audit #9 / #814): only the
		// batch-specific token minted by a stage-1 preview of THIS batch may
		// carry user_confirmed=true into a write. A draft-level token —
		// including one presented with user_confirmed=true — always degrades
		// to the preview, so the caller must have seen the exact batch
		// before anything is applied.
		if kind != tokenBatch || !in.UserConfirmed {
			batchToken, err := ConfirmToken(opts.ConfirmSecret, draft.ID, draft.UserID, draft.UpdatedAt.Unix(), batchHash(in.Edits))
			if err != nil {
				return nil, nil, err
			}
			preview, err := previewEdits(draft, in.Edits)
			if err != nil {
				return nil, nil, err
			}
			message := "show the diff to the user; re-call with user_confirmed=true and this batch confirm_token once they approve"
			if kind == tokenDraft && in.UserConfirmed {
				message = "user_confirmed requires the batch-specific token issued by a preview of THIS batch; " + message
			}
			return textResult(map[string]any{
				"status":        "needs_confirmation",
				"draft_id":      draft.ID,
				"diff":          preview,
				"confirm_token": batchToken,
				"message":       message,
			})
		}
		updated, applied, err := applyEdits(draft, in.Edits)
		if err != nil {
			return nil, nil, err
		}
		if err := store.Update(ctx, updated); err != nil {
			return nil, nil, fmt.Errorf("draft update failed: %w", err)
		}
		return textResult(map[string]any{
			"draft_id": updated.ID,
			"applied":  applied,
			"version":  updated.UpdatedAt.Unix(),
			"note":     "updated_at advanced; previous confirm_tokens are now stale",
		})
	})
}

// previewEdits validates the batch and renders a before/after diff without
// writing (stage 1 of the two-stage write gate). Every op entry carries a
// before/after pair so the preview is exactly what gets approved (audit #9
// / #814: update_tags and head insertions used to render bare op labels).
func previewEdits(draft *model.AgentDraft, edits []DraftEdit) ([]map[string]any, error) {
	updated, applied, err := applyEdits(draft, edits)
	if err != nil {
		return nil, err
	}
	before, after := paragraphList(draft.Body), paragraphList(updated.Body)
	beforeTags, afterTags := tagsList(draft.Tags), tagsList(updated.Tags)
	diff := make([]map[string]any, 0, len(edits))
	for _, op := range applied {
		entry := map[string]any{"op": op}
		n, hashIdx := indexFromOp(op)
		if hashIdx < 0 {
			// update_title / update_tags carry no "#index"; tags render a
			// full before/after list, the title gets its own entry below.
			if op == "update_tags" {
				entry["before"], entry["after"] = beforeTags, afterTags
			}
			diff = append(diff, entry)
			continue
		}
		if strings.HasPrefix(op, "insert_after") {
			at := n + 1
			if n >= 0 && n < len(before) {
				entry["before"] = truncateRunes(before[n], 120)
			} else {
				entry["before"] = "" // head insertion (#-1) has no anchor paragraph
			}
			if at >= 0 && at < len(after) {
				entry["after"] = truncateRunes(after[at], 120)
			}
		} else if n >= 0 && n < len(before) && n < len(after) {
			entry["before"] = truncateRunes(before[n], 120)
			entry["after"] = truncateRunes(after[n], 120)
		}
		diff = append(diff, entry)
	}
	if draft.Title != updated.Title {
		diff = append(diff, map[string]any{"op": "title", "before": draft.Title, "after": updated.Title})
	}
	return diff, nil
}

// tagsList decodes the draft's JSONB tag list for the preview diff; it never
// returns nil so the diff renders [] instead of null.
func tagsList(raw model.JSONB) []string {
	tags := []string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &tags)
	}
	return tags
}

func indexFromOp(op string) (int, int) {
	idx := strings.LastIndex(op, "#")
	if idx < 0 {
		return 0, -1
	}
	n, err := strconv.Atoi(op[idx+1:])
	if err != nil {
		return 0, -1
	}
	return n, idx
}

// applyEdits folds the batch onto a copy; the store row is untouched unless
// every edit validates.
func applyEdits(draft *model.AgentDraft, edits []DraftEdit) (*model.AgentDraft, []string, error) {
	updated := *draft
	paragraphs := paragraphList(updated.Body)
	applied := make([]string, 0, len(edits))
	for _, e := range edits {
		switch e.Op {
		case "replace_paragraph":
			if e.Index < 0 || e.Index >= len(paragraphs) || !boundedText(e.Text) {
				return nil, nil, fmt.Errorf("%w: replace_paragraph index/text", errEditInvalid)
			}
			paragraphs[e.Index] = e.Text
			applied = append(applied, "replace_paragraph#"+strconv.Itoa(e.Index))
		case "insert_after":
			if e.Index < -1 || e.Index >= len(paragraphs) || !boundedText(e.Text) {
				return nil, nil, fmt.Errorf("%w: insert_after index/text", errEditInvalid)
			}
			at := e.Index + 1
			paragraphs = append(paragraphs[:at], append([]string{e.Text}, paragraphs[at:]...)...)
			applied = append(applied, "insert_after#"+strconv.Itoa(e.Index))
		case "delete_paragraph":
			if e.Index < 0 || e.Index >= len(paragraphs) {
				return nil, nil, fmt.Errorf("%w: delete_paragraph index", errEditInvalid)
			}
			paragraphs = append(paragraphs[:e.Index], paragraphs[e.Index+1:]...)
			applied = append(applied, "delete_paragraph#"+strconv.Itoa(e.Index))
		case "update_title":
			title := strings.TrimSpace(e.Text)
			if title == "" || utf8.RuneCountInString(title) > maxTitleRunes {
				return nil, nil, fmt.Errorf("%w: update_title text", errEditInvalid)
			}
			updated.Title = title
			applied = append(applied, "update_title")
		case "update_tags":
			if len(e.Tags) > 10 {
				return nil, nil, fmt.Errorf("%w: update_tags count", errEditInvalid)
			}
			clean := make([]string, 0, len(e.Tags))
			for _, tag := range e.Tags {
				tag = strings.TrimSpace(tag)
				if tag == "" || utf8.RuneCountInString(tag) > 30 {
					return nil, nil, fmt.Errorf("%w: update_tags entry", errEditInvalid)
				}
				clean = append(clean, tag)
			}
			raw, _ := json.Marshal(clean)
			updated.Tags = model.JSONB(raw)
			applied = append(applied, "update_tags")
		default:
			return nil, nil, fmt.Errorf("%w: unknown op %q", errEditInvalid, e.Op)
		}
	}
	body := strings.Join(paragraphs, "\n\n")
	if utf8.RuneCountInString(body) > maxBodyRunes {
		return nil, nil, fmt.Errorf("%w: result exceeds body budget", errEditInvalid)
	}
	updated.Body = body
	updated.UpdatedAt = time.Now()
	return &updated, applied, nil
}

// --- shared helpers -----------------------------------------------------------

func paragraphList(body string) []string {
	if strings.TrimSpace(body) == "" {
		return []string{}
	}
	parts := strings.Split(body, "\n\n")
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func boundedText(text string) bool {
	return strings.TrimSpace(text) != "" && utf8.RuneCountInString(text) <= maxBodyRunes
}

func truncateRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

func suggestMetadata(paragraphs []string) (title, description string) {
	if len(paragraphs) == 0 {
		return "", ""
	}
	first := strings.TrimSpace(paragraphs[0])
	for _, p := range paragraphs {
		t := strings.TrimSpace(p)
		if strings.HasPrefix(t, "#") {
			return strings.TrimLeft(t, "# "), truncateRunes(first, metadataDescrMax)
		}
	}
	return truncateRunes(first, 40), truncateRunes(first, metadataDescrMax)
}

// suggestTags derives up to five tags from 2-gram frequency across title and
// body — deterministic, CJK-safe, no external calls.
func suggestTags(title string, paragraphs []string) []string {
	freq := map[string]int{}
	count := func(text string) {
		runes := []rune(strings.ToLower(text))
		for i := 0; i+1 < len(runes); i++ {
			a, b := runes[i], runes[i+1]
			if !wordRune(a) || !wordRune(b) {
				continue
			}
			freq[string([]rune{a, b})]++
		}
	}
	count(title)
	for _, p := range paragraphs {
		count(p)
	}
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(freq))
	for k, v := range freq {
		if v >= 2 {
			list = append(list, kv{k, v})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v != list[j].v {
			return list[i].v > list[j].v
		}
		return list[i].k < list[j].k
	})
	tags := make([]string, 0, maxSuggestTags)
	for _, item := range list {
		tags = append(tags, item.k)
		if len(tags) == maxSuggestTags {
			break
		}
	}
	return tags
}

func wordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	case r >= 0x4e00 && r <= 0x9fff:
		return true
	default:
		return false
	}
}

func textResult(payload map[string]any) (*sdkmcp.CallToolResult, any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(raw)}},
	}, nil, nil
}
