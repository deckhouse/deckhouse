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

package revocationfeed

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
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/revocation"
)

func feedClients() []client.Object {
	return []client.Object{
		cachedClient("console", "s3cr3t", true),
		cachedClient("grafana", "gr4f4na", true),
		cachedClient("kubernetes", "s3cr3t", false),
	}
}

// Unknown client, a client that is not a DexAuthenticator, and a wrong secret get the same answer,
// byte for byte: the feed tells nothing about which clients exist.
func TestServer_UnauthorizedIsAlwaysTheSame(t *testing.T) {
	t.Parallel()

	f := newTestFeed(t, testLimits(), feedClients()...)
	type answer struct {
		status          int
		wwwAuthenticate string
		contentType     string
		body            string
	}
	var answers []answer
	for _, creds := range [][2]string{
		{"grafana", "s3cr3t"},
		{"kubernetes", "s3cr3t"},
		{"console", "wrong"},
		{"", ""},
	} {
		resp := f.get(t, creds[0], creds[1], "")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		answers = append(answers, answer{
			status:          resp.StatusCode,
			wwwAuthenticate: resp.Header.Get("WWW-Authenticate"),
			contentType:     resp.Header.Get("Content-Type"),
			body:            string(body),
		})
	}
	for i, a := range answers {
		if a != answers[0] {
			t.Errorf("answer %d = %+v, answer 0 = %+v", i, a, answers[0])
		}
	}
	if answers[0].status != http.StatusUnauthorized || answers[0].wwwAuthenticate != `Basic realm="user-authn-revocations"` {
		t.Errorf("answer = %+v", answers[0])
	}
	if got := counterValue(t, f.server.metrics.connections.WithLabelValues(resultUnauthorized)); got != 4 {
		t.Errorf("unauthorized connections = %v, want 4", got)
	}
}

func TestServer_SnapshotThenEvents(t *testing.T) {
	t.Parallel()

	f := newTestFeed(t, testLimits(), append(feedClients(),
		revocationOf(t, pairRevocation("pair-a-g3", "jane", 3, "console"), feedNow.Add(-time.Minute)),
		revocationOf(t, pairRevocation("pair-b-g1", "john", 1, "grafana"), feedNow.Add(-time.Minute)),
	)...)
	resp := f.get(t, "console", "s3cr3t", "")
	for header, expected := range map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-store",
		"X-Accel-Buffering": "no",
	} {
		if got := resp.Header.Get(header); got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}
	stream := newSSE(t, resp)

	snapshot := stream.nextEvent(t)
	epoch := f.server.hub.epoch
	salt := Salt([]byte("s3cr3t"))
	expectedSnapshot := "event: snapshot\nid: " + epoch + ".0\ndata: " +
		`{"v":1,"part":0,"last":true,"retention":4200,"pairs":[{"k":"` + Key(salt, keyPair, revocation.Subject("jane", "local")) +
		`","g":3,"exp":` + unixSeconds(feedNow.Add(-time.Minute).Add(retention)) + `}],"sessions":[],"grants":[]}` + "\n\n"
	if snapshot.raw != expectedSnapshot {
		t.Errorf("snapshot =\n%q\nwant\n%q", snapshot.raw, expectedSnapshot)
	}

	// A revocation for grafana only does not reach console; the next one does.
	f.add(t, pairRevocation("pair-b-g2", "john", 2, "grafana"), feedNow)
	f.add(t, revocation.Revocation{
		Name: "grant-x", Type: revocation.TypeGrant, ConnectorID: "local", UserID: "jane",
		GrantID: "3w4l2zgokgsohvjnqzjrv7lmz", ClientID: "console", Clients: []string{"console"},
	}, feedNow)
	event := stream.nextEvent(t)
	expected := "event: revocation\nid: " + epoch + ".2\ndata: " +
		`{"v":1,"type":"grant","k":"jVFqq8vBRSjPWcgLzpRzaA","exp":` + unixSeconds(feedNow.Add(retention)) + `,"at":` + unixSeconds(feedNow) + "}\n\n"
	if event.raw != expected {
		t.Errorf("event =\n%q\nwant\n%q", event.raw, expected)
	}

	if got := counterValue(t, f.server.metrics.connections.WithLabelValues(resultSnapshot)); got != 1 {
		t.Errorf("snapshot connections = %v, want 1", got)
	}
	var m dto.Metric
	if err := f.server.metrics.delivery.Write(&m); err != nil {
		t.Fatal(err)
	}
	if m.GetHistogram().GetSampleCount() != 1 {
		t.Errorf("delivery samples = %d, want 1", m.GetHistogram().GetSampleCount())
	}
}

