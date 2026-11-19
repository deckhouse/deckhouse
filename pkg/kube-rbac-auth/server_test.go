// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kuberbacauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	certutil "k8s.io/client-go/util/cert"
)

// startTLSServer serves cfg.Handler on a local port until the test ends or stop is called.
// stop returns the result of the graceful shutdown.
func startTLSServer(t *testing.T, cfg ServerConfig, tlsConfig *tls.Config) (string, func() error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveTLS(ctx, ln, cfg, tlsConfig, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	stop := sync.OnceValue(func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			return errors.New("server did not stop")
		}
	})
	t.Cleanup(func() { _ = stop() })
	return ln.Addr().String(), stop
}

func TestServeTLSWithSelfSignedCertificate(t *testing.T) {
	cert, err := selfSignedCertificate()
	if err != nil {
		t.Fatal(err)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	addr, stop := startTLSServer(t, ServerConfig{Handler: h, ShutdownTimeout: time.Second},
		&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // the certificate is self-signed
	resp, err := client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}

	if err := stop(); err != nil {
		t.Fatalf("serveTLS returned %v after shutdown", err)
	}
}

func TestServerClosesIdleConnection(t *testing.T) {
	cert, err := selfSignedCertificate()
	if err != nil {
		t.Fatal(err)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	addr, _ := startTLSServer(t, ServerConfig{Handler: h, IdleTimeout: 50 * time.Millisecond, ShutdownTimeout: time.Second},
		&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})

	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // the certificate is self-signed
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: example\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	// The server answers and then closes the connection itself: the read ends before the deadline.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	answer, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("the idle connection was not closed: %v", err)
	}
	if !strings.HasSuffix(string(answer), "ok") {
		t.Fatalf("answer = %q", answer)
	}
}

func TestListenAndServeTLSValidatesConfig(t *testing.T) {
	h := http.NotFoundHandler()
	if err := ListenAndServeTLS(context.Background(), ServerConfig{Address: "127.0.0.1:0"}); err == nil {
		t.Fatal("no handler: want an error")
	}
	if err := ListenAndServeTLS(context.Background(), ServerConfig{Address: "127.0.0.1:0", Handler: h, CertFile: "a"}); err == nil {
		t.Fatal("cert without key: want an error")
	}
	if err := ListenAndServeTLS(context.Background(), ServerConfig{Address: "127.0.0.1:0", Handler: h, CertFile: "a", KeyFile: "b"}); err == nil {
		t.Fatal("missing files: want an error")
	}
}

func writeKeyPair(t *testing.T, dir, host string) {
	t.Helper()
	certPEM, keyPEM, err := certutil.GenerateSelfSignedCertKey(host, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"tls.crt": certPEM, "tls.key": keyPEM} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func commonName(t *testing.T, c *tls.Certificate) string {
	t.Helper()
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.Subject.CommonName
}

func TestServingCertificateIsReread(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "tls.key")
	var logs bytes.Buffer
	clock := &fakeClock{now: testNow}
	writeKeyPair(t, dir, "first")
	pair, err := newServingCertificate(filepath.Join(dir, "tls.crt"), keyFile, clock, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	served := func() string {
		t.Helper()
		cert, _ := pair.get()
		return commonName(t, cert)
	}

	if !strings.HasPrefix(served(), "first") {
		t.Fatalf("common name = %q", served())
	}

	writeKeyPair(t, dir, "second")
	clock.step(reloadInterval)
	if !strings.HasPrefix(served(), "second") {
		t.Fatalf("after rotation common name = %q", served())
	}

	// A broken pair in the middle of a rotation keeps the previous certificate in use and is reported.
	if err := os.WriteFile(keyFile, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	clock.step(reloadInterval)
	if !strings.HasPrefix(served(), "second") {
		t.Fatalf("broken pair: common name = %q", served())
	}
	if n := strings.Count(logs.String(), "cannot reload the serving certificate"); n != 1 {
		t.Fatalf("warnings = %d, want 1; log %q", n, logs.String())
	}
}
