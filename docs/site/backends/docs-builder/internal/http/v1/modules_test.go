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
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/flant/docs-builder/internal/ondemand"
)

func TestModuleAddressLeadsToTarget(t *testing.T) {
	h := newOnDemandHandler(t)
	h.onDemand.Store.SetTargets([]ondemand.Target{
		{Module: "echo", Version: "v1.0.0"},
		{Module: "branch", Version: "main", Rev: "sha256:aaa"},
	})

	for path, want := range map[string]string{
		"/en/modules/echo/":                    "/en/modules/echo/v1.0.0/",
		"/en/modules/echo/stable/":             "/en/modules/echo/v1.0.0/",
		"/ru/modules/echo/beta/faq.html":       "/ru/modules/echo/v1.0.0/faq.html",
		"/en/modules/echo/configuration.html":  "/en/modules/echo/v1.0.0/configuration.html",
		"/en/modules/echo/guides/install.html": "/en/modules/echo/v1.0.0/guides/install.html",
		"/en/modules/echo/guides/":             "/en/modules/echo/v1.0.0/guides/",
	} {
		rec := get(h, path)
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != want {
			t.Errorf("%s: code %d, location %q, want 302 to %q", path, rec.Code, rec.Header().Get("Location"), want)
		}
	}

	// No target, a name that is not a module, an unknown language.
	for _, path := range []string{"/en/modules/gone/", "/en/modules/Echo/", "/de/modules/echo/"} {
		if code := get(h, path).Code; code != http.StatusNotFound {
			t.Errorf("%s: code %d, want 404", path, code)
		}
	}
	// On the list with no target known here (a follower knows only the
	// published targets of the leader): a placeholder reloads itself.
	if err := h.onDemand.Store.WriteList(context.Background(), []ondemand.Listed{{Module: "later"}, {Module: "bare", NoDocs: true}, {Module: "broken", Failed: true}}, nil); err != nil {
		t.Fatal(err)
	}
	if rec := get(h, "/en/modules/later/"); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "once its version is published") {
		t.Errorf("later: code %d, want 503 with the placeholder", rec.Code)
	}
	// No documentation: says so and leads to the list. A 404 would get the
	// 404 page of the site from nginx instead.
	if rec := get(h, "/en/modules/bare/stable/"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "have no documentation") || !strings.Contains(rec.Body.String(), `href="/en/modules/"`) {
		t.Errorf("bare: code %d, want 200 with the no-documentation card:\n%s", rec.Code, rec.Body.String())
	}
	// A lasting failure: nothing to wait for.
	if rec := get(h, "/en/modules/broken/"); rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "have no documentation") {
		t.Errorf("broken: code %d, want the plain 404", rec.Code)
	}
	// A target whose version has no pages: the same card, not the version.
	h.onDemand.Store.SetTargets([]ondemand.Target{
		{Module: "echo", Version: "v1.0.0"},
		{Module: "branch", Version: "main", Rev: "sha256:aaa"},
		{Module: "bare", Version: "v0.1.0"},
	})
	if rec := get(h, "/en/modules/bare/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "have no documentation") {
		t.Errorf("bare with a target: code %d, location %q, want the no-documentation card", rec.Code, rec.Header().Get("Location"))
	}
	// A development branch has no version address: a placeholder reloads
	// itself until nginx serves the branch.
	if rec := get(h, "/en/modules/branch/"); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "branch main") {
		t.Errorf("branch: code %d, body without the branch:\n%s", rec.Code, rec.Body.String())
	}

	h.onDemand.Site = true
	if rec := get(h, "/en/modules/echo/stable/faq.html"); rec.Header().Get("Location") != "/modules/echo/v1.0.0/faq.html" {
		t.Errorf("site: location %q", rec.Header().Get("Location"))
	}
}

