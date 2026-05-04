package core

import (
	"net/http"
	"testing"
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
