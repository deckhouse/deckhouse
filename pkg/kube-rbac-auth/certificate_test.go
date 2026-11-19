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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	// chain is what a client sends after its own certificate: this CA and the CAs above it,
	// without the root. It is empty for a root CA.
	chain [][]byte
}

// newTestCA returns a self-signed root CA.
func newTestCA(t *testing.T, name string) *testCA {
	t.Helper()
	return signTestCA(t, name, nil)
}

// intermediate returns a CA signed by ca.
func (ca *testCA) intermediate(t *testing.T, name string) *testCA {
	t.Helper()
	return signTestCA(t, name, ca)
}

func signTestCA(t *testing.T, name string, parent *testCA) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             testNow.Add(-time.Hour),
		NotAfter:              testNow.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	signerCert, signerKey := tmpl, key
	if parent != nil {
		signerCert, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	ca := &testCA{cert: cert, key: key}
	if parent != nil {
		ca.chain = append([][]byte{der}, parent.chain...)
	}
	return ca
}

// issue returns a client certificate for the user name and groups, followed by the intermediate CAs.
func (ca *testCA) issue(t *testing.T, name string, groups []string, usage x509.ExtKeyUsage) tls.Certificate {
	t.Helper()
	return ca.issueValidFrom(t, testNow.Add(-time.Hour), name, groups, usage)
}

func (ca *testCA) issueValidFrom(t *testing.T, notBefore time.Time, name string, groups []string, usage x509.ExtKeyUsage) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name, Organization: groups},
		NotBefore:    notBefore,
		NotAfter:     testNow.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: append([][]byte{der}, ca.chain...), PrivateKey: key, Leaf: leaf}
}

func caPEM(cas ...*testCA) []byte {
	var data []byte
	for _, ca := range cas {
		data = append(data, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw})...)
	}
	return data
}

func writeFile(t *testing.T, file string, data []byte) {
	t.Helper()
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCAFile(t *testing.T, file string, cas ...*testCA) {
	t.Helper()
	writeFile(t, file, caPEM(cas...))
}

// certEnv is an env whose middleware trusts client certificates of ca.
type certEnv struct {
	*env
	ca     *testCA
	caFile string
}

func newCertEnv(t *testing.T) *certEnv {
	t.Helper()
	return newCertEnvWithLogger(t, nil)
}

func newCertEnvWithLogger(t *testing.T, logger Logger) *certEnv {
	t.Helper()
	ca := newTestCA(t, "kubernetes")
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	writeCAFile(t, caFile, ca)
	return &certEnv{env: newEnv(t, Config{ClientCAFile: caFile, Logger: logger}), ca: ca, caFile: caFile}
}

// certRequest returns a request as if the client sent the certificates of cert during the handshake.
func certRequest(t *testing.T, cert tls.Certificate) *http.Request {
	t.Helper()
	peers := make([]*x509.Certificate, 0, len(cert.Certificate))
	for _, der := range cert.Certificate {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		peers = append(peers, c)
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.TLS = &tls.ConnectionState{PeerCertificates: peers}
	return req
}

func (e *env) doCert(t *testing.T, cert tls.Certificate, auth string) *httptest.ResponseRecorder {
	t.Helper()
	return e.serve(certRequest(t, cert), auth)
}

func (e *env) lastUser() (string, []string) {
	e.api.mu.Lock()
	defer e.api.mu.Unlock()
	return e.api.lastSpec.User, e.api.lastSpec.Groups
}

func TestClientCertificateIsAuthenticatedWithoutTokenReview(t *testing.T) {
	e := newCertEnv(t)
	admin := e.ca.issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, admin, ""), http.StatusOK)
	expectCalls(t, e.api, 0, 1)

	user, groups := e.lastUser()
	if user != "kubernetes-admin" {
		t.Fatalf("user = %q", user)
	}
	if want := []string{"system:masters", "system:authenticated"}; !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
}

