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
	"sync"
	"sync/atomic"
	"time"
)

const maxRelayBody = 16 * 1024 * 1024

// ParseURLList splits a comma-separated URL string and strips all whitespace
// from each entry, including embedded newlines from copy-paste artifacts.
func ParseURLList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, p)
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

var activeURLIdx atomic.Int64

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
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:         (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 15 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}

// RelayRequest sends method+targetURL through the domain-fronted Apps Script relay chain.
// It is a convenience wrapper around RelayRequestMulti with a single URL.
func RelayRequest(
	client *http.Client,
	appScriptURL, frontDomain, authKey,
	method, targetURL string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (RelayResponse, error) {
	return RelayRequestMulti(client, []string{appScriptURL}, frontDomain, authKey, method, targetURL, headers, body, timeout)
}

// RelayRequestMulti uses a sticky circular failover across appScriptURLs.
// It sticks to the current URL until it fails (e.g. quota exhausted), then
// advances to the next one and sticks there. When the last URL fails it wraps
// back to the first, which will have had its quota reset by then.
func RelayRequestMulti(
	client *http.Client,
	appScriptURLs []string,
	frontDomain, authKey,
	method, targetURL string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (RelayResponse, error) {
	n := len(appScriptURLs)
	if n == 0 {
		return RelayResponse{}, fmt.Errorf("no Apps Script URLs configured")
	}
	payload := buildRelayPayload(authKey, method, targetURL, headers, body)
	start := int(activeURLIdx.Load()) % n
	var lastErr error
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		resp, err := tryOneURL(client, appScriptURLs[idx], frontDomain, payload, timeout)
		if err == nil {
			if i > 0 {
				activeURLIdx.Store(int64(idx))
				fmt.Printf("[relay] quota exhausted on URL %d, switched to URL %d/%d\n", start+1, idx+1, n)
			}
			return resp, nil
		}
		lastErr = err
	}
	return RelayResponse{}, lastErr
}

// Coalescer batches concurrent relay requests into a single Apps Script call
// using the existing doBatch_ / fetchAll support in Code.gs.
// Requests that arrive within window of each other are grouped (up to maxBatch).
type Coalescer struct {
	client        *http.Client
	appScriptURLs []string
	frontDomain   string
	authKey       string
	timeout       time.Duration
	window        time.Duration
	maxBatch      int
	ch            chan *coalescerItem
}

type coalescerItem struct {
	method    string
	targetURL string
	headers   map[string]string
	body      []byte
	result    chan coalescerResult
}

type coalescerResult struct {
	resp RelayResponse
	err  error
}

type batchPayloadItem struct {
	Method    string            `json:"m"`
	URL       string            `json:"u"`
	Headers   map[string]string `json:"h"`
	Body      string            `json:"b,omitempty"`
	Redirect  bool              `json:"r"`
}

type batchEnvelope struct {
	Key   string             `json:"k"`
	Items []batchPayloadItem `json:"q"`
}

type batchResponseEnvelope struct {
	Items []workerResponse `json:"q"`
}

// NewCoalescer creates and starts a request coalescer.
func NewCoalescer(client *http.Client, appScriptURLs []string, frontDomain, authKey string, timeout time.Duration) *Coalescer {
	c := &Coalescer{
		client:        client,
		appScriptURLs: appScriptURLs,
		frontDomain:   frontDomain,
		authKey:       authKey,
		timeout:       timeout,
		window:        15 * time.Millisecond,
		maxBatch:      10,
		ch:            make(chan *coalescerItem, 512),
	}
	go c.run()
	return c
}

// Submit queues a relay request and blocks until the response is ready.
func (c *Coalescer) Submit(method, targetURL string, headers map[string]string, body []byte) (RelayResponse, error) {
	item := &coalescerItem{
		method:    method,
		targetURL: targetURL,
		headers:   headers,
		body:      body,
		result:    make(chan coalescerResult, 1),
	}
	c.ch <- item
	r := <-item.result
	return r.resp, r.err
}

func (c *Coalescer) run() {
	for {
		first := <-c.ch
		batch := []*coalescerItem{first}

		timer := time.NewTimer(c.window)
	collect:
		for len(batch) < c.maxBatch {
			select {
			case item := <-c.ch:
				batch = append(batch, item)
			case <-timer.C:
				break collect
			}
		}
		timer.Stop()

		if len(batch) == 1 {
			resp, err := RelayRequestMulti(c.client, c.appScriptURLs, c.frontDomain, c.authKey,
				batch[0].method, batch[0].targetURL, batch[0].headers, batch[0].body, c.timeout)
			batch[0].result <- coalescerResult{resp, err}
			continue
		}

		// Multiple requests — send as one batch call.
		go c.flush(batch)
	}
}

func (c *Coalescer) flush(batch []*coalescerItem) {
	items := make([]batchPayloadItem, len(batch))
	for i, item := range batch {
		pi := batchPayloadItem{
			Method:   strings.ToUpper(item.method),
			URL:      item.targetURL,
			Headers:  item.headers,
			Redirect: true,
		}
		if len(item.body) > 0 {
			pi.Body = base64.StdEncoding.EncodeToString(item.body)
		}
		items[i] = pi
	}

	env := batchEnvelope{Key: c.authKey, Items: items}
	payload, err := json.Marshal(env)
	if err != nil {
		c.failAll(batch, fmt.Errorf("batch marshal: %w", err))
		return
	}

	n := len(c.appScriptURLs)
	start := int(activeURLIdx.Load()) % n
	var raw []byte
	var lastErr error
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		raw, lastErr = appsScriptRoundTrip(c.client, c.appScriptURLs[idx], c.frontDomain, string(payload), c.timeout)
		if lastErr == nil {
			if i > 0 {
				activeURLIdx.Store(int64(idx))
			}
			break
		}
	}
	if lastErr != nil {
		c.failAll(batch, lastErr)
		return
	}

	var env2 batchResponseEnvelope
	if err := json.Unmarshal(raw, &env2); err != nil || len(env2.Items) != len(batch) {
		// Fallback: retry each individually.
		var wg sync.WaitGroup
		for _, item := range batch {
			wg.Add(1)
			go func(it *coalescerItem) {
				defer wg.Done()
				resp, err := RelayRequestMulti(c.client, c.appScriptURLs, c.frontDomain, c.authKey,
					it.method, it.targetURL, it.headers, it.body, c.timeout)
				it.result <- coalescerResult{resp, err}
			}(item)
		}
		wg.Wait()
		return
	}

	for i, wr := range env2.Items {
		if wr.Error != "" {
			batch[i].result <- coalescerResult{err: fmt.Errorf("relay error: %s", wr.Error)}
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(wr.Body)
		if err != nil {
			batch[i].result <- coalescerResult{err: fmt.Errorf("invalid base64: %w", err)}
			continue
		}
		batch[i].result <- coalescerResult{resp: RelayResponse{Status: wr.Status, Headers: wr.Headers, Body: decoded}}
	}
}

func (c *Coalescer) failAll(batch []*coalescerItem, err error) {
	for _, item := range batch {
		item.result <- coalescerResult{err: err}
	}
}

func tryOneURL(client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) (RelayResponse, error) {
	raw, err := appsScriptRoundTrip(client, appScriptURL, frontDomain, payload, timeout)
	if err != nil {
		return RelayResponse{}, err
	}

	var workerResp workerResponse
	if err := json.Unmarshal(raw, &workerResp); err != nil {
		if strings.HasPrefix(strings.TrimSpace(string(raw)), "<") {
			return RelayResponse{}, fmt.Errorf("Apps Script returned an error page — quota may be exceeded or deployment is misconfigured")
		}
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
