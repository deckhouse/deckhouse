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

package ondemand

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
)

// RenderListFunc renders the page of the list of modules of every language
// from the cards in data, using workDir for temporary files.
type RenderListFunc func(ctx context.Context, data []byte, workDir string) (map[string][]byte, error)

// Listed is a module on the list of modules, as targets.json keeps it: the
// list shows every module with a registry source, enabled or not, also
// before its target is published.
type Listed struct {
	Module  string `json:"module"`
	Enabled bool   `json:"enabled,omitempty"`
	// Stage is the lifecycle stage the cluster knows of the module, shown
	// until the target brings its own.
	Stage string `json:"stage,omitempty"`
	// NoDocs marks a module with nothing to read: no version to render, or
	// the version of its target has no documentation pages.
	NoDocs bool `json:"noDocs,omitempty"`
	// Failed marks a module whose target failed for a lasting reason: its
	// card does not promise the details, they wait for the retry.
	Failed bool `json:"failed,omitempty"`
}

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

// modulesListPage is the page of the list of modules of a language in the
// cache volume, served at /<lang>/modules/.
func modulesListPage(lang string) string {
	return filepath.Join(indexDir, lang, "modules", "index.html")
}

// listedOf lists the modules of the targets, for a targets.json written
// before it kept the modules. Whether they are enabled is not known until
// the leader writes them anew.
func listedOf(targets []Target) []Listed {
	out := make([]Listed, 0, len(targets))
	for _, t := range targets {
		out = append(out, Listed{Module: t.Module})
	}

	return out
}

// WriteList builds the page of the list of modules from the modules and
// the published versions of their targets, which bring the title, the
// description and the stage (docs.Summary). A module whose target is not
// published yet is shown by its name. The page is replaced at once, and not
// rebuilt when the cards are the same as last time. The modules are those a
// pass listed: a registry or API server that does not answer fails the pass
// before, so an empty list means no modules.
func (s *Store) WriteList(ctx context.Context, modules []Listed, targets []Target) error {
	listed := make(map[string]Listed, len(modules))
	for _, m := range modules {
		listed[m.Module] = m
	}
	s.mu.Lock()
	prev := s.listed
	s.listed = listed
	s.mu.Unlock()
	if s.cfg.Relisted != nil {
		for module := range prev {
			if _, ok := listed[module]; !ok {
				s.cfg.Relisted(module)
			}
		}
		for module := range listed {
			if _, ok := prev[module]; !ok {
				s.cfg.Relisted(module)
			}
		}
	}
	if s.cfg.RenderList == nil {
		return nil
	}

	byName := byModule(targets)
	cards := make([]card, 0, len(modules))
	for _, m := range modules {
		c := card{Module: m.Module, Stage: m.Stage, Enabled: m.Enabled, NoDocs: m.NoDocs}
		if t, ok := byName[m.Module]; ok && s.published(t) {
			summary, err := docs.ReadSummary(filepath.Join(s.cfg.Dir, t.Module, t.Version))
			// A version of an older render format has none until the leader
			// renders it anew: the card keeps the name.
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				s.cfg.Logger.Warn("read module summary", slog.String("module", t.Module), slog.String("version", t.Version), log.Err(err))
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

	s.listMu.Lock()
	defer s.listMu.Unlock()
	if bytes.Equal(data, s.listState) {
		// The served page is the one of these cards.
		s.SetGauge(metrics.DocsBuilderOnDemandModulesListFailed, 0)
		return nil
	}

	err = s.renderList(ctx, data)
	if ctx.Err() != nil {
		return err
	}
	failed := 0.0
	if err != nil {
		failed = 1
	}
	s.SetGauge(metrics.DocsBuilderOnDemandModulesListFailed, failed)
	if err != nil {
		return err
	}
	s.listState = data

	return nil
}

func (s *Store) renderList(ctx context.Context, data []byte) error {
	if err := os.MkdirAll(filepath.Join(s.cfg.Dir, tmpDir), 0755); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	work, err := os.MkdirTemp(filepath.Join(s.cfg.Dir, tmpDir), "list-*")
	if err != nil {
		return fmt.Errorf("mkdir list: %w", err)
	}
	defer os.RemoveAll(work)

	pages, err := s.cfg.RenderList(ctx, data, work)
	if err != nil {
		return fmt.Errorf("render the list of modules: %w", err)
	}

	var errs []error
	for lang, page := range pages {
		path := filepath.Join(s.cfg.Dir, modulesListPage(lang))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			errs = append(errs, err)
			continue
		}
		// No gzip copy: the page carries SSI includes, nginx compresses it
		// after them. The copy an older docs-builder wrote goes.
		if err := writeFile(path, func(w io.Writer) error { _, err := w.Write(page); return err }); err != nil {
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
func (s *Store) SetTargets(targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.targets = byModule(targets)
	// A branch that is no target any more, or has moved on, fails no more.
	for t := range s.failedBranches {
		if s.targets[t.Module] != t {
			delete(s.failedBranches, t)
		}
	}
	s.SetGauge(metrics.DocsBuilderOnDemandFailedBranches, float64(len(s.failedBranches)))
}

// Target returns the target of the module.
func (s *Store) Target(module string) (Target, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.targets[module]

	return t, ok
}

// ModuleMenu is what the version menu of a module page shows.
type ModuleMenu struct {
	// Enabled marks a module enabled in the cluster: its stable channel is
	// the installed version.
	Enabled bool `json:"enabled"`
	// Channels are the served release channels and their versions.
	Channels Channels `json:"channels"`
}

// Pending reports a module on the list with documentation to come whose
// target this pod does not know yet: a follower knows only the published
// targets of the leader.
func (s *Store) Pending(module string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, listed := s.listed[module]
	_, known := s.targets[module]

	return listed && !m.NoDocs && !m.Failed && !known
}

// NoDocs reports a module on the list whose published versions have no
// documentation.
func (s *Store) NoDocs(module string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listed[module].NoDocs
}

// Menu returns the version menu of the module; false when the module is not
// on the list.
func (s *Store) Menu(module string) (ModuleMenu, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, listed := s.listed[module]
	chans, linked := s.served[module]
	if !listed && !linked {
		return ModuleMenu{}, false
	}
	if chans == nil {
		chans = Channels{}
	}

	return ModuleMenu{Enabled: m.Enabled, Channels: maps.Clone(chans)}, true
}