// The groups follow the API server: system:authenticated is not added to the anonymous user
// or to a user that already has it or system:unauthenticated.
func TestClientCertificateGroups(t *testing.T) {
	tests := []struct {
		name   string
		user   string
		groups []string
		want   []string
	}{
		{name: "no groups", user: "jane", want: []string{"system:authenticated"}},
		{name: "anonymous user", user: "system:anonymous"},
		{name: "already authenticated", user: "jane", groups: []string{"dev", "system:authenticated"}, want: []string{"dev", "system:authenticated"}},
		{name: "unauthenticated group", user: "jane", groups: []string{"system:unauthenticated"}, want: []string{"system:unauthenticated"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newCertEnv(t)
			cert := e.ca.issue(t, tt.user, tt.groups, x509.ExtKeyUsageClientAuth)

			expectCode(t, e.doCert(t, cert, ""), http.StatusOK)
			user, groups := e.lastUser()
			if user != tt.user {
				t.Fatalf("user = %q, want %q", user, tt.user)
			}
			// The organizations of a certificate have no order of their own.
			slices.Sort(groups)
			if !slices.Equal(groups, tt.want) {
				t.Fatalf("groups = %v, want %v", groups, tt.want)
			}
		})
	}
}

func TestClientCertificateWinsOverToken(t *testing.T) {
	e := newCertEnv(t)
	admin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, admin, "Bearer "+validToken), http.StatusOK)
	expectCalls(t, e.api, 0, 1)
	if user, _ := e.lastUser(); user != "kubernetes-admin" {
		t.Fatalf("user = %q", user)
	}
}

func TestDeniedClientCertificateIsForbidden(t *testing.T) {
	e := newCertEnv(t)
	e.api.set(func(f *fakeAPI) { f.allowed = false })
	jane := e.ca.issue(t, "jane", nil, x509.ExtKeyUsageClientAuth)

	rec := e.doCert(t, jane, "")
	expectCode(t, rec, http.StatusForbidden)
	if !strings.Contains(rec.Body.String(), "user=jane") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestClientCertificateFromUnknownCAFallsBackToToken(t *testing.T) {
	e := newCertEnv(t)
	stranger := newTestCA(t, "another-ca").issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, stranger, ""), http.StatusUnauthorized)
	expectCalls(t, e.api, 0, 0)

	expectCode(t, e.doCert(t, stranger, "Bearer "+validToken), http.StatusOK)
	expectCalls(t, e.api, 1, 1)
	if user, _ := e.lastUser(); user != "system:serviceaccount:d8-monitoring:prometheus" {
		t.Fatalf("user = %q", user)
	}
}

// A CA with the name of the trusted one and its own key is not trusted: the signature is checked.
func TestClientCertificateFromForgedCAIsRejected(t *testing.T) {
	e := newCertEnv(t)
	forged := newTestCA(t, "kubernetes")
	admin := forged.issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)

	// The second request is answered from the cache of chain checks.
	for range 2 {
		expectCode(t, e.doCert(t, admin, ""), http.StatusUnauthorized)
	}

	// The client sends the forged CA along with its certificate.
	admin.Certificate = append(admin.Certificate, forged.cert.Raw)
	for range 2 {
		expectCode(t, e.doCert(t, admin, ""), http.StatusUnauthorized)
	}
	expectCalls(t, e.api, 0, 0)
}

func TestClientCertificateThroughIntermediateCA(t *testing.T) {
	e := newCertEnv(t)
	admin := e.ca.intermediate(t, "kubernetes-users").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, admin, ""), http.StatusOK)
	if user, _ := e.lastUser(); user != "kubernetes-admin" {
		t.Fatalf("user = %q", user)
	}

	// Without the intermediate CA the chain to the trusted root cannot be built.
	admin.Certificate = admin.Certificate[:1]
	expectCode(t, e.doCert(t, admin, ""), http.StatusUnauthorized)
}

func TestClientCertificateChainLengthIsLimited(t *testing.T) {
	e := newCertEnv(t)
	ca := e.ca
	for range maxClientIntermediates {
		ca = ca.intermediate(t, "intermediate")
	}
	allowed := ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	tooLong := ca.intermediate(t, "intermediate").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, allowed, ""), http.StatusOK)
	expectCode(t, e.doCert(t, tooLong, ""), http.StatusUnauthorized)

	// The owner of a rejected chain can still use a token.
	expectCode(t, e.doCert(t, tooLong, "Bearer "+validToken), http.StatusOK)
	if user, _ := e.lastUser(); user != "system:serviceaccount:d8-monitoring:prometheus" {
		t.Fatalf("user = %q", user)
	}
}

