package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"zyrln/relay/core"
)

// --- loadConfig ---

func TestLoadConfig_BasicParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("foo = bar\nbaz=qux\n"), 0644)

	got := loadConfig(path)
	if got["foo"] != "bar" {
		t.Errorf("foo = %q, want bar", got["foo"])
	}
	if got["baz"] != "qux" {
		t.Errorf("baz = %q, want qux", got["baz"])
	}
}

func TestLoadConfig_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("# comment\nkey=value\n"), 0644)

	got := loadConfig(path)
	if _, ok := got["# comment"]; ok {
		t.Error("comment line should be ignored")
	}
	if got["key"] != "value" {
		t.Errorf("key = %q, want value", got["key"])
	}
}

func TestLoadConfig_IgnoresEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("\n\nkey=value\n\n"), 0644)

	got := loadConfig(path)
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got))
	}
}

func TestLoadConfig_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("token=abc=def=ghi\n"), 0644)

	got := loadConfig(path)
	if got["token"] != "abc=def=ghi" {
		t.Errorf("token = %q, want abc=def=ghi", got["token"])
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	got := loadConfig("/nonexistent/path/config.env")
	if got != nil && len(got) != 0 {
		t.Error("expected nil or empty map for missing file")
	}
}

func TestShouldStartGUIByDefault_WindowsNoArgs(t *testing.T) {
	if !shouldStartGUIByDefault("windows", []string{"zyrln-windows-amd64.exe"}) {
		t.Error("expected Windows no-arg launch to start GUI")
	}
}

func TestShouldStartGUIByDefault_PreservesExplicitCLIArgs(t *testing.T) {
	if shouldStartGUIByDefault("windows", []string{"zyrln-windows-amd64.exe", "-init-ca"}) {
		t.Error("expected explicit Windows CLI args to preserve CLI mode")
	}
}

func TestShouldStartGUIByDefault_NonWindows(t *testing.T) {
	if shouldStartGUIByDefault("linux", []string{"zyrln-linux-amd64"}) {
		t.Error("expected non-Windows no-arg launch to preserve existing default mode")
	}
}

// --- filterProbes ---

func TestFilterProbes_EmptyCategory(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
	}
	got := filterProbes(probes, "")
	if len(got) != 2 {
		t.Errorf("expected all probes, got %d", len(got))
	}
}

func TestFilterProbes_MatchesCategory(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
		{ID: "c", Category: "baseline"},
	}
	got := filterProbes(probes, "baseline")
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestFilterProbes_MultipleCategories(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
		{ID: "c", Category: "serverless"},
	}
	got := filterProbes(probes, "baseline,api")
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestFilterProbes_CaseInsensitive(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "Baseline"},
	}
	got := filterProbes(probes, "baseline")
	if len(got) != 1 {
		t.Errorf("expected 1, got %d", len(got))
	}
}

func TestFilterProbes_NoMatch(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
	}
	got := filterProbes(probes, "api")
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

// --- addQuery ---

func TestAddQuery_NoExistingQuery(t *testing.T) {
	got := addQuery("https://example.com/path", "a=1")
	if got != "https://example.com/path?a=1" {
		t.Errorf("got %q", got)
	}
}

func TestAddQuery_ExistingQuery(t *testing.T) {
	got := addQuery("https://example.com/path?x=1", "a=1")
	if got != "https://example.com/path?x=1&a=1" {
		t.Errorf("got %q", got)
	}
}

// --- summarize ---

func TestSummarize_Counts(t *testing.T) {
	results := []result{
		{OK: true, Probe: probe{Category: "baseline"}},
		{OK: true, Probe: probe{Category: "baseline"}},
		{OK: false, Probe: probe{Category: "api"}},
	}
	s := summarize(results)
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Reachable != 2 {
		t.Errorf("Reachable = %d, want 2", s.Reachable)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}
	if s.Categories["baseline"] != 2 {
		t.Errorf("Categories[baseline] = %d, want 2", s.Categories["baseline"])
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := summarize(nil)
	if s.Total != 0 || s.Reachable != 0 || s.Failed != 0 {
		t.Error("expected all zeros for empty results")
	}
}

// --- truncate ---

func TestTruncate_ShortString(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short: got %q", got)
	}
}