// A client that reconnects to the same process with the id of its last event gets what it missed
// and no snapshot.
func TestServer_ResumesInTheSameEpoch(t *testing.T) {
	t.Parallel()

	f := newTestFeed(t, testLimits(), feedClients()...)
	first := newSSE(t, f.get(t, "console", "s3cr3t", ""))
	if e := first.nextEvent(t); e.name != eventSnapshot {
		t.Fatalf("first event = %+v", e)
	}
	f.add(t, pairRevocation("pair-a-g1", "jane", 1, "console"), feedNow)
	last := first.nextEvent(t)
	f.add(t, pairRevocation("pair-a-g2", "jane", 2, "console"), feedNow)
	f.add(t, pairRevocation("pair-a-g3", "jane", 3, "console", "grafana"), feedNow)

	resumed := newSSE(t, f.get(t, "console", "s3cr3t", last.id))
	for _, g := range []uint64{2, 3} {
		e := resumed.nextEvent(t)
		if e.name != eventRevocation || decode[revocationData](t, e.data).G != g {
			t.Errorf("resumed event = %+v, want generation %d", e, g)
		}
	}
	if got := counterValue(t, f.server.metrics.connections.WithLabelValues(resultResumed)); got != 1 {
		t.Errorf("resumed connections = %v, want 1", got)
	}

	// The cursor of another process is no use: a snapshot.
	other := newSSE(t, f.get(t, "console", "s3cr3t", "ffffffffffffffff.3"))
	if e := other.nextEvent(t); e.name != eventSnapshot {
		t.Errorf("event after another epoch = %+v, want a snapshot", e)
	}
}

func TestServer_Heartbeat(t *testing.T) {
	t.Parallel()

	limits := testLimits()
	limits.Heartbeat = 20 * time.Millisecond
	f := newTestFeed(t, limits, feedClients()...)
	f.add(t, pairRevocation("pair-a-g1", "jane", 1, "grafana"), feedNow)
	stream := newSSE(t, f.get(t, "console", "s3cr3t", ""))
	stream.nextEvent(t) // snapshot
	f.add(t, pairRevocation("pair-b-g1", "john", 1, "grafana"), feedNow)

	e := stream.nextEvent(t)
	if e.name != eventHeartbeat || e.data != `{"v":1,"t":`+unixSeconds(feedNow)+`}` {
		t.Errorf("heartbeat = %+v", e)
	}
	// Nothing for console is pending, so the heartbeat carries the latest position.
	if e.id != f.server.hub.epoch+".2" {
		t.Errorf("heartbeat id = %q, want the latest position", e.id)
	}
}

