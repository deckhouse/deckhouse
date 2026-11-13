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

// Package catalog keeps the list of modules of the cache volume: its page,
// the modules on it and the target of every module.
package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/atomicfile"
)

// card is a module on the page of the list of modules
// (hugo.Data.modules_list).
type card struct {
	Module      string            `json:"module"`
	Title       map[string]string `json:"title,omitempty"`
	Description map[string]string `json:"description,omitempty"`
	Stage       string            `json:"stage,omitempty"`
	Enabled     bool              `json:"enabled,omitempty"`
	NoDocs      bool              `json:"noDocs,omitempty"`
	// Pending is a module whose version is not published here yet: its card
	// says the details come later.
	Pending bool `json:"pending,omitempty"`
}

// Config of a Catalog.
type Config struct {
	// Dir is the cache volume the versions are published in; Pages is where
	// the page of the list goes, as <lang>/modules/index.html; Tmp holds the
	// work directories of its builds.
	Dir, Pages, Tmp string
	Logger          *log.Logger
	// Render builds the page, when set; Relisted is told of a module that
	// joins or leaves the list.
	Render   ondemand.RenderListFunc
	Relisted func(module string)
	// Published reports whether the store has the version of the target
	// published; Gauge sets a gauge without labels.
	Published func(t ondemand.Target) bool
	Gauge     func(name string, v float64)
}

// Catalog is the list of modules: its page, the modules on it and the target
// of every module, which the address of the module leads to.
type Catalog struct {
	dir, pages, tmp string
	logger          *log.Logger
	render          ondemand.RenderListFunc
	relisted        func(module string)
	published       func(t ondemand.Target) bool
	gauge           func(name string, v float64)

	mu sync.Mutex
	// targets is the target of every module (SetTargets); listed are the
	// modules of the last WriteList.
	targets map[string]ondemand.Target
	listed  map[string]ondemand.Listed

	// listMu serializes the builds of the list of modules; listState is
	// the data of the last one.
	listMu    sync.Mutex
	listState []byte
}

// New makes the catalog of the store cfg describes.
func New(cfg Config) *Catalog {
	return &Catalog{
		dir:       cfg.Dir,
		pages:     cfg.Pages,
		tmp:       cfg.Tmp,
		logger:    cfg.Logger,
		render:    cfg.Render,
		relisted:  cfg.Relisted,
		published: cfg.Published,
		gauge:     cfg.Gauge,
		targets:   make(map[string]ondemand.Target),
	}
}

// page is the page of the list of modules of a language, served at
// /<lang>/modules/.
func (cat *Catalog) page(lang string) string {
	return filepath.Join(cat.pages, lang, "modules", "index.html")
}

// WriteList builds the page of the list of modules from the modules and
// the published versions of their targets, which bring the title, the
// description and the stage (docs.Summary). A module whose target is not
// published yet is shown by its name. The page is replaced at once, and not
// rebuilt when the cards are the same as last time. The modules are those a
// pass listed: a registry or API server that does not answer fails the pass
// before, so an empty list means no modules.
func (cat *Catalog) WriteList(ctx context.Context, modules []ondemand.Listed, targets []ondemand.Target) error {
	listed := make(map[string]ondemand.Listed, len(modules))
	for _, m := range modules {
		listed[m.Module] = m
	}
	cat.mu.Lock()
	prev := cat.listed
	cat.listed = listed
	cat.mu.Unlock()
	if cat.relisted != nil {
		for module := range prev {
			if _, ok := listed[module]; !ok {
				cat.relisted(module)
			}
		}
		for module := range listed {
			if _, ok := prev[module]; !ok {
				cat.relisted(module)
			}
		}
	}
	if cat.render == nil {
		return nil
	}

	byName := ondemand.ByModule(targets)
	cards := make([]card, 0, len(modules))
	for _, m := range modules {
		c := card{Module: m.Module, Stage: m.Stage, Enabled: m.Enabled, NoDocs: m.NoDocs}
		if t, ok := byName[m.Module]; ok && cat.published(t) {
			summary, err := docs.ReadSummary(filepath.Join(cat.dir, t.Module, t.Version))
			// A version of an older render format has none until the leader
			// renders it anew: the card keeps the name.
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				cat.logger.Warn("read module summary", slog.String("module", t.Module), slog.String("version", t.Version), log.Err(err))
			}
			c.Title, c.Description = summary.Title, summary.Description
			if summary.Stage != "" {
				c.Stage = summary.Stage
			}
		} else {
			c.Pending = !m.NoDocs && !m.Failed
		}
		cards = append(cards, c)
	}
	slices.SortFunc(cards, func(a, b card) int { return strings.Compare(a.Module, b.Module) })

	data, err := json.Marshal(struct {
		Modules []card `json:"modules"`
	}{cards})
	if err != nil {
		return err
	}

	cat.listMu.Lock()
	defer cat.listMu.Unlock()
	if bytes.Equal(data, cat.listState) {
		// The served page is the one of these cards.
		cat.gauge(metrics.DocsBuilderOnDemandModulesListFailed, 0)
		return nil
	}

	err = cat.renderList(ctx, data)
	if ctx.Err() != nil {
		return err
	}
	failed := 0.0
	if err != nil {
		failed = 1
	}
	cat.gauge(metrics.DocsBuilderOnDemandModulesListFailed, failed)
	if err != nil {
		return err
	}
	cat.listState = data

	return nil
}

func (cat *Catalog) renderList(ctx context.Context, data []byte) error {
	if err := os.MkdirAll(cat.tmp, 0755); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	work, err := os.MkdirTemp(cat.tmp, "list-*")
	if err != nil {
		return fmt.Errorf("mkdir list: %w", err)
	}
	defer os.RemoveAll(work)

	pages, err := cat.render(ctx, data, work)
	if err != nil {
		return fmt.Errorf("render the list of modules: %w", err)
	}

	var errs []error
	for lang, page := range pages {
		path := cat.page(lang)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			errs = append(errs, err)
			continue
		}
		// No gzip copy: the page carries SSI includes, nginx compresses it
		// after them. The copy an older docs-builder wrote goes.
		if err := atomicfile.Write(path, func(w io.Writer) error { _, err := w.Write(page); return err }); err != nil {
			errs = append(errs, fmt.Errorf("%s list of modules: %w", lang, err))
		}
		if err := os.Remove(path + ".gz"); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("%s list of modules: %w", lang, err))
		}
	}

	return errors.Join(errs...)
}

// SetTargets records the target of every module: the address of a module
// without a version leads to it while it is not published.
func (cat *Catalog) SetTargets(targets []ondemand.Target) {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	cat.targets = ondemand.ByModule(targets)
}

// Target returns the target of the module.
func (cat *Catalog) Target(module string) (ondemand.Target, bool) {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	t, ok := cat.targets[module]

	return t, ok
}

// Pending reports a module on the list with documentation to come whose
// target this pod does not know yet: a follower knows only the published
// targets of the leader.
func (cat *Catalog) Pending(module string) bool {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	m, listed := cat.listed[module]
	_, known := cat.targets[module]

	return listed && !m.NoDocs && !m.Failed && !known
}

// NoDocs reports a module on the list whose published versions have no
// documentation.
func (cat *Catalog) NoDocs(module string) bool {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	return cat.listed[module].NoDocs
}

// Listed returns the module as the last WriteList listed it; false
// when it is not on the list.
func (cat *Catalog) Listed(module string) (ondemand.Listed, bool) {
	cat.mu.Lock()
	defer cat.mu.Unlock()

	m, ok := cat.listed[module]

	return m, ok
}
