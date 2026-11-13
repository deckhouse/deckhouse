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

package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/versions"
)

// render publishes "v1.*" versions, reports "v2.*" as missing and fails
// "v3.*" and the branch "broken"; the branch "empty" has no pages. "v4.*"
// renders wait for block when it is set.
func render(block chan struct{}) ondemand.RenderFunc {
	return func(_ context.Context, _, version, _, outDir string, _ bool, _ func(ondemand.Stage)) error {
		if block != nil && strings.HasPrefix(version, "v4.") {
			<-block
		}

		return renderVersion(version, outDir)
	}
}

func renderVersion(version, outDir string) error {
	switch {
	case strings.HasPrefix(version, "v2."):
		return fmt.Errorf("source deckhouse: %w", ondemand.ErrVersionNotFound)
	case strings.HasPrefix(version, "v3."):
		return fmt.Errorf("hugo build: template error")
	case version == "broken":
		return fmt.Errorf("hugo build: %w", docs.ErrBuild)
	case version == "empty":
		return fmt.Errorf("hugo build: %w", docs.ErrNoPages)
	}

	if err := os.MkdirAll(filepath.Join(outDir, "en"), 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "en", "index.html"), []byte("page"), 0600)
}

func newOnDemandHandler(t *testing.T) *DocsBuilderHandler {
	t.Helper()

	return newOnDemandHandlerBlocking(t, nil)
}

func newOnDemandHandlerBlocking(t *testing.T, block chan struct{}) *DocsBuilderHandler {
	t.Helper()

	h := NewHandler(log.NewNop())
	h.EnableOnDemand(OnDemand{
		Store: ondemand.NewStore(ondemand.Config{
			Dir:                t.TempDir(),
			MaxSize:            1 << 20,
			MaxIdle:            time.Hour,
			MaxRenders:         2,
			MaxQueue:           3,
			RenderTimeout:      time.Minute,
			RetryAfter:         time.Hour,
			NotFoundRetryAfter: time.Hour,
			Render:             render(block),
			Logger:             log.NewNop(),
		}),
		Versions: versions.New(func(_ context.Context, module string) ([]string, error) {
			if module == "cert-manager" {
				return nil, fmt.Errorf("module %s: %w", module, ondemand.ErrNoVersions)
			}
			return []string{"stable", "v1.0.0", "v1.1.0", "v2.0.0", "v3.0.0", "v4.0.0", "v4.1.0", "v4.2.0", "v4.3.0"}, nil
		}, time.Minute, time.Minute, nil),
	})

	return h
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// settle waits until the render of the version is finished.
func settle(t *testing.T, h http.Handler, module, version string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var st ondemand.Status
		if err := json.Unmarshal(get(h, "/api/v2/render/"+module+"/"+version).Body.Bytes(), &st); err != nil {
			t.Fatal(err)
		}
		if st.Stage == ondemand.StageDone || st.Stage == ondemand.StageFailed {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s %s did not finish rendering", module, version)
}

func TestVersionPage(t *testing.T) {
	h := newOnDemandHandler(t)

	first := get(h, "/en/modules/echo/v1.0.0/install.html")
	if first.Code != http.StatusAccepted {
		t.Fatalf("first request: code %d, want 202", first.Code)
	}
	if first.Header().Get("Cache-Control") != "no-store" || !strings.Contains(first.Body.String(), `http-equiv="refresh"`) {
		t.Fatalf("placeholder does not reload itself or may be cached: %q", first.Body.String())
	}

	settle(t, h, "echo", "v1.0.0")

	// Once the version is on the volume, nginx serves its pages; a request
	// that still reaches docs-builder asks for a page the version lacks.
	if code := get(h, "/en/modules/echo/v1.0.0/missing.html").Code; code != http.StatusNotFound {
		t.Fatalf("missing page of a rendered version: code %d, want 404", code)
	}
}

func TestVersionPageFailures(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		render bool // the request starts a render; check the page after it
		want   int
	}{
		{"version does not exist", "/en/modules/echo/v2.0.0/", true, http.StatusNotFound},
		{"render broke", "/en/modules/echo/v3.0.0/", true, http.StatusBadGateway},
		{"channel instead of a version", "/en/modules/echo/stable/", false, http.StatusNotFound},
		{"version is not published", "/en/modules/echo/v9.9.9/", false, http.StatusNotFound},
		{"module has no registry source", "/en/modules/cert-manager/v1.0.0/", false, http.StatusNotFound},
		{"unknown language", "/de/modules/echo/v1.0.0/", false, http.StatusNotFound},
		{"invalid module name", "/en/modules/Echo/v1.0.0/", false, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newOnDemandHandler(t)

			if tt.render {
				get(h, tt.path)
				parts := strings.Split(tt.path, "/")
				settle(t, h, parts[3], parts[4])
			}

			rec := get(h, tt.path)
			if rec.Code != tt.want {
				t.Fatalf("code %d, want %d", rec.Code, tt.want)
			}
			// A broken render tells when it may run again; a missing
			// version has nothing to wait for.
			if retry := rec.Header().Get("Retry-After"); (rec.Code == http.StatusBadGateway) != (retry != "") {
				t.Fatalf("code %d with Retry-After %q", rec.Code, retry)
			}
		})
	}
}

