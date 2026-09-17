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
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// syncModulePackageVersions ensures a version object for every module package
// the old stack carries: embedded modules and the global one come out complete
// with the disk metadata and schemas, deployed and pending releases become
// draft stubs.
func (s *syncer) syncModulePackageVersions(ctx context.Context) error {
	if err := s.syncModulePackageVersionsFromImage(ctx); err != nil {
		return err
	}

	if err := s.syncGlobalModulePackageVersion(ctx); err != nil {
		return err
	}

	return s.syncModulePackageVersionsFromModuleReleases(ctx)
}

// syncModulePackageVersionsFromImage walks the embedded modules dir and ensures a complete
// version for every module the running image ships.
func (s *syncer) syncModulePackageVersionsFromImage(ctx context.Context) error {
	version := app.EmbeddedPackageVersion(s.deckhouseVersion)

	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return fmt.Errorf("read embedded modules dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		if err := s.ensureEmbeddedModulePackageVersion(ctx, entry.Name(), version); err != nil {
			return err
		}
	}

	return nil
}

// ensureEmbeddedModulePackageVersion ensures the complete version of one module shipped in
// the image; the metadata and the settings/values schemas come from the
// module files on disk.
func (s *syncer) ensureEmbeddedModulePackageVersion(ctx context.Context, dirName, version string) error {
	moduleDir := filepath.Join(s.embeddedModulesDir, dirName)

	def, err := loader.LoadEmbeddedDefinition(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable definition, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	name := v1alpha1.MakeModulePackageVersionName(repositoryNameEmbedded, def.Name, version)
	if !s.validModulePackageVersionName(name, def.Name) {
		return nil
	}

	// no repository offers an embedded package, so no scan ever creates its
	// catalog entry; the sync does
	if err := s.ensureModulePackageExists(ctx, def.Name); err != nil {
		return err
	}

	meta := def.ConvertToStatusMetadata()

	settingsRaw, valuesRaw, err := loader.LoadEmbeddedSchemas(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable schemas, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	schemas, err := v1alpha1.ParsePackageSchemas(settingsRaw, valuesRaw)
	if err != nil {
		s.logger.Warn("module schemas do not parse, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           def.Name,
		PackageRepositoryName: repositoryNameEmbedded,
		PackageVersion:        version,
	}

	return s.ensureFilledModulePackageVersion(ctx, name, spec, meta, schemas)
}

// syncGlobalModulePackageVersion ensures the complete version of the global module, which
// the image ships like an embedded one and names the same way.
//
// It differs from an embedded module in what disk can tell about it: the global
// hooks dir holds no definition file - the hooks are compiled into the binary -
// so the metadata stays empty and only the settings and values schemas are
// filled. An empty metadata object rather than none keeps every reader that
// reaches into it, such as the exclusive group and stage getters, off a nil
// dereference. The version is written even when the dir yields no schemas at
// all: the image always ships them, and the package and the version are what
// the module controller gates registration on, so withholding them over an
// unreadable dir would strand the global Module rather than degrade it.
func (s *syncer) syncGlobalModulePackageVersion(ctx context.Context) error {
	version := app.EmbeddedPackageVersion(s.deckhouseVersion)

	name := v1alpha1.MakeModulePackageVersionName(repositoryNameEmbedded, packageNameGlobal, version)
	if !s.validModulePackageVersionName(name, packageNameGlobal) {
		return nil
	}

	settingsRaw, valuesRaw, err := loader.LoadGlobalSchemas(s.globalHooksDir)
	if err != nil {
		s.logger.Warn("global hooks dir holds no readable schemas, skip its package version",
			slog.String("dir", s.globalHooksDir), log.Err(err))

		return nil
	}

	schemas, err := v1alpha1.ParsePackageSchemas(settingsRaw, valuesRaw)
	if err != nil {
		s.logger.Warn("global schemas do not parse, skip its package version",
			slog.String("dir", s.globalHooksDir), log.Err(err))

		return nil
	}

	// no repository offers the global package either, so the sync creates its catalog entry
	if err := s.ensureModulePackageExists(ctx, packageNameGlobal); err != nil {
		return err
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           packageNameGlobal,
		PackageRepositoryName: repositoryNameEmbedded,
		PackageVersion:        version,
	}

	return s.ensureFilledModulePackageVersion(ctx, name, spec, new(v1alpha1.ModulePackageVersionStatusMetadata), schemas)
}

// syncModulePackageVersionsFromModuleReleases ensures a draft stub for every deployed or pending release.
func (s *syncer) syncModulePackageVersionsFromModuleReleases(ctx context.Context) error {
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

		name, spec, ok := s.modulePackageVersionSpecForModuleRelease(release)
		if !ok {
			continue
		}

		if err := s.ensureModulePackageVersionStub(ctx, name, spec); err != nil {
			return err
		}
	}

	return nil
}

// modulePackageVersionSpecForModuleRelease derives the version name and spec from a release. A release
// without a source or with an unparsable version names no package version and
// is skipped with a warning.
func (s *syncer) modulePackageVersionSpecForModuleRelease(release *v1alpha1.ModuleRelease) (string, v1alpha1.ModulePackageVersionSpec, bool) {
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
	repository := PackageRepositoryNameForModuleSource(source)

	name := v1alpha1.MakeModulePackageVersionName(repository, moduleName, version)
	if !s.validModulePackageVersionName(name, moduleName) {
		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	return name, v1alpha1.ModulePackageVersionSpec{
		PackageName:           moduleName,
		PackageRepositoryName: repository,
		PackageVersion:        version,
	}, true
}

// validModulePackageVersionName reports whether the composed object name is legal, warning when it
// is not: the spec fields are immutable, so a bad name must never be created.
func (s *syncer) validModulePackageVersionName(name, moduleName string) bool {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		s.logger.Warn("package version name is not a valid object name, skip it",
			slog.String("name", name), slog.String("module", moduleName), slog.String("error", errs[0]))

		return false
	}

	return true
}

// ensureFilledModulePackageVersion converges the version to the disk content: created if missing,
// the metadata and schemas brought to what the module files hold, no draft
// label. One version name spans every rebuild of a release, so a complete
// version whose status drifted from the disk is refreshed in place; a
// matching one is left untouched. An existing draft, either a stub of an
// older build or a leftover of an interrupted fill, is completed the same
// way.
func (s *syncer) ensureFilledModulePackageVersion(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec, meta *v1alpha1.ModulePackageVersionStatusMetadata, schemas *v1alpha1.PackageVersionStatusSchemas) error {
	mpv := new(v1alpha1.ModulePackageVersion)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, mpv)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

	if apierrors.IsNotFound(err) {
		// the draft label holds until the metadata lands, so no observer can
		// take a half-created version for a complete one
		mpv, err = s.createModulePackageVersionStub(ctx, name, spec)
		if err != nil {
			return err
		}
	}

	if !mpv.IsDraft() &&
		equality.Semantic.DeepEqual(mpv.Status.PackageMetadata, meta) &&
		equality.Semantic.DeepEqual(mpv.Status.PackageSchemas, schemas) {
		return nil
	}

	if err := s.fillModulePackageVersionStatus(ctx, mpv, meta, schemas); err != nil {
		return err
	}

	if !mpv.IsDraft() {
		s.logger.Debug("module package version refreshed from disk", slog.String("name", mpv.Name))

		return nil
	}

	return s.removeModulePackageVersionDraft(ctx, mpv)
}

// ensureModulePackageVersionStub makes sure the version exists at least as a draft stub; any
// existing object, draft or complete, is left as is.
func (s *syncer) ensureModulePackageVersionStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) error {
	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.ModulePackageVersion))
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

	if _, err := s.createModulePackageVersionStub(ctx, name, spec); err != nil {
		return err
	}

	return nil
}

