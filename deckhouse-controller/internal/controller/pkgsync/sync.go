// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkgsync

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Names of the repositories the module packages come from during the migration
// off the module sources.
const (
	// repositoryNameDeckhouseModules serves the modules of the "deckhouse"
	// ModuleSource. The plain "deckhouse" name belongs to the application-packages
	// repository, while the module source points at <registry>/modules.
	repositoryNameDeckhouseModules = "deckhouse-modules"

	// repositoryNameEmbedded stands for the Deckhouse image itself and
	// resolves to no PackageRepository object.
	repositoryNameEmbedded = "embedded"

	// packageNameGlobal is the reserved name of the global module, whose files
	// live in the global hooks dir rather than under the embedded modules dir.
	packageNameGlobal = "global"

	// packageNameDeckhouse is the module whose settings carry the release channel
	// Deckhouse itself follows.
	packageNameDeckhouse = "deckhouse"
)

// syncer creates the missing package versions once at start, while the
// controllers still wait for the sync phase.
type syncer struct {
	// reader must bypass the manager cache: the ModulePackageVersion kind is
	// cached only when the module packages feature is on, and this sync runs
	// everywhere.
	reader client.Reader
	writer client.Client
	dc     dependency.Container

	deckhouseVersion   string
	embeddedModulesDir string
	globalHooksDir     string

	logger *log.Logger
}

// Option overrides a syncer default; the defaults describe the running process.
type Option func(*syncer)

// WithDeckhouseVersion sets the version every package the image ships is named after.
func WithDeckhouseVersion(deckhouseVersion string) Option {
	return func(s *syncer) { s.deckhouseVersion = deckhouseVersion }
}

// WithEmbeddedModulesDir points the sync at another embedded modules dir.
func WithEmbeddedModulesDir(dir string) Option {
	return func(s *syncer) { s.embeddedModulesDir = dir }
}

// WithGlobalHooksDir points the sync at another global hooks dir.
func WithGlobalHooksDir(dir string) Option {
	return func(s *syncer) { s.globalHooksDir = dir }
}

// Sync ensures the package objects of the old module stack for the given
// Deckhouse version, embedded modules dir and global hooks dir. The
// repositories go first, so the version stubs find them in place. A source
// naming no valid version (no module source, an unparsable release version, an
// illegal object name, an unreadable module dir, broken or missing schema
// files) is skipped with a warning; an API failure stops the sync. An embedded
// module skipped here reconciles nowhere, since the Module reconciler resolves
// the same version.
func Sync(ctx context.Context, reader client.Reader, writer client.Client, dc dependency.Container, logger *log.Logger) error {
	return newSyncer(reader, writer, dc, logger).sync(ctx)
}

// newSyncer builds a syncer over the version and the dirs the running process carries.
func newSyncer(reader client.Reader, writer client.Client, dc dependency.Container, logger *log.Logger, opts ...Option) *syncer {
	s := &syncer{
		reader: reader,
		writer: writer,
		dc:     dc,

		deckhouseVersion:   app.Version,
		embeddedModulesDir: app.EmbeddedModulesDir,
		globalHooksDir:     app.GlobalHooksDir,

		logger: logger.Named("syncer"),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// embeddedPackageVersion is the version every package the image ships carries.
func (s *syncer) embeddedPackageVersion() string {
	return app.EmbeddedPackageVersionOf(s.deckhouseVersion)
}

// sync runs the passes in order: the repositories first, so the version stubs find them in place,
// and the modules last, so the packages they name exist.
func (s *syncer) sync(ctx context.Context) error {
	if err := s.syncPackageRepositories(ctx); err != nil {
		return fmt.Errorf("sync package repositories: %w", err)
	}

	if err := s.syncModulePackageVersions(ctx); err != nil {
		return fmt.Errorf("sync module package versions: %w", err)
	}

	if err := s.syncModules(ctx); err != nil {
		return fmt.Errorf("sync modules: %w", err)
	}

	return nil
}

// PackageRepositoryNameForModuleSource maps a ModuleSource name to the name of the
// PackageRepository serving the same registry path.
func PackageRepositoryNameForModuleSource(sourceName string) string {
	if sourceName == moduleSourceNameDeckhouse {
		return repositoryNameDeckhouseModules
	}

	return sourceName
}
