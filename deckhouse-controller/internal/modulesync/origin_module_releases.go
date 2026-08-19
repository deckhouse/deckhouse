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

package modulesync

// This file resolves origins from ModuleRelease resources, which the package
// system replaces. It dies together with their deprecation; the rest of the
// package touches it only through mergeOrigins.

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// originsFromModuleReleases returns the newest deployed release per module, superseding the
// duplicates it passes - two releases both marked deployed is what a restart mid version bump
// leaves behind.
func (s *Syncer) originsFromModuleReleases(ctx context.Context) (map[string]Origin, error) {
	selector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed}

	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases, selector); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	// newest first, so every deployed release a module has after its first is superseded by it
	slices.SortFunc(releases.Items, func(a, b v1alpha1.ModuleRelease) int {
		return b.GetVersion().Compare(a.GetVersion())
	})

	origins := make(map[string]Origin, len(releases.Items))

	for _, release := range releases.Items {
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.DeletionTimestamp.IsZero() {
			continue
		}

		name := release.GetModuleName()

		// superseding is hygiene on its own: it runs even for modules a higher-precedence source owns
		if _, ok := origins[name]; ok {
			if err := s.supersedeModuleRelease(ctx, &release); err != nil {
				s.logger.Error("failed to supersede the deployed module release",
					slog.String("name", release.GetName()), log.Err(err))
			}

			continue
		}

		origins[name] = Origin{RepositoryName: release.GetModuleSource(), PackageVersion: release.GetModuleVersion()}
	}

	return origins, nil
}

// supersedeModuleRelease marks a deployed release that a newer deployed one replaced.
func (s *Syncer) supersedeModuleRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	superseded := release.DeepCopy()
	superseded.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
	superseded.Status.Message = ""
	superseded.Status.TransitionTime = metav1.NewTime(s.clock.Now().UTC())

	if err := s.writer.Status().Patch(ctx, superseded, client.MergeFrom(release)); err != nil {
		return fmt.Errorf("patch module release status: %w", err)
	}

	return nil
}
