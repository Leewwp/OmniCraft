package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
)

// SP-16 P0 (#446): the zero-leak traversal. Five-state content fixtures
// (pending≈draft, under_review, private is_public=false, soft-deleted,
// banned, plus banned-author) are seeded with unique marker strings, then
// EVERY anonymous content-bearing endpoint is hit as an anonymous viewer.
// No non-public marker may appear in any response body, and non-public
// detail endpoints must 404. The author pass keeps legitimate self-access
// working after the gates land.

type optAuthLeakFixtures struct {
	authorID, bannedAuthorID, viewerID int64

	cPub, cPending, cUnder, cPriv, cDel, cBanned, cBAuth int64

	vPriv int64 // active version row of the private fixture

	prPub, prPriv int64

	ipApproved, ipPending int64
	propPending           int64
	propApproved          int64

	discHidden, discOnPriv int64
}

// leakMarkers are the markers that must NEVER surface to anonymous callers.
var leakMarkers = []string{
	"sp16aleakPENDING", "sp16aleakUNDER", "sp16aleakPRIV", "sp16aleakDEL",
	"sp16aleakBANNED", "sp16aleakBAUTH", "sp16aleakIPPEND",
	"sp16aleakDISCHIDDEN", "sp16aleakDISCPUBPRIV",
}

// controlMarkers may appear (published public fixtures).
var controlMarkers = []string{"sp16aleakPUB", "sp16aleakIPAP"}