func TestClientCertificateIsIgnoredWithoutClientCAFile(t *testing.T) {
	e := newEnv(t, Config{})
	admin := newTestCA(t, "kubernetes").issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, admin, ""), http.StatusUnauthorized)
	expectCalls(t, e.api, 0, 0)
}

func TestUnusableClientCertificates(t *testing.T) {
	t.Run("without a common name", func(t *testing.T) {
		e := newCertEnv(t)
		cert := e.ca.issue(t, "", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)
		expectCode(t, e.doCert(t, cert, ""), http.StatusUnauthorized)
	})
	t.Run("issued for a server", func(t *testing.T) {
		e := newCertEnv(t)
		cert := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageServerAuth)
		expectCode(t, e.doCert(t, cert, ""), http.StatusUnauthorized)
	})
	t.Run("expired", func(t *testing.T) {
		e := newCertEnv(t)
		cert := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
		expectCode(t, e.doCert(t, cert, ""), http.StatusOK)
		e.clock.step(2 * time.Hour)
		expectCode(t, e.doCert(t, cert, ""), http.StatusUnauthorized)
	})
	// A cached rejection must end when the certificate becomes valid.
	t.Run("not valid yet", func(t *testing.T) {
		e := newCertEnv(t)
		cert := e.ca.issueValidFrom(t, testNow.Add(time.Second), "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
		expectCode(t, e.doCert(t, cert, ""), http.StatusUnauthorized)
		e.clock.step(time.Second)
		expectCode(t, e.doCert(t, cert, ""), http.StatusOK)
	})
	// A cached check must not outlive the certificate.
	t.Run("expired within the lifetime of a cached check", func(t *testing.T) {
		e := newCertEnv(t)
		cert := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
		e.clock.step(cert.Leaf.NotAfter.Sub(testNow) - chainCheckTTL/2)
		expectCode(t, e.doCert(t, cert, ""), http.StatusOK)
		e.clock.step(chainCheckTTL/2 + time.Second)
		expectCode(t, e.doCert(t, cert, ""), http.StatusUnauthorized)
	})
}

func newTestCertAuthenticator(t *testing.T, logger Logger, cas ...*testCA) *certAuthenticator {
	t.Helper()
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	writeCAFile(t, caFile, cas...)
	clock := &fakeClock{now: testNow}
	bundle, err := newClientCAs(caFile, clock, logger)
	if err != nil {
		t.Fatal(err)
	}
	return newCertAuthenticator(bundle, DefaultCacheSize, clock, logger)
}

func TestChainCheckIsReused(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ca := newTestCA(t, "kubernetes")
	a := newTestCertAuthenticator(t, logger, ca)
	clock := a.clock.(*fakeClock)

	admin := certRequest(t, ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth))
	first := a.authenticate(admin)
	if first == nil {
		t.Fatal("trusted certificate: no user")
	}
	if a.authenticate(admin) != first {
		t.Fatal("trusted certificate: the chain was checked again")
	}

	stranger := certRequest(t, newTestCA(t, "another-ca").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth))
	rejections := func() int { return strings.Count(logs.String(), "client certificate is not trusted") }
	for range 3 {
		if a.authenticate(stranger) != nil {
			t.Fatal("certificate from another CA: got a user")
		}
	}
	if n := rejections(); n != 1 {
		t.Fatalf("chain checks of a rejected certificate = %d, want 1", n)
	}

	clock.step(chainCheckTTL + time.Second)
	if a.authenticate(stranger) != nil {
		t.Fatal("certificate from another CA: got a user")
	}
	if n := rejections(); n != 2 {
		t.Fatalf("chain checks after the cached answer expired = %d, want 2", n)
	}
	if again := a.authenticate(admin); again == nil || again == first {
		t.Fatalf("trusted certificate after the cached answer expired: user %p, first %p", again, first)
	}
}

