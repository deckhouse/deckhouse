/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouse_release

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8updater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release/updater"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func (r *deckhouseReleaseReconciler) cleanupDeckhouseReleaseLoop(ctx context.Context) {
	for {
		err := r.cleanupDeckhouseRelease(ctx)
		if err != nil {
			r.logger.Errorf("check Deckhouse release: %s", err)
		}

		time.Sleep(24 * time.Hour)
	}
}

func (r *deckhouseReleaseReconciler) cleanupDeckhouseRelease(ctx context.Context) error {
	var releases v1alpha1.DeckhouseReleaseList
	err := r.client.List(ctx, &releases)
	if err != nil {
		return fmt.Errorf("get deckhouse releases: %w", err)
	}

	pointerReleases := make([]*v1alpha1.DeckhouseRelease, 0, len(releases.Items))
	for _, r := range releases.Items {
		pointerReleases = append(pointerReleases, &r)
	}
	sort.Sort(sort.Reverse(updater.ByVersion[*v1alpha1.DeckhouseRelease](pointerReleases)))

	now := r.dc.GetClock().Now()

	var (
		pendingReleasesIndexes  []int
		deployedReleasesIndexes []int
		outdatedReleasesIndexes []int // outdated: skipped, superseded and suspended releases
	)

	for i, release := range pointerReleases {
		switch release.Status.Phase {
		case v1alpha1.PhasePending:
			pendingReleasesIndexes = append(pendingReleasesIndexes, i)

		case v1alpha1.PhaseDeployed:
			deployedReleasesIndexes = append(deployedReleasesIndexes, i)

		case v1alpha1.PhaseSuperseded, v1alpha1.PhaseSkipped, v1alpha1.PhaseSuspended:
			outdatedReleasesIndexes = append(outdatedReleasesIndexes, i)
		}
	}

	if len(deployedReleasesIndexes) > 1 {
		// cleanup releases stacked in Deployed status
		sp, _ := json.Marshal(d8updater.StatusPatch{
			Phase:          v1alpha1.PhaseSuperseded,
			TransitionTime: metav1.NewTime(now),
		})
		// everything except the last Deployed release
		for i := 1; i < len(deployedReleasesIndexes); i++ {
			index := deployedReleasesIndexes[i]
			release := pointerReleases[index]
			err = r.client.Status().Patch(ctx, release, client.RawPatch(types.MergePatchType, sp))
			if err != nil {
				return fmt.Errorf("patch release %v: %w", release.Name, err)
			}
		}
	}

	// save only last 10 outdated releases
	if len(outdatedReleasesIndexes) > 10 {
		for i := 10; i < len(outdatedReleasesIndexes); i++ {
			index := outdatedReleasesIndexes[i]
			release := pointerReleases[index]
			err = r.client.Delete(ctx, release, client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil {
				return fmt.Errorf("delete release %v: %w", release.Name, err)
			}
		}
	}

	// some old releases, for example - when downgrade the release channel
	// mark them as Skipped
	if len(deployedReleasesIndexes) > 0 && len(pendingReleasesIndexes) > 0 {
		lastDeployed := deployedReleasesIndexes[0] // releases are reversed, that's why we have to take the first one (latest Deployed release)
		sp, _ := json.Marshal(d8updater.StatusPatch{
			Phase:          v1alpha1.PhaseSkipped,
			Message:        "Skipped by cleanup hook",
			TransitionTime: metav1.NewTime(now),
		})

		for _, index := range pendingReleasesIndexes {
			if index <= lastDeployed {
				continue
			}

			release := pointerReleases[index]
			err = r.client.Status().Patch(ctx, release, client.RawPatch(types.MergePatchType, sp))
			if err != nil {
				return fmt.Errorf("patch release %v: %w", release.Name, err)
			}
		}
	}

	return nil
}
