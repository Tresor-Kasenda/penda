package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	fwapp "penda/framework/app"
	fwctx "penda/framework/context"
)

func TestRedisStoreSessionSaveAndLoad(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := MustNewRedisStore(
		client,
		[]byte("0123456789abcdef0123456789abcdef"),
		Config{},
		RedisStoreConfig{KeyPrefix: "test:sess:", TTL: time.Hour},
	)

	server := fwapp.New()
	server.Use(RedisMiddleware(store))
	server.Get("/set", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		sess.Set("user_id", "42")
		if err := sess.Save(c); err != nil {
			return err
		}
		return c.Text(http.StatusOK, "saved")
	})
	server.Get("/get", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		value, ok := sess.Get("user_id")
		if !ok {
			return c.Text(http.StatusNotFound, "missing")
		}
		return c.Text(http.StatusOK, value)
	})

	reqSet := httptest.NewRequest(http.MethodGet, "/set", nil)
	rrSet := httptest.NewRecorder()
	server.ServeHTTP(rrSet, reqSet)
	if rrSet.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rrSet.Code)
	}
	cookies := rrSet.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	sessionID, err := store.decodeSessionIDCookie(cookies[0].Value)
	if err != nil {
		t.Fatalf("decode session cookie: %v", err)
	}
	if !mr.Exists(store.redisKey(sessionID)) {
		t.Fatalf("expected redis session key %q", store.redisKey(sessionID))
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/get", nil)
	reqGet.AddCookie(cookies[0])
	rrGet := httptest.NewRecorder()
	server.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rrGet.Code)
	}
	if strings.TrimSpace(rrGet.Body.String()) != "42" {
		t.Fatalf("unexpected body: %q", rrGet.Body.String())
	}
}

func TestRedisStoreRejectsTamperedCookie(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := MustNewRedisStore(
		client,
		[]byte("0123456789abcdef0123456789abcdef"),
		Config{},
		RedisStoreConfig{},
	)

	server := fwapp.New()
	server.Use(RedisMiddleware(store))
	server.Get("/me", func(c *fwctx.Context) error {
		return c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "penda_session", Value: "tampered.payload"})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestRedisStoreDestroyDeletesRedisKey(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := MustNewRedisStore(
		client,
		[]byte("0123456789abcdef0123456789abcdef"),
		Config{},
		RedisStoreConfig{KeyPrefix: "test:sess:", TTL: time.Hour},
	)

	server := fwapp.New()
	server.Use(RedisMiddleware(store))
	server.Get("/set", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		sess.Set("user_id", "42")
		return sess.Save(c)
	})
	server.Get("/logout", func(c *fwctx.Context) error {
		sess := MustFromContext(c)
		sess.Destroy(c)
		return c.Text(http.StatusOK, "bye")
	})

	reqSet := httptest.NewRequest(http.MethodGet, "/set", nil)
	rrSet := httptest.NewRecorder()
	server.ServeHTTP(rrSet, reqSet)
	cookies := rrSet.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	sessionID, err := store.decodeSessionIDCookie(cookies[0].Value)
	if err != nil {
		t.Fatalf("decode session cookie: %v", err)
	}
	key := store.redisKey(sessionID)
	if !mr.Exists(key) {
		t.Fatalf("expected redis session key %q", key)
	}

	reqLogout := httptest.NewRequest(http.MethodGet, "/logout", nil)
	reqLogout.AddCookie(cookies[0])
	rrLogout := httptest.NewRecorder()
	server.ServeHTTP(rrLogout, reqLogout)
	if rrLogout.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rrLogout.Code)
	}
	if mr.Exists(key) {
		t.Fatalf("expected redis session key %q to be deleted", key)
	}
	outCookies := rrLogout.Result().Cookies()
	if len(outCookies) == 0 || outCookies[0].MaxAge >= 0 {
		t.Fatal("expected expired session cookie after destroy")
	}
}
