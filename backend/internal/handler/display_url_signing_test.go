package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	redisclient "omnicraft/backend/internal/pkg/redis"
)

// B-002: display media (IP covers, content covers, avatars, gallery
// attachments) live in the private OSS bucket. Browsers and Next /_next/image
// get OSS 403 on unsigned URLs, so every display URL crossing the API
// serialization boundary must be a short-lived signed GET URL. DB and Redis
// keep canonical bare URLs; signing happens per response. Empty and external
// (non-platform) URLs must pass through untouched and OSS being unconfigured
// must fail open to the bare URL (local dev / CI).
//
// The endpoint is an IP-shaped host so the OSS SDK emits path-style URLs
// (/bucket/key), letting aliyun.ObjectKeyFromURL round-trip the signed URL —
// same trick as TestResolveScanURLSignsPlatformObjectURLAndPassesExternal.
const (
	displaySigningOSSDomain   = "http://127.0.0.1:9201/test-bucket"
	displaySigningOSSEndpoint = "http://127.0.0.1:9201"
	displaySigningBucket      = "test-bucket"
)

func displaySigningConfig() *config.Config {
	cfg := &config.Config{}
	cfg.JWT.Secret = "display-url-signing-test-secret"
	cfg.OSS = config.OSSConfig{
		Endpoint:        displaySigningOSSEndpoint,
		AccessKeyID:     "test-access-key",
		AccessKeySecret: "test-access-secret",
		BucketName:      displaySigningBucket,
		Domain:          displaySigningOSSDomain,
	}
	return cfg
}

func openDisplaySigningDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

func openDisplaySigningRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	// The service cache helpers go through the package-level client.
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })
	return rdb
}

// requireSignedDisplayURL asserts raw is a signed GET URL for the given object
// key: it must carry Expires/OSSAccessKeyId/Signature query parameters and keep
// the object key in its path.
func requireSignedDisplayURL(t *testing.T, raw, wantKey, context string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err, context)
	query := parsed.Query()
	require.NotEmpty(t, query.Get("Signature"), context+": missing Signature")
	require.NotEmpty(t, query.Get("OSSAccessKeyId"), context+": missing OSSAccessKeyId")
	require.NotEmpty(t, query.Get("Expires"), context+": missing Expires")
	// The IP-shaped test endpoint makes the SDK sign path-style URLs
	// (/bucket/key); the object key must survive signing untouched.
	require.Equal(t, "/"+displaySigningBucket+"/"+wantKey, parsed.Path, context+": object key drifted")
}