// createModulePackageVersionStub creates the version as a draft with the labels the repository
// scan puts on the versions it creates itself.
func (s *syncer) createModulePackageVersionStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) (*v1alpha1.ModulePackageVersion, error) {
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

// fillModulePackageVersionStatus writes the disk-sourced metadata and schemas into the version status.
func (s *syncer) fillModulePackageVersionStatus(ctx context.Context, mpv *v1alpha1.ModulePackageVersion, meta *v1alpha1.ModulePackageVersionStatusMetadata, schemas *v1alpha1.PackageVersionStatusSchemas) error {
	original := mpv.DeepCopy()

	mpv.Status.PackageMetadata = meta
	mpv.Status.PackageSchemas = schemas
	mpv.Status.ObservedGeneration = mpv.Generation

	metautils.SetStatusCondition(&mpv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionTrue,
		Reason:             "Succeeded",
		ObservedGeneration: mpv.Generation,
		LastTransitionTime: metav1.NewTime(s.dc.GetClock().Now()),
	})

	if err := s.writer.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package version status '%s': %w", mpv.Name, err)
	}

	return nil
}

// removeModulePackageVersionDraft completes the version: without the draft label every observer may
// treat the metadata as final.
func (s *syncer) removeModulePackageVersionDraft(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) error {
	original := mpv.DeepCopy()

	delete(mpv.Labels, v1alpha1.ModulePackageVersionLabelDraft)

	if err := s.writer.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package version '%s': %w", mpv.Name, err)
	}

	s.logger.Debug("module package version filled from disk", slog.String("name", mpv.Name))

	return nil
}
