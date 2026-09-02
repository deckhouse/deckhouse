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

// Package pkgsync creates the PackageRepository, ModulePackageVersion and
// Module objects for the module packages the old module stack already carries
// in the cluster, so the package system sees the repositories the modules come
// from, the versions the cluster runs and the modules that run them. Each
// synced resource lives in its own file.
//
// # Data sources
//
//	ModuleSource except the platform-owned "deckhouse" and "flant"
//	  └─ PackageRepository <source>: the registry settings follow the source
//	     on every start, so the module-package-version controller has a live
//	     repository to promote the draft stubs below from
//
//	embedded modules dir (the running image)
//	  ├─ embedded-<module>-<deckhouse version>, complete: the metadata and
//	  │  the settings/values schemas are filled from the module files on
//	  │  disk, no repository ever serves it
//	  ├─ ModulePackage <module>, empty: the catalog entry no scan would
//	  │  create, since no repository offers an embedded package
//	  └─ Module <module>: repository "embedded", the reduced Deckhouse
//	     version, the embedded annotation
//
//	deployed or pending ModuleRelease
//	  ├─ <repository>-<module>-<version>, where the "deckhouse" source maps
//	  │  to the "deckhouse-modules" repository; a draft stub - the
//	  │  module-package-version controller fills it once a PackageRepository
//	  │  exists
//	  └─ Module <module> from the newest deployed release: its repository
//	     and version; an older deployed duplicate is superseded
//
//	ModulePullOverride
//	  └─ Module <module>: the image tag as the version, the dev annotation,
//	     the repository resolved from the resources naming one
//
//	ModuleConfig
//	  └─ Module <module>: settings, settings version, maintenance, enabled
//	     and update policy mirrored onto the module the other sources placed;
//	     a module without a config carries none
//
// A module claims one source: the image beats a pull override, which beats a
// deployed release. A module none of them backs is deleted: the objects the
// old stack created for the modules it merely offered, an embedded module the
// image stopped shipping, a downloaded module whose files are gone. A
// downloaded module still on disk stays, since a pull override deleted
// without a rollback leaves its files in use until the next deploy. A
// condition written without a reason gets one, since the v1alpha2 schema
// requires it.
//
// A version stays a draft until its metadata lands, so no observer takes a
// half-created version for a complete one; a fill interrupted mid-way heals on
// the next start. The legacy label keeps the registry path of the module
// source world ("<module>/release"). No owner is set: the repository the spec
// names may not exist yet, and an owner reference to a missing object would
// get the version garbage-collected; the repository scan adopts the stubs it
// recognizes. An embedded version follows the disk: one version name spans
// every rebuild of a release (a dev build always counts as v2.0.0), so its
// status is refreshed when the module files change and left alone when they
// match - a no-change restart rewrites nothing. A complete release version is
// never touched. Both the version an embedded package carries and the one the
// bootstrap writes into the Module spec come from app.EmbeddedPackageVersion,
// because the reconciler composes the version's name back out of that spec.
package pkgsync

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Names of the repositories the module packages come from during the migration
// off the module sources.
const (
	// moduleSourceNameDeckhouse is the built-in module source shipped with the platform.
	moduleSourceNameDeckhouse = "deckhouse"

	// moduleSourceNameFlant is the module source present on the clusters
	// managed by Flant.
	moduleSourceNameFlant = "flant"

	// repositoryNameDeckhouseModules serves the modules of the "deckhouse"
	// ModuleSource. The plain "deckhouse" name belongs to the application-packages
	// repository, while the module source points at <registry>/modules.
	repositoryNameDeckhouseModules = "deckhouse-modules"

	// repositoryNameEmbedded stands for the Deckhouse image itself and
	// resolves to no PackageRepository object.
	repositoryNameEmbedded = "embedded"
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

	deckhouseVersion     string
	embeddedModulesDir   string
	downloadedModulesDir string

	logger *log.Logger
}

// Sync ensures the package objects of the old module stack for the given
// Deckhouse version and embedded modules dir. The repositories go first, so
// the version stubs find them in place. A source naming no valid version (no
// module source, an unparsable release version, an illegal object name, an
// unreadable module dir, broken schema files) is skipped with a warning; an
// API failure stops the sync. An embedded module skipped here reconciles
// nowhere, since the Module reconciler resolves the same version - see
// known-hazards.md.
func Sync(ctx context.Context, reader client.Reader, writer client.Client, dc dependency.Container, deckhouseVersion, embeddedModulesDir, downloadedModulesDir string, logger *log.Logger) error {
	return newSyncer(reader, writer, dc, deckhouseVersion, embeddedModulesDir, downloadedModulesDir, logger).sync(ctx)
}

// newSyncer builds a syncer for the given Deckhouse version and module dirs.
func newSyncer(reader client.Reader, writer client.Client, dc dependency.Container, deckhouseVersion, embeddedModulesDir, downloadedModulesDir string, logger *log.Logger) *syncer {
	return &syncer{
		reader: reader,
		writer: writer,
		dc:     dc,

		deckhouseVersion:     deckhouseVersion,
		embeddedModulesDir:   embeddedModulesDir,
		downloadedModulesDir: downloadedModulesDir,

		logger: logger,
	}
}

// sync runs the passes in order: repositories first, so the version stubs
// find them in place, then the versions, so the modules placed last resolve
// to an existing one.
func (s *syncer) sync(ctx context.Context) error {
	if err := s.syncPackageRepositories(ctx); err != nil {
		return err
	}

	if err := s.syncModulePackageVersions(ctx); err != nil {
		return err
	}

	return s.syncModules(ctx)
}

// RepositoryNameForSource maps a ModuleSource name to the name of the
// PackageRepository serving the same registry path.
func RepositoryNameForSource(sourceName string) string {
	if sourceName == moduleSourceNameDeckhouse {
		return repositoryNameDeckhouseModules
	}

	return sourceName
}

// SourceNameForRepository maps a PackageRepository name back to the ModuleSource
// serving the same registry path. The embedded repository stands for the image
// and names no source.
func SourceNameForRepository(repositoryName string) string {
	switch repositoryName {
	case repositoryNameDeckhouseModules:
		return moduleSourceNameDeckhouse
	case repositoryNameEmbedded:
		return ""
	}

	return repositoryName
}
