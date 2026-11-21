// Package authsession stores OIDC token bundles server-side in Redis so the
// browser only carries a small opaque session cookie. Keycloak JWTs are large
// (8KB+ of Set-Cookie headers for access+refresh+id tokens), which overflows
// default proxy buffers on intermediate nginx hops.
package authsession

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "authsession:"

// CookieName is the browser cookie that carries the opaque session ID.
const CookieName = "sso_session"

// ErrNotFound is returned when a session ID does not exist or has expired.
var ErrNotFound = core.ErrSessionNotFound

// Data is the token bundle kept server-side for one browser session.
type Data struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// redisClient is the slice of go-redis used by the store, narrowed for tests.
type redisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type Store struct {
	rdb redisClient
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func newStoreWithClient(rdb redisClient) *Store {
	return &Store{rdb: rdb}
}

// Create stores the token bundle under a fresh random session ID and returns
// the ID. TTL must be positive (use the refresh token expiry).
func (s *Store) Create(ctx context.Context, data Data, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("session ttl must be positive, got %s", ttl)
	}

	id, err := newSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	if err := s.write(ctx, id, data, ttl); err != nil {
		return "", err
	}

	return id, nil
}

// Get returns the token bundle for a session ID, or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (*Data, error) {
	if id == "" {
		return nil, ErrNotFound
	}

	raw, err := s.rdb.Get(ctx, keyPrefix+id).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	var data Data
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	return &data, nil
}

// Update replaces the token bundle for an existing session ID and resets its
// TTL (token rotation on refresh).
func (s *Store) Update(ctx context.Context, id string, data Data, ttl time.Duration) error {
	if id == "" {
		return ErrNotFound
	}
	if ttl <= 0 {
		return fmt.Errorf("session ttl must be positive, got %s", ttl)
	}

	return s.write(ctx, id, data, ttl)
}

// Delete removes a session. Deleting a missing session is not an error.
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}

	if err := s.rdb.Del(ctx, keyPrefix+id).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s *Store) write(ctx context.Context, id string, data Data, ttl time.Duration) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	if err := s.rdb.Set(ctx, keyPrefix+id, raw, ttl).Err(); err != nil {
		return fmt.Errorf("write session: %w", err)
	}

	return nil
}

func newSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// The credential payload stays host-owned; only the lifecycle contract is core.
var _ core.SessionStore[Data] = (*Store)(nil)
