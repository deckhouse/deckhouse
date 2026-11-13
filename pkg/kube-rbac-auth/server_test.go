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
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	certutil "k8s.io/client-go/util/cert"
)

func TestServeTLSWithSelfSignedCertificate(t *testing.T) {
	cert, err := selfSignedCertificate()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	go func() {
		done <- serveTLS(ctx, ln, h, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
			time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // the certificate is self-signed
	resp, err := client.Get("https://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveTLS returned %v after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
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

func writeKeyPair(t *testing.T, dir, host string, mtime time.Time) {
	t.Helper()
	certPEM, keyPEM, err := certutil.GenerateSelfSignedCertKey(host, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"tls.crt": certPEM, "tls.key": keyPEM} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
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

func TestCertLoaderRereadsChangedFiles(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Hour)
	writeKeyPair(t, dir, "first", start)
	l := &certLoader{certFile: filepath.Join(dir, "tls.crt"), keyFile: filepath.Join(dir, "tls.key")}

	c, err := l.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(commonName(t, c), "first") {
		t.Fatalf("common name = %q", commonName(t, c))
	}

	writeKeyPair(t, dir, "second", start.Add(time.Minute))
	c, err = l.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(commonName(t, c), "second") {
		t.Fatalf("after rotation common name = %q", commonName(t, c))
	}

	// A broken pair in the middle of a rotation keeps the previous certificate in use.
	if err := os.WriteFile(l.keyFile, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(l.keyFile, start.Add(2*time.Minute), start.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	c, err = l.GetCertificate(nil)
	if err != nil || !strings.HasPrefix(commonName(t, c), "second") {
		t.Fatalf("broken pair: cert %v, err %v", c, err)
	}
}
