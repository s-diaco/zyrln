package core

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ServeProxy starts the relay HTTP+HTTPS MITM proxy and blocks until it exits.
func ServeProxy(listenAddr, appScriptURL, frontDomain, authKey string, ca *CertAuthority, client *http.Client, timeout time.Duration) error {
	srv, err := listenAndServeProxy(listenAddr, appScriptURL, frontDomain, authKey, ca, client, timeout)
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

// StartProxy starts the relay proxy in the background and returns the server for shutdown.
func StartProxy(listenAddr, appScriptURL, frontDomain, authKey string, ca *CertAuthority, client *http.Client, timeout time.Duration) (*http.Server, error) {
	srv, err := listenAndServeProxy(listenAddr, appScriptURL, frontDomain, authKey, ca, client, timeout)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}

func listenAndServeProxy(listenAddr, appScriptURL, frontDomain, authKey string, ca *CertAuthority, client *http.Client, timeout time.Duration) (*http.Server, error) {
	return &http.Server{
		Addr: listenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleConnect(w, r, client, appScriptURL, frontDomain, authKey, ca, timeout)
			} else {
				handleHTTP(w, r, client, appScriptURL, frontDomain, authKey, timeout)
			}
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}, nil
}

func handleHTTP(w http.ResponseWriter, r *http.Request, client *http.Client, appScriptURL, frontDomain, authKey string, timeout time.Duration) {
	targetURL := r.URL.String()
	if !r.URL.IsAbs() {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = scheme + "://" + r.Host + r.URL.RequestURI()
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	relayResp, err := RelayRequest(client, appScriptURL, frontDomain, authKey, r.Method, targetURL, forwardHeaders(r.Header), body, timeout)
	if err != nil {
		http.Error(w, "relay failed: "+err.Error(), http.StatusBadGateway)
		fmt.Printf("%s %s -> error: %s\n", r.Method, targetURL, err)
		return
	}

	for k, v := range relayResp.Headers {
		if !skipResponseHeader(k) {
			w.Header().Set(k, v)
		}
	}
	w.WriteHeader(relayResp.Status)
	_, _ = w.Write(relayResp.Body)
	fmt.Printf("%s %s -> %d %dB\n", r.Method, targetURL, relayResp.Status, len(relayResp.Body))
}

func handleConnect(w http.ResponseWriter, r *http.Request, client *http.Client, appScriptURL, frontDomain, authKey string, ca *CertAuthority, timeout time.Duration) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	rawConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer rawConn.Close()

	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	host = strings.TrimSpace(host)
	if host == "" {
		_, _ = rawConn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	_, _ = rawConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	cert, err := ca.CertForHost(host)
	if err != nil {
		fmt.Printf("mitm cert %s: %v\n", host, err)
		return
	}

	tlsConn := tls.Server(rawConn, &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	defer tlsConn.Close()

	reader := bufio.NewReader(tlsConn)
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Printf("mitm read %s: %v\n", host, err)
			}
			return
		}

		body, err := io.ReadAll(io.LimitReader(req.Body, 8*1024*1024))
		_ = req.Body.Close()
		if err != nil {
			_, _ = tlsConn.Write([]byte("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n"))
			return
		}

		targetURL := "https://" + host + req.URL.RequestURI()
		relayResp, err := RelayRequest(client, appScriptURL, frontDomain, authKey, req.Method, targetURL, forwardHeaders(req.Header), body, timeout)
		if err != nil {
			writeHTTPError(tlsConn, http.StatusBadGateway, "relay failed: "+err.Error())
			fmt.Printf("%s %s -> error: %s\n", req.Method, targetURL, err)
			return
		}

		resp := &http.Response{
			StatusCode:    relayResp.Status,
			Status:        fmt.Sprintf("%d %s", relayResp.Status, http.StatusText(relayResp.Status)),
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(relayResp.Body)),
			ContentLength: int64(len(relayResp.Body)),
		}
		for k, v := range relayResp.Headers {
			if !skipResponseHeader(k) {
				resp.Header.Set(k, v)
			}
		}
		if strings.EqualFold(req.Header.Get("Connection"), "close") {
			resp.Header.Set("Connection", "close")
		}
		if err := resp.Write(tlsConn); err != nil {
			return
		}
		fmt.Printf("%s %s -> %d %dB\n", req.Method, targetURL, relayResp.Status, len(relayResp.Body))

		if strings.EqualFold(req.Header.Get("Connection"), "close") {
			return
		}
	}
}

func writeHTTPError(conn net.Conn, status int, msg string) {
	resp := &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(msg)),
		ContentLength: int64(len(msg)),
	}
	resp.Header.Set("Content-Type", "text/plain")
	resp.Header.Set("Connection", "close")
	_ = resp.Write(conn)
}

func forwardHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, vs := range h {
		if !skipRequestHeader(k) && len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	if _, ok := out["User-Agent"]; !ok {
		out["User-Agent"] = "zephyr/0.1"
	}
	return out
}

func skipRequestHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "connection", "content-length", "proxy-connection",
		"proxy-authorization", "transfer-encoding", "accept-encoding":
		return true
	}
	return false
}

func skipResponseHeader(key string) bool {
	switch strings.ToLower(key) {
	case "content-length", "transfer-encoding", "connection", "content-encoding":
		return true
	}
	return false
}
