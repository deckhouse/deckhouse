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
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/flant/docs-builder/internal/docs"
)

type modulesListTexts struct {
	Title     string `yaml:"title"`
	Preparing string `yaml:"preparing"`
	// Branch is the text for a development branch that is not published
	// yet; ### is the branch.
	Branch string `yaml:"branch"`
	// Module is the text for a module whose version this pod does not know
	// yet.
	Module string `yaml:"module"`
	// NoDocs is the text for a module whose published versions have no
	// documentation.
	NoDocs string `yaml:"noDocs"`
	ToList string `yaml:"toList"`
}

// modulesListRefresh is how often, in seconds, the placeholder of the list
// of modules reloads itself.
const modulesListRefresh = 5

// channelSegments are the release channels in the address of a module page:
// /<lang>/modules/<module>/<channel>/<page>.
var channelSegments = []string{"alpha", "beta", "early-access", "stable", "rock-solid", "latest"}

// handleModulesList answers for the page of the list of modules while
// nginx does not find it on the cache volume: docs-builder has not built it
// yet. The placeholder reloads itself and gets the page once it is there.
func (h *DocsBuilderHandler) handleModulesList(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	if !slices.Contains(docs.Languages, lang) {
		http.NotFound(w, r)
		return
	}

	h.writeRefreshing(w, lang, textsIn(lang).ModulesList)
}

// writeRefreshing answers with a placeholder that reloads itself until
// nginx finds the page.
func (h *DocsBuilderHandler) writeRefreshing(w http.ResponseWriter, lang string, t modulesListTexts) {
	w.Header().Set("Retry-After", strconv.Itoa(modulesListRefresh))
	h.writeCard(w, http.StatusServiceUnavailable, modulesListCard{Lang: lang, T: t, Refresh: modulesListRefresh})
}

// writeNoDocs answers for a module whose published versions have no
// documentation, with a link to the list of modules: the address of the
// module leads to nothing else. With 200: nginx replaces a 404 of
// docs-builder at a module address with the 404 page of the site
// (proxy_intercept_errors); the page is noindex.
func (h *DocsBuilderHandler) writeNoDocs(w http.ResponseWriter, lang, module string) {
	t := textsIn(lang).ModulesList
	t.Title, t.Preparing = module, t.NoDocs
	h.writeCard(w, http.StatusOK, modulesListCard{Lang: lang, T: t, ListURL: urls{site: h.onDemand.Site}.list(lang)})
}

// modulesListCard is the card moduleslist.html shows over the site: one that
// reloads itself (Refresh, in seconds) or one leading to the list (ListURL).
type modulesListCard struct {
	Lang    string
	T       modulesListTexts
	Refresh int
	ListURL string
	Chrome  chrome
}

func (h *DocsBuilderHandler) writeCard(w http.ResponseWriter, code int, card modulesListCard) {
	card.Chrome = urls{site: h.onDemand.Site}.chrome(card.Lang)

	h.writeHTML(w, code, "write a card of the list of modules", func(w io.Writer) error {
		return pages.ExecuteTemplate(w, "moduleslist.html", card)
	})
}

// handleModulePage answers for the address of a module without a version,
// /<lang>/modules/<module>/[<channel>/]<page>, while nginx has no link of
// the module to its target: the target is not published yet. It leads to
// the same page of the target version, which docs-builder renders.
func (h *DocsBuilderHandler) handleModulePage(w http.ResponseWriter, r *http.Request, lang, module, page string) {
	if !slices.Contains(docs.Languages, lang) {
		http.NotFound(w, r)
		return
	}
	// Before the target: the target of such a module may be a version
	// without pages, whose placeholder would lead back here.
	if h.onDemand.Store.NoDocs(module) {
		h.writeNoDocs(w, lang, module)
		return
	}
	target, ok := h.onDemand.Store.Target(module)
	if !ok {
		if !h.onDemand.Store.Pending(module) {
			http.NotFound(w, r)
			return
		}
		t := textsIn(lang).ModulesList
		t.Title, t.Preparing = module, t.Module
		h.writeRefreshing(w, lang, t)
		return
	}
	// A development branch has no address of a version to lead to: the pod
	// renders it in its pass, and nginx serves it once it is published.
	if target.Rev != "" {
		// Failed for a reason that stays: nothing to wait for.
		if st, failed := h.onDemand.Store.BranchFailure(target); failed {
			code := http.StatusBadGateway
			if st.NotFound {
				// Not 404: nginx would show the 404 page of the site instead.
				code = http.StatusOK
			}
			h.writePlaceholder(w, code, lang, module, target.Version, st)
			return
		}
		t := textsIn(lang).ModulesList
		t.Title, t.Preparing = module, strings.ReplaceAll(t.Branch, "###", target.Version)
		h.writeRefreshing(w, lang, t)
		return
	}
	if docs.ValidateModuleVersion(module, target.Version) != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, urls{site: h.onDemand.Site}.version(lang, module, target.Version)+page, http.StatusFound)
}

// modulePage is the page of a module address after the module, without the
// release channel.
func modulePage(segment, rest string) string {
	if slices.Contains(channelSegments, segment) {
		return rest
	}
	if rest == "" {
		return segment
	}

	return segment + "/" + rest
}

func (h *DocsBuilderHandler) handleModuleRoot(w http.ResponseWriter, r *http.Request) {
	if docs.ValidateModuleName(r.PathValue("module")) != nil {
		http.NotFound(w, r)
		return
	}
	h.handleModulePage(w, r, r.PathValue("lang"), r.PathValue("module"), r.PathValue("page"))
}

// handleMenu returns the version menu of a module page: whether the module
// is enabled and the versions of its release channels. In a cluster a module
// has stable only, its target.
func (h *DocsBuilderHandler) handleMenu(w http.ResponseWriter, r *http.Request) {
	module := r.PathValue("module")
	if err := docs.ValidateModuleName(module); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	menu, ok := h.onDemand.Store.Menu(module)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	h.writeJSON(w, menu)
}

// list is the address of the list of modules.
func (u urls) list(lang string) string {
	if u.site {
		return "/modules/"
	}

	return "/" + lang + "/modules/"
}

func (u urls) version(lang, module, version string) string {
	return u.module(lang, module) + version + "/"
}