func TestOptAuthAnonymousSurfaceZeroLeak(t *testing.T) {
	router, db, cfg, fx := buildOptAuthLeakAuditStack(t)
	_ = db

	anonGet := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	assertNoLeak := func(t *testing.T, label string, rec *httptest.ResponseRecorder, allowControl bool) {
		t.Helper()
		body := rec.Body.String()
		for _, m := range leakMarkers {
			if strings.Contains(body, m) {
				t.Errorf("[%s] anonymous response leaked non-public marker %q (status=%d, body=%.400s)", label, m, rec.Code, body)
			}
		}
		if !allowControl {
			// endpoints with no fixture-bearing payloads at all still must
			// not echo control markers unless they legitimately list the
			// published fixture; callers opt in per endpoint.
			_ = controlMarkers
		}
	}

	expectStatus := func(t *testing.T, label string, rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Errorf("[%s] status = %d, want %d; body=%.400s", label, rec.Code, want, rec.Body.String())
		}
	}

	t.Run("content detail", func(t *testing.T) {
		rec := anonGet(fmt.Sprintf("/api/v1/contents/%d", fx.cPub))
		expectStatus(t, "pub detail", rec, http.StatusOK)
		assertNoLeak(t, "pub detail", rec, true)

		for label, id := range map[string]int64{
			"pending": fx.cPending, "under_review": fx.cUnder, "private": fx.cPriv,
			"soft-deleted": fx.cDel, "banned": fx.cBanned, "banned-author": fx.cBAuth,
		} {
			rec := anonGet(fmt.Sprintf("/api/v1/contents/%d", id))
			expectStatus(t, label+" detail", rec, http.StatusNotFound)
			assertNoLeak(t, label+" detail", rec, false)
		}
	})

	t.Run("version lineage", func(t *testing.T) {
		rec := anonGet(fmt.Sprintf("/api/v1/contents/%d/versions", fx.cPub))
		expectStatus(t, "pub versions", rec, http.StatusOK)
		assertNoLeak(t, "pub versions", rec, true)

		for label, id := range map[string]int64{
			"pending": fx.cPending, "under_review": fx.cUnder, "private": fx.cPriv,
			"soft-deleted": fx.cDel, "banned": fx.cBanned, "banned-author": fx.cBAuth,
		} {
			rec := anonGet(fmt.Sprintf("/api/v1/contents/%d/versions", id))
			expectStatus(t, label+" versions", rec, http.StatusNotFound)
			assertNoLeak(t, label+" versions", rec, false)
		}

		// participant-only detail endpoint stays closed for anonymous (F-056).
		rec = anonGet(fmt.Sprintf("/api/v1/versions/%d", fx.vPriv))
		if rec.Code == http.StatusOK {
			t.Errorf("[version detail] anonymous got 200 on private content version; body=%.400s", rec.Body.String())
		}
		assertNoLeak(t, "version detail", rec, false)
	})

	t.Run("content PR list", func(t *testing.T) {
		rec := anonGet(fmt.Sprintf("/api/v1/contents/%d/prs", fx.cPub))
		expectStatus(t, "pub prs", rec, http.StatusOK)
		assertNoLeak(t, "pub prs", rec, true)

		for label, id := range map[string]int64{
			"pending": fx.cPending, "private": fx.cPriv, "banned": fx.cBanned,
		} {
			rec := anonGet(fmt.Sprintf("/api/v1/contents/%d/prs", id))
			expectStatus(t, label+" prs", rec, http.StatusNotFound)
			assertNoLeak(t, label+" prs", rec, false)
		}

		rec = anonGet(fmt.Sprintf("/api/v1/pr/%d", fx.prPriv))
		if rec.Code == http.StatusOK {
			t.Errorf("[pr detail] anonymous got 200 on private content PR; body=%.400s", rec.Body.String())
		}
		assertNoLeak(t, "pr detail", rec, false)
	})

	t.Run("related fanworks", func(t *testing.T) {
		for label, id := range map[string]int64{"private": fx.cPriv, "banned": fx.cBanned} {
			rec := anonGet(fmt.Sprintf("/api/v1/contents/%d/related-fanworks", id))
			expectStatus(t, label+" related-fanworks", rec, http.StatusNotFound)
			assertNoLeak(t, label+" related-fanworks", rec, false)
		}
	})

	t.Run("content listings and search", func(t *testing.T) {
		for _, path := range []string{
			"/api/v1/contents?page_size=50",
			"/api/v1/contents/search?q=sp16aleak",
			"/api/v1/search/suggestions?q=sp16aleak",
			"/api/v1/search/trending",
			fmt.Sprintf("/api/v1/users/%d/contents", fx.authorID),
			fmt.Sprintf("/api/v1/users/%d/contents", fx.bannedAuthorID),
		} {
			rec := anonGet(path)
			if rec.Code != http.StatusOK {
				t.Errorf("[%s] listing status = %d, want 200; body=%.300s", path, rec.Code, rec.Body.String())
			}
			assertNoLeak(t, path, rec, true)
		}
	})

	t.Run("IP surface", func(t *testing.T) {
		rec := anonGet("/api/v1/ips")
		expectStatus(t, "ip list", rec, http.StatusOK)
		assertNoLeak(t, "ip list", rec, true)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d", fx.ipApproved))
		expectStatus(t, "approved ip detail", rec, http.StatusOK)
		assertNoLeak(t, "approved ip detail", rec, true)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d", fx.ipPending))
		expectStatus(t, "pending ip detail", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip detail", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/proposals", fx.ipPending))
		expectStatus(t, "pending ip proposals", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip proposals", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/proposals/%d", fx.ipPending, fx.propPending))
		expectStatus(t, "pending ip proposal detail", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip proposal detail", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/versions", fx.ipPending))
		expectStatus(t, "pending ip versions", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip versions", rec, false)

		// approved IP governance stays readable (control).
		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/proposals/%d", fx.ipApproved, fx.propApproved))
		expectStatus(t, "approved ip proposal detail", rec, http.StatusOK)
		assertNoLeak(t, "approved ip proposal detail", rec, true)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/discussions", fx.ipPending))
		expectStatus(t, "pending ip discussions", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip discussions", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/ips/%d/discussions/search?q=sp16aleak", fx.ipPending))
		expectStatus(t, "pending ip discussions search", rec, http.StatusNotFound)
		assertNoLeak(t, "pending ip discussions search", rec, false)
	})

	t.Run("social surface", func(t *testing.T) {
		// comments on non-public content must vanish for anonymous callers.
		for label, id := range map[string]int64{
			"pending": fx.cPending, "private": fx.cPriv, "banned": fx.cBanned,
			"soft-deleted": fx.cDel,
		} {
			rec := anonGet(fmt.Sprintf("/api/v1/social/comments?content_item_id=%d", id))
			expectStatus(t, label+" comments", rec, http.StatusOK)
			if total := jsonIntField(t, rec.Body.Bytes(), "total"); total != 0 {
				t.Errorf("[%s comments] total = %d, want 0 (non-public parent must hide its comments)", label, total)
			}
			assertNoLeak(t, label+" comments", rec, false)
		}
		// control: comments on the published fixture remain readable.
		rec := anonGet(fmt.Sprintf("/api/v1/social/comments?content_item_id=%d", fx.cPub))
		expectStatus(t, "pub comments", rec, http.StatusOK)
		assertNoLeak(t, "pub comments", rec, true)

		rec = anonGet(fmt.Sprintf("/api/v1/social/discussions?content_id=%d", fx.cPriv))
		expectStatus(t, "priv discussions list", rec, http.StatusOK)
		if total := jsonIntField(t, rec.Body.Bytes(), "total"); total != 0 {
			t.Errorf("[priv discussions list] total = %d, want 0 (discussion on non-public content hidden)", total)
		}
		assertNoLeak(t, "priv discussions list", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/social/discussions/%d", fx.discHidden))
		expectStatus(t, "hidden discussion via social detail", rec, http.StatusNotFound)
		assertNoLeak(t, "hidden discussion via social detail", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/discussions/%d", fx.discHidden))
		expectStatus(t, "hidden discussion via canonical detail", rec, http.StatusNotFound)
		assertNoLeak(t, "hidden discussion via canonical detail", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/users/%d/discussions", fx.authorID))
		expectStatus(t, "author discussions", rec, http.StatusOK)
		assertNoLeak(t, "author discussions", rec, false)
	})

	t.Run("aggregates and tags", func(t *testing.T) {
		rec := anonGet("/api/v1/stats/summary")
		expectStatus(t, "stats summary", rec, http.StatusOK)
		if n := jsonIntField(t, rec.Body.Bytes(), "contents"); n != 1 {
			t.Errorf("[stats summary] contents = %d, want 1 (only the published public fixture is anonymously countable); body=%.300s", n, rec.Body.String())
		}
		assertNoLeak(t, "stats summary", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/users/%d", fx.authorID))
		expectStatus(t, "author profile", rec, http.StatusOK)
		if n := jsonIntField(t, rec.Body.Bytes(), "contents_count"); n != 1 {
			t.Errorf("[author profile] contents_count = %d, want 1 (non-public items must not inflate profile counts); body=%.300s", n, rec.Body.String())
		}
		assertNoLeak(t, "author profile", rec, false)

		rec = anonGet("/api/v1/tags/faceted?selected_tags=sp16aleakshared")
		expectStatus(t, "tags faceted", rec, http.StatusOK)
		if strings.Contains(rec.Body.String(), "sp16aleakprivonly") {
			t.Errorf("[tags faceted] co-occurrence surfaced private-only tag of is_public=false content; body=%.400s", rec.Body.String())
		}
		assertNoLeak(t, "tags faceted", rec, false)
	})

	t.Run("collection and series items", func(t *testing.T) {
		// anonymous listing without owner_id is rejected outright (control).
		rec := anonGet("/api/v1/collections")
		expectStatus(t, "collection list", rec, http.StatusUnauthorized)
		assertNoLeak(t, "collection list", rec, false)

		rec = anonGet(fmt.Sprintf("/api/v1/collections/%d", optAuthTestCollectionID))
		expectStatus(t, "collection detail", rec, http.StatusOK)
		assertNoLeak(t, "collection detail", rec, true)

		rec = anonGet(fmt.Sprintf("/api/v1/series/%d", optAuthTestSeriesID))
		expectStatus(t, "series detail", rec, http.StatusOK)
		assertNoLeak(t, "series detail", rec, true)
	})

	t.Run("author self-access preserved", func(t *testing.T) {
		token := makeRoutesSecurityToken(cfg, fx.authorID, "user")
		authGet := func(path string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		for _, tc := range []struct {
			label string
			path  string
			want  int
		}{
			{"own pending detail", fmt.Sprintf("/api/v1/contents/%d", fx.cPending), http.StatusOK},
			{"own private detail", fmt.Sprintf("/api/v1/contents/%d", fx.cPriv), http.StatusOK},
			{"own private versions", fmt.Sprintf("/api/v1/contents/%d/versions", fx.cPriv), http.StatusOK},
			{"own private prs", fmt.Sprintf("/api/v1/contents/%d/prs", fx.cPriv), http.StatusOK},
			{"own private comments", fmt.Sprintf("/api/v1/social/comments?content_item_id=%d", fx.cPriv), http.StatusOK},
			{"own pending ip detail", fmt.Sprintf("/api/v1/ips/%d", fx.ipPending), http.StatusOK},
			{"own pending ip proposals", fmt.Sprintf("/api/v1/ips/%d/proposals", fx.ipPending), http.StatusOK},
			{"own private version detail", fmt.Sprintf("/api/v1/versions/%d", fx.vPriv), http.StatusOK},
			{"own private pr detail", fmt.Sprintf("/api/v1/pr/%d", fx.prPriv), http.StatusOK},
		} {
			rec := authGet(tc.path)
			if rec.Code != tc.want {
				t.Errorf("[author %s] status = %d, want %d; body=%.300s", tc.label, rec.Code, tc.want, rec.Body.String())
			}
		}
	})
}

const (
	optAuthTestCollectionID = 421
	optAuthTestSeriesID     = 431
)

func buildOptAuthLeakAuditStack(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config, *optAuthLeakFixtures) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// 详情缓存走包级 redisclient.Client（先例：admin_content_cache_invalidation_test.go）。
	previousClient := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previousClient })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.IPTag{}, &model.IPProposal{}, &model.IPProposalVote{}, &model.IPProfileVersion{},
		&model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{}, &model.Tag{},
		&model.ContentVersion{}, &model.PullRequest{},
		&model.Reaction{}, &model.Follow{},
		&model.Collection{}, &model.CollectionItem{},
		&model.ContentSeries{}, &model.ContentSeriesItem{},
		&model.Category{},
	); err != nil {
		t.Fatalf("migrate audit models: %v", err)
	}
	// comments/discussions 携带 DEFAULT NOW() DDL，sqlite 不认，手写兼容表
	// （先例：discussion_moderation_gate_test.go）。
	mustExec(t, db, `
		CREATE TABLE comments (
			id integer PRIMARY KEY AUTOINCREMENT,
			content_item_id integer,
			discussion_id integer,
			parent_id integer,
			author_id integer NOT NULL,
			target_type text,
			target_id integer,
			content text,
			body text NOT NULL,
			status text NOT NULL DEFAULT 'published',
			like_count integer NOT NULL DEFAULT 0,
			created_at datetime,
			updated_at datetime
		)`)
	mustExec(t, db, `
		CREATE TABLE discussions (
			id integer PRIMARY KEY AUTOINCREMENT,
			ip_id integer,
			content_item_id integer,
			author_id integer NOT NULL,
			title text NOT NULL,
			body text,
			status text NOT NULL DEFAULT 'published',
			is_pinned numeric NOT NULL DEFAULT 0,
			view_count integer NOT NULL DEFAULT 0,
			reply_count integer NOT NULL DEFAULT 0,
			last_active_at datetime NOT NULL DEFAULT (datetime('now')),
			created_at datetime,
			updated_at datetime
		)`)

	cfg := &config.Config{}
	cfg.JWT.Secret = "optauth-leak-audit-secret"
	cfg.RateLimit.SearchPerMinute = 1000
	cfg.RateLimit.MaxQueryChars = 200

	ctr := container.NewContainer(db, rdb, cfg)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, cfg, ctr)

	fx := seedOptAuthLeakFixtures(t, db)
	return router, db, cfg, fx
}

