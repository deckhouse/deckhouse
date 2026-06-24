/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// genCert returns (certPEM, keyPEM, caPEM) for a self-signed ECDSA cert with
// IP SAN 127.0.0.1. Since it is self-signed the cert and CA PEM are identical.
func genCert(t *testing.T) (certPEM, keyPEM, caPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Self-signed: cert == CA
	caPEM = certPEM
	return certPEM, keyPEM, caPEM
}

// waitAddr polls srv.Addr() until non-empty (or times out after 2 seconds).
func waitAddr(t *testing.T, srv *Server) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if addr := srv.Addr(); addr != "" {
			return addr
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for server to bind")
	return ""
}

func TestServer_ServesTLS(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, caPEM := genCert(t)
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "AGENT-OK") })
	srv := New("127.0.0.1:0", certFile, keyFile, h)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Start(ctx) }()

	addr := waitAddr(t, srv)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}}}

	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = client.Get("https://" + addr + "/healthz")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if b, _ := io.ReadAll(resp.Body); string(b) != "AGENT-OK" {
		t.Fatalf("body = %q", b)
	}
}

// TestServer_listenWithRetry_cancelsWhileOccupied asserts the bind retry loop
// keeps retrying (does not error out) while the port is held, and exits on ctx
// cancellation rather than crashing.
func TestServer_listenWithRetry_cancelsWhileOccupied(t *testing.T) {
	occ, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy: %v", err)
	}
	defer occ.Close()

	s := &Server{addr: occ.Addr().String()}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := s.listenWithRetry(ctx, 10*time.Millisecond, 1_000_000); err == nil {
		t.Fatal("expected ctx error while port is occupied, got nil (should not have bound)")
	}
}

// TestServer_listenWithRetry_failsAfterMaxAttempts asserts the retry is bounded:
// a permanently-held port eventually errors out (so the pod exits and kubelet
// restarts it) rather than retrying forever.
func TestServer_listenWithRetry_failsAfterMaxAttempts(t *testing.T) {
	occ, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy: %v", err)
	}
	defer occ.Close()

	s := &Server{addr: occ.Addr().String()}
	_, err = s.listenWithRetry(context.Background(), time.Millisecond, 3)
	if err == nil {
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	if !strings.Contains(err.Error(), "still in use after") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestServer_listenWithRetry_bindsWhenFree asserts a free port binds on the first
// attempt.
func TestServer_listenWithRetry_bindsWhenFree(t *testing.T) {
	tmp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	addr := tmp.Addr().String()
	_ = tmp.Close()

	s := &Server{addr: addr}
	ln, err := s.listenWithRetry(context.Background(), 2*time.Second, 150)
	if err != nil {
		t.Fatalf("expected bind on free port, got %v", err)
	}
	defer ln.Close()
	if ln.Addr().String() != addr {
		t.Fatalf("bound %s, want %s", ln.Addr(), addr)
	}
}
