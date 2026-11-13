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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// An API server that hangs instead of refusing connections must not outlast the scrape:
// the stale decision has to be served before the client gives up.
func TestHangingAPIServesStaleWithinBudget(t *testing.T) {
	var hanging atomic.Bool
	release := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hanging.Load() {
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/tokenreviews"):
			_ = json.NewEncoder(w).Encode(authenticationv1.TokenReview{Status: authenticationv1.TokenReviewStatus{
				Authenticated: true, User: authenticationv1.UserInfo{Username: "prometheus"}}})
		case strings.HasSuffix(r.URL.Path, "/subjectaccessreviews"):
			_ = json.NewEncoder(w).Encode(authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}})
		default:
			http.NotFound(w, r)
		}
	}))
	// Release the hanging handlers before Close waits for them.
	defer api.Close()
	defer close(release)

	client, err := kubernetes.NewForConfig(&rest.Config{Host: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	clock := &fakeClock{now: time.Now()}
	m, err := newWithClock(client, Config{
		Rules:  []Rule{{Path: "/metrics", ResourceAttributes: metricsAttrs}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, clock)
	if err != nil {
		t.Fatal(err)
	}
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))

	serve := func() (int, time.Duration) {
		// Prometheus scrapes with a 10 second timeout by default.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()
		start := time.Now()
		h.ServeHTTP(rec, req)
		return rec.Code, time.Since(start)
	}
	if code, _ := serve(); code != http.StatusOK {
		t.Fatalf("first request: status %d", code)
	}

	hanging.Store(true)
	clock.step(DefaultAllowCacheTTL + time.Second)
	code, took := serve()
	if code != http.StatusOK || took >= 10*time.Second {
		t.Fatalf("hanging API server: status %d after %s, want 200 within the scrape timeout", code, took)
	}
}

// A request the API server rejects for good, such as a missing right to create TokenReviews,
// fails at once instead of retrying.
func TestPermanentAPIErrorIsNotRetried(t *testing.T) {
	e := newEnv(t, Config{})
	e.api.set(func(f *fakeAPI) {
		f.err = apierrors.NewForbidden(schema.GroupResource{Group: "authentication.k8s.io", Resource: "tokenreviews"}, "", nil)
	})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusInternalServerError)
	expectCalls(t, e.api, 1, 0)
}

// An excluded pattern must not open a protected path through path cleaning.
func TestExcludedPatternDoesNotBypassThroughDotSegments(t *testing.T) {
	e := newEnv(t, Config{ExcludePaths: []string{"/healthz*"}})
	for _, p := range []string{"/healthz/../metrics", "/healthz/%2e%2e/metrics", "//metrics", "/./metrics"} {
		rec := e.do(t, http.MethodGet, p, "")
		if rec.Code == http.StatusOK {
			t.Fatalf("%s: reached the handler without a token: %q", p, rec.Body.String())
		}
	}
}