func TestVersionPageNotPublishedIsNotRendered(t *testing.T) {
	h := newOnDemandHandler(t)

	get(h, "/en/modules/echo/v9.9.9/")

	var st ondemand.Status
	if err := json.Unmarshal(get(h, "/api/v2/render/echo/v9.9.9").Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Stage != "" {
		t.Fatalf("a version that is not published started rendering: %+v", st)
	}
}

func TestVersionPageCanceledRequestStartsNothing(t *testing.T) {
	h := newOnDemandHandler(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/en/modules/echo/v1.0.0/", nil).WithContext(ctx))

	var st ondemand.Status
	if err := json.Unmarshal(get(h, "/api/v2/render/echo/v1.0.0").Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Stage != "" {
		t.Fatalf("a request canceled before the version check started a render: %+v", st)
	}
}

func TestVersionPageBusy(t *testing.T) {
	block := make(chan struct{})
	h := newOnDemandHandlerBlocking(t, block)
	defer func() {
		close(block)
		for _, v := range []string{"v4.0.0", "v4.1.0", "v4.2.0"} {
			settle(t, h, "echo", v)
		}
	}()

	for _, v := range []string{"v4.0.0", "v4.1.0", "v4.2.0"} {
		if code := get(h, "/en/modules/echo/"+v+"/").Code; code != http.StatusAccepted {
			t.Fatalf("%s: code %d, want 202", v, code)
		}
	}

	rec := get(h, "/en/modules/echo/v4.3.0/")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("queue full: code %d, Retry-After %q; want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestRenderStatus(t *testing.T) {
	h := newOnDemandHandler(t)

	if code := get(h, "/api/v2/render/echo/stable").Code; code != http.StatusBadRequest {
		t.Fatalf("invalid version: code %d, want 400", code)
	}

	get(h, "/en/modules/echo/v2.0.0/")
	settle(t, h, "echo", "v2.0.0")

	var st ondemand.Status
	if err := json.Unmarshal(get(h, "/api/v2/render/echo/v2.0.0").Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Stage != ondemand.StageFailed || !st.NotFound {
		t.Fatalf("status = %+v, want failed and not found", st)
	}
}

func TestModuleVersions(t *testing.T) {
	h := newOnDemandHandler(t)

	rec := get(h, "/api/v2/modules/echo/versions")
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}

	var got struct {
		Module   string   `json:"module"`
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Module != "echo" || strings.Join(got.Versions, ",") != "v4.3.0,v4.2.0,v4.1.0,v4.0.0,v3.0.0,v2.0.0,v1.1.0,v1.0.0" {
		t.Fatalf("got %+v", got)
	}

	if code := get(h, "/api/v2/modules/Echo/versions").Code; code != http.StatusBadRequest {
		t.Fatalf("invalid module: code %d, want 400", code)
	}

	if code := get(h, "/api/v2/modules/cert-manager/versions").Code; code != http.StatusNotFound {
		t.Fatalf("module without a registry source: code %d, want 404", code)
	}

	// On the public site a module embedded into Deckhouse gets the minor
	// versions that kept its documentation.
	history := func(body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "embedded_modules_list.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	h.onDemand.Embedded = ondemand.NewEmbeddedVersions(history(`{"latest":["cert-manager"],"v1.9":["cert-manager","echo"],"v1.10":["cert-manager"],"v1.8":["dashboard"]}`))
	rec = get(h, "/api/v2/modules/cert-manager/versions")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("embedded: code %d, err %v", rec.Code, err)
	}
	if strings.Join(got.Versions, ",") != "v1.10,v1.9" {
		t.Fatalf("embedded versions = %v, want v1.10,v1.9", got.Versions)
	}
	// A module of the registry gets its versions there, embedded or not.
	rec = get(h, "/api/v2/modules/echo/versions")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("embedded and in the registry: code %d, err %v", rec.Code, err)
	}
	if got.Versions[0] != "v4.3.0" {
		t.Fatalf("embedded and in the registry = %v, want the registry versions", got.Versions)
	}

	// A history that cannot be read is an error, the dialog says so; a
	// module without versions would be "no versions match".
	h.onDemand.Embedded = ondemand.NewEmbeddedVersions(history(`{"v1.8":["dashboard"]}`))
	if code := get(h, "/api/v2/modules/cert-manager/versions").Code; code != http.StatusNotFound {
		t.Fatalf("a module neither in the registry nor embedded: code %d, want 404", code)
	}
	for name, path := range map[string]string{
		"corrupt": history(`{"v1.9":`),
		"missing": filepath.Join(t.TempDir(), "absent.json"),
	} {
		h.onDemand.Embedded = ondemand.NewEmbeddedVersions(path)
		if code := get(h, "/api/v2/modules/cert-manager/versions").Code; code != http.StatusBadGateway {
			t.Fatalf("%s history: code %d, want 502", name, code)
		}
	}
}

func TestModuleVersionsIncomplete(t *testing.T) {
	var fail atomic.Bool
	h := newOnDemandHandler(t)
	// No TTL: every request lists the registry again.
	h.onDemand.Versions = versions.New(func(context.Context, string) ([]string, error) {
		if fail.Load() {
			return nil, errors.New("registry is down")
		}
		return []string{"v1.0.0"}, nil
	}, 0, time.Minute, nil)

	list := func() (string, bool) {
		t.Helper()
		rec := get(h, "/api/v2/modules/echo/versions")
		var got struct {
			Versions   []string `json:"versions"`
			Incomplete bool     `json:"incomplete"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("code %d, %v: %s", rec.Code, err, rec.Body)
		}
		return rec.Body.String(), got.Incomplete && len(got.Versions) == 1
	}

	if body, incomplete := list(); incomplete || strings.Contains(body, "incomplete") {
		t.Fatalf("a full list is served as %s; want no incomplete field", body)
	}

	// The registry stops answering: the last list is served, marked.
	fail.Store(true)
	deadline := time.Now().Add(5 * time.Second)
	for {
		body, incomplete := list()
		if incomplete {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the list kept after a failed listing is served as %s; want incomplete", body)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestOnDemandDisabled(t *testing.T) {
	h := NewHandler(log.NewNop())

	if code := get(h, "/en/modules/echo/v1.0.0/").Code; code != http.StatusNotFound {
		t.Fatalf("code %d, want 404 without on-demand rendering", code)
	}
}

func TestVersionsPage(t *testing.T) {
	h := newOnDemandHandler(t)

	get(h, "/en/modules/echo/v1.1.0/")
	settle(t, h, "echo", "v1.1.0")

	ru := get(h, "/ru/modules/echo/versions/")
	if ru.Code != http.StatusOK {
		t.Fatalf("code %d", ru.Code)
	}
	body := ru.Body.String()
	vt := versionsTextsFor("ru", "echo")
	for _, want := range []string{
		`<li><a href="/ru/modules/echo/v1.1.0/" rel="nofollow">v1.1.0</a><span class="ready">` + vt.Ready + `</span></li>`,
		`<li><a href="/ru/modules/echo/v1.0.0/" rel="nofollow">v1.0.0</a></li>`,
		`<a class="button button_alt" href="/ru/modules/echo/">` + vt.Current + `</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, "stable") {
		t.Error("release channels are listed as versions")
	}

	noSource := get(h, "/en/modules/cert-manager/versions/")
	if noSource.Code != http.StatusNotFound || !strings.Contains(noSource.Body.String(), "is not published in a registry") {
		t.Fatalf("module without a source: code %d", noSource.Code)
	}

	if code := get(h, "/de/modules/echo/versions/").Code; code != http.StatusNotFound {
		t.Fatalf("unknown language: code %d, want 404", code)
	}
}

func TestVersionPagePublishedOnTheWay(t *testing.T) {
	h := newOnDemandHandler(t)

	get(h, "/en/modules/echo/v1.0.0/")
	settle(t, h, "echo", "v1.0.0")

	// nginx missed the page a moment before the version was published.
	rec := get(h, "/en/modules/echo/v1.0.0/")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "location.reload()") {
		t.Fatalf("code %d; want 202 and a page that reloads itself", rec.Code)
	}

	if code := get(h, "/en/modules/echo/v1.0.0/missing.html").Code; code != http.StatusNotFound {
		t.Fatalf("missing page: code %d, want 404", code)
	}
}

func TestReadyWaitsForOnDemand(t *testing.T) {
	var ready atomic.Bool
	h := NewHandler(log.NewNop())
	h.EnableOnDemand(OnDemand{Ready: ready.Load})

	if rec := get(h, "/readyz"); rec.Code == http.StatusOK {
		t.Fatal("ready before the versions came from S3")
	}
	ready.Store(true)
	if rec := get(h, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("readyz = %d once the versions came from S3", rec.Code)
	}
}

func TestHealthFailsWhileNotLive(t *testing.T) {
	var stuck atomic.Bool
	h := NewHandler(log.NewNop())
	h.EnableOnDemand(OnDemand{Live: func() error {
		if stuck.Load() {
			return errors.New("a pass of the pinned versions is stuck")
		}
		return nil
	}})

	if rec := get(h, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d while live", rec.Code)
	}
	stuck.Store(true)
	if rec := get(h, "/healthz"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("healthz = %d with a pass stuck, want 500", rec.Code)
	}
}

func TestSiteLinks(t *testing.T) {
	h := newOnDemandHandler(t)
	h.onDemand.Site = true

	body := get(h, "/ru/modules/echo/versions/").Body.String()
	for _, want := range []string{`<a href="/modules/echo/v1.0.0/" rel="nofollow">`, `<a class="button button_alt" href="/modules/echo/">`, `<!--#include virtual="/includes/header.html" -->`, "<dialog"} {
		if !strings.Contains(body, want) {
			t.Errorf("versions page lacks %q", want)
		}
	}

	page := get(h, "/en/modules/echo/v1.0.0/").Body.String()
	settle(t, h, "echo", "v1.0.0")
	if !strings.Contains(page, `"/modules/.render/echo/v1.0.0"`) || strings.Contains(page, "/en/modules/") {
		t.Errorf("placeholder: the status or links are not those of the site:\n%s", page)
	}
}
