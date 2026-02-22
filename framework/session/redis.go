package session

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

// RedisStoreConfig configures Redis-backed session persistence.
type RedisStoreConfig struct {
	KeyPrefix   string
	TTL         time.Duration
	TouchOnLoad bool
}

// RedisStore persists session values in Redis and stores a signed session ID in the cookie.
type RedisStore struct {
	client       redis.UniversalClient
	secret       []byte
	cookieConfig Config
	redisConfig  RedisStoreConfig
}

// NewRedisStore creates a Redis-backed session store.
func NewRedisStore(client redis.UniversalClient, secret []byte, cookieConfig Config, redisConfig RedisStoreConfig) (*RedisStore, error) {
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if len(secret) < 16 {
		return nil, errors.New("session secret must be at least 16 bytes")
	}

	return &RedisStore{
		client:       client,
		secret:       append([]byte(nil), secret...),
		cookieConfig: normalizeConfig(cookieConfig),
		redisConfig:  normalizeRedisStoreConfig(redisConfig),
	}, nil
}

// MustNewRedisStore creates a Redis-backed session store or panics.
func MustNewRedisStore(client redis.UniversalClient, secret []byte, cookieConfig Config, redisConfig RedisStoreConfig) *RedisStore {
	store, err := NewRedisStore(client, secret, cookieConfig, redisConfig)
	if err != nil {
		panic(err)
	}
	return store
}

// RedisMiddleware loads sessions from a Redis-backed store and stores them in framework context.
func RedisMiddleware(store *RedisStore) fwapp.Middleware {
	return MiddlewareLoader(store)
}

// Load loads a session from the signed cookie and Redis.
func (s *RedisStore) Load(c *fwctx.Context) (*Session, error) {
	if c == nil {
		return nil, errors.New("context cannot be nil")
	}

	sessionID, err := s.readSessionIDFromCookie(c)
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	if sessionID != "" {
		raw, err := s.client.Get(c.Request.Context(), s.redisKey(sessionID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		if err == nil && strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &values); err != nil {
				return nil, fmt.Errorf("parse redis session payload: %w", err)
			}
			if s.redisConfig.TouchOnLoad && s.redisConfig.TTL > 0 {
				_ = s.client.Expire(c.Request.Context(), s.redisKey(sessionID), s.redisConfig.TTL).Err()
			}
		}
	}

	return &Session{
		persistor: s,
		id:        sessionID,
		values:    values,
	}, nil
}

func (s *RedisStore) saveSession(c *fwctx.Context, sess *Session) error {
	if c == nil {
		return errors.New("context cannot be nil")
	}
	if sess == nil {
		return errors.New("session cannot be nil")
	}
	if sess.destroy {
		s.destroySession(c, sess)
		return nil
	}

	if strings.TrimSpace(sess.id) == "" {
		id, err := newSessionID()
		if err != nil {
			return err
		}
		sess.id = id
	}

	payload, err := json.Marshal(sess.values)
	if err != nil {
		return err
	}

	if err := s.client.Set(c.Request.Context(), s.redisKey(sess.id), payload, s.redisConfig.TTL).Err(); err != nil {
		return err
	}

	cookieValue := s.encodeSessionIDCookie(sess.id)
	s.writeCookie(c, cookieValue, s.cookieConfig.MaxAge)
	return nil
}

func (s *RedisStore) destroySession(c *fwctx.Context, sess *Session) {
	if c == nil {
		return
	}
	if sess != nil && strings.TrimSpace(sess.id) != "" {
		_ = s.client.Del(c.Request.Context(), s.redisKey(sess.id)).Err()
	}
	s.writeCookie(c, "", -1)
}

func (s *RedisStore) writeCookie(c *fwctx.Context, value string, maxAge int) {
	cfg := s.cookieConfig
	c.SetCookie(&http.Cookie{
		Name:     cfg.CookieName,
		Value:    value,
		Path:     cfg.Path,
		Domain:   cfg.Domain,
		Secure:   cfg.Secure,
		HttpOnly: cfg.HTTPOnly,
		SameSite: cfg.SameSite,
		MaxAge:   maxAge,
	})
}

func (s *RedisStore) readSessionIDFromCookie(c *fwctx.Context) (string, error) {
	cookie, err := c.Cookie(s.cookieConfig.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", nil
		}
		return "", err
	}
	if cookie == nil || strings.TrimSpace(cookie.Value) == "" {
		return "", nil
	}
	return s.decodeSessionIDCookie(cookie.Value)
}

func (s *RedisStore) encodeSessionIDCookie(sessionID string) string {
	encodedID := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	signature := s.sign(encodedID)
	return encodedID + "." + signature
}

func (s *RedisStore) decodeSessionIDCookie(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", errors.New("invalid session id cookie format")
	}
	encodedID := parts[0]
	signature := parts[1]
	expected := s.sign(encodedID)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", errors.New("invalid session id cookie signature")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encodedID)
	if err != nil {
		return "", fmt.Errorf("decode session id: %w", err)
	}
	return string(decoded), nil
}

func (s *RedisStore) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *RedisStore) redisKey(sessionID string) string {
	return s.redisConfig.KeyPrefix + sessionID
}

func normalizeRedisStoreConfig(config RedisStoreConfig) RedisStoreConfig {
	if strings.TrimSpace(config.KeyPrefix) == "" {
		config.KeyPrefix = "penda:session:"
	}
	if config.TTL <= 0 {
		config.TTL = 24 * time.Hour
	}
	return config
}

func newSessionID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

// Ping checks Redis connectivity for session storage.
func (s *RedisStore) Ping(ctx context.Context) error {
	if s == nil {
		return errors.New("redis session store cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.client.Ping(ctx).Err()
}