func TestTruncate_LongString(t *testing.T) {
	if got := truncate("hello world", 8); got != "hello..." {
		t.Errorf("truncate long: got %q", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("truncate exact: got %q", got)
	}
}

// --- preview ---

func TestPreview_Short(t *testing.T) {
	if got := preview([]byte("hello"), 100); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestPreview_Truncated(t *testing.T) {
	got := preview([]byte("hello world extra"), 10)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis, got %q", got)
	}
	if len(got) != 10 {
		t.Errorf("expected len 10, got %d", len(got))
	}
}

func TestPreview_StripsNewlines(t *testing.T) {
	got := preview([]byte("line1\nline2\r\nline3"), 100)
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("expected no newlines, got %q", got)
	}
}

func TestPreview_Empty(t *testing.T) {
	if got := preview(nil, 10); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- parseProxyConfig ---

func TestParseProxyConfig_Direct(t *testing.T) {
	for _, v := range []string{"direct", "Direct", "DIRECT", "none", "", "  "} {
		cfg, err := parseProxyConfig(v)
		if err != nil {
			t.Errorf("parseProxyConfig(%q): unexpected error: %v", v, err)
		}
		if cfg.label != "direct" {
			t.Errorf("parseProxyConfig(%q).label = %q, want direct", v, cfg.label)
		}
		if cfg.proxyFunc != nil {
			t.Errorf("parseProxyConfig(%q): expected nil proxyFunc for direct", v)
		}
	}
}

func TestParseProxyConfig_ValidURL(t *testing.T) {
	cfg, err := parseProxyConfig("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.proxyFunc == nil {
		t.Error("expected non-nil proxyFunc")
	}
	if !strings.Contains(cfg.proxyHost, "8080") {
		t.Errorf("proxyHost = %q, expected port 8080", cfg.proxyHost)
	}
}

func TestParseProxyConfig_InvalidURL(t *testing.T) {
	_, err := parseProxyConfig("not-a-url")
	if err == nil {
		t.Error("expected error for URL without scheme/host")
	}
}

// --- frontedAppScriptProbes ---

func TestFrontedAppScriptProbes_RejectsHTTP(t *testing.T) {
	_, err := frontedAppScriptProbes("http://script.google.com/x", "www.google.com", "", "https://t.com")
	if err == nil {
		t.Error("expected error for http scheme")
	}
}

func TestFrontedAppScriptProbes_RejectsMissingHost(t *testing.T) {
	_, err := frontedAppScriptProbes("https:///path", "www.google.com", "", "https://t.com")
	if err == nil {
		t.Error("expected error for missing host")
	}
}

func TestFrontedAppScriptProbes_BasicProbes(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"www.google.com", "", "https://t.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(probes) < 2 {
		t.Errorf("expected at least 2 probes, got %d", len(probes))
	}
	for _, p := range probes {
		if !strings.Contains(p.URL, "www.google.com") {
			t.Errorf("probe URL %q should contain front domain", p.URL)
		}
		if p.Host != "script.google.com" {
			t.Errorf("probe Host = %q, want script.google.com", p.Host)
		}
	}
}

func TestFrontedAppScriptProbes_WithAuthKeyAddsRelayProbe(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"www.google.com", "secretkey", "https://t.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) < 3 {
		t.Errorf("expected 3+ probes with auth key, got %d", len(probes))
	}
	found := false
	for _, p := range probes {
		if p.ID == "fronted-relay-post" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fronted-relay-post probe when auth key provided")
	}
}

func TestFrontedAppScriptProbes_DefaultFrontDomain(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"", "", "https://t.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range probes {
		if !strings.Contains(p.URL, "www.google.com") {
			t.Errorf("expected default front domain www.google.com in URL, got %q", p.URL)
		}
	}
}

// --- GUI API ---

func TestGUIStatusDefault(t *testing.T) {
	resetGUIStateForTest(t)
	handler := newTestGUIHandler(t, t.TempDir(), nil)

	var got map[string]any
	getJSON(t, handler, "/api/status", &got)

	if got["running"] != false {
		t.Fatalf("running = %v, want false", got["running"])
	}
	if got["uptime"] != "00:00:00" {
		t.Fatalf("uptime = %v, want 00:00:00", got["uptime"])
	}
}

