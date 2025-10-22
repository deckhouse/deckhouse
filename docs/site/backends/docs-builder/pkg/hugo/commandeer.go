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
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bep/clocks"
	"github.com/bep/lazycache"
	"github.com/bep/logg"
	"github.com/bep/overlayfs"
	"github.com/gohugoio/hugo/common/htime"
	"github.com/gohugoio/hugo/common/hugo"
	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/common/paths"
	"github.com/gohugoio/hugo/config"
	"github.com/gohugoio/hugo/config/allconfig"
	"github.com/gohugoio/hugo/deps"
	"github.com/gohugoio/hugo/hugofs"
	"github.com/gohugoio/hugo/hugolib"
	"github.com/spf13/afero"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func (c *command) Run() error {
	b := newHugoBuilder(c, c.logger)

	err := b.loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	err = b.build()
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	return nil
}

func (c *command) PreRun() error {
	c.Out = os.Stdout
	if c.flags.Quiet {
		c.Out = io.Discard
	}
	// Used by mkcert (server).
	stdlog.SetOutput(c.Out)

	c.Printf = func(format string, v ...interface{}) {
		if !c.flags.Quiet {
			fmt.Fprintf(c.Out, format, v...)
		}
	}
	c.Println = func(a ...interface{}) {
		if !c.flags.Quiet {
			fmt.Fprintln(c.Out, a...)
		}
	}
	var err error
	c.hugologger, err = c.createLogger(false)
	if err != nil {
		return err
	}

	c.commonConfigs = lazycache.New(lazycache.Options[int32, *commonConfig]{MaxEntries: 5})
	c.hugoSites = lazycache.New(lazycache.Options[int32, *hugolib.HugoSites]{MaxEntries: 5})

	return nil
}

func Build(flags *Flags, logger *log.Logger) error {
	cmd := &command{flags: flags, logger: logger}

	err := cmd.PreRun()
	if err != nil {
		return fmt.Errorf("pre run: %w", err)
	}

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

type commonConfig struct {
	mu      *sync.Mutex
	configs *allconfig.Configs
	cfg     config.Provider
	fs      *hugofs.Fs
}

func (c *commonConfig) validate() error {
	if c == nil {
		return errors.New("commonConfig is nil")
	}

	if c.fs == nil {
		return errors.New("commonConfig: no fs provided")
	}

	if c.configs == nil {
		return errors.New("commonConfig: no config provided")
	}

	return nil
}

// This is the root command.
type command struct {
	Printf  func(format string, v ...interface{})
	Println func(a ...interface{})
	Out     io.Writer

	hugologger loggers.Logger

	// The main cache busting key for the caches below.
	configVersionID atomic.Int32

	// Some, but not all commands need access to these.
	// Some needs more than one, so keep them in a small cache.
	commonConfigs *lazycache.Cache[int32, *commonConfig]
	hugoSites     *lazycache.Cache[int32, *hugolib.HugoSites]

	flags *Flags

	logger *log.Logger
}

type Flags struct {
	Source          string
	Environment     string
	BaseURL         string
	GC              bool
	ForceSyncStatic bool
	LogLevel        string
	Verbose         bool
	Debug           bool
	Quiet           bool
	RenderToMemory  bool
	CfgFile         string
	CfgDir          string
}

func (c *command) isVerbose() bool {
	return c.hugologger.Level() <= logg.LevelInfo
}

func (c *command) ConfigFromProvider(key int32, cfg config.Provider) (*commonConfig, error) {
	if cfg == nil {
		panic("cfg must be set")
	}

	cc, _, err := c.commonConfigs.GetOrCreate(key, func(_ int32) (*commonConfig, error) {
		var dir string
		if c.flags.Source != "" {
			dir, _ = filepath.Abs(c.flags.Source)
		} else {
			dir, _ = os.Getwd()
		}

		if cfg == nil {
			cfg = config.New()
		}

		if !cfg.IsSet("renderToDisk") {
			cfg.Set("renderToDisk", true)
		}

		if !cfg.IsSet("workingDir") {
			cfg.Set("workingDir", dir)
		} else {
			if err := os.MkdirAll(cfg.GetString("workingDir"), 0777); err != nil {
				return nil, fmt.Errorf("failed to create workingDir: %w", err)
			}
		}

		// Load the config first to allow publishDir to be configured in config file.
		configs, err := allconfig.LoadConfig(
			allconfig.ConfigSourceDescriptor{
				Flags:       cfg,
				Fs:          hugofs.Os,
				Filename:    c.flags.CfgFile,
				ConfigDir:   c.flags.CfgDir,
				Environment: c.flags.Environment,
				Logger:      c.hugologger,
			},
		)
		if err != nil {
			return nil, err
		}

		base := configs.Base

		cfg.Set("publishDir", base.PublishDir)
		cfg.Set("publishDirStatic", base.PublishDir)
		cfg.Set("publishDirDynamic", base.PublishDir)

		renderStaticToDisk := cfg.GetBool("renderStaticToDisk")

		sourceFs := hugofs.Os
		var destinationFs afero.Fs
		if cfg.GetBool("renderToDisk") {
			destinationFs = hugofs.Os
		} else {
			destinationFs = afero.NewMemMapFs()
			if renderStaticToDisk {
				// Hybrid, render dynamic content to Root.
				cfg.Set("publishDirDynamic", "/")
			} else {
				// Rendering to memoryFS, publish to Root regardless of publishDir.
				cfg.Set("publishDirDynamic", "/")
				cfg.Set("publishDirStatic", "/")
			}
		}

		fs := hugofs.NewFromSourceAndDestination(sourceFs, destinationFs, cfg)

		if renderStaticToDisk {
			dynamicFs := fs.PublishDir
			publishDirStatic := cfg.GetString("publishDirStatic")
			workingDir := cfg.GetString("workingDir")
			absPublishDirStatic := paths.AbsPathify(workingDir, publishDirStatic)
			staticFs := afero.NewBasePathFs(afero.NewOsFs(), absPublishDirStatic)

			// Serve from both the static and dynamic fs,
			// the first will take priority.
			// THis is a read-only filesystem,
			// we do all the writes to
			// fs.Destination and fs.DestinationStatic.
			fs.PublishDirServer = overlayfs.New(
				overlayfs.Options{
					Fss: []afero.Fs{
						dynamicFs,
						staticFs,
					},
				},
			)
			fs.PublishDirStatic = staticFs
		}

		if !base.C.Clock.IsZero() {
			// TODO(bep) find a better place for this.
			htime.Clock = clocks.Start(configs.Base.C.Clock)
		}

		if base.PrintPathWarnings {
			// Note that we only care about the "dynamic creates" here,
			// so skip the static fs.
			fs.PublishDir = hugofs.NewCreateCountingFs(fs.PublishDir)
		}

		commonConfig := &commonConfig{
			mu:      &sync.Mutex{},
			configs: configs,
			cfg:     cfg,
			fs:      fs,
		}

		return commonConfig, nil
	})

	return cc, err
}

func (c *command) HugFromConfig(conf *commonConfig) (*hugolib.HugoSites, error) {
	err := conf.validate()
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	h, _, err := c.hugoSites.GetOrCreate(c.configVersionID.Load(), func(_ int32) (*hugolib.HugoSites, error) {
		depsCfg := deps.DepsCfg{Configs: conf.configs, Fs: conf.fs, StdOut: c.hugologger.StdOut(), LogLevel: c.hugologger.Level()}
		return hugolib.NewHugoSites(depsCfg)
	})
	if err != nil {
		return nil, fmt.Errorf("get or create: %w", err)
	}

	h.Log = c.hugologger

	return h, nil
}

func duration(key string, d time.Duration) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.StringValue(d.String()),
	}
}

