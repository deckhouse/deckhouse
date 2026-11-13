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
	"reflect"
	"strings"
	"testing"

	"github.com/flant/docs-builder/internal/ondemand"
)

func placeholderPage(t *testing.T, lang string, st ondemand.Status) string {
	t.Helper()

	var b strings.Builder
	if err := writePlaceholder(&b, urls{}, lang, "echo", "v1.2.3", st); err != nil {
		t.Fatal(err)
	}

	return b.String()
}

func TestPlaceholder(t *testing.T) {
	ru := textsFor("ru", "echo", "v1.2.3")

	tests := []struct {
		name    string
		lang    string
		status  ondemand.Status
		want    []string
		notWant []string
	}{
		{
			name:   "rendering, ru",
			lang:   "ru",
			status: ondemand.Status{Stage: ondemand.StageDownload},
			want: []string{
				`<html lang="ru">`,
				ru.Preparing,
				`<li data-step="queued" class="done">` + ru.Queued + `</li>`,
				`<li data-step="download" class="active">` + ru.Download + `</li>`,
				`<li data-step="render">` + ru.Render + `</li>`,
				`var statusURL = "/api/v2/render/echo/v1.2.3";`,
				`<noscript><meta http-equiv="refresh" content="2"></noscript>`,
				// A status that can not be read stops the poll after a limit.
				`if (!st) { return pollFailed(); }`,
				`.catch(pollFailed);`,
			},
			notWant: []string{`id="progress" hidden>`, `.catch(function () { setTimeout(poll`},
		},
		{
			name:   "queued, en",
			lang:   "en",
			status: ondemand.Status{Stage: ondemand.StageQueued},
			want: []string{
				"Preparing the documentation of this version",
				`<li data-step="queued" class="active">Waiting in the queue</li>`,
			},
		},
		{
			name:   "version not found, ru",
			lang:   "ru",
			status: ondemand.Status{Stage: ondemand.StageFailed, NotFound: true, Error: "module version not found"},
			want: []string{
				`<div class="odp-block" id="not-found">`,
				ru.NotFound,
				// To the list: the address of the module may lead back here.
				`<a class="button button_alt" href="/ru/modules/">` + ru.ToList + `</a>`,
			},
			notWant: []string{"statusURL", `id="failed">`},
		},
		{
			name:   "page missing in a rendered version, en",
			lang:   "en",
			status: ondemand.Status{Stage: ondemand.StageDone},
			want: []string{
				`<div class="odp-block" id="no-page">`,
				"Version v1.2.3 has no such page.",
				`<a class="button button_alt" href="/en/modules/echo/v1.2.3/">Open the first page of version v1.2.3</a>`,
			},
			notWant: []string{"statusURL", `id="not-found">`},
		},
		{
			name:   "render failed, the reason is shown in words",
			lang:   "en",
			status: ondemand.Status{Stage: ondemand.StageFailed, Error: ondemand.ReasonRegistry},
			want: []string{
				`<div class="odp-block" id="failed">`,
				"Could not prepare the documentation of this version.",
				`<p class="odp-reason" id="reason">The documentation could not be downloaded from the registry.</p>`,
			},
			notWant: []string{`id="reason">registry<`},
		},
		{
			name:   "render failed, the page counts down to the retry",
			lang:   "ru",
			status: ondemand.Status{Stage: ondemand.StageFailed, Error: ondemand.ReasonTimeout, RetryIn: 42},
			want: []string{
				`<p class="odp-hint" id="retry">` + ru.Retry + `</p>`,
				`var retryIn = "` + ru.RetryIn + `";`,
				`countDown( 42 );`,
			},
			notWant: []string{"\n  setTimeout(poll, 1000);", `http-equiv="refresh"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := placeholderPage(t, tt.lang, tt.status)

			for _, s := range tt.want {
				if !strings.Contains(page, s) {
					t.Errorf("page lacks %q", s)
				}
			}
			for _, s := range tt.notWant {
				if strings.Contains(page, s) {
					t.Errorf("page contains %q", s)
				}
			}
		})
	}
}

// The placeholder is a dialog over the site: the SSI includes of its styles,
// header and footer, by the paths of the pages of a module in each mode.
func TestPlaceholderSiteChrome(t *testing.T) {
	for _, tt := range []struct {
		u    urls
		want []string
	}{
		{urls{}, []string{
			`<!--#include virtual="/includes/head-site.html" -->`,
			`<!--#include virtual="/en/includes/header.html" -->`,
			`<!--#include virtual="/en/includes/footer.html" -->`,
		}},
		{urls{site: true}, []string{
			`<!--#include virtual="/includes/head-site.html" -->`,
			`<!--#include virtual="/includes/header.html" -->`,
			`<!--#include virtual="/includes/footer.html" -->`,
		}},
	} {
		var b strings.Builder
		if err := writePlaceholder(&b, tt.u, "en", "echo", "v1.2.3", ondemand.Status{Stage: ondemand.StageRender}); err != nil {
			t.Fatal(err)
		}
		page := b.String()
		for _, s := range append(tt.want, `<dialog class="odp-dialog" open`, "showModal()") {
			if !strings.Contains(page, s) {
				t.Errorf("site %v: page lacks %q", tt.u.site, s)
			}
		}
		// The page has no styles of its own: they come with the site.
		if strings.Contains(page, "<style") {
			t.Errorf("site %v: the page carries styles", tt.u.site)
		}
	}
}

func TestPlaceholderTextsAreComplete(t *testing.T) {
	for lang, page := range texts {
		for _, section := range []any{page.Placeholder, page.Versions} {
			v := reflect.ValueOf(section)
			for i := range v.NumField() {
				if v.Field(i).String() == "" {
					t.Errorf("%s: %s.%s is empty", lang, v.Type().Name(), v.Type().Field(i).Name)
				}
			}
		}
	}

	for lang, page := range texts {
		for _, r := range []string{ondemand.ReasonTimeout, ondemand.ReasonRegistry, ondemand.ReasonBuild, ondemand.ReasonBusy} {
			if page.Placeholder.Reasons[r] == "" {
				t.Errorf("%s: no text for the reason %q", lang, r)
			}
		}
	}

	ru := textsFor("ru", "echo", "v1.2.3")
	if !strings.Contains(ru.NotFound, "v1.2.3") || !strings.Contains(ru.NotFound, "echo") {
		t.Errorf("module and version are not filled in: %q", ru.NotFound)
	}
}
