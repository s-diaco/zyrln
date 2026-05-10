package core

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// recordConn records each Write call as a separate chunk.
type recordConn struct {
	net.Conn
	mu     sync.Mutex
	chunks [][]byte
	times  []time.Time
}

func newRecordConn() *recordConn { return &recordConn{} }

func (r *recordConn) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	r.chunks = append(r.chunks, cp)
	r.times = append(r.times, time.Now())
	return len(b), nil
}

func (r *recordConn) allBytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []byte
	for _, c := range r.chunks {
		out = append(out, c...)
	}
	return out
}

func (r *recordConn) numChunks() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.chunks)
}

// Close and addr stubs so recordConn satisfies net.Conn.
func (r *recordConn) Close() error                       { return nil }
func (r *recordConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (r *recordConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (r *recordConn) SetDeadline(_ time.Time) error      { return nil }
func (r *recordConn) SetReadDeadline(_ time.Time) error  { return nil }
func (r *recordConn) SetWriteDeadline(_ time.Time) error { return nil }
func (r *recordConn) Read(_ []byte) (int, error)         { return 0, nil }

// newTestFragmentConn returns a fragmentConn backed by a recordConn.
func newTestFragmentConn(cfg FragmentConfig) (*fragmentConn, *recordConn) {
	rc := newRecordConn()
	fc := &fragmentConn{Conn: rc, cfg: cfg, firstWrite: true}
	return fc, rc
}

// --- randomSplits ---

func TestRandomSplits_Count(t *testing.T) {
	for _, n := range []int{10, 100, 300} {
		for _, count := range []int{1, 5, n - 1} {
			got := randomSplits(n, count)
			if len(got) != count {
				t.Errorf("randomSplits(%d,%d): got %d splits, want %d", n, count, len(got), count)
			}
		}
	}
}

func TestRandomSplits_Sorted(t *testing.T) {
	splits := randomSplits(300, 86)
	for i := 1; i < len(splits); i++ {
		if splits[i] <= splits[i-1] {
			t.Errorf("not sorted at index %d: %d <= %d", i, splits[i], splits[i-1])
		}
	}
}

func TestRandomSplits_Bounds(t *testing.T) {
	n := 300
	splits := randomSplits(n, 86)
	for _, s := range splits {
		if s < 1 || s >= n {
			t.Errorf("split %d out of [1,%d)", s, n)
		}
	}
}

func TestRandomSplits_Distinct(t *testing.T) {
	splits := randomSplits(300, 86)
	seen := map[int]bool{}
	for _, s := range splits {
		if seen[s] {
			t.Errorf("duplicate split point %d", s)
		}
		seen[s] = true
	}
}

func TestRandomSplits_AllBoundaries(t *testing.T) {
	// When count >= n-1, we should get every position 1..n-1.
	n := 5
	splits := randomSplits(n, n) // count > n-1
	if len(splits) != n-1 {
		t.Fatalf("got %d splits, want %d", len(splits), n-1)
	}
	for i, s := range splits {
		if s != i+1 {
			t.Errorf("splits[%d]=%d, want %d", i, s, i+1)
		}
	}
}

// --- fragmentConn ---

func TestFragmentConn_DataIntegrity(t *testing.T) {
	data := bytes.Repeat([]byte("ABCDEFGHIJ"), 30) // 300 bytes
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	n, err := fc.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("wrote %d bytes, want %d", n, len(data))
	}
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("reassembled data mismatch")
	}
}

func TestFragmentConn_NumChunks(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 300)
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	got := rc.numChunks()
	if got != 87 {
		t.Errorf("got %d chunks, want 87", got)
	}
}

func TestFragmentConn_OnlyFirstWriteFragmented(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 300)
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data) // first write — fragmented
	beforeCount := rc.numChunks()

	_, _ = fc.Write(data) // second write — must be a single chunk
	afterCount := rc.numChunks()

	if afterCount != beforeCount+1 {
		t.Errorf("second write produced %d extra chunks, want 1", afterCount-beforeCount)
	}
}

func TestFragmentConn_SmallData(t *testing.T) {
	// Data smaller than NumChunks — should still fragment but capped to len(data).
	data := []byte("hi") // 2 bytes
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("small data mismatch")
	}
}

func TestFragmentConn_SingleByte(t *testing.T) {
	data := []byte("Z")
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	// 1 byte: cannot split, should be single write
	if got := rc.numChunks(); got != 1 {
		t.Errorf("single byte: got %d chunks, want 1", got)
	}
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("single byte data mismatch")
	}
}

func TestFragmentConn_DelayApplied(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 10)
	cfg := FragmentConfig{NumChunks: 3, Delay: 10 * time.Millisecond}
	fc, rc := newTestFragmentConn(cfg)

	start := time.Now()
	_, _ = fc.Write(data)
	elapsed := time.Since(start)

	_ = rc
	// 3 chunks → 3 delays of 10ms each → at least 20ms total (conservative)
	if elapsed < 20*time.Millisecond {
		t.Errorf("delay too short: %v, expected >= 20ms", elapsed)
	}
}

// --- FragmentConfig.numChunksFor ---

func TestNumChunksFor(t *testing.T) {
	cfg := FragmentConfig{NumChunks: 87}
	cases := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{87, 87},
		{300, 87},
		{50, 50}, // n < NumChunks → cap to n
	}
	for _, tc := range cases {
		got := cfg.numChunksFor(tc.n)
		if got != tc.want {
			t.Errorf("numChunksFor(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// --- IsDirectDomain ---

func TestIsDirectDomain(t *testing.T) {
	SetDirectEnabled(true)
	cases := []struct {
		host string
		want bool
	}{
		{"youtube.com", true},
		{"www.youtube.com", true},
		{"i.ytimg.com", true},
		{"googleapis.com", true},
		{"maps.googleapis.com", true},
		{"mail.google.com", true},
		{"gstatic.com", true},
		{"www.gstatic.com", true},
		{"instagram.com", false},
		{"twitter.com", false},
		{"example.com", false},
		{"notgoogle.com", false},
		// with port
		{"youtube.com:443", true},
		{"instagram.com:443", false},
	}
	for _, tc := range cases {
		got := IsDirectDomain(tc.host)
		if got != tc.want {
			t.Errorf("IsDirectDomain(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestIsDirectDomain_DisabledFlag(t *testing.T) {
	orig := GetDirectEnabled()
	defer SetDirectEnabled(orig)

	SetDirectEnabled(false)
	if IsDirectDomain("youtube.com") {
		t.Error("IsDirectDomain should return false when DirectEnabled=false")
	}
}
