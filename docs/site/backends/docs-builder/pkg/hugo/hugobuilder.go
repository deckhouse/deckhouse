// Copyright 2023 Flant JSC
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

package hugo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gohugoio/hugo/common/herrors"
	"github.com/gohugoio/hugo/common/hugo"
	"github.com/gohugoio/hugo/common/maps"
	"github.com/gohugoio/hugo/config"
	"github.com/gohugoio/hugo/helpers"
	"github.com/gohugoio/hugo/hugolib"
	"github.com/gohugoio/hugo/hugolib/filesystems"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/fsync"
	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// TODO: refactor hugo build to one structure
// mutex for entire hugo
var mu sync.Mutex

type hugoBuilder struct {
	c *command

	confmu sync.Mutex
	conf   *commonConfig

	flags *Flags

	logger *log.Logger
}

func newHugoBuilder(c *command, logger *log.Logger) *hugoBuilder {
	return &hugoBuilder{
		c:      c,
		flags:  c.flags,
		logger: logger,
	}
}

func (b *hugoBuilder) withConfE(fn func(conf *commonConfig) error) error {
	b.confmu.Lock()
	defer b.confmu.Unlock()
	return fn(b.conf)
}

func (b *hugoBuilder) withConf(fn func(conf *commonConfig)) {
	b.confmu.Lock()
	defer b.confmu.Unlock()
	fn(b.conf)
}

func (b *hugoBuilder) build() error {
	err := b.fullBuild()
	if err != nil {
		return fmt.Errorf("full build: %w", err)
	}

	if !b.flags.Quiet {
		h, err := b.hugo()
		if err != nil {
			return fmt.Errorf("hugo: %w", err)
		}

		stats := map[string]any{}
		err = mapstructure.Decode(h.ProcessingStats, &stats)
		if err != nil {
			return fmt.Errorf("decode processing stats: %w", err)
		}

		attrs := make([]any, 0, len(stats))
		for k, v := range stats {
			attrs = append(attrs, slog.Any(strings.ToLower(k), v))
		}

		b.logger.Info("processing stats", attrs...)
	}

	return nil
}

