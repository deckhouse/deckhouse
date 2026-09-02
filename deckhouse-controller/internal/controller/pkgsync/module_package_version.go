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
	"strconv"
	"strings"

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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// syncModulePackageVersions ensures a version object for every module package
// the old stack carries: embedded modules come out complete with the disk
// metadata and schemas, deployed and pending releases become draft stubs.
func (s *syncer) syncModulePackageVersions(ctx context.Context) error {
	if err := s.syncVersionsFromImage(ctx); err != nil {
		return err
	}

	return s.syncVersionsFromReleases(ctx)
}

// syncVersionsFromImage walks the embedded modules dir and ensures a complete
// version for every module the running image ships.
func (s *syncer) syncVersionsFromImage(ctx context.Context) error {
	version := app.EmbeddedPackageVersion(s.deckhouseVersion)

	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return fmt.Errorf("read embedded modules dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		if err := s.ensureEmbeddedVersion(ctx, entry.Name(), version); err != nil {
			return err
		}
	}

	return nil
}

// ensureEmbeddedVersion ensures the complete version of one module shipped in
// the image; the metadata and the settings/values schemas come from the
// module files on disk.
func (s *syncer) ensureEmbeddedVersion(ctx context.Context, dirName, version string) error {
	moduleDir := filepath.Join(s.embeddedModulesDir, dirName)

	def, err := loader.LoadEmbeddedDefinition(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable definition, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	name := v1alpha1.MakeModulePackageVersionName(repositoryNameEmbedded, def.Name, version)
	if !s.validName(name, def.Name) {
		return nil
	}

	// no repository offers an embedded package, so no scan ever creates its
	// catalog entry; the sync does
	if err := s.ensureModulePackageExists(ctx, def.Name); err != nil {
		return err
	}

	meta, schemas, err := versionFromDir(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable schemas, skip its package version",
			slog.String("dir", moduleDir), log.Err(err))

		return nil
	}

	// an embedded module carries its weight in the directory name prefix, which
	// the definition file usually omits
	if meta.Weight == 0 {
		meta.Weight = weightFromDirName(dirName)
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           def.Name,
		PackageRepositoryName: repositoryNameEmbedded,
		PackageVersion:        version,
	}

	return s.ensureFilled(ctx, name, spec, meta, schemas)
}

// versionFromDir reads what a version carries from the module files: the
// definition as the metadata and the settings/values schemas.
func versionFromDir(moduleDir string) (*v1alpha1.ModulePackageVersionStatusMetadata, *v1alpha1.PackageVersionStatusSchemas, error) {
	def, err := loader.LoadEmbeddedDefinition(moduleDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load definition: %w", err)
	}

	settingsRaw, valuesRaw, err := loader.LoadEmbeddedSchemas(moduleDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load schemas: %w", err)
	}

	schemas, err := v1alpha1.ParsePackageSchemas(settingsRaw, valuesRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse schemas: %w", err)
	}

	return def.ConvertToStatusMetadata(), schemas, nil
}

// EnsureModulePackageVersion completes the version of a module installed from a
// repository with the module files in moduleDir: the version is created as a
// draft when missing, its metadata and schemas are filled from the files and
// the draft label is dropped. A version that is already complete is final:
// the repository scan described it, and the disk must not overwrite that. The
// callers run where the files are certainly on disk, the module loader after a
// restore and the release controller at deploy, so the metadata reaches the
// readers without the package feature and its promoter.
func EnsureModulePackageVersion(ctx context.Context, reader client.Reader, writer client.Client, dc dependency.Container, spec v1alpha1.ModulePackageVersionSpec, moduleDir string, logger *log.Logger) error {
	return newSyncer(reader, writer, dc, "", "", "", logger).ensureDraftFilled(ctx, spec, moduleDir)
}