// A development branch that fails for a lasting reason tells it at the
// address of the module, until the branch moves on.
func TestModuleAddressOfFailedBranch(t *testing.T) {
	h := newOnDemandHandler(t)
	s := h.onDemand.Store
	branch := ondemand.Target{Module: "branch", Version: "broken", Rev: "sha256:aaa"}
	s.SetTargets([]ondemand.Target{branch})
	if err := s.RenderPinned(context.Background(), branch); err == nil {
		t.Fatal("the broken branch rendered")
	}

	rec := get(h, "/en/modules/branch/")
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), `id="failed" hidden`) || !strings.Contains(rec.Body.String(), "The pages could not be built.") {
		t.Fatalf("code %d, want 502 with the failed placeholder and the reason:\n%s", rec.Code, rec.Body.String())
	}

	branch.Rev = "sha256:bbb"
	s.SetTargets([]ondemand.Target{branch})
	if rec := get(h, "/en/modules/branch/"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a new revision: code %d, want 503 while it renders", rec.Code)
	}

	// A branch without pages: the not-found placeholder with 200, since
	// nginx replaces a 404 at the module address with the 404 page of the site.
	empty := ondemand.Target{Module: "branch", Version: "empty", Rev: "sha256:ccc"}
	s.SetTargets([]ondemand.Target{empty})
	if err := s.RenderPinned(context.Background(), empty); err == nil {
		t.Fatal("the branch without pages rendered")
	}
	if rec := get(h, "/en/modules/branch/"); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `id="not-found" hidden`) {
		t.Fatalf("no pages: code %d, want 200 with the not-found placeholder:\n%s", rec.Code, rec.Body.String())
	}
}

func TestModulesListPlaceholder(t *testing.T) {
	h := newOnDemandHandler(t)

	rec := get(h, "/ru/modules/")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("code %d, cache %q, want 503 not cached", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if body := rec.Body.String(); !strings.Contains(body, `http-equiv="refresh"`) || !strings.Contains(body, `lang="ru"`) {
		t.Fatalf("placeholder does not reload itself:\n%s", body)
	}
	if body := rec.Body.String(); !strings.Contains(body, `<!--#include virtual="/ru/includes/header.html" -->`) || !strings.Contains(body, "<dialog") {
		t.Fatalf("placeholder is not a dialog over the site:\n%s", body)
	}
	if code := get(h, "/de/modules/").Code; code != http.StatusNotFound {
		t.Fatalf("unknown language: code %d, want 404", code)
	}
}

func TestModuleMenu(t *testing.T) {
	h := newOnDemandHandler(t)
	s := h.onDemand.Store
	if err := s.WriteList(context.Background(), []ondemand.Listed{{Module: "echo", Enabled: true}, {Module: "bare"}}, nil); err != nil {
		t.Fatal(err)
	}
	// The menu shows the version the channel link points at: published.
	get(h, "/en/modules/echo/v1.0.0/")
	settle(t, h, "echo", "v1.0.0")
	s.SetChannels(map[string]ondemand.Channels{"echo": {"stable": "v1.0.0"}})

	var menu ondemand.ModuleMenu
	rec := get(h, "/api/v2/modules/echo/menu")
	if err := json.Unmarshal(rec.Body.Bytes(), &menu); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("code %d, err %v", rec.Code, err)
	}
	if want := (ondemand.ModuleMenu{Enabled: true, Channels: ondemand.Channels{"stable": "v1.0.0"}}); !reflect.DeepEqual(menu, want) {
		t.Fatalf("menu = %+v, want %+v", menu, want)
	}

	rec = get(h, "/api/v2/modules/bare/menu")
	menu = ondemand.ModuleMenu{}
	if err := json.Unmarshal(rec.Body.Bytes(), &menu); err != nil || !reflect.DeepEqual(menu, ondemand.ModuleMenu{Channels: ondemand.Channels{}}) {
		t.Fatalf("menu of a module with no channels = %+v, err %v", menu, err)
	}
	if code := get(h, "/api/v2/modules/gone/menu").Code; code != http.StatusNotFound {
		t.Fatalf("module not on the list: code %d, want 404", code)
	}
}