// rejectionGate counts the rejected chain checks and holds the first one until release is closed.
type rejectionGate struct {
	slog.Handler
	checks  atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (g *rejectionGate) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "client certificate is not trusted" && g.checks.Add(1) == 1 {
		close(g.started)
		<-g.release
	}
	return g.Handler.Handle(ctx, r)
}

// Requests of one connection run in parallel over HTTP/2 and carry the same certificates.
func TestConcurrentRequestsShareOneChainCheck(t *testing.T) {
	gate := &rejectionGate{
		Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	a := newTestCertAuthenticator(t, slog.New(gate), newTestCA(t, "kubernetes"))
	stranger := newTestCA(t, "another-ca")
	sameChain := stranger.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	otherChain := stranger.issue(t, "jane", nil, x509.ExtKeyUsageClientAuth)

	var wg sync.WaitGroup
	authenticate := func(cert tls.Certificate) <-chan struct{} {
		req := certRequest(t, cert)
		done := make(chan struct{})
		wg.Go(func() {
			defer close(done)
			if a.authenticate(req) != nil {
				t.Error("certificate from another CA: got a user")
			}
		})
		return done
	}

	authenticate(sameChain)
	<-gate.started
	// The first check is held. Requests with the same chain must wait for it,
	// and a request with another chain must be checked without waiting.
	for range 20 {
		authenticate(sameChain)
	}
	select {
	case <-authenticate(otherChain):
	case <-time.After(5 * time.Second):
		t.Error("a request with another chain waited for the held check")
	}
	// Give a request that wrongly started its own check the time to be counted.
	time.Sleep(50 * time.Millisecond)
	close(gate.release)
	wg.Wait()

	if n := gate.checks.Load(); n != 2 {
		t.Fatalf("chain checks = %d, want 2: one for each chain", n)
	}
}

// An authenticator without a bundle must reject every certificate. Go would otherwise check the chain
// against the CAs of the operating system. On Linux they are read from SSL_CERT_FILE, which makes
// the CA of the test certificate one of them; on other systems the test cannot tell the difference.
func TestCertificateIsRejectedWithoutBundle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ca := newTestCA(t, "kubernetes")
	dir := t.TempDir()
	systemRoots := filepath.Join(dir, "system.crt")
	writeCAFile(t, systemRoots, ca)
	t.Setenv("SSL_CERT_FILE", systemRoots)

	clock := &fakeClock{now: testNow}
	noBundle := &reloader[x509.CertPool]{name: "client CA bundle", files: []string{filepath.Join(dir, "absent")}, clock: clock, log: logger}
	a := newCertAuthenticator(noBundle, DefaultCacheSize, clock, logger)
	admin := certRequest(t, ca.issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth))
	if user := a.authenticate(admin); user != nil {
		t.Fatalf("user = %+v, want none", user)
	}
}

func TestClientCAFileIsReread(t *testing.T) {
	e := newCertEnv(t)
	oldAdmin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	newCA := newTestCA(t, "kubernetes-rotated")
	newAdmin := newCA.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.doCert(t, oldAdmin, ""), http.StatusOK)
	expectCode(t, e.doCert(t, newAdmin, ""), http.StatusUnauthorized)

	// The file is not read again before the interval passes.
	writeCAFile(t, e.caFile, newCA)
	e.clock.step(reloadInterval - time.Second)
	expectCode(t, e.doCert(t, oldAdmin, ""), http.StatusOK)
	e.clock.step(time.Second)
	expectCode(t, e.doCert(t, oldAdmin, ""), http.StatusUnauthorized)
	expectCode(t, e.doCert(t, newAdmin, ""), http.StatusOK)

	// A broken bundle in the middle of a rotation keeps the previous one in use.
	// The clock moves past the cached answer, so the certificate is checked again.
	writeFile(t, e.caFile, []byte("broken"))
	e.clock.step(chainCheckTTL + time.Second)
	expectCode(t, e.doCert(t, newAdmin, ""), http.StatusOK)

	if err := os.Remove(e.caFile); err != nil {
		t.Fatal(err)
	}
	e.clock.step(chainCheckTTL + time.Second)
	expectCode(t, e.doCert(t, newAdmin, ""), http.StatusOK)
}

