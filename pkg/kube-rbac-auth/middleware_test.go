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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const validToken = "dh-test-secret-0123456789abcdef"

var metricsAttrs = ResourceAttributes{
	Namespace:   "d8-example",
	APIGroup:    "apps",
	APIVersion:  "v1",
	Resource:    "deployments",
	Subresource: "prometheus-metrics",
	Name:        "example",
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) step(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeAPI answers TokenReview and SubjectAccessReview the way a test sets, and counts the calls.
type fakeAPI struct {
	mu       sync.Mutex
	down     bool
	err      error
	allowed  bool
	tokenOK  bool
	tokenN   int
	sarN     int
	lastSpec authorizationv1.SubjectAccessReviewSpec
}

func (f *fakeAPI) set(fn func(*fakeAPI)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeAPI) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenN, f.sarN
}

func (f *fakeAPI) client() *fake.Clientset {
	c := fake.NewClientset()
	c.PrependReactor("create", "tokenreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.tokenN++
		if f.err != nil {
			return true, nil, f.err
		}
		if f.down {
			return true, nil, errors.New("connection refused")
		}
		tr := a.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview).DeepCopy()
		tr.Status.Authenticated = f.tokenOK && tr.Spec.Token == validToken
		if tr.Status.Authenticated {
			tr.Status.User = authenticationv1.UserInfo{
				Username: "system:serviceaccount:d8-monitoring:prometheus",
				UID:      "uid-1",
				Groups:   []string{"system:serviceaccounts"},
			}
		}
		return true, tr, nil
	})
	c.PrependReactor("create", "subjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.sarN++
		if f.down {
			return true, nil, errors.New("connection refused")
		}
		sar := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview).DeepCopy()
		f.lastSpec = sar.Spec
		sar.Status.Allowed = f.allowed
		return true, sar, nil
	})
	return c
}

type env struct {
	api     *fakeAPI
	clock   *fakeClock
	handler http.Handler
}

func newEnv(t *testing.T, cfg Config) *env {
	t.Helper()
	api := &fakeAPI{allowed: true, tokenOK: true}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if cfg.Rules == nil {
		cfg.Rules = []Rule{{Path: "/metrics", ResourceAttributes: metricsAttrs}}
	}
	if cfg.RetryBackoff == nil {
		cfg.RetryBackoff = &wait.Backoff{Duration: time.Millisecond, Factor: 1, Steps: 3}
	}
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	m, err := newWithClock(api.client(), cfg, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok "+r.URL.Path)
	})
	return &env{api: api, clock: clock, handler: m.Handler(next)}
}

func (e *env) do(t *testing.T, method, path, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func expectCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d, body %q", rec.Code, want, rec.Body.String())
	}
}

func expectCalls(t *testing.T, api *fakeAPI, wantToken, wantSAR int) {
	t.Helper()
	if tok, sar := api.counts(); tok != wantToken || sar != wantSAR {
		t.Fatalf("calls: TokenReview %d, SubjectAccessReview %d; want %d, %d", tok, sar, wantToken, wantSAR)
	}
}

func TestNoTokenIsUnauthorized(t *testing.T) {
	e := newEnv(t, Config{})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", ""), http.StatusUnauthorized)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Basic dXNlcm5hbWU6cGFzc3dvcmQ="), http.StatusUnauthorized)
	expectCalls(t, e.api, 0, 0)
}

func TestInvalidTokenIsUnauthorizedAndCached(t *testing.T) {
	e := newEnv(t, Config{})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer wrong"), http.StatusUnauthorized)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer wrong"), http.StatusUnauthorized)
	expectCalls(t, e.api, 1, 0)

	e.clock.step(DefaultDenyCacheTTL + time.Second)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer wrong"), http.StatusUnauthorized)
	expectCalls(t, e.api, 2, 0)
}

func TestAllowedRequestReachesHandler(t *testing.T) {
	e := newEnv(t, Config{})
	rec := e.do(t, http.MethodGet, "/metrics", "bearer "+validToken)
	expectCode(t, rec, http.StatusOK)
	if rec.Body.String() != "ok /metrics" {
		t.Fatalf("body = %q", rec.Body.String())
	}

	spec := e.api.lastSpec
	if spec.User != "system:serviceaccount:d8-monitoring:prometheus" || spec.UID != "uid-1" {
		t.Fatalf("SubjectAccessReview user = %q uid = %q", spec.User, spec.UID)
	}
	want := authorizationv1.ResourceAttributes{
		Namespace: "d8-example", Verb: "get", Group: "apps", Version: "v1",
		Resource: "deployments", Subresource: "prometheus-metrics", Name: "example",
	}
	if *spec.ResourceAttributes != want {
		t.Fatalf("resource attributes = %+v, want %+v", *spec.ResourceAttributes, want)
	}
}