func (b *hugoBuilder) buildSites() error {
	h, err := b.hugo()
	if err != nil {
		return fmt.Errorf("hugo: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// NoBuildLock is true to prevent hugo creating lock file
	// we use mutex to lock the entire hugo build
	err = h.Build(hugolib.BuildCfg{NoBuildLock: true})
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	return nil
}

func (b *hugoBuilder) copyStatic() (map[string]uint64, error) {
	m, err := b.doWithPublishDirs(b.copyStaticTo)
	if err == nil || herrors.IsNotExist(err) {
		return m, nil
	}

	return m, fmt.Errorf("do with publish dirs: %w", err)
}

func (b *hugoBuilder) copyStaticTo(sourceFs *filesystems.SourceFilesystem) (uint64, error) {
	logentry := b.logger.WithGroup("copy static")

	publishDir := helpers.FilePathSeparator

	if sourceFs.PublishFolder != "" {
		publishDir = filepath.Join(publishDir, sourceFs.PublishFolder)
	}

	fs := &countingStatFs{Fs: sourceFs.Fs}

	syncer := fsync.NewSyncer()
	syncer.NoChmod = true
	syncer.NoTimes = true

	b.withConf(func(conf *commonConfig) {
		syncer.NoTimes = conf.configs.Base.NoTimes
		syncer.NoChmod = conf.configs.Base.NoChmod
		syncer.ChmodFilter = chmodFilter

		syncer.DestFs = conf.fs.PublishDirStatic
		// Now that we are using a unionFs for the static directories
		// We can effectively clean the publishDir on initial sync
		syncer.Delete = conf.configs.Base.CleanDestinationDir
	})

	syncer.SrcFs = fs

	if syncer.Delete {
		logentry.Info("removing all files from destination that don't exist in static dirs")

		syncer.DeleteFilter = func(f fsync.FileInfo) bool {
			return f.IsDir() && strings.HasPrefix(f.Name(), ".")
		}
	}

	logentry.Info("syncing static files to", slog.String("publish_dir", publishDir))

	// because we are using a baseFs (to get the union right).
	// set sync src to root
	err := syncer.Sync(publishDir, helpers.FilePathSeparator)
	if err != nil {
		return 0, err
	}

	// Sync runs Stat 3 times for every source file (which sounds much)
	numFiles := fs.statCounter / 3

	return numFiles, err
}

func (b *hugoBuilder) doWithPublishDirs(f func(sourceFs *filesystems.SourceFilesystem) (uint64, error)) (map[string]uint64, error) {
	langCount := make(map[string]uint64)

	h, err := b.hugo()
	if err != nil {
		return nil, err
	}
	staticFilesystems := h.BaseFs.SourceFilesystems.Static

	if len(staticFilesystems) == 0 {
		b.logger.Info("No static directories found to sync")
		return langCount, nil
	}

	for lang, fs := range staticFilesystems {
		cnt, err := f(fs)
		if err != nil {
			return langCount, err
		}
		if lang == "" {
			// Not multihost
			b.withConf(func(conf *commonConfig) {
				for _, l := range conf.configs.Languages {
					langCount[l.Lang] = cnt
				}
			})
		} else {
			langCount[lang] = cnt
		}
	}

	return langCount, nil
}

func (b *hugoBuilder) fullBuild() error {
	var (
		g         errgroup.Group
		langCount map[string]uint64
	)

	b.logger.Info("Start building sites … ")
	b.logger.Info(hugo.BuildVersionString())

	copyStaticFunc := func() error {
		cnt, err := b.copyStatic()
		if err != nil {
			return fmt.Errorf("error copying static files: %w", err)
		}
		langCount = cnt
		return nil
	}
	buildSitesFunc := func() error {
		if err := b.buildSites(); err != nil {
			return fmt.Errorf("error building site: %w", err)
		}
		return nil
	}
	// Do not copy static files and build sites in parallel if cleanDestinationDir is enabled.
	// This flag deletes all static resources in /public folder that are missing in /static,
	// and it does so at the end of copyStatic() call.
	var cleanDestinationDir bool
	b.withConf(func(conf *commonConfig) {
		cleanDestinationDir = conf.configs.Base.CleanDestinationDir
	})
	if cleanDestinationDir {
		if err := copyStaticFunc(); err != nil {
			return err
		}
		if err := buildSitesFunc(); err != nil {
			return err
		}
	} else {
		g.Go(copyStaticFunc)
		g.Go(buildSitesFunc)
		if err := g.Wait(); err != nil {
			return err
		}
	}

	h, err := b.hugo()
	if err != nil {
		return err
	}
	for _, s := range h.Sites {
		s.ProcessingStats.Static = langCount[s.Language().Lang]
	}

	if b.flags.GC {
		count, err := h.GC()
		if err != nil {
			return err
		}
		for _, s := range h.Sites {
			// We have no way of knowing what site the garbage belonged to.
			s.ProcessingStats.Cleaned = uint64(count)
		}
	}

	return nil
}

func (b *hugoBuilder) hugo() (*hugolib.HugoSites, error) {
	var h *hugolib.HugoSites
	if err := b.withConfE(func(conf *commonConfig) error {
		var err error
		h, err = b.c.HugFromConfig(conf)
		return err
	}); err != nil {
		return nil, err
	}

	return h, nil
}

func (b *hugoBuilder) loadConfig() error {
	cfg := config.New()
	cfg.Set("renderToDisk", !b.flags.RenderToMemory)
	if b.flags.Environment == "" {
		// We need to set the environment as early as possible because we need it to load the correct config.
		// Check if the user has set it in env.
		if env := os.Getenv("HUGO_ENVIRONMENT"); env != "" {
			b.flags.Environment = env
		} else if env := os.Getenv("HUGO_ENV"); env != "" {
			b.flags.Environment = env
		} else {
			b.flags.Environment = hugo.EnvironmentProduction
		}
	}
	cfg.Set("environment", b.flags.Environment)

	cfg.Set("internal", maps.Params{
		"running": false,
		"watch":   false,
		"verbose": b.c.isVerbose(),
	})

	conf, err := b.c.ConfigFromProvider(b.c.configVersionID.Load(), cfg)
	if err != nil {
		return err
	}

	if len(conf.configs.LoadingInfo.ConfigFiles) == 0 {
		return errors.New("unable to locate config file or config directory, perhaps you need to create a new site, run `hugo help new` for details")
	}

	conf.configs.Base.Markup.DefaultMarkdownHandler = "goldmark"
	conf.configs.Base.Markup.Goldmark.DuplicateResourceFiles = true

	b.conf = conf

	return nil
}
