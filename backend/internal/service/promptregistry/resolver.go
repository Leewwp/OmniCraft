package promptregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// Store is the persistence seam; repository.PromptRegistryRepository
// satisfies it and tests use fakes.
type Store interface {
	GetByLabel(ctx context.Context, name, label string) (*model.PromptRegistry, error)
	CreateVersion(ctx context.Context, row *model.PromptRegistry) error
	EnsureLabel(ctx context.Context, name, label string, version int) error
	SetLabel(ctx context.Context, name, label string, version int) error
}

// ProductionLabel is the runtime resolution label; staging exists for
// pre-release validation via a temporary label move.
const ProductionLabel = "production"

// resolveTTL bounds how long a production lookup is cached. 30s keeps
// admin edits effectively hot while shielding the DB from per-turn reads.
const resolveTTL = 30 * time.Second

type cacheEntry struct {
	content   string
	version   int
	expiresAt time.Time
}

// PromptResolver resolves slot templates: builtin by default, DB production
// version overriding, short-TTL cache in front, write-side invalidation.
// Failures degrade to the builtin (the agent must keep answering when the
// registry is down). A nil resolver resolves builtins.
type PromptResolver struct {
	store Store
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func NewPromptResolver(store Store) *PromptResolver {
	return &PromptResolver{store: store, cache: map[string]cacheEntry{}}
}

// Resolve returns the effective template for a slot plus the registry
// version it came from (0 = builtin). Version feeds the trace
// prompt_name/prompt_version columns (T2).
func (r *PromptResolver) Resolve(ctx context.Context, slot PromptSlot) (string, int) {
	if r == nil || r.store == nil {
		return slot.Builtin, 0
	}
	if cached, ok := r.cached(slot.Name); ok {
		return cached.content, cached.version
	}
	row, err := r.store.GetByLabel(ctx, slot.Name, ProductionLabel)
	if err != nil || row == nil {
		// Cache the builtin briefly too: a missing or unreachable registry
		// must not turn every turn into a DB round trip.
		slog.Debug("prompt registry miss, using builtin", "slot", slot.Name)
		r.storeCached(slot.Name, cacheEntry{content: slot.Builtin, version: 0, expiresAt: time.Now().Add(resolveTTL)})
		return slot.Builtin, 0
	}
	entry := cacheEntry{content: row.Content, version: row.Version, expiresAt: time.Now().Add(resolveTTL)}
	r.storeCached(slot.Name, entry)
	return entry.content, entry.version
}

// RenderSlot resolves then renders in one call — the shape call sites use.
func (r *PromptResolver) RenderSlot(ctx context.Context, slot PromptSlot, values map[string]string) string {
	template, _ := r.Resolve(ctx, slot)
	return Render(template, values)
}

// Invalidate drops cached entries (all slots: admin writes are rare and
// cross-slot effects are not worth tracking).
func (r *PromptResolver) Invalidate() {
	if r == nil {
		return
	}
	r.mu.Lock()
	clear(r.cache)
	r.mu.Unlock()
}

func (r *PromptResolver) cached(name string) (cacheEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.cache[name]
	if !ok || time.Now().After(entry.expiresAt) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (r *PromptResolver) storeCached(name string, entry cacheEntry) {
	r.mu.Lock()
	r.cache[name] = entry
	r.mu.Unlock()
}

// SeedV1 inserts version 1 of every slot (byte-identical to the builtin)
// and points production at it — both only when absent, so re-runs and
// admin-managed states are never disturbed. Server and worker both run it;
// the ON CONFLICT paths make that safe.
func SeedV1(ctx context.Context, store Store) error {
	if store == nil {
		return nil
	}
	for _, slot := range Slots {
		placeholders, err := json.Marshal(slot.RequiredPlaceholders)
		if err != nil {
			return err
		}
		if err := store.CreateVersion(ctx, &model.PromptRegistry{
			Name:                 slot.Name,
			Version:              1,
			Content:              slot.Builtin,
			RequiredPlaceholders: model.JSONB(placeholders),
		}); err != nil && !errors.Is(err, repository.ErrPromptVersionExists) {
			return err
		}
		if err := store.EnsureLabel(ctx, slot.Name, ProductionLabel, 1); err != nil {
			return err
		}
	}
	return nil
}

// UpgradeSeed is a code-shipped version bump for an existing slot: the new
// immutable version's number and content. Content must keep the slot's
// required placeholders (validated here before any DB I/O).
type UpgradeSeed struct {
	SlotName string
	Version  int
	Content  string
}

// RegistryUpgrades is the ordered list of shipped upgrades. Entries are
// append-only: an already-shipped upgrade never changes content (versions
// are immutable); a further bump adds a new entry with the next version.
var RegistryUpgrades = []UpgradeSeed{
	{SlotName: SlotAgentSystem.Name, Version: 2, Content: agentSystemV2()},
}

// SeedUpgrades applies code-shipped version bumps after SeedV1. Each entry
// creates its version only when absent; the production label moves forward
// exactly once — in the boot that freshly created the row. An upgrade found
// already in the registry (re-run, second process, or an admin rollback to
// an older version) never touches the label, so admin-managed states are
// final after the first ship.
func SeedUpgrades(ctx context.Context, store Store) error {
	if store == nil {
		return nil
	}
	for _, up := range RegistryUpgrades {
		slot, ok := SlotByName(up.SlotName)
		if !ok {
			return fmt.Errorf("prompt upgrade references unknown slot %q", up.SlotName)
		}
		if err := ValidateTemplate(slot, up.Content); err != nil {
			return fmt.Errorf("prompt upgrade %s v%d: %w", up.SlotName, up.Version, err)
		}
		placeholders, err := json.Marshal(slot.RequiredPlaceholders)
		if err != nil {
			return err
		}
		if err := store.CreateVersion(ctx, &model.PromptRegistry{
			Name:                 up.SlotName,
			Version:              up.Version,
			Content:              up.Content,
			RequiredPlaceholders: model.JSONB(placeholders),
		}); err != nil {
			if errors.Is(err, repository.ErrPromptVersionExists) {
				continue
			}
			return err
		}
		if err := store.SetLabel(ctx, up.SlotName, ProductionLabel, up.Version); err != nil {
			return err
		}
		slog.Info("prompt registry upgrade shipped", "slot", up.SlotName, "version", up.Version)
	}
	return nil
}
