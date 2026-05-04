package core

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// --- proxy handler integration ---

func TestHandleHTTP_RelaysRequest(t *testing.T) {
	appScript := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := workerResponse{
			Status:  200,
			Headers: map[string]string{"content-type": "text/plain"},
			Body:    base64.StdEncoding.EncodeToString([]byte("proxied")),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer appScript.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/test", nil)

	handleHTTP(w, r, appScript.Client(), []string{appScript.URL}, appScript.Listener.Addr().String(), "k", 5*time.Second)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "proxied" {
		t.Errorf("body = %q, want proxied", body)
	}
}

func TestHandleHTTP_RelayError_Returns502(t *testing.T) {
	// Apps Script returns 500 — relay should return 502 to client.
	appScript := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer appScript.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/test", nil)

	handleHTTP(w, r, appScript.Client(), []string{appScript.URL}, appScript.Listener.Addr().String(), "k", 5*time.Second)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