// A bundle rewritten in place can be read halfway: the first CA is there, the second is not yet.
// The modification time is already the final one, so only the content tells the two reads apart.
func TestClientCAFileReadHalfwayIsReadAgain(t *testing.T) {
	e := newCertEnv(t)
	second := newTestCA(t, "kubernetes-second")
	admin := second.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	mtime := testNow.Add(time.Minute)

	writeCAFile(t, e.caFile, e.ca)
	if err := os.Chtimes(e.caFile, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	e.clock.step(reloadInterval)
	expectCode(t, e.doCert(t, admin, ""), http.StatusUnauthorized)

	writeCAFile(t, e.caFile, e.ca, second)
	if err := os.Chtimes(e.caFile, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	e.clock.step(reloadInterval)
	expectCode(t, e.doCert(t, admin, ""), http.StatusOK)
}

// A bundle with a damaged certificate is not loaded in part: that would drop a CA silently.
func TestDamagedClientCAFileIsNotLoaded(t *testing.T) {
	var logs bytes.Buffer
	e := newCertEnvWithLogger(t, slog.New(slog.NewTextHandler(&logs, nil)))
	admin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	otherCA := newTestCA(t, "another-ca")
	otherAdmin := otherCA.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	// The certificate of the trusted CA is cut short, as in a file read halfway through a rewrite.
	trusted := caPEM(e.ca)
	writeFile(t, e.caFile, append(caPEM(otherCA), trusted[:len(trusted)/2]...))
	e.clock.step(reloadInterval)

	expectCode(t, e.doCert(t, admin, ""), http.StatusOK)
	expectCode(t, e.doCert(t, otherAdmin, ""), http.StatusUnauthorized)
	if n := strings.Count(logs.String(), "cannot reload the client CA bundle"); n != 1 {
		t.Fatalf("warnings = %d, want 1; log %q", n, logs.String())
	}
}

func TestBrokenClientCAFileIsReportedOnce(t *testing.T) {
	var logs bytes.Buffer
	e := newCertEnvWithLogger(t, slog.New(slog.NewTextHandler(&logs, nil)))
	admin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	writeFile(t, e.caFile, []byte("broken"))
	for range 3 {
		e.clock.step(reloadInterval)
		expectCode(t, e.doCert(t, admin, ""), http.StatusOK)
	}
	if n := strings.Count(logs.String(), "cannot reload the client CA bundle"); n != 1 {
		t.Fatalf("warnings = %d, want 1; log %q", n, logs.String())
	}
}

func TestInvalidClientCAFile(t *testing.T) {
	rules := []Rule{{Path: "/metrics", ResourceAttributes: metricsAttrs}}
	api := &fakeAPI{}
	dir := t.TempDir()
	first, second := caPEM(newTestCA(t, "kubernetes")), caPEM(newTestCA(t, "kubernetes-second"))
	lines := strings.Split(strings.TrimSpace(string(second)), "\n")
	join := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

	files := map[string][]byte{
		"absent":               nil,
		"empty":                {},
		"without certificates": []byte("not a certificate"),
		"with a block that is not a certificate": join(first,
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not a certificate")})),
		"with a block of another type": join(first,
			bytes.Replace(second, []byte("CERTIFICATE"), []byte("TRUSTED CERTIFICATE"), 2)),
		"with a block with headers": join(first,
			bytes.Replace(second, []byte("-----\n"), []byte("-----\nComment: second\n\n"), 1)),
		"with a block cut short":       join(first, []byte(strings.Join(lines[:len(lines)/2], "\n")+"\n")),
		"with a block without its end": join([]byte(strings.Join(lines[:len(lines)-1], "\n")+"\n"), first),
		// Without its first line the block looks like text, and the certificate after it is valid.
		"with a block without its beginning": join([]byte(strings.Join(lines[1:], "\n")+"\n"), first),
		// Files concatenated without a line break at the end of the first one.
		"with blocks glued together": join(bytes.TrimSuffix(first, []byte("\n")), second),
		"with damaged base64":        join(first, bytes.Replace(second, []byte(lines[2]), []byte("!"+lines[2][1:]), 1)),
	}
	for name, data := range files {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
			if data != nil {
				writeFile(t, file, data)
			}
			_, err := New(api.client(), Config{Rules: rules, ClientCAFile: file})
			if err == nil || !strings.HasPrefix(err.Error(), "invalid config: ") {
				t.Fatalf("err = %v, want an invalid config error", err)
			}
		})
	}
}

// Line endings, space and text around the certificates do not make a bundle invalid.
// Every certificate of the bundle is loaded, and a certificate in the text is not.
func TestClientCAFileLayout(t *testing.T) {
	first, second, old := newTestCA(t, "kubernetes"), newTestCA(t, "kubernetes-second"), newTestCA(t, "kubernetes-old")
	firstPEM, secondPEM := string(caPEM(first)), string(caPEM(second))
	commentedOut := "# " + strings.ReplaceAll(strings.TrimSuffix(string(caPEM(old)), "\n"), "\n", "\n# ") + "\n"
	byteOrderMark := "\xEF\xBB\xBF"
	bundles := map[string]string{
		"space and line endings": "\n  " + firstPEM + "\n\n" + strings.ReplaceAll(secondPEM, "\n", "\r\n") + "\n \n",
		// As in the bundles of RHEL.
		"comments": "# kubernetes\n" + firstPEM + "# kubernetes-second\n" + secondPEM,
		// As openssl x509 -text and openssl s_client -showcerts print it.
		"output of openssl": "Certificate:\n    Data:\n        Subject: CN = kubernetes\n" + firstPEM +
			" 1 s:CN = kubernetes-second\n   i:CN = kubernetes-second\n" + secondPEM + "---\n",
		"commented out certificate": firstPEM + commentedOut + secondPEM,
		// Two files saved by an editor on Windows and joined with cat.
		"byte order marks": byteOrderMark + firstPEM + byteOrderMark + secondPEM,
	}
	for name, bundle := range bundles {
		t.Run(name, func(t *testing.T) {
			caFile := filepath.Join(t.TempDir(), "ca.crt")
			writeFile(t, caFile, []byte(bundle))

			e := newEnv(t, Config{ClientCAFile: caFile})
			for _, ca := range []*testCA{first, second} {
				expectCode(t, e.doCert(t, ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth), ""), http.StatusOK)
			}
			expectCode(t, e.doCert(t, old.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth), ""), http.StatusUnauthorized)
		})
	}
}

