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

package release

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type releasePredictor struct {
	ts metav1.Time

	releases              []*v1alpha1.ModuleRelease
	currentReleaseIndex   int
	desiredReleaseIndex   int
	skippedPatchesIndexes []int
}

func newReleasePredictor(releases []*v1alpha1.ModuleRelease) *releasePredictor {
	return &releasePredictor{
		ts:       metav1.NewTime(time.Now().UTC()),
		releases: releases,

		currentReleaseIndex:   -1,
		desiredReleaseIndex:   -1,
		skippedPatchesIndexes: make([]int, 0),
	}
}

func (rp *releasePredictor) calculateRelease() {
	for index, rl := range rp.releases {
		if rl.Status.Phase == v1alpha1.PhaseDeployed {
			rp.currentReleaseIndex = index
			break
		}
	}

	for index, rl := range rp.releases {
		if rl.Status.Phase == v1alpha1.PhasePending {
			if rp.desiredReleaseIndex >= 0 {
				previousPredictedRelease := rp.releases[rp.desiredReleaseIndex]
				if previousPredictedRelease.Spec.Version.Major() != rl.Spec.Version.Major() {
					continue
				}

				if previousPredictedRelease.Spec.Version.Minor() != rl.Spec.Version.Minor() {
					continue
				}
				// it's a patch for predicted release, continue
				rp.skippedPatchesIndexes = append(rp.skippedPatchesIndexes, rp.desiredReleaseIndex)
			}

			// if we have a deployed a release
			if rp.currentReleaseIndex >= 0 {
				// if deployed version is greater than the pending one, this pending release should be superseded
				if rp.releases[rp.currentReleaseIndex].Spec.Version.GreaterThan(rl.Spec.Version) {
					rp.skippedPatchesIndexes = append(rp.skippedPatchesIndexes, index)
					continue
				}
			}
			// in other cases we have a new desired version of a module
			rp.desiredReleaseIndex = index
		}
	}
}
