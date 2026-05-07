package core

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSkipRequestHeader(t *testing.T) {
	skip := []string{
		"Host", "host", "CONNECTION", "Content-Length",
		"Proxy-Connection", "Proxy-Authorization", "Transfer-Encoding", "Accept-Encoding",
	}
	for _, h := range skip {
		if !skipRequestHeader(h) {
			t.Errorf("expected skipRequestHeader(%q) = true", h)
		}
	}

	keep := []string{"User-Agent", "Authorization", "Content-Type", "Accept", "X-Custom"}
	for _, h := range keep {
		if skipRequestHeader(h) {
			t.Errorf("expected skipRequestHeader(%q) = false", h)
		}
	}
}

func TestSkipResponseHeader(t *testing.T) {
	skip := []string{
		"Content-Length", "Transfer-Encoding", "Connection", "Content-Encoding",
		"content-length", "TRANSFER-ENCODING",
	}
	for _, h := range skip {
		if !skipResponseHeader(h) {
			t.Errorf("expected skipResponseHeader(%q) = true", h)
		}
	}

	keep := []string{"Content-Type", "Cache-Control", "Set-Cookie", "X-Custom"}
	for _, h := range keep {
		if skipResponseHeader(h) {
			t.Errorf("expected skipResponseHeader(%q) = false", h)
		}
	}
}

func TestForwardHeaders_FiltersHopByHop(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Host", "example.com")
	h.Set("Connection", "keep-alive")
	h.Set("Transfer-Encoding", "chunked")
	h.Set("Accept-Encoding", "gzip")
	h.Set("X-Custom", "value")

	out := forwardHeaders(h)

	if _, ok := out["Host"]; ok {
		t.Error("Host should be stripped")
	}
	if _, ok := out["Connection"]; ok {
		t.Error("Connection should be stripped")
	}
	if _, ok := out["Transfer-Encoding"]; ok {
		t.Error("Transfer-Encoding should be stripped")
	}
	if out["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", out["Content-Type"])
	}
	if out["X-Custom"] != "value" {
		t.Errorf("X-Custom = %q, want value", out["X-Custom"])
	}
}

func TestForwardHeaders_DefaultUserAgent(t *testing.T) {
	out := forwardHeaders(http.Header{})
	if out["User-Agent"] == "" {
		t.Error("expected default User-Agent to be set when absent")
	}
}

func TestForwardHeaders_PreservesExistingUserAgent(t *testing.T) {
	h := http.Header{}
	h.Set("User-Agent", "MyBot/1.0")
	out := forwardHeaders(h)
	if out["User-Agent"] != "MyBot/1.0" {
		t.Errorf("User-Agent = %q, want MyBot/1.0", out["User-Agent"])
	}
}

func TestForwardHeaders_MultiValueTakesFirst(t *testing.T) {
	h := http.Header{}
	h["Accept"] = []string{"text/html", "application/json"}
	out := forwardHeaders(h)
	if out["Accept"] != "text/html" {
		t.Errorf("Accept = %q, want text/html (first value)", out["Accept"])
	}
}

