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

// Package versionsync creates a ModulePackageVersion for every module package
// the old module stack already carries in the cluster, so the package system
// sees the versions the cluster runs or is about to run.
//
// The versions are created as draft stubs: only the spec is derivable from the
// older resources. The draft label hands a stub to the module-package-version
// controller, which fills the metadata from the package repository once one
// exists; the legacy label keeps the registry path of the module source world
// ("<module>/release"). No owner is set: the repository the spec names may not
// exist yet, and an owner reference to a missing object would get the version
// garbage-collected. The repository scan adopts the stubs it recognizes.
//
// # Data sources for the versions
//
//	v1alpha1 Module marked embedded
//	  └─ embedded-<module>-<deckhouse version>
//
//	deployed or pending ModuleRelease
//	  └─ <repository>-<module>-<version>, where the "deckhouse" source
//	     maps to the "deckhouse-modules" repository
//
// An existing version is never touched, so a restart changes nothing.
package versionsync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Sync ensures the package versions of the old module stack, reading the
// cluster through reader and creating the missing ones through writer. It runs
// once at start, while the controllers still wait for the sync phase.
//
// A release naming no valid version (no source, an unparsable version, an
// illegal object name) is skipped with a warning; an API failure stops the
// sync.
func Sync(ctx context.Context, reader client.Reader, writer client.Client, deckhouseVersion string, logger *log.Logger) error {
	modules := new(v1alpha1.ModuleList)
	if err := reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	releases := new(v1alpha1.ModuleReleaseList)
	if err := reader.List(ctx, releases); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for idx := range modules.Items {
		module := &modules.Items[idx]
		if !module.IsEmbedded() {
			continue
		}

		if err := ensureEmbeddedVersion(ctx, reader, writer, module.Name, deckhouseVersion, logger); err != nil {
			return err
		}
	}

	for idx := range releases.Items {
		release := &releases.Items[idx]
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed &&
			release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		name, spec, ok := specForRelease(release, logger)
		if !ok {
			continue
		}

		if err := ensureVersion(ctx, reader, writer, name, spec, logger); err != nil {
			return err
		}
	}

	return nil
}

// ensureEmbeddedVersion ensures the version of a module shipped in the running
// image. The object name carries a sanitized version, while the spec keeps the
// raw one.
func ensureEmbeddedVersion(ctx context.Context, reader client.Reader, writer client.Client, moduleName, deckhouseVersion string, logger *log.Logger) error {
	name := v1alpha1.MakeEmbeddedModulePackageVersionName(moduleName, deckhouseVersion)
	if !validName(name, moduleName, logger) {
		return nil
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           moduleName,
		PackageRepositoryName: v1alpha1.PackageRepositoryNameEmbedded,
		PackageVersion:        deckhouseVersion,
	}

	return ensureVersion(ctx, reader, writer, name, spec, logger)
}

// specForRelease derives the version name and spec from a release. A release
// without a source or with an unparsable version names no package version and
// is skipped with a warning.
func specForRelease(release *v1alpha1.ModuleRelease, logger *log.Logger) (string, v1alpha1.ModulePackageVersionSpec, bool) {
	moduleName := release.GetModuleName()

	source := release.GetModuleSource()
	if source == "" {
		logger.Warn("release has no module source, skip its package version",
			slog.String("release", release.Name))

		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	parsed, err := semver.NewVersion(release.Spec.Version)
	if err != nil {
		logger.Warn("release version is not a semver, skip its package version",
			slog.String("release", release.Name), slog.String("version", release.Spec.Version), log.Err(err))

		return "", v1alpha1.ModulePackageVersionSpec{}, false
	}

	version := "v" + parsed.String()
	repository := v1alpha1.PackageRepositoryNameForModuleSource(source)

	name := v1alpha1.MakeModulePackageVersionName(repository, moduleName, version)
	if !validName(name, moduleName, logger) {
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
func validName(name, moduleName string, logger *log.Logger) bool {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		logger.Warn("package version name is not a valid object name, skip it",
			slog.String("name", name), slog.String("module", moduleName), slog.String("error", errs[0]))

		return false
	}

	return true
}

// ensureVersion makes sure the version exists at least as a draft stub; any
// existing object, draft or complete, is left as is. The stub carries the
// labels the repository scan puts on the versions it creates itself.
func ensureVersion(ctx context.Context, reader client.Reader, writer client.Client, name string, spec v1alpha1.ModulePackageVersionSpec, logger *log.Logger) error {
	err := reader.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.ModulePackageVersion))
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package version '%s': %w", name, err)
	}

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

	// a concurrent writer may have created the version after the read
	if err := writer.Create(ctx, mpv); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create module package version '%s': %w", name, err)
	}

	logger.Debug("module package version created", slog.String("name", name))

	return nil
}
