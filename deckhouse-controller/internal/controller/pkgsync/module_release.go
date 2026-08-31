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

// This file reads the module releases once per sync: the version pass takes
// draft stubs from the snapshot, the module pass takes origins. The file dies
// together with the ModuleRelease deprecation.

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// releaseFacts is what one pass over the module releases yields.
type releaseFacts struct {
	// stubs name the package versions the surviving deployed and the pending
	// releases carry.
	stubs []releaseStub

	// origins point every module with a deployed release at its package.
	origins map[string]Origin
}

// releaseStub is a package version a release names, to be created as a draft.
type releaseStub struct {
	name string
	spec v1alpha1.ModulePackageVersionSpec
}

// deployedRelease is a deployed release with its version already parsed.
type deployedRelease struct {
	release *v1alpha1.ModuleRelease
	version *semver.Version
}

// resolveReleases derives the version stubs and the module origins from one
// snapshot of the module releases. A duplicate deployed release of one module
// is demoted to Superseded and names no stub and no origin - two releases
// both marked deployed is what a restart mid version bump leaves behind. A
// release with a non-semver version or without a source names none either
// and is skipped with a warning.
func (s *syncer) resolveReleases(ctx context.Context) (releaseFacts, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases); err != nil {
		return releaseFacts{}, fmt.Errorf("list module releases: %w", err)
	}

	facts := releaseFacts{origins: make(map[string]Origin)}
	deployed := make([]deployedRelease, 0, len(releases.Items))

	for idx := range releases.Items {
		release := &releases.Items[idx]

		// a release on its way out names no package version and no origin
		if !release.DeletionTimestamp.IsZero() {
			continue
		}

		phase := release.Status.Phase
		if phase != v1alpha1.ModuleReleasePhaseDeployed && phase != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		version, err := semver.NewVersion(release.Spec.Version)
		if err != nil {
			s.logger.Warn("release version is not a semver, skip the release",
				slog.String("release", release.Name), slog.String("version", release.Spec.Version), log.Err(err))

			continue
		}

		if phase == v1alpha1.ModuleReleasePhasePending {
			if stub, ok := s.stubForRelease(release, version); ok {
				facts.stubs = append(facts.stubs, stub)
			}

			continue
		}

		deployed = append(deployed, deployedRelease{release: release, version: version})
	}

	// newest first, so the first deployed release a module shows is the one that stays
	slices.SortFunc(deployed, func(a, b deployedRelease) int {
		return b.version.Compare(a.version)
	})

	for _, item := range deployed {
		moduleName := item.release.GetModuleName()

		// superseding is hygiene on its own: it runs even when a
		// higher-precedence source owns the module
		if _, ok := facts.origins[moduleName]; ok {
			if err := s.supersedeModuleRelease(ctx, item.release); err != nil {
				s.logger.Error("failed to supersede the deployed module release",
					slog.String("name", item.release.Name), log.Err(err))
			}

			continue
		}

		// a release with neither a ModuleSource owner nor a source label names
		// no repository, so it cannot say where the package comes from
		source := item.release.GetModuleSource()
		if source == "" {
			s.logger.Warn("release has no module source, skip the release",
				slog.String("release", item.release.Name))

			continue
		}

		facts.origins[moduleName] = Origin{
			RepositoryName: source,
			PackageVersion: "v" + item.version.String(),
		}

		if stub, ok := s.stubForRelease(item.release, item.version); ok {
			facts.stubs = append(facts.stubs, stub)
		}
	}

	return facts, nil
}

// stubForRelease derives the version name and spec a release names. A release
// without a source or with an illegal composed name yields no stub and is
// skipped with a warning: the spec fields are immutable, so a bad name must
// never be created.
func (s *syncer) stubForRelease(release *v1alpha1.ModuleRelease, version *semver.Version) (releaseStub, bool) {
	moduleName := release.GetModuleName()

	source := release.GetModuleSource()
	if source == "" {
		s.logger.Warn("release has no module source, skip its package version",
			slog.String("release", release.Name))

		return releaseStub{}, false
	}

	packageVersion := "v" + version.String()
	repository := repositoryNameForSource(source)

	name := v1alpha1.MakeModulePackageVersionName(repository, moduleName, packageVersion)
	if !s.validName(name, moduleName) {
		return releaseStub{}, false
	}

	return releaseStub{
		name: name,
		spec: v1alpha1.ModulePackageVersionSpec{
			PackageName:           moduleName,
			PackageRepositoryName: repository,
			PackageVersion:        packageVersion,
		},
	}, true
}

// supersedeModuleRelease marks a deployed release that a newer deployed one replaced.
func (s *syncer) supersedeModuleRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	superseded := release.DeepCopy()
	superseded.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
	superseded.Status.Message = ""
	superseded.Status.TransitionTime = metav1.NewTime(s.dc.GetClock().Now().UTC())

	if err := s.writer.Status().Patch(ctx, superseded, client.MergeFrom(release)); err != nil {
		return fmt.Errorf("patch module release status: %w", err)
	}

	return nil
}
