package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// Agent access token (PAT) lifecycle and authentication (SP-16 #450, spec
// D2). The token format is "oc_pat_" + 43 base62 characters (~256 bits of
// entropy, matching the SHA-256 storage). Plaintext exists only in the
// Issue response; every other surface works from the SHA-256 hex digest.

const (
	AgentTokenScopeDownload = "download"
	AgentTokenScopeUpload   = "upload"

	// AgentTokenPrefix is the shared bearer prefix identifying the machine
	// channel; the auth middleware and CSRF exemption both key off it.
	AgentTokenPrefix        = "oc_pat_"
	agentTokenSecretLength  = 43
	agentTokenTotalLength   = len(AgentTokenPrefix) + agentTokenSecretLength
	agentTokenTouchInterval = time.Minute // bounds last_used_at write amplification

	agentTokenNameMaxLength = 64
)

var (
	ErrAgentTokenNameInvalid   = errors.New("agent token name invalid")
	ErrAgentTokenScopesInvalid = errors.New("agent token scopes invalid")
	ErrAgentTokenLimitReached  = errors.New("agent token limit reached")
	ErrAgentTokenNotFound      = errors.New("agent token not found")
	ErrAgentTokenFormatInvalid = errors.New("agent token format invalid")
)

// AgentAccessScopes is the closed scope vocabulary.
var AgentAccessScopes = []string{AgentTokenScopeDownload, AgentTokenScopeUpload}

// AgentAccessTokenInfo is the secret-free projection used by list responses.
type AgentAccessTokenInfo struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// IssuedAgentAccessToken carries the one-time plaintext next to the stored
// projection; the plaintext is never persisted or logged.
type IssuedAgentAccessToken struct {
	Token string               `json:"token"`
	Info  AgentAccessTokenInfo `json:"token_info"`
}

// AgentAccessTokenIdentity is what the auth middleware resolves from a raw
// bearer token.
type AgentAccessTokenIdentity struct {
	TokenID int64
	UserID  int64
	Scopes  []string
}

type AgentAccessTokenService struct {
	repo *repository.AgentAccessTokenRepository
	cfg  *config.Config
	now  func() time.Time
}

func NewAgentAccessTokenService(repo *repository.AgentAccessTokenRepository, cfg *config.Config) *AgentAccessTokenService {
	return &AgentAccessTokenService{repo: repo, cfg: cfg, now: time.Now}
}

// normalizeAgentTokenScopes validates the requested scope set and returns
// the canonical (deduplicated, sorted, download-before-upload) order.
func normalizeAgentTokenScopes(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, ErrAgentTokenScopesInvalid
	}
	seen := make(map[string]bool, len(requested))
	scopes := make([]string, 0, len(requested))
	for _, s := range requested {
		switch s {
		case AgentTokenScopeDownload, AgentTokenScopeUpload:
			if !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		default:
			return nil, ErrAgentTokenScopesInvalid
		}
	}
	sort.Strings(scopes)
	return scopes, nil
}