func TestServer_Refusals(t *testing.T) {
	t.Parallel()

	t.Run("failed sign-ins from one address", func(t *testing.T) {
		t.Parallel()
		limits := testLimits()
		limits.FailureBurst = 3
		f := newTestFeed(t, limits, feedClients()...)
		for range 3 {
			if resp := f.get(t, "console", "wrong", ""); resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", resp.StatusCode)
			}
		}
		// Even the right secret waits now.
		resp := f.get(t, "console", "s3cr3t", "")
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "10" {
			t.Errorf("status = %d, Retry-After = %q; want 429 and 10", resp.StatusCode, resp.Header.Get("Retry-After"))
		}
	})

	t.Run("streams of a client", func(t *testing.T) {
		t.Parallel()
		limits := testLimits()
		limits.StreamsPerClient = 1
		f := newTestFeed(t, limits, feedClients()...)
		newSSE(t, f.get(t, "console", "s3cr3t", "")).nextEvent(t)
		resp := f.get(t, "console", "s3cr3t", "")
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "30" {
			t.Errorf("status = %d, Retry-After = %q; want 429 and 30", resp.StatusCode, resp.Header.Get("Retry-After"))
		}
		// Another client is not limited by it.
		newSSE(t, f.get(t, "grafana", "gr4f4na", "")).nextEvent(t)
	})

	t.Run("streams of the replica", func(t *testing.T) {
		t.Parallel()
		limits := testLimits()
		limits.Streams = 1
		f := newTestFeed(t, limits, feedClients()...)
		newSSE(t, f.get(t, "console", "s3cr3t", "")).nextEvent(t)
		resp := f.get(t, "grafana", "gr4f4na", "")
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "30" {
			t.Errorf("status = %d, Retry-After = %q; want 429 and 30", resp.StatusCode, resp.Header.Get("Retry-After"))
		}
	})

	t.Run("new connections of a client", func(t *testing.T) {
		t.Parallel()
		limits := testLimits()
		limits.ConnectionBurst = 2
		f := newTestFeed(t, limits, feedClients()...)
		for range 2 {
			resp := f.get(t, "console", "s3cr3t", "")
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
		resp := f.get(t, "console", "s3cr3t", "")
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "5" {
			t.Errorf("status = %d, Retry-After = %q; want 429 and 5", resp.StatusCode, resp.Header.Get("Retry-After"))
		}
		if got := counterValue(t, f.server.metrics.connections.WithLabelValues(resultLimited)); got != 1 {
			t.Errorf("limited connections = %v, want 1", got)
		}
	})

	t.Run("not an event stream", func(t *testing.T) {
		t.Parallel()
		f := newTestFeed(t, testLimits(), feedClients()...)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.http.URL+Path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth("console", "s3cr3t")
		req.Header.Set("Accept", "application/json")
		resp, err := f.http.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotAcceptable {
			t.Errorf("status = %d, want 406", resp.StatusCode)
		}
	})

	t.Run("another method", func(t *testing.T) {
		t.Parallel()
		f := newTestFeed(t, testLimits(), feedClients()...)
		resp, err := f.http.Client().Post(f.http.URL+Path, "text/plain", strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})
}

// A stream that fell behind is closed; the client reconnects and catches up.
func TestServer_SlowConsumerIsDisconnected(t *testing.T) {
	t.Parallel()

	limits := testLimits()
	limits.Queue = 1
	f := newTestFeed(t, limits, feedClients()...)
	resp := f.get(t, "console", "s3cr3t", "")
	stream := newSSE(t, resp)
	stream.nextEvent(t) // snapshot

	// The hub drops a stream whose queue is full (TestHub_SlowConsumer); here it drops this one.
	f.server.hub.mu.Lock()
	for st := range f.server.hub.streams {
		f.server.hub.dropLocked(st)
	}
	f.server.hub.mu.Unlock()

	deadline := time.After(5 * time.Second)
	for {
		if _, err := stream.next(); err != nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the stream was not closed")
		default:
		}
	}
	if perClient, _ := f.server.hub.stats(feedNow); perClient["console"] != 0 {
		t.Errorf("streams of console = %d, want 0", perClient["console"])
	}
}

// errConnectionDropped is what a write to a connection the kernel has dropped returns.
var errConnectionDropped = errors.New("write tcp: connection timed out")

// droppedWriter is a ResponseWriter whose writes fail once drop is closed, as the writes to a
// connection fail after the kernel gave up on a peer that is gone (TCP_USER_TIMEOUT).
type droppedWriter struct {
	header  http.Header
	drop    chan struct{}
	written atomic.Int64
}

func (w *droppedWriter) Header() http.Header { return w.header }

func (w *droppedWriter) WriteHeader(int) {}

func (w *droppedWriter) Write(p []byte) (int, error) {
	select {
	case <-w.drop:
		return 0, errConnectionDropped
	default:
	}
	w.written.Add(int64(len(p)))
	return len(p), nil
}

func (w *droppedWriter) FlushError() error { return nil }

func (w *droppedWriter) SetWriteDeadline(time.Time) error { return nil }

// F2: once the kernel drops the connection of a client that is gone, the next write - a heartbeat
// at the latest - fails, the stream closes, and the subscribers gauge no longer counts it.
func TestServer_FailedWriteClosesTheStream(t *testing.T) {
	t.Parallel()

	limits := testLimits()
	limits.Heartbeat = 5 * time.Millisecond
	f := newTestFeed(t, limits, feedClients()...)
	registry := prometheus.NewPedanticRegistry()
	if err := register(registry, f.server); err != nil {
		t.Fatal(err)
	}

	w := &droppedWriter{header: http.Header{}, drop: make(chan struct{})}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, Path, nil)
	req.SetBasicAuth("console", "s3cr3t")
	req.Header.Set("Accept", "text/event-stream")
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.server.Handler().ServeHTTP(w, req)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for subscribersOf(t, registry)["console"] != 1 || w.written.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the stream did not start")
		}
		time.Sleep(time.Millisecond)
	}

	close(w.drop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not close after a failed write")
	}
	if got := subscribersOf(t, registry); got["console"] != 0 {
		t.Errorf("subscribers = %v, want console at 0", got)
	}
}