func TestDecisionsAreCached(t *testing.T) {
	e := newEnv(t, Config{})
	for range 3 {
		expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
	}
	expectCalls(t, e.api, 1, 1)

	e.clock.step(DefaultTokenCacheTTL + time.Second)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
	expectCalls(t, e.api, 2, 1)

	e.clock.step(DefaultAllowCacheTTL)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
	expectCalls(t, e.api, 3, 2)
}

func TestDeniedIsForbiddenUntilDenyCacheExpires(t *testing.T) {
	e := newEnv(t, Config{})
	e.api.set(func(f *fakeAPI) { f.allowed = false })
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusForbidden)

	e.api.set(func(f *fakeAPI) { f.allowed = true })
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusForbidden)

	e.clock.step(DefaultDenyCacheTTL + time.Second)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
}

func TestAPIDownWithoutCacheIsError(t *testing.T) {
	e := newEnv(t, Config{})
	e.api.set(func(f *fakeAPI) { f.down = true })
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusInternalServerError)
	// Every step of the backoff is tried before the request fails.
	expectCalls(t, e.api, 3, 0)
}

func TestStaleCacheServesWhileAPIIsDown(t *testing.T) {
	e := newEnv(t, Config{StaleCacheTTL: time.Hour})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)

	e.api.set(func(f *fakeAPI) { f.down = true })
	e.clock.step(30 * time.Minute)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)

	e.clock.step(time.Hour)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusInternalServerError)
}

func TestStaleCacheCanBeDisabled(t *testing.T) {
	e := newEnv(t, Config{StaleCacheTTL: -1})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)

	e.api.set(func(f *fakeAPI) { f.down = true })
	e.clock.step(DefaultAllowCacheTTL + time.Second)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusInternalServerError)
}

func TestExcludedAndUnknownPaths(t *testing.T) {
	e := newEnv(t, Config{ExcludePaths: []string{"/healthz", "/ready*"}})
	expectCode(t, e.do(t, http.MethodGet, "/healthz", ""), http.StatusOK)
	expectCode(t, e.do(t, http.MethodGet, "/readyz", ""), http.StatusOK)
	expectCode(t, e.do(t, http.MethodGet, "/other", "Bearer "+validToken), http.StatusNotFound)
	expectCode(t, e.do(t, http.MethodGet, "/metrics/extra", "Bearer "+validToken), http.StatusNotFound)
	expectCalls(t, e.api, 0, 0)
}

func TestRulesSelectResourceByPath(t *testing.T) {
	other := metricsAttrs
	other.Name = "other"
	e := newEnv(t, Config{Rules: []Rule{
		{Path: "/", ResourceAttributes: metricsAttrs},
		{Path: "/other/", ResourceAttributes: other},
	}})
	expectCode(t, e.do(t, http.MethodGet, "/other/metrics", "Bearer "+validToken), http.StatusOK)
	if got := e.api.lastSpec.ResourceAttributes.Name; got != "other" {
		t.Fatalf("name = %q, want other", got)
	}
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
	if got := e.api.lastSpec.ResourceAttributes.Name; got != "example" {
		t.Fatalf("name = %q, want example", got)
	}
}

func TestVerbs(t *testing.T) {
	for method, verb := range map[string]string{
		http.MethodGet: "get", http.MethodHead: "get", http.MethodPost: "create",
		http.MethodPut: "update", http.MethodPatch: "patch", http.MethodDelete: "delete", http.MethodOptions: "*",
	} {
		e := newEnv(t, Config{})
		expectCode(t, e.do(t, method, "/metrics", "Bearer "+validToken), http.StatusOK)
		if got := e.api.lastSpec.ResourceAttributes.Verb; got != verb {
			t.Fatalf("%s: verb = %q, want %q", method, got, verb)
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no rules":       {},
		"relative path":  {Rules: []Rule{{Path: "metrics", ResourceAttributes: metricsAttrs}}},
		"duplicate path": {Rules: []Rule{{Path: "/m", ResourceAttributes: metricsAttrs}, {Path: "/m", ResourceAttributes: metricsAttrs}}},
		"no resource":    {Rules: []Rule{{Path: "/m"}}},
		"bad exclude":    {Rules: []Rule{{Path: "/m", ResourceAttributes: metricsAttrs}}, ExcludePaths: []string{"["}},
	} {
		if _, err := New(fake.NewClientset(), cfg); err == nil || !strings.Contains(err.Error(), "invalid config") {
			t.Fatalf("%s: err = %v, want invalid config", name, err)
		}
	}
	if _, err := New(nil, Config{Rules: []Rule{{Path: "/m", ResourceAttributes: metricsAttrs}}}); err == nil {
		t.Fatal("nil client: want an error")
	}
}