func TestGUIConfigRoundTrip(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	body := bytes.NewBufferString(`{"fronted-appscript-url":"https://script.google.com/macros/s/ABC/exec","auth-key":"secret","listen":"127.0.0.1:8085"}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/config", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/config status = %d, body=%s", resp.Code, resp.Body.String())
	}

	var got map[string]string
	getJSON(t, handler, "/api/config", &got)
	if got["fronted-appscript-url"] != "https://script.google.com/macros/s/ABC/exec" {
		t.Errorf("url = %q", got["fronted-appscript-url"])
	}
	if got["auth-key"] != "secret" {
		t.Errorf("auth-key = %q", got["auth-key"])
	}
	if got["listen"] != "127.0.0.1:8085" {
		t.Errorf("listen = %q", got["listen"])
	}
}

func TestGUIExportReadsConfig(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\n")
	handler := newTestGUIHandler(t, dir, nil)

	var got map[string]string
	getJSON(t, handler, "/api/export", &got)

	if got["url"] != "https://script.google.com/macros/s/ABC/exec" {
		t.Errorf("url = %q", got["url"])
	}
	if got["key"] != "secret" {
		t.Errorf("key = %q", got["key"])
	}
}

func TestGUIInitCAAndDownload(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/init-ca", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/init-ca status = %d, body=%s", resp.Code, resp.Body.String())
	}
	var initResp map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("decode init response: %v", err)
	}
	if initResp["status"] != "ok" || initResp["serial"] == "" || initResp["serial"] == "unknown" {
		t.Fatalf("unexpected init response: %#v", initResp)
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/download-ca", nil))
	if download.Code != http.StatusOK {
		t.Fatalf("GET /api/download-ca status = %d, body=%s", download.Code, download.Body.String())
	}
	if got := download.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(download.Body.String(), "BEGIN CERTIFICATE") {
		t.Error("downloaded CA does not look like PEM")
	}
}

func TestGUIStartRequiresConfig(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("empty config status = %d, want %d", resp.Code, http.StatusBadRequest)
	}

	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\n")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing auth key status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestGUIStartRequiresExistingCA(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\n")
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing CA status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "Install certificate first") {
		t.Fatalf("missing CA body = %q", resp.Body.String())
	}
}

func TestGUIStartStopWithInjectedStarter(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\nlisten = 127.0.0.1:9090\n")

	var started atomic.Bool
	starter := func(listen string, urls []string, key string, ca *core.CertAuthority) (*http.Server, net.Listener, error) {
		started.Store(true)
		if listen != "127.0.0.1:9090" {
			t.Errorf("listen = %q", listen)
		}
		if len(urls) != 1 || urls[0] != "https://script.google.com/macros/s/ABC/exec" {
			t.Errorf("urls = %v", urls)
		}
		if key != "secret" {
			t.Errorf("key = %q", key)
		}
		if ca == nil {
			t.Error("ca is nil")
		}
		return &http.Server{}, noopListener{}, nil
	}
	handler := newTestGUIHandler(t, dir, starter)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/init-ca", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/init-ca status = %d, body=%s", resp.Code, resp.Body.String())
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/start status = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !started.Load() {
		t.Fatal("starter was not called")
	}

	var status map[string]any
	getJSON(t, handler, "/api/status", &status)
	if status["running"] != true {
		t.Fatalf("running after start = %v, want true", status["running"])
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/stop", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/stop status = %d, body=%s", resp.Code, resp.Body.String())
	}
	getJSON(t, handler, "/api/status", &status)
	if status["running"] != false {
		t.Fatalf("running after stop = %v, want false", status["running"])
	}
}

func newTestGUIHandler(t *testing.T, dir string, starter guiProxyStarter) http.Handler {
	t.Helper()
	if starter == nil {
		starter = func(string, []string, string, *core.CertAuthority) (*http.Server, net.Listener, error) {
			t.Fatal("unexpected proxy start")
			return nil, nil, nil
		}
	}
	configPath := filepath.Join(dir, "config.env")
	caCertPath := filepath.Join(dir, "certs", "zyrln-ca.pem")
	caKeyPath := filepath.Join(dir, "certs", "zyrln-ca-key.pem")
	return newGUIHandler(configPath, caCertPath, caKeyPath, starter, func(string) {})
}

func resetGUIStateForTest(t *testing.T) {
	t.Helper()
	guiMu.Lock()
	if guiProxyLn != nil {
		_ = guiProxyLn.Close()
	}
	if guiProxyServer != nil {
		_ = guiProxyServer.Close()
	}
	guiProxyServer = nil
	guiProxyLn = nil
	guiProxyStartTime = time.Time{}
	guiMu.Unlock()
	atomic.StoreInt64(&guiRequestCount, 0)
}

func writeConfigForTest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.env"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, handler http.Handler, path string, out any) {
	t.Helper()
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body=%s", path, resp.Code, resp.Body.String())
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s JSON: %v; body=%s", path, err, data)
	}
}

type noopListener struct{}

func (noopListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (noopListener) Close() error              { return nil }
func (noopListener) Addr() net.Addr            { return noopAddr("noop") }

type noopAddr string

func (a noopAddr) Network() string { return string(a) }
func (a noopAddr) String() string  { return string(a) }
