package core

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cacheMaxEntries = 256
	cacheMaxBytes   = 16 * 1024 * 1024 // 16 MB total across all entries
)

type cacheEntry struct {
	status  int
	headers map[string]string
	body    []byte
	expiry  time.Time
}

type responseCache struct {
	mu        sync.Mutex
	entries   map[string]*cacheEntry
	totalSize int
}

func newResponseCache() *responseCache {
	return &responseCache{entries: make(map[string]*cacheEntry)}
}

func (rc *responseCache) get(key string) *cacheEntry {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	e, ok := rc.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(e.expiry) {
		rc.totalSize -= len(e.body)
		delete(rc.entries, key)
		return nil
	}
	return e
}

func (rc *responseCache) set(key string, e *cacheEntry) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// evict expired entries first if we are near a limit
	if len(rc.entries) >= cacheMaxEntries || rc.totalSize+len(e.body) > cacheMaxBytes {
		now := time.Now()
		for k, v := range rc.entries {
			if now.After(v.expiry) {
				rc.totalSize -= len(v.body)
				delete(rc.entries, k)
			}
		}
	}

	// if still over capacity, skip — never evict live entries
	if len(rc.entries) >= cacheMaxEntries || rc.totalSize+len(e.body) > cacheMaxBytes {
		return
	}

	// replace existing entry for same key
	if old, ok := rc.entries[key]; ok {
		rc.totalSize -= len(old.body)
	}

	rc.entries[key] = e
	rc.totalSize += len(e.body)
}

// cacheableMaxAge returns the TTL if this response may be cached, or 0 if not.
// Rules (conservative, browser-compatible):
//   - GET 200 only
//   - No Authorization on the request
//   - No Set-Cookie on the response
//   - Cache-Control must not contain no-store, no-cache, or private
//   - Cache-Control must have max-age > 0
func cacheableMaxAge(method string, reqHeaders, respHeaders map[string]string, status int) time.Duration {
	if method != "GET" || status != 200 {
		return 0
	}
	if reqHeaders["Authorization"] != "" {
		return 0
	}
	if respHeaders["Set-Cookie"] != "" {
		return 0
	}

	cc := respHeaders["Cache-Control"]
	if cc == "" {
		return 0
	}

	maxAge := -1
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		switch {
		case part == "no-store", part == "no-cache", part == "private":
			return 0
		case strings.HasPrefix(part, "max-age="):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
			if err == nil {
				maxAge = n
			}
		}
	}

	if maxAge <= 0 {
		return 0
	}
	return time.Duration(maxAge) * time.Second
}