// The whole path: the server asks for a certificate, the client presents it during a real handshake
// and the middleware checks it.
func TestServerAuthenticatesClientCertificate(t *testing.T) {
	e := newCertEnv(t)
	tlsConfig, err := newTLSConfig(ServerConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTLSServer(t, ServerConfig{Handler: e.handler, ShutdownTimeout: time.Second}, tlsConfig)

	get := func(token string, certs ...tls.Certificate) int {
		t.Helper()
		client := &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, Certificates: certs}, //nolint:gosec // the certificate is self-signed
		}}
		req, err := http.NewRequest(http.MethodGet, "https://"+addr+"/metrics", nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	admin := e.ca.issue(t, "kubernetes-admin", []string{"system:masters"}, x509.ExtKeyUsageClientAuth)
	chained := e.ca.intermediate(t, "kubernetes-users").issue(t, "jane", nil, x509.ExtKeyUsageClientAuth)
	stranger := newTestCA(t, "another-ca").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	if code := get(""); code != http.StatusUnauthorized {
		t.Fatalf("no credentials: status = %d", code)
	}
	if code := get("", admin); code != http.StatusOK {
		t.Fatalf("trusted certificate: status = %d", code)
	}
	expectCalls(t, e.api, 0, 1)
	if code := get("", chained); code != http.StatusOK {
		t.Fatalf("trusted certificate with an intermediate CA: status = %d", code)
	}
	if user, _ := e.lastUser(); user != "jane" {
		t.Fatalf("user = %q", user)
	}
	if code := get("", stranger); code != http.StatusUnauthorized {
		t.Fatalf("certificate from another CA: status = %d", code)
	}
	if code := get(validToken, stranger); code != http.StatusOK {
		t.Fatalf("certificate from another CA with a token: status = %d", code)
	}
	if code := get(validToken); code != http.StatusOK {
		t.Fatalf("token: status = %d", code)
	}
}

