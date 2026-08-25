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

// Package syncer creates the ModulePackageVersion and ModulePackage objects
// for the module packages the old module stack already carries in the
// cluster, so the package system sees the versions the cluster runs and the
// packages its sources offer. Each synced resource lives in its own file.
//
// # Data sources
//
//	embedded modules dir (the running image)
//	  └─ embedded-<module>-<deckhouse version>, complete: the metadata is
//	     filled from the module files on disk, no repository ever serves it
//
//	deployed or pending ModuleRelease
//	  └─ <repository>-<module>-<version>, where the "deckhouse" source maps
//	     to the "deckhouse-modules" repository; a draft stub - the
//	     module-package-version controller fills it once a PackageRepository
//	     exists
//
//	v1alpha1 Module with a non-empty availableSources
//	  └─ ModulePackage <module>: availableRepositories carries the sources
//	     mapped to repository names; the repository scan later adopts the
//	     package and appends the repositories it really serves
//
// A version stays a draft until its metadata lands, so no observer takes a
// half-created version for a complete one; a fill interrupted mid-way heals on
// the next start. The legacy label keeps the registry path of the module
// source world ("<module>/release"). No owner is set: the repository the spec
// names may not exist yet, and an owner reference to a missing object would
// get the version garbage-collected; the repository scan adopts the stubs it
// recognizes. An existing complete version is never touched, so a restart
// changes nothing.
package syncer

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Syncer creates the missing package versions once at start, while the
// controllers still wait for the sync phase.
type Syncer struct {
	// reader must bypass the manager cache: the ModulePackageVersion kind is
	// cached only when the module packages feature is on, and this sync runs
	// everywhere.
	reader client.Reader
	writer client.Client
	dc     dependency.Container

	deckhouseVersion   string
	embeddedModulesDir string

	logger *log.Logger
}

// New builds a Syncer for the given Deckhouse version and embedded modules dir.
func New(reader client.Reader, writer client.Client, dc dependency.Container, deckhouseVersion, embeddedModulesDir string, logger *log.Logger) *Syncer {
	return &Syncer{
		reader: reader,
		writer: writer,
		dc:     dc,

		deckhouseVersion:   deckhouseVersion,
		embeddedModulesDir: embeddedModulesDir,

		logger: logger,
	}
}

// Sync ensures the package versions of the old module stack. A source naming
// no valid version (no module source, an unparsable version, an illegal object
// name, an unreadable module dir) is skipped with a warning; an API failure
// stops the sync.
func (s *Syncer) Sync(ctx context.Context) error {
	if err := s.syncEmbedded(ctx); err != nil {
		return err
	}

	if err := s.syncReleases(ctx); err != nil {
		return err
	}

	return s.syncPackages(ctx)
}

// Names of the repositories the module packages come from during the migration
// off the module sources.
const (
	// moduleSourceNameDeckhouse is the built-in module source shipped with the platform.
	moduleSourceNameDeckhouse = "deckhouse"

	// repositoryNameDeckhouseModules serves the modules of the "deckhouse"
	// ModuleSource. The plain "deckhouse" name belongs to the application-packages
	// repository, while the module source points at <registry>/modules.
	repositoryNameDeckhouseModules = "deckhouse-modules"

	// repositoryNameEmbedded stands for the Deckhouse image itself and
	// resolves to no PackageRepository object.
	repositoryNameEmbedded = "embedded"
)

// repositoryNameForSource maps a ModuleSource name to the name of the
// PackageRepository serving the same registry path.
func repositoryNameForSource(sourceName string) string {
	if sourceName == moduleSourceNameDeckhouse {
		return repositoryNameDeckhouseModules
	}

	return sourceName
}
