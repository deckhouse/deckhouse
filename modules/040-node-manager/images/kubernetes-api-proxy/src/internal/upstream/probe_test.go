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

package upstream

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newTestCA(t *testing.T) testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return testCA{
		cert: cert,
		key:  key,
		pem:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

type issued struct {
	tls     tls.Certificate
	certPEM []byte
	keyPEM  []byte
}

func (ca testCA) issue(t *testing.T, usage x509.ExtKeyUsage) issued {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "kubernetes-api-proxy"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	out := issued{
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}

	out.tls, err = tls.X509KeyPair(out.certPEM, out.keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

type probeServer struct {
	srv     *httptest.Server
	cert    atomic.Pointer[tls.Certificate]
	conns   atomic.Int32
	status  atomic.Int32
	delay   atomic.Int64
	lastURI atomic.Pointer[string]
}

func newProbeServer(t *testing.T, serverCert tls.Certificate) *probeServer {
	t.Helper()

	s := &probeServer{}
	s.cert.Store(&serverCert)
	s.status.Store(http.StatusOK)

	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := r.URL.RequestURI()
		s.lastURI.Store(&uri)

		select {
		case <-time.After(time.Duration(s.delay.Load())):
		case <-r.Context().Done():
			return
		}

		w.WriteHeader(int(s.status.Load()))
	}))
	s.srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			s.conns.Add(1)
		}
	}
	s.srv.Listener = tls.NewListener(s.srv.Listener, &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return s.cert.Load(), nil },
		NextProtos:     []string{"h2", "http/1.1"},
	})

	s.srv.Start()
	t.Cleanup(s.srv.Close)

	return s
}

func (s *probeServer) addr() string {
	return s.srv.Listener.Addr().String()
}

func (s *probeServer) uri() string {
	if p := s.lastURI.Load(); p != nil {
		return *p
	}

	return ""
}

type probeFiles struct {
	ca, cert, key string
}

func writeProbeFiles(t *testing.T, caPEM []byte, client issued) probeFiles {
	t.Helper()

	dir := t.TempDir()
	f := probeFiles{
		ca:   filepath.Join(dir, "ca.crt"),
		cert: filepath.Join(dir, "cl.crt"),
		key:  filepath.Join(dir, "cl.key"),
	}

	mustWrite(t, f.ca, caPEM)
	mustWrite(t, f.cert, client.certPEM)
	mustWrite(t, f.key, client.keyPEM)

	return f
}

func (f probeFiles) getter() (*rest.Config, error) {
	return &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{CAFile: f.ca, CertFile: f.cert, KeyFile: f.key},
	}, nil
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

type probeFixture struct {
	files  probeFiles
	server *probeServer
	u      *Upstream
}

func newProbeFixture(t *testing.T) probeFixture {
	t.Helper()

	ca := newTestCA(t)
	files := writeProbeFiles(t, ca.pem, ca.issue(t, x509.ExtKeyUsageClientAuth))
	server := newProbeServer(t, ca.issue(t, x509.ExtKeyUsageServerAuth).tls)

	return probeFixture{
		files:  files,
		server: server,
		u:      NewUpstream(server.addr(), files.getter),
	}
}

func (f probeFixture) probe(t *testing.T, timeout time.Duration) probeResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r, err := f.u.probe(ctx)
	if err != nil {
		t.Fatalf("probe() error = %v", err)
	}

	return r
}

func TestProbe_Classification(t *testing.T) {
	f := newProbeFixture(t)

	tests := []struct {
		name   string
		status int32
		delay  time.Duration
		want   probeKind
	}{
		{name: "200 is ready", status: http.StatusOK, want: probeReady},
		{name: "500 is not ready", status: http.StatusInternalServerError, want: probeNotReady},
		{name: "hang on an open connection is soft", status: http.StatusOK, delay: time.Second, want: probeSoft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.server.status.Store(tt.status)
			f.server.delay.Store(int64(tt.delay))

			got := f.probe(t, 200*time.Millisecond)
			if got.kind != tt.want {
				t.Fatalf("kind = %v, want %v", got.kind, tt.want)
			}

			if tt.want != probeSoft && got.latency <= 0 {
				t.Errorf("latency = %v, want > 0", got.latency)
			}
		})
	}

	if got := f.server.uri(); got != readyzPath {
		t.Errorf("request URI = %q, want %q", got, readyzPath)
	}
}

func TestProbe_NoConnectionIsHard(t *testing.T) {
	f := newProbeFixture(t)

	silent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = silent.Close() })

	go func() {
		for {
			conn, err := silent.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	tests := []struct {
		name string
		addr string
	}{
		{name: "closed port", addr: "127.0.0.1:1"},
		{name: "TLS handshake never completes", addr: silent.Addr().String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewUpstream(tt.addr, f.files.getter)

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			r, err := u.probe(ctx)
			if err != nil {
				t.Fatalf("probe() error = %v", err)
			}

			if r.kind != probeHard {
				t.Errorf("kind = %v, want %v", r.kind, probeHard)
			}
		})
	}
}

func TestProbe_ReusesConnection(t *testing.T) {
	f := newProbeFixture(t)

	for range 50 {
		f.probe(t, time.Second)
	}

	if got := f.server.conns.Load(); got != 1 {
		t.Errorf("server connections = %d, want 1", got)
	}
}

func TestProbe_CARotationWithoutRestart(t *testing.T) {
	f := newProbeFixture(t)
	f.probe(t, time.Second)

	ca2 := newTestCA(t)
	srv2 := ca2.issue(t, x509.ExtKeyUsageServerAuth)
	f.server.cert.Store(&srv2.tls)
	f.server.srv.CloseClientConnections()

	if got := f.probe(t, time.Second); got.kind != probeHard {
		t.Fatalf("kind with stale ca.crt = %v, want %v", got.kind, probeHard)
	}

	mustWrite(t, f.files.ca, ca2.pem)

	if got := f.probe(t, time.Second); got.kind != probeReady {
		t.Errorf("kind after ca.crt rewrite = %v, want %v", got.kind, probeReady)
	}
}

func TestProbe_BrokenCAIsGetterError(t *testing.T) {
	f := newProbeFixture(t)
	mustWrite(t, f.files.ca, []byte("not a pem\n"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := f.u.probe(ctx); err == nil {
		t.Fatal("probe() error = nil, want transport error")
	}
}

func TestProbe_EmptyCAIsHard(t *testing.T) {
	f := newProbeFixture(t)
	mustWrite(t, f.files.ca, nil)

	if got := f.probe(t, time.Second); got.kind != probeHard {
		t.Errorf("kind = %v, want %v", got.kind, probeHard)
	}
}

func TestProbe_NilGetterIsError(t *testing.T) {
	u := NewUpstream("127.0.0.1:1", nil)

	if _, err := u.probe(context.Background()); !errors.Is(err, errNoProbeConfig) {
		t.Errorf("probe() error = %v, want %v", err, errNoProbeConfig)
	}
}