// A client with several certificates sends the first one unless the server names the CAs it accepts.
func TestServerNamesAcceptedClientCAs(t *testing.T) {
	e := newCertEnv(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	serve := func(cfg ServerConfig) string {
		t.Helper()
		cfg.Handler, cfg.ShutdownTimeout = e.handler, time.Second
		tlsConfig, err := newTLSConfig(cfg, logger)
		if err != nil {
			t.Fatal(err)
		}
		addr, _ := startTLSServer(t, cfg, tlsConfig)
		return addr
	}
	get := func(addr string, certs ...tls.Certificate) int {
		t.Helper()
		client := &http.Client{Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, Certificates: certs}, //nolint:gosec // the certificate is self-signed
		}}
		resp, err := client.Get("https://" + addr + "/metrics")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		// Naming the CAs must not cost the connection HTTP/2.
		if resp.ProtoMajor != 2 {
			t.Fatalf("protocol = %s, want HTTP/2", resp.Proto)
		}
		return resp.StatusCode
	}

	stranger := newTestCA(t, "another-ca").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	admin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	chained := e.ca.intermediate(t, "kubernetes-users").issue(t, "jane", nil, x509.ExtKeyUsageClientAuth)

	anyCA := serve(ServerConfig{})
	if code := get(anyCA, stranger, admin); code != http.StatusUnauthorized {
		t.Fatalf("no CAs named: status = %d", code)
	}

	named := serve(ServerConfig{ClientCAs: e.auth.ClientCAs})
	if code := get(named, stranger, admin); code != http.StatusOK {
		t.Fatalf("CAs named: status = %d", code)
	}
	if code := get(named, stranger, chained); code != http.StatusOK {
		t.Fatalf("CAs named, certificate with an intermediate CA: status = %d", code)
	}
	if code := get(named, stranger); code != http.StatusUnauthorized {
		t.Fatalf("CAs named, no certificate of theirs: status = %d", code)
	}

	// The named CAs follow the reloaded bundle.
	rotated := newTestCA(t, "kubernetes-rotated")
	rotatedAdmin := rotated.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	if code := get(named, rotatedAdmin, admin); code != http.StatusOK {
		t.Fatalf("before the rotation: status = %d", code)
	}
	writeCAFile(t, e.caFile, rotated)
	e.clock.step(reloadInterval)
	if code := get(named, admin, rotatedAdmin); code != http.StatusOK {
		t.Fatalf("after the rotation: status = %d", code)
	}
	if code := get(named, admin); code != http.StatusUnauthorized {
		t.Fatalf("after the rotation, certificate of the previous CA: status = %d", code)
	}
}

// Without HTTP/2 the server must not offer it while naming the CAs: the client would agree to it
// and get a connection that nothing serves.
func TestServerNamesClientCAsWithoutHTTP2(t *testing.T) {
	t.Setenv("GODEBUG", "http2server=0")
	e := newCertEnv(t)
	cfg := ServerConfig{Handler: e.handler, ClientCAs: e.auth.ClientCAs, ShutdownTimeout: time.Second}
	tlsConfig, err := newTLSConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTLSServer(t, cfg, tlsConfig)

	admin := e.ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)
	client := &http.Client{Transport: &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, Certificates: []tls.Certificate{admin}}, //nolint:gosec // the certificate is self-signed
	}}
	resp, err := client.Get("https://" + addr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.ProtoMajor != 1 || resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %d, want HTTP/1.1 200", resp.Proto, resp.StatusCode)
	}
}

func TestClientCAsWithoutClientCAFile(t *testing.T) {
	if cas := newEnv(t, Config{}).auth.ClientCAs(); cas != nil {
		t.Fatalf("CAs = %v, want none", cas)
	}
}
