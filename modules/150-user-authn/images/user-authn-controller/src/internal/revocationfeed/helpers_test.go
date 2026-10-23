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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

// feedNow is the time of the tests: revocations of a minute ago are fresh at it.
var feedNow = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

const retention = 70 * time.Minute

// rawClient is an OAuth2Client as Dex stores it; da marks the client of a DexAuthenticator.
func rawClient(id, secret string, da bool) *unstructured.Unstructured {
	obj := controller.Object(controller.OAuth2ClientGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(strings.ReplaceAll(id, ":", "-") + "-client")
	obj.SetResourceVersion("7")
	if da {
		obj.SetLabels(map[string]string{DexAuthenticatorLabel: "console", "app": "dex"})
	} else {
		obj.SetLabels(map[string]string{"app": "dex"})
	}
	obj.Object["id"] = id
	obj.Object["secret"] = secret
	return obj
}

// cachedClient is the client as the cache keeps it.
func cachedClient(id, secret string, da bool) *unstructured.Unstructured {
	out, err := TransformClient(rawClient(id, secret, da))
	if err != nil {
		panic(err)
	}
	obj, ok := out.(*unstructured.Unstructured)
	if !ok {
		panic(fmt.Sprintf("TransformClient = %T", out))
	}
	obj.SetResourceVersion("")
	return obj
}

func newTestReader(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "dex.coreos.com", Version: "v1"}})
	m.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)
	m.Add(controller.RevocationGVK, meta.RESTScopeNamespace)
	builder := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithRESTMapper(m).WithObjects(objs...)
	for _, index := range controller.Indexes() {
		builder = builder.WithIndex(index.Object, index.Field, index.Extract)
	}
	return builder.Build()
}

// revocationOf is a Revocation object created at the time at.
func revocationOf(t *testing.T, rev revocation.Revocation, at time.Time) *unstructured.Unstructured {
	t.Helper()
	rev.CreatedAt = at
	if rev.Source == "" {
		rev.Source = revocation.SourceController
	}
	obj, err := rev.Object()
	if err != nil {
		t.Fatal(err)
	}
	obj.SetCreationTimestamp(metav1.NewTime(at.Truncate(time.Second)))
	return obj
}

func pairRevocation(name, userID string, generation uint64, clients ...string) revocation.Revocation {
	return revocation.Revocation{
		Name:        name,
		Type:        revocation.TypePair,
		ConnectorID: naming.LocalConnectorID,
		UserID:      userID,
		Subject:     revocation.Subject(userID, naming.LocalConnectorID),
		Generation:  generation,
		Clients:     clients,
		Reason:      revocation.ReasonLock,
	}
}

func testLimits() Limits {
	l := DefaultLimits()
	l.ConnectionBurst = 100
	l.Heartbeat = time.Hour
	return l
}

type testFeed struct {
	server *Server
	reader client.WithWatch
	http   *httptest.Server
}

func newTestFeed(t *testing.T, limits Limits, objs ...client.Object) *testFeed {
	t.Helper()
	reader := newTestReader(t, objs...)
	s, err := New(reader, Config{Retention: retention, Limits: limits}, logr.Discard(), func() time.Time { return feedNow })
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objs {
		if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == controller.OAuth2ClientGVK.Kind {
			s.clients.set(u)
		}
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &testFeed{server: s, reader: reader, http: srv}
}

// add creates a Revocation and hands it to the feed, as the informer does.
func (f *testFeed) add(t *testing.T, rev revocation.Revocation, at time.Time) {
	t.Helper()
	obj := revocationOf(t, rev, at)
	if err := f.reader.Create(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	f.server.onRevocation(obj, false)
}

func (f *testFeed) get(t *testing.T, id, secret, lastEventID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.http.URL+Path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		req.SetBasicAuth(id, secret)
	}
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := f.http.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// sse reads server-sent events.
type sse struct {
	r *bufio.Reader
}

type sseEvent struct {
	name string
	id   string
	data string
	// raw is the event as it was written.
	raw string
}

func newSSE(t *testing.T, resp *http.Response) *sse {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	s := &sse{r: bufio.NewReader(resp.Body)}
	first, err := s.next()
	if err != nil {
		t.Fatalf("read the first block: %v", err)
	}
	if first.raw != "retry: 5000\n\n" {
		t.Fatalf("first block = %q, want the retry", first.raw)
	}
	return s
}

func (s *sse) next() (sseEvent, error) {
	var e sseEvent
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return e, err
		}
		e.raw += line
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			return e, nil
		}
		field, value, _ := strings.Cut(line, ": ")
		switch field {
		case "event":
			e.name = value
		case "id":
			e.id = value
		case "data":
			e.data = value
		}
	}
}

// nextEvent reads the next event, failing the test after a while.
func (s *sse) nextEvent(t *testing.T) sseEvent {
	t.Helper()
	type result struct {
		e   sseEvent
		err error
	}
	ch := make(chan result, 1)
	go func() {
		e, err := s.next()
		ch <- result{e, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read an event: %v", r.err)
		}
		return r.e
	case <-time.After(5 * time.Second):
		t.Fatal("no event within 5 seconds")
		return sseEvent{}
	}
}

func decode[T any](t *testing.T, data string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return v
}

func stringOf(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}
