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

// Package versionsync creates the ModulePackageVersion and ModulePackage
// objects for the module packages the old module stack already carries in the
// cluster, so the package system sees the versions the cluster runs and the
// packages its sources offer.
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
package versionsync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/metadata"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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

// syncEmbedded walks the embedded modules dir and ensures a complete version
// for every module the running image ships.
func (s *Syncer) syncEmbedded(ctx context.Context) error {
	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return fmt.Errorf("read embedded modules dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		if err := s.ensureEmbeddedVersion(ctx, entry.Name()); err != nil {
			return err
		}
	}

	return nil
}

// ensureEmbeddedVersion ensures the complete version of one module shipped in
// the image: the object name carries a sanitized version, the spec keeps the
// raw one, and the metadata comes from the module files on disk.
func (s *Syncer) ensureEmbeddedVersion(ctx context.Context, dirName string) error {
	moduleDir := filepath.Join(s.embeddedModulesDir, dirName)

	def, err := loader.LoadEmbeddedDefinition(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable definition, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	name := v1alpha1.MakeEmbeddedModulePackageVersionName(def.Name, s.deckhouseVersion)
	if !s.validName(name, def.Name) {
		return nil
	}

	meta := metadata.FromPackageDefinition(def)

	// an embedded module carries its weight in the directory name prefix, which
	// the definition file usually omits
	if meta.Weight == 0 {
		meta.Weight = weightFromDirName(dirName)
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           def.Name,
		PackageRepositoryName: v1alpha1.PackageRepositoryNameEmbedded,
		PackageVersion:        s.deckhouseVersion,
	}

	return s.ensureFilled(ctx, name, spec, meta)
}

// weightFromDirName parses the "<weight>-<name>" contract of the embedded
// modules dir; a name without the prefix yields zero.
func weightFromDirName(dirName string) int32 {
	prefix, _, found := strings.Cut(dirName, "-")
	if !found {
		return 0
	}

	weight, err := strconv.Atoi(prefix)
	if err != nil {
		return 0
	}

	return int32(weight)
}

// syncReleases ensures a draft stub for every deployed or pending release.
func (s *Syncer) syncReleases(ctx context.Context) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for idx := range releases.Items {
		release := &releases.Items[idx]
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed &&
			release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		name, spec, ok := s.specForRelease(release)
		if !ok {
			continue
		}

		if err := s.ensureStub(ctx, name, spec); err != nil {
			return err
		}
	}

	return nil
}

// specForRelease derives the version name and spec from a release. A release
// without a source or with an unparsable version names no package version and
// is skipped with a warning.
func (s *Syncer) specForRelease(release *v1alpha1.ModuleRelease) (string, v1alpha1.ModulePackageVersionSpec, bool) {
	moduleName := release.GetModuleName()

	source := release.GetModuleSource()
	if source == "" {
		s.logger.Warn("release has no module source, skip its package version",
			slog.String("release", release.Name))

		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	parsed, err := semver.NewVersion(release.Spec.Version)
	if err != nil {
		s.logger.Warn("release version is not a semver, skip its package version",
			slog.String("release", release.Name), slog.String("version", release.Spec.Version), log.Err(err))

		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	version := "v" + parsed.String()
	repository := v1alpha1.PackageRepositoryNameForModuleSource(source)

	name := v1alpha1.MakeModulePackageVersionName(repository, moduleName, version)
	if !s.validName(name, moduleName) {
		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	return name, v1alpha1.ModulePackageVersionSpec{
		PackageName:           moduleName,
		PackageRepositoryName: repository,
		PackageVersion:        version,
	}, true
}

// validName reports whether the composed object name is legal, warning when it
// is not: the spec fields are immutable, so a bad name must never be created.
func (s *Syncer) validName(name, moduleName string) bool {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		s.logger.Warn("package version name is not a valid object name, skip it",
			slog.String("name", name), slog.String("module", moduleName), slog.String("error", errs[0]))

		return false
	}

	return true
}

// ensureFilled converges the version to its complete form: created if missing,
// the metadata filled, no draft label. An existing complete version is left
// untouched. An existing draft, either a stub of an older build or a leftover
// of an interrupted fill, is completed in place.
func (s *Syncer) ensureFilled(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec, meta *v1alpha1.ModulePackageVersionStatusMetadata) error {
	mpv := new(v1alpha1.ModulePackageVersion)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, mpv)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

	if apierrors.IsNotFound(err) {
		// the draft label holds until the metadata lands, so no observer can
		// take a half-created version for a complete one
		mpv, err = s.createStub(ctx, name, spec)
		if err != nil {
			return err
		}
	}

	if !mpv.IsDraft() {
		return nil
	}

	if err := s.fillMetadata(ctx, mpv, meta); err != nil {
		return err
	}

	return s.removeDraft(ctx, mpv)
}

// ensureStub makes sure the version exists at least as a draft stub; any
// existing object, draft or complete, is left as is.
func (s *Syncer) ensureStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) error {
	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.ModulePackageVersion))
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

	if _, err := s.createStub(ctx, name, spec); err != nil {
		return err
	}

	return nil
}

