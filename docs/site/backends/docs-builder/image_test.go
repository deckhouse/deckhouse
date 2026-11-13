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

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The image of the public site needs CA certificates: without them
// docs-builder can not read the module registries over TLS, and the site has
// no list of modules. scratch has none, base/distroless has them.
func TestSiteImageHasCACertificates(t *testing.T) {
	raw, err := os.ReadFile("../../werf-docs-builder.inc.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// The final image is the first document, up to its imageSpec.
	final, _, ok := strings.Cut(string(raw), "imageSpec:")
	if !ok || !strings.Contains(final, "from: common/distroless") {
		t.Fatal("the final image of docs-builder is not where the test looks for it")
	}
	if strings.Contains(final, "from: scratch") || !strings.Contains(final, `from: {{ index $.Images "base/distroless" }}`) {
		t.Fatal("the final image of the site is not built from base/distroless")
	}
}

// Only an image built for a merge request or a branch of a cluster module
// carries pprof: a release (a tag) and the public site never do.
func TestPprofOnlyInDevelopmentImages(t *testing.T) {
	raw, err := os.ReadFile("../../werf-docs-builder.inc.yaml")
	if err != nil {
		t.Fatal(err)
	}
	build := `- go build {{ if and (ne .ModuleName "website-docs") (eq (env "CI_COMMIT_TAG" "") "") }}-tags pprof {{ end }}-ldflags`
	if !strings.Contains(string(raw), build) {
		t.Fatalf("no build line %s", build)
	}
	if n := strings.Count(string(raw), "-tags pprof"); n != 1 {
		t.Fatalf("-tags pprof appears %d times, want once", n)
	}
}

// siteLocations are the locations of the nginx of the public site, each
// from its match up to the next location.
func siteLocations(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile("../../.helm/templates/10-cm-moduleslibrary.yaml")
	if err != nil {
		t.Fatal(err)
	}

	return regexp.MustCompile(`\n\s*location `).Split(string(raw), -1)[1:]
}

// siteLocation is the location of the nginx of the public site whose match
// starts with prefix.
func siteLocation(t *testing.T, prefix string) string {
	t.Helper()

	for _, location := range siteLocations(t) {
		if strings.HasPrefix(location, prefix) {
			return location
		}
	}
	t.Fatalf("no location %s", prefix)

	return ""
}

// An if of nginx that is true without returning drops try_files from its
// location ("if is evil"): a version of a module not rendered yet would get
// the 404 of nginx instead of docs-builder, which renders it.
func TestSiteNginxKeepsTryFiles(t *testing.T) {
	ifBlock := regexp.MustCompile(`if \([^)]*\) \{([^}]*)\}`)
	for _, location := range siteLocations(t) {
		if !strings.Contains(location, "try_files") {
			continue
		}
		for _, m := range ifBlock.FindAllStringSubmatch(location, -1) {
			if !strings.Contains(m[1], "return ") {
				t.Errorf("an if without return in a location with try_files:\n%s", strings.SplitN(location, "\n", 2)[0])
			}
		}
	}
}

func TestSiteNginxServesTheOnDemandPages(t *testing.T) {
	// The list carries SSI includes: a gzip copy would keep them unexpanded.
	if list := siteLocation(t, "~* ^/modules/$ "); strings.Contains(list, "gzip_static") {
		t.Error("the list of modules is served from a gzip copy")
	}

	// A module address the bucket does not have goes to docs-builder, and
	// the 404 of docs-builder to the 404 page of the site.
	if module := siteLocation(t, "~* ^/modules/([^./]+)/"); !strings.Contains(module, "error_page 403 404 = @ondemand_module;") {
		t.Error("the module address does not fall back to docs-builder")
	}
	if fallback := siteLocation(t, "@ondemand_module "); !strings.Contains(fallback, "recursive_error_pages on;") {
		t.Error("the 404 of docs-builder for a module address does not get the 404 page of the site")
	}

	for prefix, upstream := range map[string]string{
		`~ ^/modules/\.menu/`:     "proxy_pass http://127.0.0.1:8081/api/v2/modules/$menu_module/menu;",
		`~ ^/modules/\.versions/`: "proxy_pass http://127.0.0.1:8081/api/v2/modules/$versions_module/versions;",
		`~ ^/modules/\.render/`:   "proxy_pass http://127.0.0.1:8081/api/v2/render/$render_module/$render_version;",
	} {
		if location := siteLocation(t, prefix); !strings.Contains(location, upstream) {
			t.Errorf("%s does not pass to %s", prefix, upstream)
		}
	}
}