func (c *command) createLogger(running bool) (loggers.Logger, error) {
	level := logg.LevelWarn

	if c.flags.LogLevel != "" {
		switch strings.ToLower(c.flags.LogLevel) {
		case "debug":
			level = logg.LevelDebug
		case "info":
			level = logg.LevelInfo
		case "warn", "warning":
			level = logg.LevelWarn
		case "error":
			level = logg.LevelError
		default:
			return nil, fmt.Errorf("invalid log level: %q, must be one of debug, warn, info or error", c.flags.LogLevel)
		}
	} else {
		if c.flags.Verbose {
			hugo.Deprecate("--verbose", "use --logLevel", "v0.119.0")
			level = logg.LevelInfo
		}

		if c.flags.Debug {
			hugo.Deprecate("--debug", "use --logLevel", "v0.119.0")
			level = logg.LevelDebug
		}
	}

	optsLogger := loggers.Options{
		DistinctLevel: level,
		Level:         level,
		StdOut:        c.Out,
		StdErr:        c.Out,
		StoreErrors:   running,
		HandlerPost: func(e *logg.Entry) error {
			opts := make([]any, 0, len(e.Fields))

			for _, f := range e.Fields {
				switch f.Name {
				case "__h_field__cmd":
					f.Name = "hugo_command"
				case "duration":
					opts = append(opts, duration(f.Name, f.Value.(time.Duration)))
					continue
				}
				opts = append(opts, slog.Any(f.Name, f.Value))
			}

			switch e.Level {
			case logg.LevelDebug:
				c.logger.Debug(e.Message, opts...)
			case logg.LevelError:
				c.logger.Error(e.Message, opts...)
			case logg.LevelWarn:
				c.logger.Warn(e.Message, opts...)
			case logg.LevelInfo:
				c.logger.Info(e.Message, opts...)
			}
			return nil
		},
	}

	return loggers.New(optsLogger), nil
}