func seedOptAuthLeakFixtures(t *testing.T, db *gorm.DB) *optAuthLeakFixtures {
	t.Helper()
	now := time.Now()
	deletedAt := now

	mkUser := func(id int64, username string, banned bool, role string) {
		u := model.User{
			ID: id, Email: username + "@example.com", Username: username,
			PasswordHash: "hash", Reputation: 10, Role: role, IsBanned: banned,
			EmailVerifiedAt: &now,
		}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user %s: %v", username, err)
		}
	}
	mkUser(501, "sp16a_author", false, "user")
	mkUser(502, "sp16a_banned_author", true, "user")
	mkUser(503, "sp16a_viewer", false, "user")

	fx := &optAuthLeakFixtures{authorID: 501, bannedAuthorID: 502, viewerID: 503}

	// six non-public states + one published public control, all with the
	// marker embedded in every leakable field (title, description, version
	// body, PR message, comment body, attachment oss_key).
	contentSpec := []struct {
		id      int64
		marker  string
		status  string
		isPub   bool
		deleted bool
		author  int64
	}{
		{601, "sp16aleakPUB", "published", true, false, 501},
		{602, "sp16aleakPENDING", "pending", true, false, 501},
		{603, "sp16aleakUNDER", "under_review", true, false, 501},
		{604, "sp16aleakPRIV", "published", false, false, 501},
		{605, "sp16aleakDEL", "published", true, true, 501},
		{606, "sp16aleakBANNED", "banned", true, false, 501},
		{607, "sp16aleakBAUTH", "published", true, false, 502},
	}
	for _, s := range contentSpec {
		c := model.ContentItem{
			ID: s.id, Title: s.marker + " title", Description: s.marker + " description",
			AuthorID: s.author, Zone: "original", Category: "game", ContentType: "article",
			Status: s.status, IsPublic: s.isPub, AllowCopy: true,
		}
		if s.deleted {
			c.DeletedAt = &deletedAt
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed content %d: %v", s.id, err)
		}

		att := model.ContentAttachment{
			ID: 700 + s.id, ContentItemID: s.id, FileType: "file",
			OSSKey: s.marker + "_att", ScanStatus: "not_required",
		}
		if err := db.Create(&att).Error; err != nil {
			t.Fatalf("seed attachment for %d: %v", s.id, err)
		}

		v := model.ContentVersion{
			ID: 800 + s.id, ContentItemID: s.id, AuthorID: s.author,
			VersionNumber: 1, StorageType: "full", StorageKey: s.marker + "_vfull",
			Status: "active", IsLatest: true,
		}
		if err := db.Create(&v).Error; err != nil {
			t.Fatalf("seed version for %d: %v", s.id, err)
		}

		pr := model.PullRequest{
			ID: 900 + s.id, ContentItemID: s.id, SubmitterID: 503,
			Status: "open", Message: s.marker + "_prmsg",
		}
		if err := db.Create(&pr).Error; err != nil {
			t.Fatalf("seed pr for %d: %v", s.id, err)
		}

		cid := s.id
		cmt := model.Comment{
			ID: 950 + s.id, ContentItemID: &cid, AuthorID: 503,
			TargetType: "content", TargetID: s.id, Body: s.marker + "_cmt", Status: "published",
		}
		if err := db.Create(&cmt).Error; err != nil {
			t.Fatalf("seed comment for %d: %v", s.id, err)
		}
	}

	fx.cPub, fx.cPending, fx.cUnder, fx.cPriv, fx.cDel, fx.cBanned, fx.cBAuth = 601, 602, 603, 604, 605, 606, 607
	fx.vPriv = 800 + 604 // version rows are seeded as 800+contentID
	fx.prPub, fx.prPriv = 900+601, 900+604

	// tags: shared tag on pub+private; priv-only tag must never surface in
	// anonymous co-occurrence counts.
	for _, tg := range []struct{ name string; usage int }{
		{"sp16aleakshared", 2}, {"sp16aleakprivonly", 1},
	} {
		if err := db.Create(&model.Tag{Name: tg.name, Category: "theme", UsageCount: tg.usage}).Error; err != nil {
			t.Fatalf("seed tag %s: %v", tg.name, err)
		}
	}
	for _, ct := range []struct {
		contentID int64
		tag       string
	}{
		{601, "sp16aleakshared"}, {604, "sp16aleakshared"}, {604, "sp16aleakprivonly"},
	} {
		if err := db.Create(&model.ContentTag{ContentItemID: ct.contentID, Tag: ct.tag}).Error; err != nil {
			t.Fatalf("seed content tag: %v", err)
		}
	}

	// discussions: hidden one (route-parity leak) + published one attached to
	// the private content (parent-visibility leak) + one on the pending IP.
	discHidden := model.Discussion{
		ID: 961, ContentItemID: &fx.cPriv, AuthorID: 503,
		Title: "sp16aleakDISCHIDDEN title", Body: "sp16aleakDISCHIDDEN body",
		Status: "hidden", LastActiveAt: now,
	}
	if err := db.Create(&discHidden).Error; err != nil {
		t.Fatalf("seed hidden discussion: %v", err)
	}
	discOnPriv := model.Discussion{
		ID: 962, ContentItemID: &fx.cPriv, AuthorID: 503,
		Title: "sp16aleakDISCPUBPRIV title", Body: "sp16aleakDISCPUBPRIV body",
		Status: "published", LastActiveAt: now,
	}
	if err := db.Create(&discOnPriv).Error; err != nil {
		t.Fatalf("seed discussion on private: %v", err)
	}
	discOnPendingIP := model.Discussion{
		ID: 963, IPID: ptrInt64(402), AuthorID: 503,
		Title: "sp16aleakIPPEND discussion", Body: "sp16aleakIPPEND body",
		Status: "published", LastActiveAt: now,
	}
	if err := db.Create(&discOnPendingIP).Error; err != nil {
		t.Fatalf("seed discussion on pending ip: %v", err)
	}
	fx.discHidden, fx.discOnPriv = 961, 962

	// IPs: approved control + pending leak fixture.
	ipApproved := model.IP{ID: 401, Name: "sp16aleakIPAP name", Slug: "sp16a-ip-ap", Description: "sp16aleakIPAP description", Category: "game", CreatorID: ptrInt64(501), Status: "approved"}
	if err := db.Create(&ipApproved).Error; err != nil {
		t.Fatalf("seed approved ip: %v", err)
	}
	ipPending := model.IP{ID: 402, Name: "sp16aleakIPPEND name", Slug: "sp16a-ip-pend", Description: "sp16aleakIPPEND description", Category: "game", CreatorID: ptrInt64(501), Status: "pending"}
	if err := db.Create(&ipPending).Error; err != nil {
		t.Fatalf("seed pending ip: %v", err)
	}
	fx.ipApproved, fx.ipPending = 401, 402

	descPend := "sp16aleakIPPEND proposal change"
	propPending := model.IPProposal{
		ID: 911, IPID: 402, ProposerID: 503, Status: "open",
		DescriptionChange: &descPend, ModerationState: "approved",
		DeadlineAt: now.Add(48 * time.Hour),
	}
	if err := db.Create(&propPending).Error; err != nil {
		t.Fatalf("seed pending ip proposal: %v", err)
	}
	descAp := "sp16aleakIPAP proposal change"
	propApproved := model.IPProposal{
		ID: 912, IPID: 401, ProposerID: 503, Status: "open",
		DescriptionChange: &descAp, ModerationState: "approved",
		DeadlineAt: now.Add(48 * time.Hour),
	}
	if err := db.Create(&propApproved).Error; err != nil {
		t.Fatalf("seed approved ip proposal: %v", err)
	}
	fx.propPending, fx.propApproved = 911, 912

	ipVersion := model.IPProfileVersion{
		ID: 921, IPID: 402, ProposalID: 911,
		Snapshot: "{}", Changes: "{}",
	}
	if err := db.Create(&ipVersion).Error; err != nil {
		t.Fatalf("seed ip profile version: %v", err)
	}

	// collection + series embedding the private fixture (item-level gate).
	col := model.Collection{ID: optAuthTestCollectionID, UserID: 503, Title: "sp16aleakCOL", Zone: "original", IsPublic: true, SortOrder: 1}
	if err := db.Create(&col).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	for i, cid := range []int64{601, 604} {
		item := model.CollectionItem{ID: int64(4210 + i), CollectionID: col.ID, ContentItemID: cid, Note: "sp16aleakCOL note " + fmt.Sprint(cid)}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed collection item: %v", err)
		}
	}

	series := model.ContentSeries{ID: optAuthTestSeriesID, Title: "sp16aleakSERIES", OwnerID: 503, Zone: "original"}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("seed series: %v", err)
	}
	for i, cid := range []int64{601, 604} {
		item := model.ContentSeriesItem{ID: int64(4310 + i), SeriesID: series.ID, ContentItemID: cid, SortOrder: i}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed series item: %v", err)
		}
	}

	return fx
}

func ptrInt64(v int64) *int64 { return &v }

func mustExec(t *testing.T, db *gorm.DB, ddl string) {
	t.Helper()
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("exec ddl: %v", err)
	}
}

// jsonIntField walks an arbitrary JSON payload for the first numeric field
// with the given key (responses nest stats at varying depths).
func jsonIntField(t *testing.T, raw []byte, key string) int64 {
	t.Helper()
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal json for %q: %v; raw=%.300s", key, err, raw)
	}
	var walk func(node any) (int64, bool)
	walk = func(node any) (int64, bool) {
		switch v := node.(type) {
		case map[string]any:
			if n, ok := v[key].(float64); ok {
				return int64(n), true
			}
			for _, child := range v {
				if n, ok := walk(child); ok {
					return n, true
				}
			}
		case []any:
			for _, child := range v {
				if n, ok := walk(child); ok {
					return n, true
				}
			}
		}
		return 0, false
	}
	n, ok := walk(payload)
	if !ok {
		t.Fatalf("key %q not found in payload; raw=%.300s", key, raw)
	}
	return n
}
