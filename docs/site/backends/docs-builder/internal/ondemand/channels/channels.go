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

// Package channels points the release channels of the modules at their
// published versions on the cache volume, with links nginx serves.
package channels

import (
	"errors"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
)

// Links points the release channels of the modules at their published
// versions for nginx: .site/<lang>/modules/<module>/<channel> is a link like
// the one of the version, so the pages of a version serve the URLs of the
// channels as well (their links are relative, see docs.RenderVersion).
type Links struct {
	dir    string
	logger *log.Logger
	// published reports whether the store has the version of the target
	// published.
	published func(t ondemand.Target) bool

	// setMu keeps two set calls (the leader and a follower falling back) from
	// undoing each other.
	setMu sync.Mutex

	mu sync.Mutex
	// stable is the version the stable channel of a module links to: the
	// site serves it at the root of the module.
	stable map[string]string
	// served are the versions the links of the channels point at.
	served map[string]ondemand.Channels
	// modules are the modules with channel links (set).
	modules map[string]bool
}

// New keeps the channel links of the link trees under dir, the cache volume;
// published reports whether the store has a version published.
func New(dir string, logger *log.Logger, published func(ondemand.Target) bool) *Links {
	return &Links{
		dir:       dir,
		logger:    logger,
		published: published,
		stable:    make(map[string]string),
		served:    make(map[string]ondemand.Channels),
		modules:   make(map[string]bool),
	}
}

// Link points the release channels of the module at their published versions
// in the link tree site. A channel whose version is not published keeps its
// link; a channel that is gone loses it. Each link is replaced at once, never
// missing in between.
func (c *Links) Link(site, module string, chans ondemand.Channels) {
	// The module and the versions may come from targets.json in S3 and
	// become paths here.
	if err := docs.ValidateModuleName(module); err != nil {
		c.logger.Warn("skip the release channels of a module", log.Err(err))
		return
	}
	valid := make(ondemand.Channels, len(chans))
	for channel, version := range chans {
		if err := docs.ValidateModuleTag(module, version); err != nil {
			c.logger.Warn("skip a release channel", slog.String("channel", channel), log.Err(err))
			continue
		}
		valid[channel] = version
	}
	chans = valid

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.stable, module)
	if v, ok := chans["stable"]; ok && c.published(ondemand.Target{Module: module, Version: v}) {
		c.stable[module] = v
	}

	// served are the versions the links point at: a channel whose new
	// version is not published keeps the old one, and the version menu
	// shows that one.
	served := make(ondemand.Channels)
	for _, lang := range docs.Languages {
		dir := filepath.Join(c.dir, site, lang, "modules", module)
		for _, channel := range ondemand.SiteChannels {
			link := filepath.Join(dir, channel)
			version, ok := chans[channel]
			if !ok {
				if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
					c.logger.Warn("remove channel link", slog.String("path", link), log.Err(err))
				}
				continue
			}
			published := c.published(ondemand.Target{Module: module, Version: version})
			if _, err := os.Stat(filepath.Join(c.dir, module, version, lang)); !published || err != nil {
				if target, err := os.Readlink(link); err == nil && served[channel] == "" {
					served[channel] = filepath.Base(filepath.Dir(target))
				}
				continue
			}

			if err := linkChannel(dir, link, module, version, lang); err != nil {
				c.logger.Warn("link channel", slog.String("path", link), log.Err(err))
				continue
			}
			served[channel] = version
		}
	}
	if len(served) == 0 {
		delete(c.served, module)
		return
	}
	c.served[module] = served
}

func linkChannel(dir, link, module, version, lang string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := link + ".new"
	os.Remove(tmp)
	if err := os.Symlink(filepath.Join("..", "..", "..", "..", module, version, lang), tmp); err != nil {
		return err
	}

	return os.Rename(tmp, link)
}

// Set links the release channels of every module in chans in the link tree
// site and removes the channel links of the modules no longer in it: a module
// gone from the registries leaves no channel pointing at a version that is
// evicted.
func (c *Links) Set(site string, chans map[string]ondemand.Channels) {
	c.setMu.Lock()
	defer c.setMu.Unlock()

	c.mu.Lock()
	prev := c.modules
	c.modules = make(map[string]bool, len(chans))
	for module := range chans {
		c.modules[module] = true
	}
	c.mu.Unlock()

	for module, chs := range chans {
		c.Link(site, module, chs)
	}
	for module := range prev {
		if _, ok := chans[module]; !ok {
			c.Link(site, module, nil)
		}
	}
}

// Keep carries the channel links of the link tree from into the link tree
// site, for the versions still published: a store building a new tree keeps
// the channel pages served until the channels are linked again.
func (c *Links) Keep(from, site string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, lang := range docs.Languages {
		modulesDir := filepath.Join(c.dir, from, lang, "modules")
		modules, err := os.ReadDir(modulesDir)
		if err != nil {
			continue
		}
		for _, m := range modules {
			for _, channel := range ondemand.SiteChannels {
				target, err := os.Readlink(filepath.Join(modulesDir, m.Name(), channel))
				if err != nil {
					continue
				}
				version := filepath.Base(filepath.Dir(target))
				if !c.published(ondemand.Target{Module: m.Name(), Version: version}) || target != filepath.Join("..", "..", "..", "..", m.Name(), version, lang) {
					continue
				}
				dir := filepath.Join(c.dir, site, lang, "modules", m.Name())
				c.modules[m.Name()] = true
				if err := linkChannel(dir, filepath.Join(dir, channel), m.Name(), version, lang); err != nil {
					c.logger.Warn("keep channel link", slog.String("module", m.Name()), slog.String("channel", channel), log.Err(err))
				}
			}
		}
	}
}

// Stable returns the version the stable channel of the module links to, ""
// when there is none.
func (c *Links) Stable(module string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.stable[module]
}

// Served returns the versions the links of the channels of the module point
// at; false when the module has no channel links.
func (c *Links) Served(module string) (ondemand.Channels, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	chans, ok := c.served[module]

	return maps.Clone(chans), ok
}
