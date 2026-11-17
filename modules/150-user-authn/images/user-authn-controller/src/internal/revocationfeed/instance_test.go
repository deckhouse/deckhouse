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
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/revocation"
)

// instanceClient is the cached OAuth2Client of the DexAuthenticator of a namespace.
func instanceClient(id, secret, namespace string) *unstructured.Unstructured {
	obj := cachedClient(id, secret, true)
	labels := obj.GetLabels()
	labels[DexAuthenticatorNamespaceLabel] = namespace
	obj.SetLabels(labels)
	return obj
}

// reviewTokens is a TokenReviewer that knows the tokens of users.
func reviewTokens(users map[string]string) TokenReviewer {
	return func(_ context.Context, token string) (string, bool, error) {
		if token == "broken-review" {
			return "", false, errors.New("the API server does not answer")
		}
		username, ok := users[token]
		return username, ok, nil
	}
}

const (
	systemToken = "token-of-the-system-instance"
	userToken   = "token-of-the-user-instance"
)

func instanceTokens() TokenReviewer {
	return reviewTokens(map[string]string{
		systemToken:          "system:serviceaccount:d8-user-authn:dex-authenticator-system",
		userToken:            "system:serviceaccount:d8-user-authn:dex-authenticator",
		"token-of-dex":       "system:serviceaccount:d8-user-authn:dex",
		"token-of-a-project": "system:serviceaccount:project:dex-authenticator",
		"token-of-a-user":    "jane.doe@example.com",
	})
}

// instanceClients are two clients of the system instance, two of the user instance, and a client
// that is not of a DexAuthenticator.
func instanceClients() []client.Object {
	return []client.Object{
		instanceClient("console", "s3cr3t", "d8-console"),
		instanceClient("grafana", "gr4f4na", "d8-monitoring"),
		instanceClient("shop", "sh0p", "shop"),
		instanceClient("wiki", "w1k1", "wiki"),
		cachedClient("kubernetes", "s3cr3t", false),
	}
}

func newInstanceFeed(t *testing.T, review TokenReviewer, objs ...client.Object) *testFeed {
	t.Helper()
	reader := newTestReader(t, objs...)
	s, err := New(reader, Config{Retention: retention, Limits: testLimits(), TokenReview: review}, logr.Discard(), func() time.Time { return feedNow })
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objs {
		if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == controller.OAuth2ClientGVK.Kind {
			s.clients.set(u)
		}
	}
	f := &testFeed{server: s, reader: reader}
	f.http = newHTTPServer(t, s)
	return f
}

func (f *testFeed) getWithToken(t *testing.T, token, lastEventID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.http.URL+Path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

// keysOf returns the keys of the pairs of a snapshot part, sorted.
func keysOf(entries []snapshotEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.K)
	}
	slices.Sort(keys)
	return keys
}

func pairKey(secret, userID string) string {
	return Key(Salt([]byte(secret)), keyPair, revocation.Subject(userID, "local"))
}

// Only the token of the ServiceAccount of an instance opens the stream of that instance; every
// other token gets the answer a wrong client secret gets.
func TestServer_InstanceAuthentication(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		review         TokenReviewer
		token          string
		expectedStatus int
	}{
		{name: "system instance", review: instanceTokens(), token: systemToken, expectedStatus: http.StatusOK},
		{name: "user instance", review: instanceTokens(), token: userToken, expectedStatus: http.StatusOK},
		{name: "another service account of the namespace", review: instanceTokens(), token: "token-of-dex", expectedStatus: http.StatusUnauthorized},
		{name: "service account of the same name in another namespace", review: instanceTokens(), token: "token-of-a-project", expectedStatus: http.StatusUnauthorized},
		{name: "user", review: instanceTokens(), token: "token-of-a-user", expectedStatus: http.StatusUnauthorized},
		{name: "token the API server refuses", review: instanceTokens(), token: "unknown", expectedStatus: http.StatusUnauthorized},
		{name: "feed without a token review", review: nil, token: systemToken, expectedStatus: http.StatusUnauthorized},
		{name: "review that fails", review: instanceTokens(), token: "broken-review", expectedStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newInstanceFeed(t, tt.review, instanceClients()...)
			resp := f.getWithToken(t, tt.token, "")
			if resp.StatusCode != tt.expectedStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.expectedStatus)
			}
			if tt.expectedStatus != http.StatusUnauthorized {
				return
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			basic := f.get(t, "console", "wrong", "")
			basicBody, err := io.ReadAll(basic.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != string(basicBody) || resp.Header.Get("WWW-Authenticate") != basic.Header.Get("WWW-Authenticate") {
				t.Errorf("answer = %q %q, a wrong client secret gets %q %q", body, resp.Header.Get("WWW-Authenticate"), basicBody, basic.Header.Get("WWW-Authenticate"))
			}
		})
	}
}

