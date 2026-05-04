package core

import (
	"testing"
)

// go test -bench=. -benchtime=5s ./relay/core/

func BenchmarkGenerateCA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		_ = GenerateCA(dir+"/ca.pem", dir+"/ca-key.pem")
	}
}

func BenchmarkCertForHost(b *testing.B) {
	dir := b.TempDir()
	_ = GenerateCA(dir+"/ca.pem", dir+"/ca-key.pem")
	ca, _ := LoadCA(dir+"/ca.pem", dir+"/ca-key.pem")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ca.mu.Lock()
		delete(ca.cache, "bench.example.com")
		ca.mu.Unlock()
		_, _ = ca.CertForHost("bench.example.com")
	}
}

func BenchmarkCertForHost_Cached(b *testing.B) {
	dir := b.TempDir()
	_ = GenerateCA(dir+"/ca.pem", dir+"/ca-key.pem")
	ca, _ := LoadCA(dir+"/ca.pem", dir+"/ca-key.pem")
	_, _ = ca.CertForHost("bench.example.com") // warm cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ca.CertForHost("bench.example.com")
	}
}