// ensureDraftFilled fills the version named by the spec from the module files
// unless it is already complete.
func (s *syncer) ensureDraftFilled(ctx context.Context, spec v1alpha1.ModulePackageVersionSpec, moduleDir string) error {
	name := v1alpha1.MakeModulePackageVersionName(spec.PackageRepositoryName, spec.PackageName, spec.PackageVersion)
	if !s.validName(name, spec.PackageName) {
		return nil
	}

	mpv := new(v1alpha1.ModulePackageVersion)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, mpv)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

	missing := apierrors.IsNotFound(err)
	if !missing && !mpv.IsDraft() {
		return nil
	}

	meta, schemas, err := versionFromDir(moduleDir)
	if err != nil {
		s.logger.Warn("module dir holds no readable package files, leave its package version as is",
			slog.String("dir", moduleDir), slog.String("name", name), log.Err(err))

		return nil
	}

	if missing {
		mpv, err = s.createStub(ctx, name, spec)
		if err != nil {
			return err
		}
	}

	if err := s.fillMetadata(ctx, mpv, meta, schemas); err != nil {
		return err
	}

	return s.removeDraft(ctx, mpv)
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

// syncVersionsFromReleases ensures a draft stub for every deployed or pending release.
func (s *syncer) syncVersionsFromReleases(ctx context.Context) error {
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
func (s *syncer) specForRelease(release *v1alpha1.ModuleRelease) (string, v1alpha1.ModulePackageVersionSpec, bool) {
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
	repository := RepositoryNameForSource(source)

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
func (s *syncer) validName(name, moduleName string) bool {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		s.logger.Warn("package version name is not a valid object name, skip it",
			slog.String("name", name), slog.String("module", moduleName), slog.String("error", errs[0]))

		return false
	}

	return true
}

// ensureFilled converges the version to the disk content: created if missing,
// the metadata and schemas brought to what the module files hold, no draft
// label. One version name spans every rebuild of a release, so a complete
// version whose status drifted from the disk is refreshed in place; a
// matching one is left untouched. An existing draft, either a stub of an
// older build or a leftover of an interrupted fill, is completed the same
// way.
func (s *syncer) ensureFilled(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec, meta *v1alpha1.ModulePackageVersionStatusMetadata, schemas *v1alpha1.PackageVersionStatusSchemas) error {
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

	if !mpv.IsDraft() &&
		equality.Semantic.DeepEqual(mpv.Status.PackageMetadata, meta) &&
		equality.Semantic.DeepEqual(mpv.Status.PackageSchemas, schemas) {
		return nil
	}

	if err := s.fillMetadata(ctx, mpv, meta, schemas); err != nil {
		return err
	}

	if !mpv.IsDraft() {
		s.logger.Debug("module package version refreshed from disk", slog.String("name", mpv.Name))

		return nil
	}

	return s.removeDraft(ctx, mpv)
}

// ensureStub makes sure the version exists at least as a draft stub; any
// existing object, draft or complete, is left as is.
func (s *syncer) ensureStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) error {
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
func (s *syncer) createStub(ctx context.Context, name string, spec v1alpha1.ModulePackageVersionSpec) (*v1alpha1.ModulePackageVersion, error) {
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

// fillMetadata writes the disk-sourced metadata and schemas into the version status.
func (s *syncer) fillMetadata(ctx context.Context, mpv *v1alpha1.ModulePackageVersion, meta *v1alpha1.ModulePackageVersionStatusMetadata, schemas *v1alpha1.PackageVersionStatusSchemas) error {
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

// removeDraft completes the version: without the draft label every observer may
// treat the metadata as final.
func (s *syncer) removeDraft(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) error {
	original := mpv.DeepCopy()

	delete(mpv.Labels, v1alpha1.ModulePackageVersionLabelDraft)

	if err := s.writer.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package version '%s': %w", mpv.Name, err)
	}

	s.logger.Debug("module package version filled from disk", slog.String("name", mpv.Name))

	return nil
}