// The stream of an instance starts with one snapshot of the revocations of every client of the
// instance, each keyed with the salt of its client, and carries nothing of the other instance.
func TestServer_InstanceSnapshot(t *testing.T) {
	t.Parallel()

	f := newInstanceFeed(t, instanceTokens(), append(instanceClients(),
		revocationOf(t, pairRevocation("pair-jane", "jane", 3, "console", "shop"), feedNow.Add(-time.Minute)),
		revocationOf(t, pairRevocation("pair-john", "john", 1, "grafana"), feedNow.Add(-time.Minute)),
		revocationOf(t, pairRevocation("pair-mary", "mary", 2, "wiki", "kubernetes"), feedNow.Add(-time.Minute)),
	)...)

	for _, tt := range []struct {
		token    string
		expected []string
	}{
		{token: systemToken, expected: []string{pairKey("s3cr3t", "jane"), pairKey("gr4f4na", "john")}},
		{token: userToken, expected: []string{pairKey("sh0p", "jane"), pairKey("w1k1", "mary")}},
	} {
		events := newSSE(t, f.getWithToken(t, tt.token, ""))
		e := events.nextEvent(t)
		if e.name != eventSnapshot {
			t.Fatalf("first event = %q, want a snapshot", e.name)
		}
		snapshot := decode[snapshotData](t, e.data)
		if !snapshot.Last {
			t.Errorf("the snapshot of %s has more parts", tt.token)
		}
		slices.Sort(tt.expected)
		if got := keysOf(snapshot.Pairs); !slices.Equal(got, tt.expected) {
			t.Errorf("pairs of %s = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

// A revocation reaches the stream of an instance once for every client of the instance it names,
// with the key of that client, and never the stream of the other instance.
func TestServer_InstanceEvents(t *testing.T) {
	t.Parallel()

	f := newInstanceFeed(t, instanceTokens(), instanceClients()...)
	system := newSSE(t, f.getWithToken(t, systemToken, ""))
	user := newSSE(t, f.getWithToken(t, userToken, ""))
	for _, events := range []*sse{system, user} {
		if e := events.nextEvent(t); e.name != eventSnapshot {
			t.Fatalf("first event = %q, want a snapshot", e.name)
		}
	}

	// The first revocation names both clients of the system instance and one of the user instance;
	// the second only a client of the user instance.
	f.add(t, pairRevocation("pair-jane", "jane", 3, "console", "grafana", "shop"), feedNow)
	f.add(t, pairRevocation("pair-john", "john", 1, "wiki"), feedNow)

	var systemKeys []string
	var ids []string
	for range 2 {
		e := system.nextEvent(t)
		if e.name != eventRevocation {
			t.Fatalf("event = %q, want a revocation", e.name)
		}
		systemKeys = append(systemKeys, decode[revocationData](t, e.data).K)
		ids = append(ids, e.id)
	}
	slices.Sort(systemKeys)
	expected := []string{pairKey("s3cr3t", "jane"), pairKey("gr4f4na", "jane")}
	slices.Sort(expected)
	if !slices.Equal(systemKeys, expected) {
		t.Errorf("keys of the system instance = %v, want %v", systemKeys, expected)
	}
	if ids[0] != ids[1] {
		t.Errorf("the events of one revocation have the cursors %q and %q", ids[0], ids[1])
	}

	for _, expectedKey := range []string{pairKey("sh0p", "jane"), pairKey("w1k1", "john")} {
		e := user.nextEvent(t)
		if got := decode[revocationData](t, e.data).K; got != expectedKey {
			t.Errorf("key of the user instance = %q, want %q", got, expectedKey)
		}
	}
}

// A client that appears among the clients of an instance, or gets another secret, brings what is
// revoked for it into the live streams of the instance: the keys of the client are new to them.
func TestServer_InstanceGetsANewClient(t *testing.T) {
	t.Parallel()

	f := newInstanceFeed(t, instanceTokens(), append(instanceClients(),
		revocationOf(t, pairRevocation("pair-jane", "jane", 3, "console", "blog"), feedNow.Add(-time.Minute)),
	)...)
	user := newSSE(t, f.getWithToken(t, userToken, ""))
	if snapshot := decode[snapshotData](t, user.nextEvent(t).data); len(snapshot.Pairs) != 0 {
		t.Fatalf("pairs = %v, want none: the client blog is not known yet", snapshot.Pairs)
	}

	f.server.clients.set(instanceClient("blog", "bl0g", "blog"))
	e := user.nextEvent(t)
	if e.name != eventRevocation {
		t.Fatalf("event = %q, want a revocation", e.name)
	}
	if got := decode[revocationData](t, e.data); got.K != pairKey("bl0g", "jane") || got.G != 3 {
		t.Errorf("event = %+v, want the pair of jane for blog", got)
	}

	// The same object again changes nothing; another secret gives other keys.
	f.server.clients.set(instanceClient("blog", "bl0g", "blog"))
	f.server.clients.set(instanceClient("blog", "r0tated", "blog"))
	if got := decode[revocationData](t, user.nextEvent(t).data).K; got != pairKey("r0tated", "jane") {
		t.Errorf("key = %q, want the one of the new secret", got)
	}
}

// The stream of an instance does not resume from a cursor: its clients may have changed while it
// was away, so it always starts with a snapshot.
func TestServer_InstanceAlwaysStartsWithASnapshot(t *testing.T) {
	t.Parallel()

	f := newInstanceFeed(t, instanceTokens(), instanceClients()...)
	first := newSSE(t, f.getWithToken(t, systemToken, ""))
	cursorOfSnapshot := first.nextEvent(t).id
	f.add(t, pairRevocation("pair-jane", "jane", 3, "console"), feedNow)
	last := first.nextEvent(t).id

	for _, since := range []string{cursorOfSnapshot, last} {
		again := newSSE(t, f.getWithToken(t, systemToken, since))
		if e := again.nextEvent(t); e.name != eventSnapshot {
			t.Errorf("first event after the cursor %q = %q, want a snapshot", since, e.name)
		}
	}
}

// The streams of an instance are counted as the streams of one client, and their queue is the
// long one.
func TestHub_InstanceStream(t *testing.T) {
	t.Parallel()

	limits := testLimits()
	limits.StreamsPerClient = 2
	limits.Queue = 1
	limits.InstanceQueue = 8
	h, err := newHub(limits)
	if err != nil {
		t.Fatal(err)
	}
	h.instanceOf = func(id string) string {
		if id == "console" {
			return InstanceSystem
		}
		return InstanceUser
	}

	var subs []subscription
	for range 2 {
		sub, err := h.subscribe(feedClient{id: instanceID(InstanceSystem)}, InstanceSystem, nil, feedNow)
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}
	if _, err := h.subscribe(feedClient{id: instanceID(InstanceSystem)}, InstanceSystem, nil, feedNow); !errors.Is(err, errTooManyClientStreams) {
		t.Errorf("third stream of the instance: %v, want errTooManyClientStreams", err)
	}
	if got := cap(subs[0].stream.events); got != 8 {
		t.Errorf("queue of an instance stream = %d, want 8", got)
	}

	for i := range 5 {
		h.publish(record{clients: []string{"console"}, generation: uint64(i)})
		h.publish(record{clients: []string{"shop"}})
	}
	if got := len(subs[0].stream.events); got != 5 {
		t.Errorf("the system instance got %d records, want the 5 of its client", got)
	}
}

// The reviewer asks the API server for the audience of the feed and accepts only a token the API
// server authenticated for it.
func TestNewTokenReviewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       authenticationv1.TokenReviewStatus
		createErr    error
		expectedUser string
		expectedOK   bool
		expectsErr   bool
	}{
		{
			name: "token of the instance for the feed",
			status: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{TokenAudience},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:d8-user-authn:dex-authenticator"},
			},
			expectedUser: "system:serviceaccount:d8-user-authn:dex-authenticator",
			expectedOK:   true,
		},
		{
			name:   "token the API server does not know",
			status: authenticationv1.TokenReviewStatus{Authenticated: false},
		},
		{
			// An API server that ignores the audiences of the request answers with its own.
			name: "token for another audience",
			status: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{"https://kubernetes.default.svc"},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:d8-user-authn:dex-authenticator"},
			},
		},
		{name: "API server that fails", createErr: errors.New("the API server does not answer"), expectsErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var asked authenticationv1.TokenReviewSpec
			scheme := runtime.NewScheme()
			if err := authenticationv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					review, ok := obj.(*authenticationv1.TokenReview)
					if !ok {
						t.Fatalf("created %T", obj)
					}
					asked = review.Spec
					review.Status = tt.status
					return tt.createErr
				},
			}).Build()

			username, ok, err := NewTokenReviewer(c)(t.Context(), "the-token")

			if (err != nil) != tt.expectsErr {
				t.Fatalf("err = %v", err)
			}
			if username != tt.expectedUser || ok != tt.expectedOK {
				t.Errorf("review = %q, %v; want %q, %v", username, ok, tt.expectedUser, tt.expectedOK)
			}
			if asked.Token != "the-token" || !slices.Equal(asked.Audiences, []string{TokenAudience}) {
				t.Errorf("asked = %+v", asked)
			}
		})
	}
}
