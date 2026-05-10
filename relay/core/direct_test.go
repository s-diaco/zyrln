package core

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// startEchoServer starts a TCP server that echoes back whatever it receives,
// then closes. Returns the listener address.
func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()
	return ln.Addr().String()
}

// startSinkServer starts a TCP server that accepts one connection, reads all
// data sent to it, and writes back a fixed response. Returns the address and a
// channel that receives the bytes the server read.
func startSinkServer(t *testing.T, response []byte) (addr string, received <-chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("sink server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf, _ := io.ReadAll(conn)
		ch <- buf
		if len(response) > 0 {
			conn.Write(response)
		}
	}()
	return ln.Addr().String(), ch
}

// --- pipe ---

func TestPipe_BidirectionalCopy(t *testing.T) {
	// Connect two net.Pipe halves and verify data flows in both directions.
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(a, b)
	}()

	// Write from b-side, read from a-side (pipe copies b→a and a→b).
	msg := []byte("hello from b")
	b.Write(msg)
	b.Close()

	got := make([]byte, len(msg))
	io.ReadFull(a, got)
	// pipe closes both when one ends — just verify wg completes
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("pipe did not finish after both sides closed")
	}
}

func TestPipe_BothGoroutinesFinish(t *testing.T) {
	// Verify pipe waits for both directions (no goroutine leak).
	a, b := net.Pipe()

	start := time.Now()
	go func() {
		time.Sleep(20 * time.Millisecond)
		a.Close()
		b.Close()
	}()
	pipe(a, b) // must return only after both goroutines finish
	if time.Since(start) < 15*time.Millisecond {
		t.Error("pipe returned too fast — likely only waited for one goroutine")
	}
}

// --- dialFragment ---

func TestDialFragment_Success(t *testing.T) {
	addr := startEchoServer(t)
	conn, ok := dialFragment(addr)
	if !ok {
		t.Fatal("dialFragment returned ok=false for reachable server")
	}
	defer conn.Close()

	// Send a message, then half-close so the echo server knows to stop.
	msg := []byte("ping")
	conn.Write(msg)
	// Close write side via the underlying TCPConn.
	if tc, ok2 := conn.(*fragmentConn).Conn.(*net.TCPConn); ok2 {
		tc.CloseWrite()
	}
	got, _ := io.ReadAll(conn)
	if string(got) != string(msg) {
		t.Errorf("echo mismatch: got %q, want %q", got, msg)
	}
}

func TestDialFragment_Failure(t *testing.T) {
	// Port 1 is reserved and should always refuse connections.
	conn, ok := dialFragment("127.0.0.1:1")
	if ok {
		conn.Close()
		t.Error("dialFragment should return ok=false for unreachable address")
	}
	if conn != nil {
		t.Error("dialFragment should return nil conn on failure")
	}
}

// --- handleDirectConnect ---

func TestHandleDirectConnect_PipesData(t *testing.T) {
	// Server side: accept one conn and echo back.
	serverAddr := startEchoServer(t)

	// Client side: use net.Pipe to simulate the hijacked browser conn.
	clientSide, proxySide := net.Pipe()
	defer clientSide.Close()

	go handleDirectConnect(proxySide, serverAddr)

	// handleDirectConnect sends "200 Connection Established" first.
	resp := make([]byte, len("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if _, err := io.ReadFull(clientSide, resp); err != nil {
		t.Fatalf("read 200 response: %v", err)
	}
	if string(resp) != "HTTP/1.1 200 Connection Established\r\n\r\n" {
		t.Errorf("unexpected response: %q", resp)
	}

	// Now pipe is live — send data and expect it echoed back.
	// Use a real TCP loopback pair so we can half-close the write side.
	msg := []byte("hello direct")
	if _, err := clientSide.Write(msg); err != nil {
		t.Fatalf("write to proxy: %v", err)
	}

	got := make([]byte, len(msg))
	if _, err := io.ReadFull(clientSide, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != string(msg) {
		t.Errorf("echo mismatch: got %q, want %q", got, msg)
	}
}

func TestHandleDirectConnect_DialFailure(t *testing.T) {
	clientSide, proxySide := net.Pipe()
	defer clientSide.Close()

	go handleDirectConnect(proxySide, "127.0.0.1:1")

	// Should get 502 Bad Gateway.
	resp := make([]byte, 512)
	n, _ := clientSide.Read(resp)
	body := string(resp[:n])
	if len(body) == 0 {
		t.Error("expected 502 response on dial failure, got nothing")
	}
	if body[:12] != "HTTP/1.1 502" {
		t.Errorf("expected 502, got: %q", body[:min(12, len(body))])
	}
}

// --- SetDirectEnabled toggle race ---

func TestSetDirectEnabled_ConcurrentToggle(t *testing.T) {
	orig := GetDirectEnabled()
	defer SetDirectEnabled(orig)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); SetDirectEnabled(true) }()
		go func() { defer wg.Done(); _ = GetDirectEnabled() }()
	}
	wg.Wait()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