// createStub creates the version as a draft with the labels the repository
// scan puts on the versions it creates itself.
func (s *Syncer) createStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) (*v1alpha1.ModulePackageVersion, error) {
	mpv := &v1alpha1.ModulePackageVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ModulePackageVersionGVK.GroupVersion().String(),
			Kind:       v1alpha1.ModulePackageVersionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage": "deckhouse",
				v1alpha1.ModulePackageVersionLabelRepository: spec.PackageRepositoryName,
				v1alpha1.ModulePackageVersionLabelPackage:    spec.PackageName,
				v1alpha1.ModulePackageVersionLabelDraft:      "true",
				v1alpha1.ModulePackageVersionLabelLegacy:     "true",
			},
		},
		Spec: spec,
	}

	if err := s.writer.Create(ctx, mpv); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create module package version '%s': %w", name, err)
		}

		// another writer created it between the read and this call; converge on
		// whatever exists now
		if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, mpv); err != nil {
			return nil, fmt.Errorf("get module package version '%s': %w", name, err)
		}
	}

	s.logger.Debug("module package version created", slog.String("name", name))

	return mpv, nil
}

// fillMetadata writes the disk-sourced metadata into the version status.
func (s *Syncer) fillMetadata(ctx context.Context, mpv *v1alpha1.ModulePackageVersion, meta *v1alpha1.ModulePackageVersionStatusMetadata) error {
	original := mpv.DeepCopy()

	mpv.Status.PackageMetadata = meta
	mpv.Status.ObservedGeneration = mpv.Generation

	metautils.SetStatusCondition(&mpv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ModulePackageVersionConditionReasonFilledFromDisk,
		ObservedGeneration: mpv.Generation,
		LastTransitionTime: metav1.NewTime(s.dc.GetClock().Now()),
	})

	if err := s.writer.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package version status '%s': %w", mpv.Name, err)
	}

	return nil
}

// removeDraft completes the version: without the draft label every observer may
// treat the metadata as final.
func (s *Syncer) removeDraft(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) error {
	original := mpv.DeepCopy()

	delete(mpv.Labels, v1alpha1.ModulePackageVersionLabelDraft)

	if err := s.writer.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package version '%s': %w", mpv.Name, err)
	}

	s.logger.Debug("module package version filled from disk", slog.String("name", mpv.Name))

	return nil
}

// syncPackages ensures a ModulePackage for every module the old module sources
// offer, so the catalog knows where each package is available before any
// repository scan ran.
func (s *Syncer) syncPackages(ctx context.Context) error {
	modules := new(v1alpha1.ModuleList)
	if err := s.reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for idx := range modules.Items {
		module := &modules.Items[idx]

		// an embedded module comes from no source; the scan never creates a
		// package for it either
		if module.IsEmbedded() || len(module.Properties.AvailableSources) == 0 {
			continue
		}

		if err := s.ensurePackage(ctx, module.Name, repositoriesForSources(module.Properties.AvailableSources)); err != nil {
			return err
		}
	}

	return nil
}

// repositoriesForSources maps module source names onto repository names,
// dropping the duplicates the mapping may fold together.
func repositoriesForSources(sources []string) []string {
	repositories := make([]string, 0, len(sources))

	for _, source := range sources {
		repository := v1alpha1.PackageRepositoryNameForModuleSource(source)
		if !slices.Contains(repositories, repository) {
			repositories = append(repositories, repository)
		}
	}

	return repositories
}

// ensurePackage makes sure the package exists with the given repositories in
// its status. An existing package is left untouched unless its repository list
// is empty, which a create interrupted before the status patch leaves behind;
// such a package is completed in place. No owner is set: the repositories the
// status names may not exist yet, and the scan adopts the package once one
// appears.
func (s *Syncer) ensurePackage(ctx context.Context, name string, repositories []string) error {
	pkg := new(v1alpha1.ModulePackage)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package '%s': %w", name, err)
	}

	if apierrors.IsNotFound(err) {
		pkg = &v1alpha1.ModulePackage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ModulePackageGVK.GroupVersion().String(),
				Kind:       v1alpha1.ModulePackageKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"heritage": "deckhouse"},
			},
		}

		if err := s.writer.Create(ctx, pkg); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("create module package '%s': %w", name, err)
			}

			// another writer created it between the read and this call;
			// converge on whatever exists now
			if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
				return fmt.Errorf("get module package '%s': %w", name, err)
			}
		}
	}

	if len(pkg.Status.AvailableRepositories) > 0 {
		return nil
	}

	original := pkg.DeepCopy()
	pkg.Status.AvailableRepositories = repositories

	if err := s.writer.Status().Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package status '%s': %w", name, err)
	}

	s.logger.Debug("module package created", slog.String("name", name))

	return nil
}
