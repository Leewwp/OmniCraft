package service

import (
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/repository"
)

// #381 auth 并发 refresh token rotation race：
// ① 同秒签发的 refresh token 必须互不相同（jti），否则轮换「拉黑旧」会连刚签发的「新」一起拉黑；
// ② 并发 N 路同 token refresh 只允许一次真实轮换，其余复用同一新 token 对，且零错误；
// ③ 已轮换的旧 token 在 grace 窗口内重放返回同一新 token 对（导航打断错过 Set-Cookie 的自愈），
//    grace 过期后必须拒绝。

func setupRefreshRaceService(t *testing.T) (*AuthService, *miniredis.Miniredis, *model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user model: %v", err)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cfg := &config.Config{}
	cfg.JWT.Secret = "refresh-race-secret"
	cfg.JWT.AccessTokenTTL = 120
	cfg.JWT.RefreshTokenTTL = 7

	user := &model.User{
		Email:            "refresh-race@example.test",
		Username:         "refresh-race-user",
		PasswordHash:     "hash",
		Role:             "user",
		EmailVerifiedAt:  timePtrForRefreshRace(time.Now()),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return NewAuthService(repository.NewUserRepository(db), rdb, cfg), mr, user
}

func timePtrForRefreshRace(t time.Time) *time.Time { return &t }

// ①：同秒内 login→refresh→refresh 链路必须存活；新 token 不得与旧 token 同串。
func TestRefreshTokenRotationSameSecondDoesNotSelfBlacklist(t *testing.T) {
	svc, _, user := setupRefreshRaceService(t)

	pair1, err := svc.IssueTokenPairForUser(user)
	if err != nil {
		t.Fatalf("issue pair: %v", err)
	}
	pair2, err := svc.RefreshToken(pair1.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if pair2.RefreshToken == pair1.RefreshToken {
		t.Fatal("rotation minted a refresh token identical to the one being rotated (missing jti)")
	}
	if _, err := svc.RefreshToken(pair2.RefreshToken); err != nil {
		t.Fatalf("second refresh in the same second failed (self-blacklisted): %v", err)
	}
}

// ②：并发 N 路同 token refresh——恰好一次真实轮换，其余复用同一新 token 对，零错误。
func TestRefreshTokenConcurrentRotationSingleFlightOnServer(t *testing.T) {
	svc, _, user := setupRefreshRaceService(t)

	pair1, err := svc.IssueTokenPairForUser(user)
	if err != nil {
		t.Fatalf("issue pair: %v", err)
	}

	const n = 8
	type result struct {
		pair *jwtutil.TokenPair
		err  error
	}
	results := make([]result, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			pair, err := svc.RefreshToken(pair1.RefreshToken)
			results[i] = result{pair, err}
		}(i)
	}
	close(start)
	wg.Wait()

	for i, r := range results {
		if r.err != nil {
			t.Fatalf("concurrent refresh #%d errored (401 leak): %v", i, r.err)
		}
		if r.pair == nil || r.pair.RefreshToken == "" {
			t.Fatalf("concurrent refresh #%d returned empty pair", i)
		}
		if r.pair.RefreshToken != results[0].pair.RefreshToken {
			t.Fatalf("concurrent refresh #%d diverged into a second rotation lineage", i)
		}
	}
	if results[0].pair.RefreshToken == pair1.RefreshToken {
		t.Fatal("rotation did not mint a new refresh token")
	}
}

// ③：已轮换旧 token 在 grace 窗口内重放 → 返回同一新 token 对；grace 过期 → 拒绝。
func TestRefreshTokenStaleReplayWithinGraceReturnsRotatedPair(t *testing.T) {
	svc, mr, user := setupRefreshRaceService(t)

	pair1, err := svc.IssueTokenPairForUser(user)
	if err != nil {
		t.Fatalf("issue pair: %v", err)
	}
	pair2, err := svc.RefreshToken(pair1.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}

	replayed, err := svc.RefreshToken(pair1.RefreshToken)
	if err != nil {
		t.Fatalf("stale token replay within grace must return the rotated pair, got: %v", err)
	}
	if replayed.RefreshToken != pair2.RefreshToken {
		t.Fatal("stale replay must return the same rotated refresh token")
	}

	mr.FastForward(61 * time.Second)
	if _, err := svc.RefreshToken(pair1.RefreshToken); err == nil {
		t.Fatal("stale token after grace window must be rejected")
	}
}