// subscribersOf gathers the subscribers gauge by client from registry.
func subscribersOf(t *testing.T, registry prometheus.Gatherer) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, family := range families {
		if family.GetName() != subscribersMetricName {
			continue
		}
		for _, m := range family.GetMetric() {
			out[m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
		}
	}
	return out
}

func TestServer_InitialListIsNotNumbered(t *testing.T) {
	t.Parallel()

	f := newTestFeed(t, testLimits(), feedClients()...)
	obj := revocationOf(t, pairRevocation("pair-a-g1", "jane", 1, "console"), feedNow)
	f.server.onRevocation(obj, true)
	if f.server.hub.seq != 0 {
		t.Errorf("seq = %d after the initial list, want 0", f.server.hub.seq)
	}
	f.server.onRevocation(obj, false)
	if f.server.hub.seq != 1 {
		t.Errorf("seq = %d after a new object, want 1", f.server.hub.seq)
	}
	if f.server.NeedLeaderElection() {
		t.Error("the feed must run on every replica")
	}
}

func TestServer_Metrics(t *testing.T) {
	t.Parallel()

	f := newTestFeed(t, testLimits(), feedClients()...)
	newSSE(t, f.get(t, "console", "s3cr3t", "")).nextEvent(t)

	registry := prometheus.NewPedanticRegistry()
	if err := register(registry, f.server); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	subscribers := map[string]float64{}
	var lag *float64
	for _, family := range families {
		switch family.GetName() {
		case subscribersMetricName:
			for _, m := range family.GetMetric() {
				subscribers[m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
			}
		case lagMetricName:
			v := family.GetMetric()[0].GetGauge().GetValue()
			lag = &v
		}
	}
	// grafana is a known DexAuthenticator client without streams; kubernetes is not one.
	if len(subscribers) != 2 || subscribers["console"] != 1 || subscribers["grafana"] != 0 {
		t.Errorf("subscribers = %v", subscribers)
	}
	if lag == nil || *lag < 0 || *lag > 10 {
		t.Errorf("lag = %v", lag)
	}
}

// Start-to-end over TLS: the stream ends when the manager stops.
func TestServer_ServeTLS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pool := writeCertificate(t, dir)
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"))
	if err != nil {
		t.Fatal(err)
	}
	reader := newTestReader(t, feedClients()...)
	s, err := New(reader, Config{TLSDir: dir, Retention: retention, Limits: testLimits()}, logr.Discard(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() {
		served <- s.serve(ctx, ln, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})
	}()

	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "revocation-feed.d8-user-authn", MinVersion: tls.VersionTLS12}}}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+ln.Addr().String()+Path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("console", "s3cr3t")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	stream := newSSE(t, resp)
	stream.nextEvent(t)

	cancel()
	if _, err := io.ReadAll(resp.Body); err != nil && !strings.Contains(err.Error(), "closed") {
		t.Logf("read after stop: %v", err)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Errorf("serve = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the stop")
	}
}

func TestServer_StartFailsWithoutACertificate(t *testing.T) {
	t.Parallel()

	s, err := New(newTestReader(t), Config{Addr: "127.0.0.1:0", TLSDir: t.TempDir(), Retention: retention, Limits: testLimits()}, logr.Discard(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(t.Context()); err == nil {
		t.Fatal("Start served without a certificate")
	}
}

// writeCertificate writes a self-signed certificate of the feed's Service into dir and returns a
// pool that trusts it.
func writeCertificate(t *testing.T, dir string) *x509.CertPool {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "revocation-feed.d8-user-authn"},
		DNSNames:              []string{"revocation-feed.d8-user-authn"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return pool
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetCounter().GetValue()
}

func unixSeconds(at time.Time) string {
	return strconv.FormatInt(at.Unix(), 10)
}