func seedDisplaySigningUser(t *testing.T, db *gorm.DB, id int64, avatarURL string) model.User {
	t.Helper()
	now := time.Now()
	user := model.User{
		ID:              id,
		Email:           "display-" + strconv.FormatInt(id, 10) + "@example.com",
		Username:        "display-user-" + strconv.FormatInt(id, 10),
		PasswordHash:    "hash",
		AvatarURL:       avatarURL,
		Reputation:      10,
		Role:            "user",
		EmailVerifiedAt: &now,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedDisplaySigningIP(t *testing.T, db *gorm.DB, id int64, coverURL, status string) model.IP {
	t.Helper()
	ip := model.IP{
		ID:       id,
		Name:     "display-ip-" + strconv.FormatInt(id, 10),
		Slug:     "display-ip-" + strconv.FormatInt(id, 10),
		CoverURL: coverURL,
		Category: "gaming",
		Status:   status,
	}
	require.NoError(t, db.Create(&ip).Error)
	return ip
}

func TestGetIPResponseSignsPlatformCoverURLEveryTime(t *testing.T) {
	cfg := displaySigningConfig()
	db := openDisplaySigningDB(t, &model.User{}, &model.IP{})
	rdb := openDisplaySigningRedis(t)

	const coverKey = "uploads/7/image/ip-cover.png"
	seedDisplaySigningIP(t, db, 11, aliyun.ObjectURL(cfg.OSS.Domain, coverKey), "approved")

	handler := NewIPHandlerWithCache(db, rdb, cfg)
	router := gin.New()
	router.GET("/api/v1/ips/:id", middleware.OptionalAuth(cfg, rdb, db), handler.GetIP)

	// Two sequential reads: the second one is served from the Redis detail
	// cache, and must still carry a freshly issued signature (the signature
	// must never be frozen into the cache).
	for attempt := 1; attempt <= 2; attempt++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ips/11", nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var payload struct {
			IP struct {
				CoverURL string `json:"cover_url"`
			} `json:"ip"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		requireSignedDisplayURL(t, payload.IP.CoverURL, coverKey,
			"GetIP attempt "+strconv.Itoa(attempt))
	}
}

func TestListIPsSignsOnlyPlatformCoverURLs(t *testing.T) {
	cfg := displaySigningConfig()
	db := openDisplaySigningDB(t, &model.User{}, &model.IP{})
	rdb := openDisplaySigningRedis(t)

	const platformKey = "uploads/7/image/list-cover.png"
	const externalURL = "https://external.example.net/seed-cover.png"
	seedDisplaySigningIP(t, db, 21, aliyun.ObjectURL(cfg.OSS.Domain, platformKey), "approved")
	seedDisplaySigningIP(t, db, 22, externalURL, "approved")
	seedDisplaySigningIP(t, db, 23, "", "approved")

	handler := NewIPHandlerWithCache(db, rdb, cfg)
	router := gin.New()
	router.GET("/api/v1/ips", middleware.OptionalAuth(cfg, rdb, db), handler.ListIPs)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ips", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		IPs []struct {
			ID       int64  `json:"id"`
			CoverURL string `json:"cover_url"`
		} `json:"ips"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.IPs, 3)

	covers := map[int64]string{}
	for _, ip := range payload.IPs {
		covers[ip.ID] = ip.CoverURL
	}
	requireSignedDisplayURL(t, covers[21], platformKey, "platform cover")
	require.Equal(t, externalURL, covers[22], "external seed URL must pass through unsigned")
	require.Equal(t, "", covers[23], "empty cover must stay empty")
}

func TestGetContentSignsCoverAvatarAndAttachmentURLs(t *testing.T) {
	cfg := displaySigningConfig()
	db := openDisplaySigningDB(t,
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{},
		&model.BrowseHistory{}, &model.ContentSeries{}, &model.ContentSeriesItem{},
		&model.Collection{}, &model.CollectionItem{},
	)

	const (
		avatarKey     = "uploads/31/avatar/me.png"
		coverKey      = "uploads/31/image/cover.png"
		attachmentKey = "uploads/31/image/gallery-01.png"
	)
	seedDisplaySigningUser(t, db, 31, aliyun.ObjectURL(cfg.OSS.Domain, avatarKey))
	content := model.ContentItem{
		ID:            41,
		Title:         "display signing content",
		AuthorID:      31,
		Zone:          "original",
		Category:      "gaming",
		ContentType:   "image",
		CoverImageURL: aliyun.ObjectURL(cfg.OSS.Domain, coverKey),
		Status:        "published",
		IsPublic:      true,
		AllowCopy:     true,
	}
	require.NoError(t, db.Create(&content).Error)
	attachment := model.ContentAttachment{
		ContentItemID: 41,
		FileType:      "image",
		OSSKey:        attachmentKey,
		MimeType:      "image/png",
	}
	require.NoError(t, db.Create(&attachment).Error)

	handler := NewContentHandler(db, cfg, nil)
	router := gin.New()
	router.GET("/api/v1/contents/:id", middleware.OptionalAuth(cfg, nil, db), handler.GetContent)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/contents/41", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Content struct {
			CoverImageURL string `json:"cover_image_url"`
			Author        struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"author"`
		} `json:"content"`
		Attachments []struct {
			OSSKey string `json:"oss_key"`
			OSSURL string `json:"oss_url"`
		} `json:"attachments"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	requireSignedDisplayURL(t, payload.Content.CoverImageURL, coverKey, "content cover")
	requireSignedDisplayURL(t, payload.Content.Author.AvatarURL, avatarKey, "author avatar")
	require.Len(t, payload.Attachments, 1)
	require.Equal(t, attachmentKey, payload.Attachments[0].OSSKey, "oss_key must stay canonical")
	requireSignedDisplayURL(t, payload.Attachments[0].OSSURL, attachmentKey, "attachment oss_url")
}

func TestDisplayURLSigningFailsOpenWhenOSSUnconfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.JWT.Secret = "display-url-signing-unconfigured-secret"

	db := openDisplaySigningDB(t, &model.User{}, &model.IP{})
	const bareCover = "uploads/7/image/unconfigured-cover.png"
	bareURL := aliyun.ObjectURL("https://cdn.example.test", bareCover)
	seedDisplaySigningIP(t, db, 61, bareURL, "approved")

	rdb := openDisplaySigningRedis(t)
	handler := NewIPHandlerWithCache(db, rdb, cfg)
	router := gin.New()
	router.GET("/api/v1/ips/:id", middleware.OptionalAuth(cfg, rdb, db), handler.GetIP)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ips/61", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		IP struct {
			CoverURL string `json:"cover_url"`
		} `json:"ip"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, bareURL, payload.IP.CoverURL, "unconfigured OSS must fail open to the bare URL")
}

func TestAdminPendingIPListSignsPlatformCoverURLs(t *testing.T) {
	cfg := displaySigningConfig()
	db := openDisplaySigningDB(t, &model.User{}, &model.IP{})

	const pendingKey = "uploads/7/image/pending-cover.png"
	seedDisplaySigningIP(t, db, 71, aliyun.ObjectURL(cfg.OSS.Domain, pendingKey), "pending")

	handler := NewAdminHandler(db, cfg, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/ips", handler.ListPendingIPs)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/ips", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		IPs []struct {
			CoverURL string `json:"cover_url"`
		} `json:"ips"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.IPs, 1)
	requireSignedDisplayURL(t, payload.IPs[0].CoverURL, pendingKey, "admin pending ip cover")
}
