package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- pure helpers ---

func TestIsRedirect(t *testing.T) {
	redirects := []int{301, 302, 303, 307, 308}
	for _, code := range redirects {
		if !isRedirect(code) {
			t.Errorf("expected %d to be a redirect", code)
		}
	}
	nonRedirects := []int{200, 204, 400, 404, 500}
	for _, code := range nonRedirects {
		if isRedirect(code) {
			t.Errorf("expected %d NOT to be a redirect", code)
		}
	}
}

func TestEffectiveFrontDomain(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "www.google.com"},
		{"  ", "www.google.com"},
		{"www.gstatic.com", "www.gstatic.com"},
	}
	for _, c := range cases {
		if got := effectiveFrontDomain(c.in); got != c.want {
			t.Errorf("effectiveFrontDomain(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPreviewBytes(t *testing.T) {
	cases := []struct {
		input []byte
		max   int
		want  string
	}{
		{nil, 10, ""},
		{[]byte("hello"), 10, "hello"},
		{[]byte("hello world"), 8, "hello..."},
		{[]byte("  hi  "), 10, "hi"},
		{[]byte("a\nb\rc"), 20, "a b c"},
	}
	for _, c := range cases {
		got := previewBytes(c.input, c.max)
		if got != c.want {
			t.Errorf("previewBytes(%q, %d) = %q, want %q", c.input, c.max, got, c.want)
		}
	}
}

func TestBuildRelayPayload(t *testing.T) {
	headers := map[string]string{"User-Agent": "test"}
	body := []byte("hello")

	raw := buildRelayPayload("mykey", "GET", "https://example.com", headers, body)

	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	if p["k"] != "mykey" {
		t.Errorf("k = %v, want mykey", p["k"])
	}
	if p["m"] != "GET" {
		t.Errorf("m = %v, want GET", p["m"])
	}
	if p["u"] != "https://example.com" {
		t.Errorf("u = %v, want https://example.com", p["u"])
	}

	// Body should be base64 encoded.
	gotB, _ := base64.StdEncoding.DecodeString(p["b"].(string))
	if string(gotB) != "hello" {
		t.Errorf("b decoded = %q, want hello", gotB)
	}
}

func TestBuildRelayPayload_NoBody(t *testing.T) {
	raw := buildRelayPayload("k", "GET", "https://x.com", map[string]string{}, nil)
	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := p["b"]; ok {
		t.Error("b key should be absent when body is nil")
	}
}

func TestBuildRelayPayload_MethodUppercased(t *testing.T) {
	raw := buildRelayPayload("k", "post", "https://x.com", map[string]string{}, nil)
	var p map[string]any
	json.Unmarshal([]byte(raw), &p)
	if p["m"] != "POST" {
		t.Errorf("method should be uppercased, got %v", p["m"])
	}
}

// --- newFrontedPOST ---

func TestNewFrontedPOST_SwapsHost(t *testing.T) {
	req, err := newFrontedPOST(
		context.Background(),
		"https://script.google.com/macros/s/ABC/exec",
		"www.google.com",
		`{"k":"x"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Host != "www.google.com" {
		t.Errorf("URL host = %q, want www.google.com", req.URL.Host)
	}
	if req.Host != "script.google.com" {
		t.Errorf("Host header = %q, want script.google.com", req.Host)
	}
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

func TestNewFrontedPOST_DefaultFrontDomain(t *testing.T) {
	req, err := newFrontedPOST(context.Background(), "https://script.google.com/x", "", "payload")
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Host != "www.google.com" {
		t.Errorf("default front domain should be www.google.com, got %q", req.URL.Host)
	}
}

func TestNewFrontedPOST_RejectsHTTP(t *testing.T) {
	_, err := newFrontedPOST(context.Background(), "http://script.google.com/x", "", "")
	if err == nil {
		t.Error("expected error for non-https URL")
	}
}

// --- newFrontedGET ---

func TestNewFrontedGET_AbsoluteLocation(t *testing.T) {
	req, err := newFrontedGET(
		context.Background(),
		"www.google.com",
		"https://script.googleusercontent.com/macros/run?id=ABC",
		"https://script.google.com/macros/s/ABC/exec",
	)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Host != "www.google.com" {
		t.Errorf("URL host = %q, want www.google.com", req.URL.Host)
	}
	if req.Host != "script.googleusercontent.com" {
		t.Errorf("Host header = %q, want script.googleusercontent.com", req.Host)
	}
}

// --- relay round-trip with mock server ---

func mockAppsScriptServer(t *testing.T, targetStatus int, targetBody string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Apps Script returning a relay response.
		resp := workerResponse{
			Status: targetStatus,
			Headers: map[string]string{"content-type": "text/plain"},
			Body:   base64.StdEncoding.EncodeToString([]byte(targetBody)),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func srvHost(srv *httptest.Server) string {
	u, _ := url.Parse(srv.URL)
	return u.Host
}

func TestRelayRequest_Success(t *testing.T) {
	srv := mockAppsScriptServer(t, 200, "hello from target")
	defer srv.Close()

	client := srv.Client()
	resp, err := RelayRequest(
		client,
		srv.URL, srvHost(srv), "testkey",
		"GET", "https://example.com",
		map[string]string{},
		nil,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("RelayRequest failed: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if string(resp.Body) != "hello from target" {
		t.Errorf("body = %q, want hello from target", resp.Body)
	}
}

func TestRelayRequest_RelayError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(workerResponse{Error: "unauthorized"})
	}))
	defer srv.Close()

	_, err := RelayRequest(srv.Client(), srv.URL, srvHost(srv), "key", "GET", "https://x.com", nil, nil, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}

func TestRelayRequest_FollowsOneRedirect(t *testing.T) {
	var redirected bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !redirected {
			redirected = true
			w.Header().Set("Location", "/final")
			w.WriteHeader(302)
			return
		}
		resp := workerResponse{
			Status: 200,
			Body:   base64.StdEncoding.EncodeToString([]byte("after redirect")),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	resp, err := RelayRequest(srv.Client(), srv.URL, srvHost(srv), "k", "GET", "https://x.com", nil, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Body) != "after redirect" {
		t.Errorf("body = %q, want 'after redirect'", resp.Body)
	}
}

func TestRelayRequest_ServerError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	_, err := RelayRequest(srv.Client(), srv.URL, srvHost(srv), "k", "GET", "https://x.com", nil, nil, 5*time.Second)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// --- RelayRequestMulti ---

func TestRelayRequestMulti_SingleURL(t *testing.T) {
	good := mockAppsScriptServer(t, 200, "ok")
	defer good.Close()

	resp, err := RelayRequestMulti(
		good.Client(), []string{good.URL}, srvHost(good), "k",
		"GET", "https://example.com", nil, nil, 5*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Body) != "ok" {
		t.Errorf("body = %q, want ok", resp.Body)
	}
}

func TestRelayRequestMulti_Rotates(t *testing.T) {
	// In real usage both Apps Script URLs share the same Google IP — rotation
	// happens at the URL path level. Simulate that with one server, two paths.
	var mu sync.Mutex
	hits := map[string]int{}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		resp := workerResponse{
			Status: 200,
			Body:   base64.StdEncoding.EncodeToString([]byte("ok")),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rrCounter.Store(0)

	urls := []string{srv.URL + "/s/ID1/exec", srv.URL + "/s/ID2/exec"}
	for i := 0; i < 4; i++ {
		if _, err := RelayRequestMulti(srv.Client(), urls, srvHost(srv), "k", "GET", "https://x.com", nil, nil, 5*time.Second); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if hits["/s/ID1/exec"] != 2 || hits["/s/ID2/exec"] != 2 {
		t.Errorf("expected 2 hits each, got %v", hits)
	}
}

func TestRelayRequestMulti_FallsBackOnError(t *testing.T) {
	bad := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "server error")
	}))
	defer bad.Close()

	good := mockAppsScriptServer(t, 200, "fallback ok")
	defer good.Close()

	// Use good.Client() which trusts both test TLS certs (same pool).
	client := good.Client()
	resp, err := RelayRequestMulti(
		client, []string{bad.URL, good.URL}, srvHost(good), "k",
		"GET", "https://example.com", nil, nil, 5*time.Second,
	)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if string(resp.Body) != "fallback ok" {
		t.Errorf("body = %q, want 'fallback ok'", resp.Body)
	}
}

func TestRelayRequestMulti_FallsBackOnQuotaHTMLPage(t *testing.T) {
	quota := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body>Service Unavailable</body></html>")
	}))
	defer quota.Close()

	good := mockAppsScriptServer(t, 204, "")
	defer good.Close()

	client := good.Client()
	resp, err := RelayRequestMulti(
		client, []string{quota.URL, good.URL}, srvHost(good), "k",
		"GET", "https://example.com", nil, nil, 5*time.Second,
	)
	if err != nil {
		t.Fatalf("expected fallback after quota HTML, got error: %v", err)
	}
	if resp.Status != 204 {
		t.Errorf("status = %d, want 204", resp.Status)
	}
}

func TestRelayRequestMulti_AllFail(t *testing.T) {
	bad1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer bad1.Close()
	bad2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer bad2.Close()

	client := bad1.Client()
	_, err := RelayRequestMulti(
		client, []string{bad1.URL, bad2.URL}, srvHost(bad1), "k",
		"GET", "https://example.com", nil, nil, 5*time.Second,
	)
	if err == nil {
		t.Error("expected error when all URLs fail")
	}
}

func TestRelayRequestMulti_EmptyList(t *testing.T) {
	_, err := RelayRequestMulti(
		&http.Client{}, []string{}, "", "k",
		"GET", "https://example.com", nil, nil, 5*time.Second,
	)
	if err == nil {
		t.Error("expected error for empty URL list")
	}
}
