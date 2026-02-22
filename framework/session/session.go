package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

// ContextKey stores the request-scoped session instance.
const ContextKey = "session"

// Config configures the signed session cookie.
type Config struct {
	CookieName string
	Path       string
	Domain     string
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	MaxAge     int
}

// Store signs and verifies cookie-backed sessions.
type Store struct {
	secret []byte
	config Config
}

// Session is a mutable request-scoped map persisted into a signed cookie.
// Values are string-based for predictable JSON serialization across requests.
type Session struct {
	store   *Store
	values  map[string]string
	destroy bool
}

// NewStore creates a signed cookie session store.
func NewStore(secret []byte, config Config) (*Store, error) {
	if len(secret) < 16 {
		return nil, errors.New("session secret must be at least 16 bytes")
	}
	cfg := normalizeConfig(config)
	return &Store{
		secret: append([]byte(nil), secret...),
		config: cfg,
	}, nil
}

// MustNewStore creates a store or panics.
func MustNewStore(secret []byte, config Config) *Store {
	store, err := NewStore(secret, config)
	if err != nil {
		panic(err)
	}
	return store
}

// Middleware loads a session from the request cookie and stores it in framework context.
func Middleware(store *Store) fwapp.Middleware {
	if store == nil {
		panic("session store cannot be nil")
	}

	return func(next fwapp.Handler) fwapp.Handler {
		return func(c *fwctx.Context) error {
			sess, err := store.Load(c)
			if err != nil {
				return fwctx.NewHTTPError(http.StatusBadRequest, "invalid session cookie", err)
			}
			c.Set(ContextKey, sess)
			return next(c)
		}
	}
}

// Load builds a session from the request cookie or returns an empty session.
func (s *Store) Load(c *fwctx.Context) (*Session, error) {
	if c == nil {
		return nil, errors.New("context cannot be nil")
	}

	values := map[string]string{}
	cookie, err := c.Cookie(s.config.CookieName)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return nil, err
	}
	if err == nil && cookie != nil && strings.TrimSpace(cookie.Value) != "" {
		decoded, decErr := s.decode(cookie.Value)
		if decErr != nil {
			return nil, decErr
		}
		values = decoded
	}

	return &Session{
		store:  s,
		values: values,
	}, nil
}

// FromContext retrieves a session attached by the middleware.
func FromContext(c *fwctx.Context) (*Session, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(ContextKey)
	if !ok {
		return nil, false
	}
	sess, ok := value.(*Session)
	return sess, ok
}

// MustFromContext retrieves a session or panics.
func MustFromContext(c *fwctx.Context) *Session {
	sess, ok := FromContext(c)
	if !ok {
		panic("session is not set in request context")
	}
	return sess
}

// Get returns a value from the session.
func (s *Session) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	value, ok := s.values[key]
	return value, ok
}

// Set stores a value in the session.
func (s *Session) Set(key, value string) {
	if s == nil {
		return
	}
	s.destroy = false
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
}

// Delete removes a key from the session.
func (s *Session) Delete(key string) {
	if s == nil || s.values == nil {
		return
	}
	delete(s.values, key)
}

// Clear removes all values from the session.
func (s *Session) Clear() {
	if s == nil {
		return
	}
	s.values = map[string]string{}
}

// Values returns a copy of all session values.
func (s *Session) Values() map[string]string {
	out := map[string]string{}
	if s == nil {
		return out
	}
	for k, v := range s.values {
		out[k] = v
	}
	return out
}

// Destroy marks the session for deletion and writes an expired cookie.
func (s *Session) Destroy(c *fwctx.Context) {
	if s == nil {
		return
	}
	s.destroy = true
	s.values = map[string]string{}
	if c != nil {
		s.writeCookie(c, "", -1)
	}
}

// Save writes the current session values to the response as a signed cookie.
func (s *Session) Save(c *fwctx.Context) error {
	if s == nil {
		return errors.New("session cannot be nil")
	}
	if c == nil {
		return errors.New("context cannot be nil")
	}
	if s.store == nil {
		return errors.New("session store is not configured")
	}
	if s.destroy {
		s.writeCookie(c, "", -1)
		return nil
	}

	value, err := s.store.encode(s.values)
	if err != nil {
		return err
	}
	s.writeCookie(c, value, s.store.config.MaxAge)
	return nil
}

func (s *Session) writeCookie(c *fwctx.Context, value string, maxAge int) {
	cfg := s.store.config
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

func (s *Store) encode(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.sign(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (s *Store) decode(raw string) (map[string]string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid session cookie format")
	}

	payloadPart := parts[0]
	signaturePart := parts[1]
	expectedSignature := s.sign(payloadPart)
	if hmac.Equal([]byte(signaturePart), []byte(expectedSignature)) == false {
		return nil, errors.New("invalid session cookie signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, fmt.Errorf("decode session payload: %w", err)
	}

	values := map[string]string{}
	if err := json.Unmarshal(payloadBytes, &values); err != nil {
		return nil, fmt.Errorf("parse session payload: %w", err)
	}
	return values, nil
}

func (s *Store) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.CookieName) == "" {
		config.CookieName = "penda_session"
	}
	if strings.TrimSpace(config.Path) == "" {
		config.Path = "/"
	}
	// Signed session cookies should not be readable by JS by default.
	if !config.HTTPOnly {
		config.HTTPOnly = true
	}
	return config
}
