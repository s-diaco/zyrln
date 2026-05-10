package core

import (
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// directEnabled controls whether Google domains are routed directly via TLS
// fragmentation. Atomic so toggling from UI while requests are in flight is safe.
var directEnabled atomic.Bool

func init() { directEnabled.Store(true) }

// SetDirectEnabled sets the direct-mode flag safely from any goroutine.
func SetDirectEnabled(v bool) { directEnabled.Store(v) }

// GetDirectEnabled returns the current direct-mode flag value.
func GetDirectEnabled() bool { return directEnabled.Load() }

// googleDomains lists suffixes that should be dialed directly using TLS
// fragmentation instead of being routed through the Apps Script relay.
// These domains are not IP-blocked in Iran — only SNI-filtered.
var googleDomains = []string{
	".google.com",
	".googleapis.com",
	".googlevideo.com",
	".googleusercontent.com",
	".gstatic.com",
	".youtube.com",
	".ytimg.com",
	".ggpht.com",
	".googletagmanager.com",
	".googletagservices.com",
	".googlesyndication.com",
	".gmail.com",
	".googlemail.com",
	".google-analytics.com",
	".googleadservices.com",
	".doubleclick.net",
	".android.com",
	".appspot.com",
	".withgoogle.com",
}

// IsDirectDomain reports whether host should bypass the relay and be dialed
// directly using the TLS fragment technique. Returns false when direct mode is off.
func IsDirectDomain(host string) bool {
	if !GetDirectEnabled() {
		return false
	}
	h := strings.ToLower(host)
	// strip port if present
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		h = h[:idx]
	}
	for _, suffix := range googleDomains {
		if h == suffix[1:] || strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

// handleDirectConnect handles an HTTP CONNECT tunnel to a Google domain by
// dialing directly with a fragmented TLS ClientHello, then piping bytes.
// No CA or MITM is involved — the app's TLS stack talks end-to-end.
func handleDirectConnect(clientConn net.Conn, targetHost string) {
	serverConn, err := defaultFragmentDialer.DialTCP(targetHost)
	if err != nil {
		logf("error", "direct dial %s: %v", targetHost, err)
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer serverConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	logf("info", "DIRECT %s", targetHost)
	pipe(clientConn, serverConn)
}


// pipe bidirectionally copies between two connections until both directions close.
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		// Unblock the other direction by expiring its deadline.
		_ = dst.SetDeadline(time.Now())
		_ = src.SetDeadline(time.Now())
		done <- struct{}{}
	}
	go cp(b, a)
	go cp(a, b)
	<-done
	<-done
}

// dialFragment dials addr with fragmentation. Returns (conn, true) on success,
// (nil, false) on error — caller must handle the false case before piping.
func dialFragment(addr string) (net.Conn, bool) {
	conn, err := defaultFragmentDialer.DialTCP(addr)
	if err != nil {
		logf("error", "direct dial %s: %v", addr, err)
		return nil, false
	}
	return conn, true
}

