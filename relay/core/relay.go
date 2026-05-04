package core

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRelayBody = 16 * 1024 * 1024

type workerResponse struct {
	Status  int               `json:"s"`
	Headers map[string]string `json:"h"`
	Body    string            `json:"b"`
	Error   string            `json:"e"`
}

// RelayResponse is the decoded response from the relay chain.
type RelayResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

// NewHTTPClient returns an http.Client configured for relay use.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:     (&net.Dialer{Timeout: timeout}).DialContext,
		},
	}
}

// RelayRequest sends method+targetURL through the domain-fronted Apps Script relay chain.
func RelayRequest(
	client *http.Client,
	appScriptURL, frontDomain, authKey,
	method, targetURL string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (RelayResponse, error) {
	payload := buildRelayPayload(authKey, method, targetURL, headers, body)
	raw, err := appsScriptRoundTrip(client, appScriptURL, frontDomain, payload, timeout)
	if err != nil {
		return RelayResponse{}, err
	}

	var workerResp workerResponse
	if err := json.Unmarshal(raw, &workerResp); err != nil {
		return RelayResponse{}, fmt.Errorf("invalid relay JSON: %w; body=%s", err, previewBytes(raw, 256))
	}
	if workerResp.Error != "" {
		return RelayResponse{}, fmt.Errorf("relay error: %s", workerResp.Error)
	}

	decoded, err := base64.StdEncoding.DecodeString(workerResp.Body)
	if err != nil {
		return RelayResponse{}, fmt.Errorf("invalid base64 body: %w", err)
	}

	return RelayResponse{Status: workerResp.Status, Headers: workerResp.Headers, Body: decoded}, nil
}

// appsScriptRoundTrip posts payload to the fronted Apps Script URL, following one redirect if needed.
func appsScriptRoundTrip(client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) ([]byte, error) {
	noRedir := noRedirectClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := newFrontedPOST(ctx, appScriptURL, frontDomain, payload)
	if err != nil {
		return nil, err
	}

	status, location, body, errStr := doHTTP(noRedir, req)
	if errStr != "" {
		return nil, fmt.Errorf("relay POST failed: %s", errStr)
	}

	if isRedirect(status) && location != "" {
		req2, err := newFrontedGET(ctx, frontDomain, location, appScriptURL)
		if err != nil {
			return nil, err
		}
		status, _, body, errStr = doHTTP(noRedir, req2)
		if errStr != "" {
			return nil, fmt.Errorf("relay redirect failed: %s", errStr)
		}
	}

	if status < 200 || status >= 500 {
		return nil, fmt.Errorf("relay returned %d: %s", status, previewBytes(body, 256))
	}
	return body, nil
}

func newFrontedPOST(ctx context.Context, appScriptURL, frontDomain, payload string) (*http.Request, error) {
	parsed, err := url.Parse(appScriptURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("expected https Apps Script URL")
	}
	front := effectiveFrontDomain(frontDomain)

	fronted := *parsed
	fronted.Host = front

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fronted.String(), strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Host = parsed.Host
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func newFrontedGET(ctx context.Context, frontDomain, location, baseURL string) (*http.Request, error) {
	loc, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	if loc.Scheme == "" || loc.Host == "" {
		base, _ := url.Parse(baseURL)
		loc = base.ResolveReference(loc)
	}

	originalHost := loc.Host
	front := effectiveFrontDomain(frontDomain)
	fronted := *loc
	fronted.Host = front

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fronted.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Host = originalHost
	return req, nil
}

func doHTTP(client *http.Client, req *http.Request) (status int, location string, body []byte, errStr string) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, compactErr(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxRelayBody))
	return resp.StatusCode, resp.Header.Get("Location"), data, ""
}

func buildRelayPayload(authKey, method, targetURL string, headers map[string]string, body []byte) string {
	payload := map[string]any{
		"k": authKey,
		"m": strings.ToUpper(method),
		"u": targetURL,
		"h": headers,
		"r": true,
	}
	if len(body) > 0 {
		payload["b"] = base64.StdEncoding.EncodeToString(body)
	}
	if ct := headers["Content-Type"]; ct != "" {
		payload["ct"] = ct
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func noRedirectClient(src *http.Client) *http.Client {
	c := *src
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func effectiveFrontDomain(frontDomain string) string {
	if strings.TrimSpace(frontDomain) == "" {
		return "www.google.com"
	}
	return frontDomain
}

func previewBytes(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func compactErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		msg = urlErr.Err.Error()
	}
	return strings.ReplaceAll(msg, "\n", " ")
}
