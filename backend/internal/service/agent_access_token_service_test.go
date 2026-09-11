package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// agentTokenFixedNow anchors the injected service clock so last_used_at
// throttling is deterministic in tests.
var agentTokenFixedNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

var agentTokenBase62Re = regexp.MustCompile(`^[0-9A-Za-z]+$`)

func setupAgentAccessTokenServiceTest(t *testing.T) (*AgentAccessTokenService, *gorm.DB) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "079_agent_access_tokens.sql"))

	cfg := &config.Config{
		AgentAccess: config.AgentAccessConfig{MaxTokensPerUser: 2},
	}
	svc := NewAgentAccessTokenService(repository.NewAgentAccessTokenRepository(db), cfg)
	svc.now = func() time.Time { return agentTokenFixedNow }
	return svc, db
}

func seedAgentTokenUser(t *testing.T, db *gorm.DB, username string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.Raw(
		`INSERT INTO users (email, username) VALUES (?, ?) RETURNING id`,
		username+"@example.com", username,
	).Scan(&id).Error)
	return id
}

func TestIssueAgentAccessTokenShape(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_shape_owner")

	issued, err := svc.Issue(context.Background(), userID, "my agent token", []string{"download", "upload"})
	require.NoError(t, err)

	// format: oc_pat_ prefix + 43 base62 chars = 50 total (spec D2)
	require.Equal(t, "oc_pat_", issued.Token[:7])
	require.Len(t, issued.Token, 50)
	require.Regexp(t, agentTokenBase62Re, issued.Token[7:])

	// list identity keeps the first 12 chars including the prefix
	require.Equal(t, issued.Token[:12], issued.Info.TokenPrefix)
	require.Equal(t, "my agent token", issued.Info.Name)
	require.Equal(t, []string{"download", "upload"}, issued.Info.Scopes)
	require.Nil(t, issued.Info.LastUsedAt)

	// storage keeps only the SHA-256 of the full token, never the plaintext
	sum := sha256.Sum256([]byte(issued.Token))
	var storedHash string
	require.NoError(t, db.Raw(
		`SELECT token_hash FROM agent_access_tokens WHERE user_id = ?`, userID,
	).Scan(&storedHash).Error)
	require.Equal(t, hex.EncodeToString(sum[:]), storedHash)

	var matches int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_access_tokens WHERE name LIKE '%' || ? || '%' OR token_hash = ?`,
		issued.Token, issued.Token,
	).Scan(&matches).Error)
	require.Zero(t, matches, "plaintext token must never be persisted")
}

func TestIssueAgentAccessTokenScopeNormalization(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_scope_owner")

	issued, err := svc.Issue(context.Background(), userID, "downloads only", []string{"download"})
	require.NoError(t, err)
	require.Equal(t, []string{"download"}, issued.Info.Scopes)

	// duplicate scopes collapse; order is canonical (download before upload)
	issued, err = svc.Issue(context.Background(), userID, "dup order", []string{"upload", "download", "upload"})
	require.NoError(t, err)
	require.Equal(t, []string{"download", "upload"}, issued.Info.Scopes)
}

func TestIssueAgentAccessTokenValidation(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_validate_owner")
	ctx := context.Background()

	_, err := svc.Issue(ctx, userID, "", []string{"download"})
	require.ErrorIs(t, err, ErrAgentTokenNameInvalid)

	_, err = svc.Issue(ctx, userID, string(make([]byte, 65)), []string{"download"})
	require.ErrorIs(t, err, ErrAgentTokenNameInvalid)

	_, err = svc.Issue(ctx, userID, "no scopes", nil)
	require.ErrorIs(t, err, ErrAgentTokenScopesInvalid)

	_, err = svc.Issue(ctx, userID, "bad scope", []string{"read"})
	require.ErrorIs(t, err, ErrAgentTokenScopesInvalid)

	_, err = svc.Issue(ctx, 0, "no user", []string{"download"})
	require.ErrorIs(t, err, ErrAgentTokenNameInvalid, "any validation error is acceptable for a zero user")
}

func TestIssueAgentAccessTokenLimit(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_limit_owner")
	ctx := context.Background()

	_, err := svc.Issue(ctx, userID, "one", []string{"download"})
	require.NoError(t, err)
	second, err := svc.Issue(ctx, userID, "two", []string{"upload"})
	require.NoError(t, err)

	_, err = svc.Issue(ctx, userID, "three", []string{"download"})
	require.ErrorIs(t, err, ErrAgentTokenLimitReached)

	// revoking frees a slot
	require.NoError(t, svc.Revoke(ctx, userID, second.Info.ID))
	_, err = svc.Issue(ctx, userID, "three", []string{"download"})
	require.NoError(t, err)
}

func TestAuthenticateAgentAccessToken(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_auth_owner")
	ctx := context.Background()

	issued, err := svc.Issue(ctx, userID, "roundtrip", []string{"download", "upload"})
	require.NoError(t, err)

	identity, err := svc.Authenticate(ctx, issued.Token)
	require.NoError(t, err)
	require.Equal(t, userID, identity.UserID)
	require.Equal(t, issued.Info.ID, identity.TokenID)
	require.Equal(t, []string{"download", "upload"}, identity.Scopes)

	// a random same-shape token must not authenticate
	other, err := svc.Issue(ctx, userID, "other", []string{"download"})
	require.NoError(t, err)
	require.NotEqual(t, issued.Token, other.Token)
	_, err = svc.Authenticate(ctx, other.Token)
	require.NoError(t, err)

	// malformed tokens are rejected before any lookup
	for _, malformed := range []string{"", "oc_pat_short", "oc_pat_" + makeString(44), "jwt-looking-token", "OC_PAT_" + makeString(43)} {
		_, err = svc.Authenticate(ctx, malformed)
		require.ErrorIs(t, err, ErrAgentTokenFormatInvalid, "malformed token %q", malformed[:min(len(malformed), 12)])
	}

	// unknown-but-wellformed token maps to the not-found error
	_, err = svc.Authenticate(ctx, "oc_pat_"+makeString(43))
	require.ErrorIs(t, err, ErrAgentTokenNotFound)
}

func TestRevokeAgentAccessToken(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_revoke_owner")
	otherUserID := seedAgentTokenUser(t, db, "pat_revoke_other")
	ctx := context.Background()

	issued, err := svc.Issue(ctx, userID, "to revoke", []string{"download"})
	require.NoError(t, err)

	// another user must not revoke someone else's token
	err = svc.Revoke(ctx, otherUserID, issued.Info.ID)
	require.ErrorIs(t, err, ErrAgentTokenNotFound)

	require.NoError(t, svc.Revoke(ctx, userID, issued.Info.ID))

	// revoked tokens stop authenticating immediately (leak stop drill)
	_, err = svc.Authenticate(ctx, issued.Token)
	require.ErrorIs(t, err, ErrAgentTokenNotFound)

	// revoked tokens disappear from the list and cannot be revoked twice
	tokens, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, tokens)
	err = svc.Revoke(ctx, userID, issued.Info.ID)
	require.ErrorIs(t, err, ErrAgentTokenNotFound)

	// unknown token id
	err = svc.Revoke(ctx, userID, 424242)
	require.ErrorIs(t, err, ErrAgentTokenNotFound)
}

func TestAuthenticateAgentAccessTokenTouchesLastUsed(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_touch_owner")
	ctx := context.Background()

	issued, err := svc.Issue(ctx, userID, "touch", []string{"download"})
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, issued.Token)
	require.NoError(t, err)

	var lastUsed *time.Time
	require.NoError(t, db.Raw(
		`SELECT last_used_at FROM agent_access_tokens WHERE id = ?`, issued.Info.ID,
	).Scan(&lastUsed).Error)
	require.NotNil(t, lastUsed, "first authentication must set last_used_at")

	// within the throttle window the timestamp must not be rewritten
	svc.now = func() time.Time { return agentTokenFixedNow.Add(30 * time.Second) }
	_, err = svc.Authenticate(ctx, issued.Token)
	require.NoError(t, err)
	var lastUsed2 *time.Time
	require.NoError(t, db.Raw(
		`SELECT last_used_at FROM agent_access_tokens WHERE id = ?`, issued.Info.ID,
	).Scan(&lastUsed2).Error)
	require.WithinDuration(t, *lastUsed, *lastUsed2, time.Second, "throttle window must skip the write")

	// past the window it updates
	svc.now = func() time.Time { return agentTokenFixedNow.Add(2 * time.Minute) }
	_, err = svc.Authenticate(ctx, issued.Token)
	require.NoError(t, err)
	var lastUsed3 *time.Time
	require.NoError(t, db.Raw(
		`SELECT last_used_at FROM agent_access_tokens WHERE id = ?`, issued.Info.ID,
	).Scan(&lastUsed3).Error)
	require.True(t, lastUsed3.After(*lastUsed), "past the throttle window last_used_at must advance")
}

func TestListAgentTokens(t *testing.T) {
	svc, db := setupAgentAccessTokenServiceTest(t)
	userID := seedAgentTokenUser(t, db, "pat_list_owner")
	otherUser := seedAgentTokenUser(t, db, "pat_list_other")
	ctx := context.Background()

	first, err := svc.Issue(ctx, userID, "first", []string{"download"})
	require.NoError(t, err)
	second, err := svc.Issue(ctx, userID, "second", []string{"upload"})
	require.NoError(t, err)
	_, err = svc.Issue(ctx, otherUser, "not mine", []string{"download"})
	require.NoError(t, err)

	// the second user's token is invisible; newest first
	tokens, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tokens, 2)
	require.Equal(t, second.Info.ID, tokens[0].ID)
	require.Equal(t, first.Info.ID, tokens[1].ID)

	// the list projection must not carry secrets: JSON has no plaintext
	// token and no full hash
	blob, err := json.Marshal(tokens)
	require.NoError(t, err)
	require.NotContains(t, string(blob), `"token"`, "list projection must not carry the plaintext token")
	require.NotContains(t, string(blob), second.Token)
	require.NotContains(t, string(blob), first.Token)
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
