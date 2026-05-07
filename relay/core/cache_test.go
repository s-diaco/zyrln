package core

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// --- cacheableMaxAge ---

func TestCacheableMaxAge_Cacheable(t *testing.T) {
	cases := []struct {
		cc  string
		ttl time.Duration
	}{
		{"max-age=3600", time.Hour},
		{"public, max-age=86400", 24 * time.Hour},
		{"max-age=60, must-revalidate", time.Minute},
	}
	for _, c := range cases {
		got := cacheableMaxAge("GET", map[string]string{}, map[string][]string{"cache-control": {c.cc}}, 200, "https://example.com/page")
		if got != c.ttl {
			t.Errorf("Cache-Control %q: got %v, want %v", c.cc, got, c.ttl)
		}
	}
}

func TestCacheableMaxAge_NotCacheable(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		status      int
		reqHeaders  map[string]string
		respHeaders map[string][]string
	}{
		{"POST", "POST", 200, map[string]string{}, map[string][]string{"cache-control": {"max-age=3600"}}},
		{"non-200", "GET", 301, map[string]string{}, map[string][]string{"cache-control": {"max-age=3600"}}},
		{"no-store", "GET", 200, map[string]string{}, map[string][]string{"cache-control": {"no-store"}}},
		{"no-cache", "GET", 200, map[string]string{}, map[string][]string{"cache-control": {"no-cache"}}},
		{"private", "GET", 200, map[string]string{}, map[string][]string{"cache-control": {"private, max-age=3600"}}},
		{"no Cache-Control", "GET", 200, map[string]string{}, map[string][]string{}},
		{"max-age=0", "GET", 200, map[string]string{}, map[string][]string{"cache-control": {"max-age=0"}}},
		{"Authorization", "GET", 200, map[string]string{"Authorization": "Bearer tok"}, map[string][]string{"cache-control": {"max-age=3600"}}},
		{"Set-Cookie", "GET", 200, map[string]string{}, map[string][]string{"cache-control": {"max-age=3600"}, "set-cookie": {"sid=x"}}},
	}
	for _, c := range cases {
		got := cacheableMaxAge(c.method, c.reqHeaders, c.respHeaders, c.status, "https://example.com/page")
		if got != 0 {
			t.Errorf("case %q: expected 0, got %v", c.name, got)
		}
	}
}

// --- responseCache ---

func TestResponseCache_HitAndMiss(t *testing.T) {
	rc := newResponseCache()
	rc.set("https://example.com/a.js", &cacheEntry{
		status:  200,
		headers: map[string][]string{"content-type": {"application/javascript"}},
		body:    []byte("console.log(1)"),
		expiry:  time.Now().Add(time.Hour),
	})

	e := rc.get("https://example.com/a.js")
	if e == nil {
		t.Fatal("expected cache hit")
	}
	if string(e.body) != "console.log(1)" {
		t.Errorf("body = %q", e.body)
	}

	if rc.get("https://example.com/missing.js") != nil {
		t.Error("expected cache miss for unknown key")
	}
}

func TestResponseCache_ExpiredEntryNotReturned(t *testing.T) {
	rc := newResponseCache()
	rc.set("https://example.com/old.css", &cacheEntry{
		status: 200,
		body:   []byte("body{}"),
		expiry: time.Now().Add(-time.Second), // already expired
	})

	if rc.get("https://example.com/old.css") != nil {
		t.Error("expected expired entry to be treated as a miss")
	}
}

func TestResponseCache_EvictsExpiredWhenFull(t *testing.T) {
	rc := newResponseCache()
	// Fill with expired entries.
	for i := 0; i < cacheMaxEntries; i++ {
		key := "https://example.com/" + string(rune('a'+i%26)) + ".js"
		rc.set(key, &cacheEntry{
			status: 200,
			body:   []byte("x"),
			expiry: time.Now().Add(-time.Second),
		})
	}
	// A new entry should still be accepted after expired ones are evicted.
	rc.set("https://example.com/new.js", &cacheEntry{
		status: 200,
		body:   []byte("new"),
		expiry: time.Now().Add(time.Hour),
	})
	if e := rc.get("https://example.com/new.js"); e == nil {
		t.Error("expected new entry to be cached after eviction of expired entries")
	}
}

// --- Coalescer cache integration ---

func TestCoalescer_CacheHitSkipsRelay(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(workerResponse{
			Status:  200,
			Headers: map[string]any{"cache-control": []string{"max-age=3600"}},
			Body:    base64.StdEncoding.EncodeToString([]byte("cached-body")),
		})
	}))
	defer srv.Close()

	coal := fakeCoalescer(t, srv)

	// First request — hits relay.
	resp1, err := coal.Submit("GET", "https://example.com/a.js", map[string]string{}, nil)
	if err != nil || string(resp1.Body) != "cached-body" {
		t.Fatalf("first request: err=%v body=%q", err, resp1.Body)
	}

	// Second request — must come from cache, relay not called again.
	resp2, err := coal.Submit("GET", "https://example.com/a.js", map[string]string{}, nil)
	if err != nil || string(resp2.Body) != "cached-body" {
		t.Fatalf("second request: err=%v body=%q", err, resp2.Body)
	}

	if n := int(callCount.Load()); n != 1 {
		t.Errorf("relay called %d times, want exactly 1 (second should hit cache)", n)
	}
}

func TestCoalescer_NoCacheForNoStore(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(workerResponse{
			Status:  200,
			Headers: map[string]any{"cache-control": []string{"no-store"}},
			Body:    base64.StdEncoding.EncodeToString([]byte("dynamic")),
		})
	}))
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	coal.Submit("GET", "https://example.com/api", map[string]string{}, nil)
	coal.Submit("GET", "https://example.com/api", map[string]string{}, nil)

	if n := int(callCount.Load()); n != 2 {
		t.Errorf("relay called %d times, want 2 (no-store must not be cached)", n)
	}
}

func TestCoalescer_NoCacheForPost(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(workerResponse{
			Status:  200,
			Headers: map[string]any{"cache-control": []string{"max-age=3600"}},
			Body:    base64.StdEncoding.EncodeToString([]byte("resp")),
		})
	}))
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	coal.Submit("POST", "https://example.com/submit", map[string]string{}, []byte("data"))
	coal.Submit("POST", "https://example.com/submit", map[string]string{}, []byte("data"))

	if n := int(callCount.Load()); n != 2 {
		t.Errorf("relay called %d times, want 2 (POST must never be cached)", n)
	}
}

func TestCoalescer_NoCacheForSetCookie(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(workerResponse{
			Status:  200,
			Headers: map[string]any{"cache-control": []string{"max-age=3600"}, "set-cookie": []string{"sid=abc"}},
			Body:    base64.StdEncoding.EncodeToString([]byte("resp")),
		})
	}))
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	coal.Submit("GET", "https://example.com/login", map[string]string{}, nil)
	coal.Submit("GET", "https://example.com/login", map[string]string{}, nil)

	if n := int(callCount.Load()); n != 2 {
		t.Errorf("relay called %d times, want 2 (Set-Cookie must not be cached)", n)
	}
}
