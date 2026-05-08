package mobile

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCA_CreatesInstallableCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "certs", "ca.pem")
	keyPath := filepath.Join(dir, "certs", "ca.key")

	if err := GenerateCA(certPath, keyPath); err != "" {
		t.Fatalf("GenerateCA returned error: %s", err)
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("generated cert/key pair is invalid: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("generated cert is not certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse generated cert: %v", err)
	}
	if !cert.IsCA || !cert.BasicConstraintsValid {
		t.Fatalf("generated cert is not a valid CA: IsCA=%v BasicConstraintsValid=%v", cert.IsCA, cert.BasicConstraintsValid)
	}
}

func TestStart_RequiresExistingCA(t *testing.T) {
	Stop()
	t.Cleanup(Stop)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "certs", "ca.pem")
	keyPath := filepath.Join(dir, "certs", "ca.key")

	err := Start("", "secret", "127.0.0.1:0", certPath, keyPath)
	if !strings.Contains(err, "load CA:") {
		t.Fatalf("Start error = %q, want missing CA error", err)
	}
	if IsRunning() {
		t.Fatal("Start should not leave proxy running when CA is missing")
	}
	if _, statErr := os.Stat(certPath); !os.IsNotExist(statErr) {
		t.Fatalf("Start should not generate cert file implicitly, stat error: %v", statErr)
	}
	if _, statErr := os.Stat(keyPath); !os.IsNotExist(statErr) {
		t.Fatalf("Start should not generate key file implicitly, stat error: %v", statErr)
	}
}

func TestStart_LoadsExistingCA(t *testing.T) {
	Stop()
	t.Cleanup(Stop)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "certs", "ca.pem")
	keyPath := filepath.Join(dir, "certs", "ca.key")
	if err := GenerateCA(certPath, keyPath); err != "" {
		t.Fatalf("GenerateCA returned error: %s", err)
	}

	err := Start("", "secret", "127.0.0.1:0", certPath, keyPath)
	if !strings.Contains(err, "no Apps Script URLs configured") {
		t.Fatalf("Start error = %q, want missing Apps Script config error after CA load", err)
	}
	if IsRunning() {
		t.Fatal("Start should not leave proxy running when config is incomplete")
	}
}