// fakeAppScript builds a TLS test server that responds like Apps Script single-relay.
func fakeAppScript(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := workerResponse{
			Status:  status,
			Headers: map[string]any{"content-type": []string{"text/plain"}},
			Body:    base64.StdEncoding.EncodeToString([]byte(body)),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func fakeCoalescer(t *testing.T, srv *httptest.Server) *Coalescer {
	t.Helper()
	return NewCoalescer(srv.Client(), []string{srv.URL}, srv.Listener.Addr().String(), "k", 5*time.Second)
}

// --- handleHTTP ---

func TestHandleHTTP_RelaysRequest(t *testing.T) {
	srv := fakeAppScript(t, "proxied", 200)
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/test", nil)
	handleHTTP(w, r, fakeCoalescer(t, srv))

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if body, _ := io.ReadAll(w.Body); string(body) != "proxied" {
		t.Errorf("body = %q, want proxied", body)
	}
}

func TestHandleHTTP_RelayError_Returns502(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/test", nil)
	handleHTTP(w, r, fakeCoalescer(t, srv))

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

// --- Coalescer ---

// TestCoalescer_SingleRequest checks that a lone request gets a correct response.
func TestCoalescer_SingleRequest(t *testing.T) {
	srv := fakeAppScript(t, "hello", 200)
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	resp, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if string(resp.Body) != "hello" {
		t.Errorf("body = %q, want hello", resp.Body)
	}
}

// TestCoalescer_BatchesConcurrentRequests verifies that N concurrent requests
// are fused into fewer Apps Script calls than N (batching is actually happening).
func TestCoalescer_BatchesConcurrentRequests(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		callCount.Add(1)

		// batch request has "q" array; single has "m"
		if items, ok := req["q"].([]any); ok {
			results := make([]workerResponse, len(items))
			for i := range items {
				results[i] = workerResponse{
					Status: 200,
					Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"q": results})
		} else {
			json.NewEncoder(w).Encode(workerResponse{
				Status: 200,
				Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
			})
		}
	}))
	defer srv.Close()

	const n = 8
	coal := NewCoalescer(srv.Client(), []string{srv.URL}, srv.Listener.Addr().String(), "k", 5*time.Second)

	errc := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
			errc <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errc; err != nil {
			t.Errorf("request %d failed: %v", i, err)
		}
	}

	calls := int(callCount.Load())
	if calls >= n {
		t.Errorf("expected fewer than %d Apps Script calls (batching should merge requests), got %d", n, calls)
	}
	t.Logf("%d concurrent requests → %d Apps Script call(s)", n, calls)
}

// TestCoalescer_ErrorPropagates checks that a relay failure is returned to the caller.
func TestCoalescer_ErrorPropagates(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	_, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error from failing relay, got nil")
	}
}

// TestCoalescer_AllRequestsReceiveResponse ensures every caller in a batch
// gets its own response and none are dropped.
func TestCoalescer_AllRequestsReceiveResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		if items, ok := req["q"].([]any); ok {
			results := make([]workerResponse, len(items))
			for i := range items {
				results[i] = workerResponse{
					Status: 200,
					Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"q": results})
		} else {
			json.NewEncoder(w).Encode(workerResponse{
				Status: 200,
				Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
			})
		}
	}))
	defer srv.Close()

	const n = 10
	coal := NewCoalescer(srv.Client(), []string{srv.URL}, srv.Listener.Addr().String(), "k", 5*time.Second)

	type res struct {
		resp RelayResponse
		err  error
	}
	results := make(chan res, n)
	for i := 0; i < n; i++ {
		go func() {
			resp, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
			results <- res{resp, err}
		}()
	}

	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("request %d: unexpected error: %v", i, r.err)
		} else if r.resp.Status != 200 {
			t.Errorf("request %d: status = %d, want 200", i, r.resp.Status)
		}
	}
}

// TestCoalescer_Warmup verifies that Warmup fires without blocking and does not
// interfere with subsequent real requests.
func TestCoalescer_Warmup(t *testing.T) {
	srv := fakeAppScript(t, "ok", 200)
	defer srv.Close()

	coal := fakeCoalescer(t, srv)
	coal.Warmup() // must not block

	resp, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error after warmup: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

// TestCoalescer_AdaptiveWindow verifies that a burst of requests arriving at
// once is still fully batched (wider window kicks in).
func TestCoalescer_AdaptiveWindow(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		callCount.Add(1)

		if items, ok := req["q"].([]any); ok {
			results := make([]workerResponse, len(items))
			for i := range items {
				results[i] = workerResponse{Status: 200, Body: base64.StdEncoding.EncodeToString([]byte("ok"))}
			}
			json.NewEncoder(w).Encode(map[string]any{"q": results})
		} else {
			json.NewEncoder(w).Encode(workerResponse{Status: 200, Body: base64.StdEncoding.EncodeToString([]byte("ok"))})
		}
	}))
	defer srv.Close()

	// Pre-fill the channel before the coalescer drains it to simulate a burst.
	coal := NewCoalescer(srv.Client(), []string{srv.URL}, srv.Listener.Addr().String(), "k", 5*time.Second)

	const n = 6
	errc := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
			errc <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errc; err != nil {
			t.Errorf("request %d failed: %v", i, err)
		}
	}

	calls := int(callCount.Load())
	if calls >= n {
		t.Errorf("expected fewer than %d Apps Script calls (adaptive window should batch burst), got %d", n, calls)
	}
	t.Logf("burst of %d requests → %d Apps Script call(s)", n, calls)
}

