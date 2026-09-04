package rageval

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
)

// H1: ParseViewerContext reads the v2 principal_key; legacy numeric
// viewer_user_id rows keep parsing unchanged.
func TestParseViewerContextPrincipalKey(t *testing.T) {
	vc, err := ParseViewerContext(model.JSONB(`{"principal_key":"anon"}`))
	if err != nil {
		t.Fatalf("parse v2 viewer context: %v", err)
	}
	if vc.PrincipalKey != "anon" || vc.ViewerUserID != 0 {
		t.Fatalf("v2 parse = %+v, want principal_key=anon viewer_user_id=0", vc)
	}

	legacy, err := ParseViewerContext(model.JSONB(`{"viewer_user_id":42}`))
	if err != nil {
		t.Fatalf("parse legacy viewer context: %v", err)
	}
	if legacy.PrincipalKey != "" || legacy.ViewerUserID != 42 {
		t.Fatalf("legacy parse = %+v, want empty principal_key viewer_user_id=42", legacy)
	}

	combined, err := ParseViewerContext(model.JSONB(`{"viewer_user_id":7,"principal_key":"fixture:viewer-anon"}`))
	if err != nil {
		t.Fatalf("parse combined viewer context: %v", err)
	}
	if combined.PrincipalKey != "fixture:viewer-anon" {
		t.Fatalf("combined parse = %+v, want principal_key to survive", combined)
	}
}

func TestResolveViewerIdentityPrecedence(t *testing.T) {
	ctx := context.Background()
	registry := StaticPrincipalRegistry{}

	// principal_key wins over a stale numeric id.
	got, err := ResolveViewerIdentity(ctx, ViewerContext{ViewerUserID: 9, PrincipalKey: "anon"}, registry)
	if err != nil {
		t.Fatalf("resolve anon: %v", err)
	}
	if !got.IsAnonymous || got.ViewerUserID != 0 || got.PrincipalKey != "anon" {
		t.Fatalf("anon identity = %+v", got)
	}

	// legacy numeric path: 0 = anonymous, N = that user.
	got, err = ResolveViewerIdentity(ctx, ViewerContext{ViewerUserID: 0}, registry)
	if err != nil {
		t.Fatalf("resolve legacy 0: %v", err)
	}
	if !got.IsAnonymous || got.PrincipalKey != "anon" {
		t.Fatalf("legacy 0 identity = %+v, want anonymous", got)
	}
	got, err = ResolveViewerIdentity(ctx, ViewerContext{ViewerUserID: 12}, registry)
	if err != nil {
		t.Fatalf("resolve legacy 12: %v", err)
	}
	if got.IsAnonymous || got.ViewerUserID != 12 {
		t.Fatalf("legacy 12 identity = %+v, want user 12", got)
	}

	// empty registry + principal_key → explicit error, not silent fallback.
	if _, err := ResolveViewerIdentity(ctx, ViewerContext{PrincipalKey: "anon"}, nil); err == nil {
		t.Fatal("principal_key without a registry must error")
	}
}

func TestStaticPrincipalRegistry(t *testing.T) {
	registry := StaticPrincipalRegistry{}

	identity, err := registry.ResolvePrincipal(context.Background(), PrincipalAnon)
	if err != nil || !identity.IsAnonymous {
		t.Fatalf("anon resolution = %+v, %v", identity, err)
	}

	// fixture keys need the DB-backed registry: a loud error, not a silent
	// anonymous downgrade.
	if _, err := registry.ResolvePrincipal(context.Background(), PrincipalFixtureViewerAnon); err == nil {
		t.Fatal("static registry must refuse fixture principals")
	}

	// author fixtures are v2.1-reserved.
	_, err = registry.ResolvePrincipal(context.Background(), "fixture:author:a07")
	if !errors.Is(err, ErrPrincipalV21Reserved) {
		t.Fatalf("author principal error = %v, want ErrPrincipalV21Reserved", err)
	}

	if _, err := registry.ResolvePrincipal(context.Background(), "nonsense"); err == nil {
		t.Fatal("unknown principal must error")
	}
}

func TestFixturePrincipalRegistry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fixture-principal?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL)`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, email) VALUES (77, ?), (78, 'other@example.com')`, FixtureViewerAnonEmail).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	registry := &FixturePrincipalRegistry{DB: db}
	ctx := context.Background()

	identity, err := registry.ResolvePrincipal(ctx, PrincipalFixtureViewerAnon)
	if err != nil {
		t.Fatalf("resolve fixture viewer: %v", err)
	}
	if identity.ViewerUserID != 77 || identity.IsAnonymous || !identity.IsFixture {
		t.Fatalf("fixture identity = %+v, want user 77 fixture", identity)
	}

	identity, err = registry.ResolvePrincipal(ctx, PrincipalAnon)
	if err != nil || !identity.IsAnonymous || identity.ViewerUserID != 0 {
		t.Fatalf("anon via fixture registry = %+v, %v", identity, err)
	}

	if _, err := registry.ResolvePrincipal(ctx, "fixture:author:a01"); !errors.Is(err, ErrPrincipalV21Reserved) {
		t.Fatalf("author principal error = %v, want ErrPrincipalV21Reserved", err)
	}

	// a database without the fixture account fails loudly at resolution time
	fresh, _ := gorm.Open(sqlite.Open("file:fixture-missing?mode=memory"), &gorm.Config{})
	_ = fresh.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`)
	if _, err := (&FixturePrincipalRegistry{DB: fresh}).ResolvePrincipal(ctx, PrincipalFixtureViewerAnon); err == nil {
		t.Fatal("missing fixture account must error")
	}
}

func TestPrincipalKeysForLayer(t *testing.T) {
	if got := PrincipalKeysForCase(LayerVisibility, PrincipalAnon); len(got) != 2 || got[0] != PrincipalAnon || got[1] != PrincipalFixtureViewerAnon {
		t.Fatalf("visibility layer principals = %v, want both identities", got)
	}
	if got := PrincipalKeysForCase(LayerSemanticDiscovery, "fixture:viewer-anon"); len(got) != 1 || got[0] != "fixture:viewer-anon" {
		t.Fatalf("annotated principal = %v, want the annotation only", got)
	}
	if got := PrincipalKeysForCase(LayerKnownItemExact, ""); len(got) != 1 || got[0] != PrincipalAnon {
		t.Fatalf("missing annotation = %v, want anon default", got)
	}
}

// H1 regression: the committed v1 fixture rows still parse through the
// extended ViewerContext.
func TestViewerContextV1FixtureRowsParse(t *testing.T) {
	for _, raw := range []string{`{"viewer_user_id":0}`, `{"viewer_user_id":5}`} {
		vc, err := ParseViewerContext(model.JSONB(raw))
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if vc.PrincipalKey != "" {
			t.Fatalf("v1 row %s must not gain a principal_key", raw)
		}
	}
}
