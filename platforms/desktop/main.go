package main

import (
	"context"
	"crypto/tls"
	"zyrln/relay/core"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultProxyAddress = "direct"

type probe struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Host        string            `json:"host,omitempty"`
	FrontDomain string            `json:"front_domain,omitempty"`
	Expectation string            `json:"expectation"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
}

type result struct {
	Probe      probe  `json:"probe"`
	Attempt    int    `json:"attempt"`
	OK         bool   `json:"ok"`
	Status     string `json:"status,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Location   string `json:"location,omitempty"`
	Remote     string `json:"remote,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Bytes      int64  `json:"bytes"`
	Preview    string `json:"preview,omitempty"`
	Error      string `json:"error,omitempty"`
}

type report struct {
	GeneratedAt string   `json:"generated_at"`
	Proxy       string   `json:"proxy"`
	Guard       string   `json:"guard"`
	TimeoutMS   int64    `json:"timeout_ms"`
	Repeat      int      `json:"repeat"`
	Results     []result `json:"results"`
	Summary     summary  `json:"summary"`
}

type summary struct {
	Total      int            `json:"total"`
	Reachable  int            `json:"reachable"`
	Failed     int            `json:"failed"`
	Categories map[string]int `json:"reachable_by_category"`
}

type proxyConfig struct {
	label     string
	guard     string
	proxyFunc func(*http.Request) (*url.URL, error)
	proxyHost string
}

func parseProxyConfig(raw string) (proxyConfig, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "direct") || strings.EqualFold(value, "none") {
		return proxyConfig{
			label: "direct",
			guard: "direct dialing enabled for real in-country use",
		}, nil
	}

	proxyURL, err := url.Parse(value)
	if err != nil {
		return proxyConfig{}, err
	}
	if proxyURL.Scheme == "" || proxyURL.Host == "" {
		return proxyConfig{}, fmt.Errorf("expected proxy URL like http://host:port, or 'direct'")
	}

	proxyHost := proxyURL.Host
	if !strings.Contains(proxyHost, ":") {
		proxyHost = net.JoinHostPort(proxyHost, "80")
	}

	return proxyConfig{
		label:     proxyURL.String(),
		guard:     fmt.Sprintf("direct dialing disabled; only %s may be dialed", proxyHost),
		proxyFunc: http.ProxyURL(proxyURL),
		proxyHost: proxyHost,
	}, nil
}

func (p proxyConfig) dialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if p.proxyHost == "" {
		return (&net.Dialer{Timeout: timeout}).DialContext
	}
	return proxyOnlyDialer(p.proxyHost, timeout)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Zyrln — domain-fronting reachability tool

Modes:
  (default)          run reachability probes and print a table
  -init-ca           generate a local CA cert for HTTPS proxy interception
  -serve-proxy       start a local HTTP+HTTPS proxy backed by the relay
  -relay-fetch-url   fetch one URL through the full relay chain
  -export-config     print config as JSON for importing into the Android app

Config: flags can be set in config.env (one key=value per line, flag name as key).

Flags:
`)
		flag.PrintDefaults()
	}

	configFlag := flag.String("config", "config.env", "path to config file (key=value, flag names as keys)")
	proxyFlag := flag.String("proxy", defaultProxyAddress, "HTTP proxy URL for lab testing, or 'direct'/'none' for real in-country use")
	timeoutFlag := flag.Duration("timeout", 12*time.Second, "per-probe timeout")
	repeatFlag := flag.Int("repeat", 1, "number of times to run each probe")
	formatFlag := flag.String("format", "table", "output format: table or json")
	outFlag := flag.String("out", "", "optional path to write the full JSON report")
	categoryFlag := flag.String("category", "", "optional comma-separated category filter")
	appScriptURLFlag := flag.String("appscript-url", "", "optional deployed Apps Script web app URL to probe with GET and POST")
	frontedAppScriptURLFlag := flag.String("fronted-appscript-url", "", "optional deployed Apps Script URL to probe using domain fronting")
	frontDomainFlag := flag.String("front-domain", "www.google.com", "front domain for domain-fronted probes")
	authKeyFlag := flag.String("auth-key", "", "auth key for the relay")
	targetURLFlag := flag.String("target-url", "https://www.gstatic.com/generate_204", "target URL for relay probe and relay-fetch")
	relayFetchURLFlag := flag.String("relay-fetch-url", "", "fetch this target URL through the full relay chain and print the decoded response")
	bodyOutFlag := flag.String("body-out", "", "optional path to write the decoded relay response body")
	serveProxyFlag := flag.Bool("serve-proxy", false, "start a local HTTP proxy backed by the relay")
	listenFlag := flag.String("listen", "127.0.0.1:8085", "listen address for -serve-proxy")
	exportConfigFlag := flag.Bool("export-config", false, "print config as JSON for importing into the Android app")
	initCAFlag := flag.Bool("init-ca", false, "generate a local CA certificate for HTTPS proxy interception")
	caCertFlag := flag.String("ca-cert", "certs/zyrln-ca.pem", "local CA certificate path for HTTPS proxy interception")
	caKeyFlag := flag.String("ca-key", "certs/zyrln-ca-key.pem", "local CA private key path for HTTPS proxy interception")
	frontRedirectsFlag := flag.Bool("front-redirects", false, "when a fronted probe gets a redirect, retry the Location using the front domain and encrypted Host override")
	followRedirectsFlag := flag.Bool("follow-redirects", true, "follow HTTP redirects")
	flag.Parse()

	// Apply config file values for flags not set on the CLI.
	setCLI := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setCLI[f.Name] = true })
	for key, value := range loadConfig(*configFlag) {
		if !setCLI[key] {
			_ = flag.Set(key, value)
		}
	}

	if *repeatFlag < 1 {
		fmt.Fprintln(os.Stderr, "repeat must be at least 1")
		os.Exit(1)
	}

	if *exportConfigFlag {
		url := strings.TrimSpace(*frontedAppScriptURLFlag)
		key := strings.TrimSpace(*authKeyFlag)
		if url == "" || key == "" {
			fmt.Fprintln(os.Stderr, "-export-config requires -fronted-appscript-url and -auth-key (or config.env)")
			os.Exit(1)
		}
		out, _ := json.Marshal(map[string]string{"url": url, "key": key})
		fmt.Println(string(out))
		return
	}

	if *initCAFlag {
		if err := core.GenerateCA(*caCertFlag, *caKeyFlag); err != nil {
			fmt.Fprintf(os.Stderr, "failed to generate CA: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("generated CA certificate: %s\n", *caCertFlag)
		fmt.Printf("generated CA private key: %s\n", *caKeyFlag)
		fmt.Printf("install the certificate, not the key, as a trusted CA on the test device\n")
		return
	}

	proxyCfg, err := parseProxyConfig(*proxyFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid proxy: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Timeout: *timeoutFlag,
		Transport: &http.Transport{
			Proxy:           proxyCfg.proxyFunc,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:     proxyCfg.dialContext(*timeoutFlag),
		},
	}
	if !*followRedirectsFlag {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	if strings.TrimSpace(*relayFetchURLFlag) != "" {
		if strings.TrimSpace(*frontedAppScriptURLFlag) == "" {
			fmt.Fprintln(os.Stderr, "-relay-fetch-url requires -fronted-appscript-url")
			os.Exit(1)
		}
		if strings.TrimSpace(*authKeyFlag) == "" {
			fmt.Fprintln(os.Stderr, "-relay-fetch-url requires -auth-key")
			os.Exit(1)
		}
		if err := relayFetch(client, *frontedAppScriptURLFlag, *frontDomainFlag, *authKeyFlag, *relayFetchURLFlag, *bodyOutFlag, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "relay fetch failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *serveProxyFlag {
		if strings.TrimSpace(*frontedAppScriptURLFlag) == "" {
			fmt.Fprintln(os.Stderr, "-serve-proxy requires -fronted-appscript-url")
			os.Exit(1)
		}
		if strings.TrimSpace(*authKeyFlag) == "" {
			fmt.Fprintln(os.Stderr, "-serve-proxy requires -auth-key")
			os.Exit(1)
		}
		ca, err := core.LoadCA(*caCertFlag, *caKeyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load CA: %v\nrun -init-ca first\n", err)
			os.Exit(1)
		}
		fmt.Printf("relay HTTP proxy listening on http://%s\n", *listenFlag)
		fmt.Printf("mode: HTTP and HTTPS via local CA MITM; install %s as trusted CA for browsers\n", *caCertFlag)
		if err := core.ServeProxy(*listenFlag, *frontedAppScriptURLFlag, *frontDomainFlag, *authKeyFlag, ca, client, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "proxy failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	probes := filterProbes(defaultProbes(), *categoryFlag)
	if strings.TrimSpace(*appScriptURLFlag) != "" {
		probes = append(probes, appScriptProbes(strings.TrimSpace(*appScriptURLFlag))...)
	}
	if strings.TrimSpace(*frontedAppScriptURLFlag) != "" {
		fp, err := frontedAppScriptProbes(
			strings.TrimSpace(*frontedAppScriptURLFlag),
			strings.TrimSpace(*frontDomainFlag),
			strings.TrimSpace(*authKeyFlag),
			strings.TrimSpace(*targetURLFlag),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid fronted Apps Script URL: %v\n", err)
			os.Exit(1)
		}
		probes = append(probes, fp...)
	}
	if len(probes) == 0 {
		fmt.Fprintln(os.Stderr, "no probes selected")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "running %d probe(s)...\n", len(probes)**repeatFlag)
	results := make([]result, 0, len(probes)**repeatFlag)
	for attempt := 1; attempt <= *repeatFlag; attempt++ {
		for _, p := range probes {
			results = append(results, runProbe(client, p, attempt, *timeoutFlag, *frontRedirectsFlag))
		}
	}

	rep := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Proxy:       proxyCfg.label,
		Guard:       proxyCfg.guard,
		TimeoutMS:   timeoutFlag.Milliseconds(),
		Repeat:      *repeatFlag,
		Results:     results,
		Summary:     summarize(results),
	}

	if *outFlag != "" {
		if err := writeJSONReport(*outFlag, rep); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
	}

	switch strings.ToLower(*formatFlag) {
	case "table":
		printTable(rep)
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(rep); err != nil {
			fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q; use table or json\n", *formatFlag)
		os.Exit(1)
	}
}

func relayFetch(client *http.Client, appScriptURL, frontDomain, authKey, targetURL, bodyOut string, timeout time.Duration) error {
	resp, err := core.RelayRequest(client, appScriptURL, frontDomain, authKey, "GET", targetURL, map[string]string{"User-Agent": "zyrln/0.1"}, nil, timeout)
	if err != nil {
		return err
	}
	if bodyOut != "" {
		if err := writeBody(bodyOut, resp.Body); err != nil {
			return err
		}
	}
	fmt.Printf("relay fetch ok\ntarget: %s\nstatus: %d\nheaders: %d\nbody bytes: %d\n", targetURL, resp.Status, len(resp.Headers), len(resp.Body))
	if bodyOut != "" {
		fmt.Printf("body written: %s\n", bodyOut)
	}
	if len(resp.Body) > 0 {
		fmt.Printf("preview: %s\n", preview(resp.Body, 1200))
	}
	return nil
}

func defaultProbes() []probe {
	return []probe{
		{ID: "google-home", Name: "Google search edge", Category: "baseline", Method: http.MethodHead, URL: "https://www.google.com/", Expectation: "baseline HTTPS reachability"},
		{ID: "android-204", Name: "Android connectivity check", Category: "baseline", Method: http.MethodGet, URL: "https://clients3.google.com/generate_204", Expectation: "small Google HTTPS response used by Android captive-portal checks"},
		{ID: "gstatic-204", Name: "Gstatic static edge", Category: "baseline", Method: http.MethodGet, URL: "https://www.gstatic.com/generate_204", Expectation: "Google static/CDN hostname"},
		{ID: "googleapis-discovery", Name: "Google APIs root", Category: "api", Method: http.MethodGet, URL: "https://www.googleapis.com/discovery/v1/apis", Expectation: "Google API surface without app-specific backend"},
		{ID: "google-doh", Name: "Google DoH JSON", Category: "api", Method: http.MethodGet, URL: "https://dns.google/resolve?name=google.com&type=A", Expectation: "DNS-over-HTTPS reachability through Google"},
		{ID: "apps-script", Name: "Apps Script", Category: "serverless", Method: http.MethodHead, URL: "https://script.google.com/", Expectation: "possible serverless web-app front door"},
		{ID: "apps-script-content", Name: "Apps Script content host", Category: "serverless", Method: http.MethodHead, URL: "https://script.googleusercontent.com/", Expectation: "Apps Script web apps often redirect here for execution output"},
		{ID: "firebase-hosting", Name: "Firebase hosting", Category: "serverless", Method: http.MethodHead, URL: "https://firebase.google.com/", Expectation: "Firebase-hosted HTTPS surface"},
		{ID: "cloud-run-api", Name: "Cloud Run API", Category: "serverless", Method: http.MethodGet, URL: "https://run.googleapis.com/", Expectation: "Cloud Run control/API hostname reachability"},
		{ID: "cloud-functions-api", Name: "Cloud Functions API", Category: "serverless", Method: http.MethodGet, URL: "https://cloudfunctions.googleapis.com/", Expectation: "Cloud Functions API hostname reachability"},
		{ID: "storage-api", Name: "Google storage API", Category: "serverless", Method: http.MethodGet, URL: "https://storage.googleapis.com/", Expectation: "Google Cloud Storage public edge"},
		{
			ID: "websocket-shape", Name: "WebSocket upgrade shape", Category: "transport",
			Method: http.MethodGet, URL: "https://www.google.com/",
			Headers: map[string]string{
				"Connection": "Upgrade", "Upgrade": "websocket",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==", "Sec-WebSocket-Version": "13",
			},
			Expectation: "checks whether upgrade-shaped HTTPS reaches the edge; 101 is not expected from Google",
		},
	}
}

func appScriptProbes(rawURL string) []probe {
	return []probe{
		{ID: "appscript-deployed-get", Name: "Apps Script deployed GET", Category: "serverless-live", Method: http.MethodGet, URL: addQuery(rawURL, "mode=probe&size=small"), Expectation: "deployed Apps Script web app accepts small GET messages"},
		{ID: "appscript-deployed-post", Name: "Apps Script deployed POST", Category: "serverless-live", Method: http.MethodPost, URL: rawURL, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"mode":"probe","size":"small","message":"zyrln probe"}`, Expectation: "deployed Apps Script web app accepts small POST messages"},
	}
}

func frontedAppScriptProbes(rawURL, frontDomain, authKey, targetURL string) ([]probe, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("expected https URL, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if frontDomain == "" {
		frontDomain = "www.google.com"
	}

	frontedBase := *parsed
	frontedBase.Host = frontDomain

	probes := []probe{
		{ID: "fronted-appscript-get", Name: "Fronted Apps Script GET", Category: "domain-front", Method: http.MethodGet, URL: addQuery(frontedBase.String(), "mode=probe&size=small"), Host: parsed.Host, FrontDomain: frontDomain, Expectation: "domain-fronted GET"},
		{ID: "fronted-appscript-post", Name: "Fronted Apps Script POST", Category: "domain-front", Method: http.MethodPost, URL: frontedBase.String(), Host: parsed.Host, FrontDomain: frontDomain, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"mode":"probe","size":"small","message":"zyrln domain-front probe"}`, Expectation: "domain-fronted POST"},
	}

	if strings.TrimSpace(authKey) != "" {
		payload := map[string]any{
			"k": authKey, "m": "GET", "u": targetURL,
			"h": map[string]string{"User-Agent": "zyrln/0.1"},
			"ct": nil, "r": true,
		}
		encoded, _ := json.Marshal(payload)
		probes = append(probes, probe{
			ID: "fronted-relay-post", Name: "Fronted relay POST", Category: "domain-front",
			Method: http.MethodPost, URL: frontedBase.String(), Host: parsed.Host, FrontDomain: frontDomain,
			Headers: map[string]string{"Content-Type": "application/json"}, Body: string(encoded),
			Expectation: "relay payload through fronted Apps Script",
		})
	}

	return probes, nil
}

func filterProbes(probes []probe, categoryCSV string) []probe {
	if strings.TrimSpace(categoryCSV) == "" {
		return probes
	}
	allowed := map[string]bool{}
	for _, raw := range strings.Split(categoryCSV, ",") {
		if c := strings.ToLower(strings.TrimSpace(raw)); c != "" {
			allowed[c] = true
		}
	}
	filtered := make([]probe, 0, len(probes))
	for _, p := range probes {
		if allowed[strings.ToLower(p.Category)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func proxyOnlyDialer(proxyHost string, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != proxyHost {
			return nil, fmt.Errorf("blocked direct dial to %s; only proxy %s is allowed", address, proxyHost)
		}
		return dialer.DialContext(ctx, network, address)
	}
}

func runProbe(client *http.Client, p probe, attempt int, timeout time.Duration, frontRedirects bool) result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return runProbeCtx(ctx, client, p, attempt, frontRedirects)
}

func runProbeCtx(ctx context.Context, client *http.Client, p probe, attempt int, frontRedirects bool) result {
	res := executeProbe(ctx, client, p, attempt)

	if frontRedirects && p.FrontDomain != "" && isRedirect(res.StatusCode) && res.Location != "" {
		next, err := frontedRedirectProbe(p, res.Location)
		if err != nil {
			res.Error = "front redirect build failed: " + err.Error()
			return res
		}
		nextRes := runProbeCtx(ctx, client, next, attempt, false)
		nextRes.Probe.ID = p.ID + "-front-redirect"
		nextRes.Probe.Name = p.Name + " redirect"
		nextRes.Probe.Expectation = p.Expectation + " redirected with fronting"
		return nextRes
	}
	return res
}

func executeProbe(ctx context.Context, client *http.Client, p probe, attempt int) result {
	var remote string
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn != nil {
				remote = info.Conn.RemoteAddr().String()
			}
		},
	}

	var body io.Reader
	if p.Body != "" {
		body = strings.NewReader(p.Body)
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), p.Method, p.URL, body)
	if err != nil {
		return result{Probe: p, Attempt: attempt, Error: err.Error()}
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	if p.Host != "" {
		req.Host = p.Host
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		return result{Probe: p, Attempt: attempt, DurationMS: elapsed.Milliseconds(), Remote: remote, Error: compactError(err)}
	}
	defer resp.Body.Close()

	limited, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	return result{
		Probe:      p,
		Attempt:    attempt,
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 500,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Location:   resp.Header.Get("Location"),
		Remote:     remote,
		DurationMS: elapsed.Milliseconds(),
		Bytes:      int64(len(limited)),
		Preview:    preview(limited, 512),
	}
}

func frontedRedirectProbe(original probe, location string) (probe, error) {
	redirectURL, err := url.Parse(location)
	if err != nil {
		return probe{}, err
	}
	if redirectURL.Scheme == "" || redirectURL.Host == "" {
		base, err := url.Parse(original.URL)
		if err != nil {
			return probe{}, err
		}
		redirectURL = base.ResolveReference(redirectURL)
	}

	frontedURL := *redirectURL
	frontedURL.Host = original.FrontDomain

	return probe{
		ID: original.ID + "-front-redirect", Name: original.Name + " redirect",
		Category: original.Category, Method: http.MethodGet,
		URL: frontedURL.String(), Host: redirectURL.Host, FrontDomain: original.FrontDomain,
		Expectation: "fronted follow-up to " + location,
	}, nil
}

func isRedirect(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func addQuery(rawURL, query string) string {
	if strings.Contains(rawURL, "?") {
		return rawURL + "&" + query
	}
	return rawURL + "?" + query
}

func summarize(results []result) summary {
	s := summary{Total: len(results), Categories: map[string]int{}}
	for _, r := range results {
		if r.OK {
			s.Reachable++
			s.Categories[r.Probe.Category]++
		} else {
			s.Failed++
		}
	}
	return s
}

func writeJSONReport(path string, rep report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func writeBody(path string, body []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, body, 0644)
}

func printTable(rep report) {
	fmt.Printf("proxy: %s\nguard: %s\ngenerated: %s\nsummary: %d reachable, %d failed, %d total\n\n",
		rep.Proxy, rep.Guard, rep.GeneratedAt, rep.Summary.Reachable, rep.Summary.Failed, rep.Summary.Total)

	fmt.Printf("%-4s %-24s %-10s %-5s %-12s %-8s %-8s %s\n", "TRY", "PROBE", "CATEGORY", "OK", "STATUS", "PROTO", "TIME", "REMOTE/ERROR")
	fmt.Printf("%s\n", strings.Repeat("-", 120))

	for _, r := range rep.Results {
		ok := "no"
		if r.OK {
			ok = "yes"
		}
		status := r.Status
		if status == "" {
			status = "-"
		}
		proto := r.Proto
		if proto == "" {
			proto = "-"
		}
		remoteOrError := r.Remote
		if r.Error != "" {
			remoteOrError = r.Error
		} else if r.Location != "" {
			remoteOrError = "redirect " + r.Location
		}
		fmt.Printf("%-4d %-24s %-10s %-5s %-12s %-8s %-8s %s\n",
			r.Attempt, truncate(r.Probe.Name, 24), truncate(r.Probe.Category, 10),
			ok, truncate(status, 12), proto, fmt.Sprintf("%dms", r.DurationMS), remoteOrError)
	}

	fmt.Printf("\nNotes:\n")
	fmt.Printf("- Any HTTP status below 500 counts as reachable.\n")
	fmt.Printf("- HTTP/3/QUIC is not tested here (TCP-based path only).\n")
	fmt.Printf("- WebSocket support needs a real WebSocket backend.\n")
}

func preview(body []byte, max int) string {
	if len(body) == 0 {
		return ""
	}
	value := strings.TrimSpace(string(body))
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}

func compactError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var urlErr *url.Error
	if urlErr2, ok := err.(*url.Error); ok && urlErr2.Err != nil {
		_ = urlErr
		msg = urlErr2.Err.Error()
	}
	return strings.ReplaceAll(msg, "\n", " ")
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func loadConfig(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" {
			values[key] = value
		}
	}
	return values
}
