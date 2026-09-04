package rageval

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Golden Set v2 principal keys (annotation contract 2026-09-04 §4). Frozen
// data never stores environment-specific numeric user ids; the harness
// resolves the annotated key to a runtime identity at eval time.
const (
	// PrincipalAnon is the not-logged-in viewer: no token, viewer id 0.
	PrincipalAnon = "anon"
	// PrincipalFixtureViewerAnon is the frozen corpus fixture viewer: logged
	// in, follows nothing (follower edges are frozen in the corpus itself).
	PrincipalFixtureViewerAnon = "fixture:viewer-anon"
	// PrincipalAuthorPrefix is reserved for v2.1 authorized_private cases and
	// is not runnable by the v2 harness.
	PrincipalAuthorPrefix = "fixture:author:"
)

// FixtureViewerAnonEmail is the frozen login identity of the viewer fixture.
const FixtureViewerAnonEmail = "viewer-anon@corpus.omnicraft.local"

// ErrPrincipalV21Reserved marks principal keys the v2 harness must refuse:
// author fixtures belong to the v2.1 authorized_private extension and have no
// runtime mapping yet.
var ErrPrincipalV21Reserved = errors.New("principal key reserved for golden set v2.1 (authorized_private); not runnable in v2")

// ViewerIdentity is the runtime resolution of one principal key. ViewerUserID
// is the numeric identity the production visibility scope consumes; anonymous
// principals carry 0 and skip token-bearing paths.
type ViewerIdentity struct {
	PrincipalKey string
	ViewerUserID int64
	IsAnonymous  bool
	IsFixture    bool
}

// PrincipalRegistry resolves an annotated principal key to a runtime viewer
// identity (contract §4: anon = no token; fixture principals log in as the
// frozen fixture account).
type PrincipalRegistry interface {
	ResolvePrincipal(ctx context.Context, key string) (ViewerIdentity, error)
}

// ResolveViewerIdentity maps a parsed viewer_context to a runtime identity.
// Precedence: v2 principal_key first; a v1 numeric viewer_user_id remains the
// legacy path (H1 compatibility). An empty context resolves to anonymous.
func ResolveViewerIdentity(ctx context.Context, vc ViewerContext, registry PrincipalRegistry) (ViewerIdentity, error) {
	key := strings.TrimSpace(vc.PrincipalKey)
	if key != "" {
		if registry == nil {
			return ViewerIdentity{}, fmt.Errorf("principal_key %q needs a principal registry", key)
		}
		return registry.ResolvePrincipal(ctx, key)
	}
	// Legacy v1 numeric identity: 0 means anonymous.
	return ViewerIdentity{
		PrincipalKey: PrincipalAnon,
		ViewerUserID: vc.ViewerUserID,
		IsAnonymous:  vc.ViewerUserID == 0,
	}, nil
}

// StaticPrincipalRegistry resolves the principal keys that need no database:
// anon only. Fixture keys return a descriptive error so a missing DB registry
// is caught at resolution time instead of silently degrading to anonymous.
type StaticPrincipalRegistry struct{}

// ResolvePrincipal implements PrincipalRegistry.
func (StaticPrincipalRegistry) ResolvePrincipal(_ context.Context, key string) (ViewerIdentity, error) {
	switch key {
	case PrincipalAnon:
		return ViewerIdentity{PrincipalKey: PrincipalAnon, IsAnonymous: true}, nil
	case PrincipalFixtureViewerAnon:
		return ViewerIdentity{}, fmt.Errorf("principal %q requires FixturePrincipalRegistry (database-backed fixture lookup)", key)
	default:
		return ViewerIdentity{}, classifyPrincipalError(key)
	}
}

// FixturePrincipalRegistry is the database-backed registry for corpus fixture
// principals: fixture:viewer-anon resolves to the numeric id of the frozen
// fixture account. Author fixtures stay v2.1-reserved.
type FixturePrincipalRegistry struct {
	DB *gorm.DB
}

// ResolvePrincipal implements PrincipalRegistry.
func (r *FixturePrincipalRegistry) ResolvePrincipal(ctx context.Context, key string) (ViewerIdentity, error) {
	switch key {
	case PrincipalAnon:
		return ViewerIdentity{PrincipalKey: PrincipalAnon, IsAnonymous: true}, nil
	case PrincipalFixtureViewerAnon:
		if r.DB == nil {
			return ViewerIdentity{}, fmt.Errorf("fixture principal %q needs a database handle", key)
		}
		var id int64
		err := r.DB.WithContext(ctx).
			Table("users").
			Select("id").
			Where("email = ?", FixtureViewerAnonEmail).
			Take(&id).Error
		if err != nil {
			return ViewerIdentity{}, fmt.Errorf("resolve fixture principal %q by email %s: %w", key, FixtureViewerAnonEmail, err)
		}
		return ViewerIdentity{PrincipalKey: key, ViewerUserID: id, IsFixture: true}, nil
	default:
		return ViewerIdentity{}, classifyPrincipalError(key)
	}
}

func classifyPrincipalError(key string) error {
	if strings.HasPrefix(key, PrincipalAuthorPrefix) {
		return fmt.Errorf("principal %q: %w", key, ErrPrincipalV21Reserved)
	}
	return fmt.Errorf("unknown principal_key %q (expected %q, %q or a %q-prefixed fixture)",
		key, PrincipalAnon, PrincipalFixtureViewerAnon, PrincipalAuthorPrefix)
}

// PrincipalKeysForCase returns the principal identities one case must run
// under. The visibility layer runs every case under both identities (contract
// §1.2: double-principal, four leak surfaces must all be zero); every other
// layer runs the case's own annotated principal.
func PrincipalKeysForCase(layer, annotated string) []string {
	if layer == LayerVisibility {
		return []string{PrincipalAnon, PrincipalFixtureViewerAnon}
	}
	if annotated == "" {
		return []string{PrincipalAnon}
	}
	return []string{annotated}
}
