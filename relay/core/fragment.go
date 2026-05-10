package core

import (
	"math/rand/v2"
	"net"
	"syscall"
	"time"
)

// FragmentConfig controls how the TLS ClientHello is split into TCP segments.
// Based on gfw_resist_tls_proxy / gfw_resist_HTTPS_proxy parameters tuned for
// Iran's SNDPI (Hamrah-e Aval / Irancell ranges from the upstream research).
type FragmentConfig struct {
	// NumChunks is how many TCP segments the ClientHello is split into.
	// Iran SNDPI: 80–250 recommended; default 87 matches upstream.
	NumChunks int
	// Delay is the pause between each chunk send.
	// Iran SNDPI: 2–20 ms per chunk; default 5 ms.
	Delay time.Duration
}

// DefaultFragmentConfig is calibrated for Iran's SNDPI based on
// gfw_resist_HTTPS_proxy parameters (num_fragment=87, fragment_sleep=5ms).
var DefaultFragmentConfig = FragmentConfig{
	NumChunks: 87,
	Delay:     5 * time.Millisecond,
}

type fragmentDialer struct {
	cfg FragmentConfig
}

var defaultFragmentDialer = &fragmentDialer{cfg: DefaultFragmentConfig}

func (d *fragmentDialer) DialTCP(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	// TCP_NODELAY forces the kernel to send each Write as its own TCP segment
	// rather than coalescing them (Nagle's algorithm). Without this, the OS
	// would reassemble our fragments before they leave the machine.
	setTCPNoDelay(conn)
	return &fragmentConn{Conn: conn, cfg: d.cfg, firstWrite: true}, nil
}

// setTCPNoDelay disables Nagle's algorithm on conn if it is a *net.TCPConn.
func setTCPNoDelay(conn net.Conn) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
	})
}

// fragmentConn wraps a net.Conn and fragments only the very first Write call
// (which carries the TLS ClientHello) into NumChunks random-boundary segments
// with Delay between each. Subsequent writes go through unmodified.
type fragmentConn struct {
	net.Conn
	cfg        FragmentConfig
	firstWrite bool
}

func (c *fragmentConn) Write(b []byte) (int, error) {
	if !c.firstWrite {
		return c.Conn.Write(b)
	}
	c.firstWrite = false

	n := len(b)
	// Need at least 2 bytes to split; also cap chunks to data length.
	numChunks := c.cfg.numChunksFor(n)
	if numChunks <= 1 {
		return c.Conn.Write(b)
	}

	// Generate (numChunks-1) random split points in [1, n-1], sorted.
	splits := randomSplits(n, numChunks-1)

	written := 0
	prev := 0
	for _, s := range splits {
		nw, err := c.Conn.Write(b[prev:s])
		written += nw
		if err != nil {
			return written, err
		}
		prev = s
		time.Sleep(c.cfg.Delay)
	}
	// Final chunk
	nw, err := c.Conn.Write(b[prev:])
	written += nw
	return written, err
}

// numChunksFor returns the actual number of chunks to use given data length n.
func (cfg FragmentConfig) numChunksFor(n int) int {
	if n < 2 {
		return 1
	}
	if cfg.NumChunks > n {
		return n
	}
	return cfg.NumChunks
}

// randomSplits returns count sorted random positions in (0, n), all distinct.
func randomSplits(n, count int) []int {
	if count >= n-1 {
		// Every byte boundary — just return 1..n-1
		out := make([]int, n-1)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	// Fisher-Yates partial shuffle over [1, n-1]
	pool := make([]int, n-1)
	for i := range pool {
		pool[i] = i + 1
	}
	for i := 0; i < count; i++ {
		j := i + rand.IntN(n-1-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	chosen := pool[:count]
	// Sort ascending
	sortInts(chosen)
	return chosen
}

func sortInts(a []int) {
	// insertion sort — count is small (≤ n-1 splits, typically ≤ 87)
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}
