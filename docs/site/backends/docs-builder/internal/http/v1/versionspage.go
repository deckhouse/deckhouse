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
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
)

type versionsTexts struct {
	Title    string `yaml:"title"`
	Intro    string `yaml:"intro"`
	Ready    string `yaml:"ready"`
	Empty    string `yaml:"empty"`
	NotFound string `yaml:"notFound"`
	Failed   string `yaml:"failed"`
	Current  string `yaml:"current"`
}

func versionsTextsFor(lang, module string) versionsTexts {
	t := textsIn(lang).Versions
	t.NotFound = fill(t.NotFound, module, "")

	return t
}

type versionLink struct {
	Version, URL string
	Ready        bool
}

type versionsPage struct {
	Lang, Module, ModuleURL string
	T                       versionsTexts
	Versions                []versionLink
	NotFound, Failed        bool
	Chrome                  chrome
}

// handleVersionsPage lists the versions of a module with links to their
// documentation.
//
// ponytail: a stand-in for the list of module versions the cluster will have.
func (h *DocsBuilderHandler) handleVersionsPage(w http.ResponseWriter, r *http.Request) {
	lang, module := r.PathValue("lang"), r.PathValue("module")
	if !slices.Contains(docs.Languages, lang) || docs.ValidateModuleName(module) != nil {
		http.NotFound(w, r)
		return
	}

	page := versionsPage{
		Lang:      lang,
		Module:    module,
		ModuleURL: urls{site: h.onDemand.Site}.module(lang, module),
		T:         versionsTextsFor(lang, module),
		Chrome:    urls{site: h.onDemand.Site}.chrome(lang),
	}

	code := http.StatusOK
	versions, err := h.onDemand.Versions.List(r.Context(), module)
	switch {
	case ondemand.Unpublished(err):
		page.NotFound = true
		code = http.StatusNotFound
	case err != nil:
		h.logger.Warn("list module versions", slog.String("module", module), log.Err(err))
		page.Failed = true
		code = http.StatusBadGateway
	}

	for _, v := range versions {
		page.Versions = append(page.Versions, versionLink{
			Version: v,
			URL:     page.ModuleURL + v + "/",
			Ready:   h.onDemand.Store.Status(module, v).Stage == ondemand.StageDone,
		})
	}

	h.writeHTML(w, code, "write versions page", func(w io.Writer) error {
		return pages.ExecuteTemplate(w, "versions.html", page)
	})
}