// generateAgentTokenSecret returns 43 uniform base62 characters from
// crypto/rand via rejection sampling (256 raw values map onto 62 symbols
// with bias below 2^-245 — cryptographically irrelevant).
func generateAgentTokenSecret() (string, error) {
	var sb strings.Builder
	sb.Grow(agentTokenSecretLength)
	buf := make([]byte, 64)
	for sb.Len() < agentTokenSecretLength {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("agent token entropy: %w", err)
		}
		for _, b := range buf {
			if b >= 248 { // 62*4: reject the non-uniform tail
				continue
			}
			sb.WriteByte(base62Alphabet[int(b)%62])
			if sb.Len() == agentTokenSecretLength {
				break
			}
		}
	}
	return sb.String(), nil
}

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// AgentAccessTokenHash is the storage digest of a full plaintext token.
func AgentAccessTokenHash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// Issue creates a new PAT for the user. The returned struct embeds the only
// plaintext copy that will ever exist.
func (s *AgentAccessTokenService) Issue(ctx context.Context, userID int64, name string, scopes []string) (*IssuedAgentAccessToken, error) {
	name = strings.TrimSpace(name)
	if userID <= 0 || name == "" || len(name) > agentTokenNameMaxLength {
		return nil, ErrAgentTokenNameInvalid
	}
	normalized, err := normalizeAgentTokenScopes(scopes)
	if err != nil {
		return nil, err
	}

	limit := s.cfg.AgentAccess.MaxTokensPerUser
	if limit <= 0 {
		limit = 20
	}
	count, err := s.repo.CountActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count agent tokens: %w", err)
	}
	if int(count) >= limit {
		return nil, ErrAgentTokenLimitReached
	}

	secret, err := generateAgentTokenSecret()
	if err != nil {
		return nil, err
	}
	raw := AgentTokenPrefix + secret

	row := &model.AgentAccessToken{
		UserID:      userID,
		Name:        name,
		TokenHash:   AgentAccessTokenHash(raw),
		TokenPrefix: raw[:12],
		Scopes:      strings.Join(normalized, ","),
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, fmt.Errorf("persist agent token: %w", err)
	}

	return &IssuedAgentAccessToken{
		Token: raw,
		Info: AgentAccessTokenInfo{
			ID:          row.ID,
			Name:        row.Name,
			TokenPrefix: row.TokenPrefix,
			Scopes:      normalized,
			LastUsedAt:  nil,
			CreatedAt:   row.CreatedAt,
		},
	}, nil
}

// List returns the user's live tokens, newest first, without secrets.
func (s *AgentAccessTokenService) List(ctx context.Context, userID int64) ([]AgentAccessTokenInfo, error) {
	rows, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list agent tokens: %w", err)
	}
	out := make([]AgentAccessTokenInfo, 0, len(rows))
	for i := range rows {
		out = append(out, agentTokenRowToInfo(&rows[i]))
	}
	return out, nil
}

// Revoke immediately disables the user's own token (leak stop lever).
func (s *AgentAccessTokenService) Revoke(ctx context.Context, userID, tokenID int64) error {
	affected, err := s.repo.RevokeByIDAndUser(ctx, userID, tokenID, s.now())
	if err != nil {
		return fmt.Errorf("revoke agent token: %w", err)
	}
	if affected == 0 {
		return ErrAgentTokenNotFound
	}
	return nil
}

// Authenticate resolves a raw bearer token to its identity, rejecting
// malformed tokens and anything unknown or revoked. last_used_at advances
// at most once per agentTokenTouchInterval.
func (s *AgentAccessTokenService) Authenticate(ctx context.Context, rawToken string) (*AgentAccessTokenIdentity, error) {
	if len(rawToken) != agentTokenTotalLength ||
		!strings.HasPrefix(rawToken, AgentTokenPrefix) {
		return nil, ErrAgentTokenFormatInvalid
	}
	secret := rawToken[len(AgentTokenPrefix):]
	for i := 0; i < len(secret); i++ {
		c := secret[i]
		isBase62 := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		if !isBase62 {
			return nil, ErrAgentTokenFormatInvalid
		}
	}

	row, err := s.repo.FindActiveByHash(ctx, AgentAccessTokenHash(rawToken))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentTokenNotFound
		}
		return nil, fmt.Errorf("lookup agent token: %w", err)
	}

	now := s.now()
	if row.LastUsedAt == nil || now.Sub(*row.LastUsedAt) >= agentTokenTouchInterval {
		if touchErr := s.repo.TouchLastUsed(ctx, row.ID, now); touchErr != nil {
			// last_used_at is observability, not auth: never fail the
			// request over it.
			_ = touchErr
		}
	}

	return &AgentAccessTokenIdentity{
		TokenID: row.ID,
		UserID:  row.UserID,
		Scopes:  strings.Split(row.Scopes, ","),
	}, nil
}

func agentTokenRowToInfo(row *model.AgentAccessToken) AgentAccessTokenInfo {
	return AgentAccessTokenInfo{
		ID:          row.ID,
		Name:        row.Name,
		TokenPrefix: row.TokenPrefix,
		Scopes:      strings.Split(row.Scopes, ","),
		LastUsedAt:  row.LastUsedAt,
		CreatedAt:   row.CreatedAt,
	}
}
