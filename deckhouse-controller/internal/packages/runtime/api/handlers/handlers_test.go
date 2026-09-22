// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api/handlers"
	v1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api/handlers/v1"
)

// fakeProvider serves canned state to every domain of the tree under test.
type fakeProvider struct {
	renderErr error
}

func (fakeProvider) Dump() any { return map[string]any{"apps": map[string]any{}} }

func (fakeProvider) DumpByName(name string) any {
	if name != "known" {
		return nil
	}

	return map[string]any{"name": name}
}

func (fakeProvider) DumpGlobal() any { return map[string]any{"name": "global"} }

func (f fakeProvider) Render(_ context.Context, _ string) (string, error) {
	return "kind: Deployment\n", f.renderErr
}

func (fakeProvider) Snapshots(name string) (any, bool) {
	return map[string]any{"hook": []string{"object"}}, name == "known"
}

func (fakeProvider) DumpQueues(string) any { return map[string]any{"queues": map[string]any{}} }

func newDeps(provider fakeProvider) v1.Deps {
	return v1.Deps{
		Packages:     provider,
		Queues:       provider,
		Scheduler:    provider,
		Requirements: func() map[string]any { return map[string]any{"deckhouseVersion": "dev"} },
	}
}

func get(t *testing.T, router http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))

	return recorder
}

// TestPublicRouterHidesPackages pins the split the transports rely on: the
// packages subtree carries package values and Secret contents, so it must be
// absent from the tree the TCP listener serves.
func TestPublicRouterHidesPackages(t *testing.T) {
	tests := []struct {
		give string
		want int
	}{
		{give: "/api/v1/packages/dump", want: http.StatusNotFound},
		{give: "/api/v1/packages/global/dump", want: http.StatusNotFound},
		{give: "/api/v1/packages/render/known", want: http.StatusNotFound},
		{give: "/api/v1/packages/snapshots/known", want: http.StatusNotFound},
		{give: "/api/v1/queues/dump", want: http.StatusOK},
		{give: "/api/v1/scheduler/dump", want: http.StatusOK},
		{give: "/api/v1/requirements/dump", want: http.StatusOK},
		{give: "/healthz", want: http.StatusOK},
		{give: "/endpoints", want: http.StatusOK},
	}

	router := handlers.NewRootHandler(v1.NewPublicHandler(newDeps(fakeProvider{})))

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			assert.Equal(t, tt.want, get(t, router, tt.give).Code)
		})
	}
}

// TestPrivateRouterServesPackages is the other half: the socket tree carries
// everything the public one has plus the packages subtree.
func TestPrivateRouterServesPackages(t *testing.T) {
	tests := []struct {
		give        string
		want        int
		wantType    string
		wantPayload string
	}{
		{give: "/api/v1/packages/dump", want: http.StatusOK, wantType: "application/json", wantPayload: `{"apps":{}}` + "\n"},
		{give: "/api/v1/packages/dump?name=known", want: http.StatusOK, wantType: "application/json", wantPayload: `{"name":"known"}` + "\n"},
		{give: "/api/v1/packages/global/dump", want: http.StatusOK, wantType: "application/json", wantPayload: `{"name":"global"}` + "\n"},
		{give: "/api/v1/packages/render/known", want: http.StatusOK, wantType: "application/yaml", wantPayload: "kind: Deployment\n"},
		{give: "/api/v1/packages/snapshots/known", want: http.StatusOK, wantType: "application/json", wantPayload: `{"hook":["object"]}` + "\n"},
		{give: "/api/v1/packages/snapshots/missing", want: http.StatusNotFound, wantType: "text/plain; charset=utf-8", wantPayload: "package not found\n"},
	}

	router := handlers.NewRootHandler(v1.NewPrivateHandler(newDeps(fakeProvider{})))

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			recorder := get(t, router, tt.give)

			require.Equal(t, tt.want, recorder.Code)
			assert.Equal(t, tt.wantType, recorder.Header().Get("Content-Type"))
			assert.Equal(t, tt.wantPayload, recorder.Body.String())
		})
	}
}

// TestOutputFormat covers the output parameter: JSON by default, YAML on
// request, and a refusal instead of a silent default on anything else.
func TestOutputFormat(t *testing.T) {
	tests := []struct {
		give     string
		want     int
		wantType string
	}{
		{give: "/api/v1/requirements/dump", want: http.StatusOK, wantType: "application/json"},
		{give: "/api/v1/requirements/dump?output=json", want: http.StatusOK, wantType: "application/json"},
		{give: "/api/v1/requirements/dump?output=yaml", want: http.StatusOK, wantType: "application/yaml"},
		{give: "/api/v1/requirements/dump?output=xml", want: http.StatusBadRequest, wantType: "text/plain; charset=utf-8"},
	}

	router := handlers.NewRootHandler(v1.NewPublicHandler(newDeps(fakeProvider{})))

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			recorder := get(t, router, tt.give)

			require.Equal(t, tt.want, recorder.Code)
			assert.Equal(t, tt.wantType, recorder.Header().Get("Content-Type"))
		})
	}
}

// TestRenderErrors keeps the render endpoint's error mapping: a package without
// a chart is the caller's mistake, anything else is ours.
func TestRenderErrors(t *testing.T) {
	tests := []struct {
		give        string
		giveErr     error
		want        int
		wantPayload string
	}{
		{give: "not a helm package", giveErr: nelm.ErrPackageNotHelm, want: http.StatusBadRequest, wantPayload: "package has no Helm chart\n"},
		{give: "render failed", giveErr: assert.AnError, want: http.StatusInternalServerError, wantPayload: "render failed: " + assert.AnError.Error() + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			router := handlers.NewRootHandler(v1.NewPrivateHandler(newDeps(fakeProvider{renderErr: tt.giveErr})))

			recorder := get(t, router, "/api/v1/packages/render/known")

			require.Equal(t, tt.want, recorder.Code)
			assert.Equal(t, tt.wantPayload, recorder.Body.String())
		})
	}
}