// --- perURLTimeout ---

func TestPerURLTimeout_SingleURL(t *testing.T) {
	got := perURLTimeout(45*time.Second, 1)
	if got != 45*time.Second {
		t.Errorf("single URL: got %v, want 45s", got)
	}
}

func TestPerURLTimeout_SplitsEvenly(t *testing.T) {
	got := perURLTimeout(45*time.Second, 3)
	if got != 15*time.Second {
		t.Errorf("3 URLs: got %v, want 15s", got)
	}
}

func TestPerURLTimeout_RespectsMinimum(t *testing.T) {
	// 10 URLs would give 4.5s each — must be clamped to 8s minimum.
	got := perURLTimeout(45*time.Second, 10)
	if got < 8*time.Second {
		t.Errorf("minimum not enforced: got %v, want >= 8s", got)
	}
}

// --- serveSSEKeepalive ---

func TestServeSSEKeepalive_SendsHeadersAndKeepalive(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		serveSSEKeepalive(server)
		server.Close()
	}()

	// Read the HTTP response headers — must arrive immediately (no relay call).
	client.SetDeadline(time.Now().Add(2 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	client.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestServeSSEKeepalive_NoRelayCallMade(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		json.NewEncoder(w).Encode(workerResponse{
			Status: 200,
			Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
		})
	}))
	defer srv.Close()

	// serveSSEKeepalive must not touch the relay at all.
	client, server := net.Pipe()
	go func() {
		serveSSEKeepalive(server)
		server.Close()
	}()
	client.Close()

	if n := int(callCount.Load()); n != 0 {
		t.Errorf("relay called %d times, want 0 for SSE keepalive path", n)
	}

	// A normal request through the coalescer must still reach the relay.
	coal := fakeCoalescer(t, srv)
	_, err := coal.Submit("GET", "http://example.com/", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("normal request failed: %v", err)
	}
	if n := int(callCount.Load()); n == 0 {
		t.Error("expected relay to be called for a normal GET request")
	}
}

// TestRelayRequestMulti_FailoverTransparent verifies that when the first URL
// fails fast, the second URL is tried within the same call — no user refresh needed.
func TestRelayRequestMulti_FailoverTransparent(t *testing.T) {
	// URL 0: always returns 500 (fast failure).
	bad := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer bad.Close()

	// URL 1: returns a valid relay response.
	good := fakeAppScript(t, "fallback-ok", 200)
	defer good.Close()

	client := good.Client() // TLS client that trusts both test servers
	resp, err := RelayRequestMulti(
		client,
		[]string{bad.URL, good.URL},
		good.Listener.Addr().String(),
		"k", "GET", "http://example.com/", map[string]string{}, nil,
		10*time.Second,
	)
	if err != nil {
		t.Fatalf("expected transparent failover, got error: %v", err)
	}
	if string(resp.Body) != "fallback-ok" {
		t.Errorf("body = %q, want fallback-ok", resp.Body)
	}
}

func TestRelayRequestMulti_SplitsTimeout(t *testing.T) {
	// 2 URLs, 20s total → 10s each (above 8s minimum).
	got := perURLTimeout(20*time.Second, 2)
	if got != 10*time.Second {
		t.Errorf("got %v, want 10s", got)
	}

	// 2 URLs, 4s total → below minimum, clamped to 8s.
	got = perURLTimeout(4*time.Second, 2)
	if got != 8*time.Second {
		t.Errorf("got %v, want 8s (minimum)", got)
	}
}
