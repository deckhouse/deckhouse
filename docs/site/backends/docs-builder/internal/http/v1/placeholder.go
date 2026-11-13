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
	"embed"
	"html/template"
	"io"

	"github.com/flant/docs-builder/internal/ondemand"
)

//go:embed *.html
var pagesFS embed.FS

// pages are the HTML pages of on-demand rendering: placeholder.html is shown
// while a module version renders and when it can not be shown (while
// rendering, it polls the render status and reloads the page once the version
// is on the cache volume); versions.html lists the versions of a module.
var pages = template.Must(template.ParseFS(pagesFS, "*.html"))

type placeholderTexts struct {
	Docs      string `yaml:"docs"`
	Preparing string `yaml:"preparing"`
	Queued    string `yaml:"queued"`
	Download  string `yaml:"download"`
	Render    string `yaml:"render"`
	NotFound  string `yaml:"notFound"`
	NoPage    string `yaml:"noPage"`
	Failed    string `yaml:"failed"`
	Retry     string `yaml:"retry"`
	// RetryIn counts down to the automatic retry; ### is the seconds.
	RetryIn string `yaml:"retryIn"`
	// Reasons of a failed render by ondemand.Reason*.
	Reasons        map[string]string `yaml:"reasons"`
	CurrentVersion string            `yaml:"currentVersion"`
	VersionStart   string            `yaml:"versionStart"`
	ToList         string            `yaml:"toList"`
}

func textsFor(lang, module, version string) placeholderTexts {
	t := textsIn(lang).Placeholder
	t.NotFound = fill(t.NotFound, module, version)
	t.NoPage = fill(t.NoPage, module, version)
	t.VersionStart = fill(t.VersionStart, module, version)

	return t
}

type placeholderStep struct {
	Key   ondemand.Stage
	Label string
	State string // "done", "active" or empty
}

type placeholder struct {
	Lang, Module, Version string
	Status                ondemand.Status
	T                     placeholderTexts
	Steps                 []placeholderStep

	InProgress, NoPage, Failed bool

	StatusURL, ModuleURL, VersionURL, ListURL string

	Chrome chrome
}

// writePlaceholder renders the placeholder for a module version page that is
// not on the cache volume.
// u builds its links (siteURLs on the public site).
func writePlaceholder(w io.Writer, u urls, lang, module, version string, st ondemand.Status) error {
	t := textsFor(lang, module, version)

	p := placeholder{
		Lang:       lang,
		Module:     module,
		Version:    version,
		Status:     st,
		T:          t,
		InProgress: st.Stage != ondemand.StageDone && st.Stage != ondemand.StageFailed,
		NoPage:     st.Stage == ondemand.StageDone,
		Failed:     st.Stage == ondemand.StageFailed && !st.NotFound,
		StatusURL:  u.status(module, version),
		ModuleURL:  u.module(lang, module),
		VersionURL: u.module(lang, module) + version + "/",
		ListURL:    u.list(lang),
		Chrome:     u.chrome(lang),
	}

	steps := []placeholderStep{
		{Key: ondemand.StageQueued, Label: t.Queued},
		{Key: ondemand.StageDownload, Label: t.Download},
		{Key: ondemand.StageRender, Label: t.Render},
	}
	current := -1
	for i, s := range steps {
		if s.Key == st.Stage {
			current = i
		}
	}
	for i := range steps {
		switch {
		case i < current:
			steps[i].State = "done"
		case i == current:
			steps[i].State = "active"
		}
	}
	p.Steps = steps

	return pages.ExecuteTemplate(w, "placeholder.html", p)
}

// urls are the links of the pages docs-builder writes. In a cluster the
// language is the first segment of the path; the public site keeps it in the
// host, and its Ingress passes only /modules, so the render status is asked
// at /modules/.render/ (the nginx of the site sends it to /api/v2/render/).
type urls struct{ site bool }

func (u urls) module(lang, module string) string {
	if u.site {
		return "/modules/" + module + "/"
	}

	return "/" + lang + "/modules/" + module + "/"
}

// chrome are the SSI includes of the site: its styles and scripts, header
// and footer. The pages of docs-builder show their card as a dialog over
// them, like the pages of a module do. nginx expands them, in a cluster that
// of the module, on the public site the Ingress; the paths are those of
// baseof.html of docs-builder-template. lang is one of docs.Languages.
type chrome struct{ Head, Header, Footer template.HTML }

func (u urls) chrome(lang string) chrome {
	prefix := "/" + lang
	if u.site {
		prefix = ""
	}
	include := func(path string) template.HTML {
		return template.HTML(`<!--#include virtual="` + path + `" -->`) //nolint:gosec // constant paths, lang is validated
	}

	return chrome{
		Head:   include("/includes/head-site.html"),
		Header: include(prefix+"/includes/header.html") + include(prefix+"/includes/banner.html"),
		Footer: include(prefix + "/includes/footer.html"),
	}
}

func (u urls) status(module, version string) string {
	if u.site {
		return "/modules/.render/" + module + "/" + version
	}

	return "/api/v2/render/" + module + "/" + version
}
