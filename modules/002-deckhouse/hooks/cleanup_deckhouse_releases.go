/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/internal/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/internal/updater"
)

/*
  This hook handle invalid situation when more then 1 Deployed release exists at the moment:
    Hook move all releases except the latest one to the Outdated state

  The hook will keep only 10 Outdated releases, removing others
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/cleanup_deckhouse_release",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "DeckhouseRelease",
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(true),
			FilterFunc:                   filterDeckhouseRelease,
		},
	},
}, cleanupReleases)

func cleanupReleases(input *go_hook.HookInput) error {
	snap := input.Snapshots["releases"]
	if len(snap) == 0 {
		return nil
	}

	now := time.Now().UTC()

	releases := make([]updater.DeckhouseRelease, 0, len(snap))
	for _, sn := range snap {
		releases = append(releases, sn.(updater.DeckhouseRelease))
	}

	sort.Sort(sort.Reverse(updater.ByVersion(releases)))

	var (
		pendingReleasesIndexes  []int
		deployedReleasesIndexes []int
		outdatedReleasesIndexes []int
	)

	for i, release := range releases {
		switch release.Status.Phase {
		case v1alpha1.PhasePending:
			pendingReleasesIndexes = append(pendingReleasesIndexes, i)

		case v1alpha1.PhaseDeployed:
			deployedReleasesIndexes = append(deployedReleasesIndexes, i)

		case v1alpha1.PhaseOutdated, v1alpha1.PhaseSuspended:
			outdatedReleasesIndexes = append(outdatedReleasesIndexes, i)
		}
	}

	if len(deployedReleasesIndexes) > 1 {
		// cleanup releases stacked in Deployed status
		sp := updater.StatusPatch{
			Phase:          v1alpha1.PhaseOutdated,
			TransitionTime: now,
		}
		// everything except the last Deployed release
		for i := 1; i < len(deployedReleasesIndexes); i++ {
			release := releases[i]
			input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}
	}

	// save only last 10 outdated releases
	if len(outdatedReleasesIndexes) > 10 {
		for i := 10; i < len(outdatedReleasesIndexes); i++ {
			release := releases[i]
			input.PatchCollector.Delete("deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.InBackground())
		}
	}

	// some old releases, for example - when downgrade the release channel
	// mark them as Outdated
	if len(deployedReleasesIndexes) > 0 && len(pendingReleasesIndexes) > 0 {
		lastDeployed := deployedReleasesIndexes[0] // releases are reversed, that's why we have to take the first one (latest Deployed release)
		sp := updater.StatusPatch{
			Phase:          v1alpha1.PhaseOutdated,
			Message:        "Outdated by cleanup hook",
			TransitionTime: now,
		}

		for _, index := range pendingReleasesIndexes {
			if index <= lastDeployed {
				continue
			}

			release := releases[index]
			input.PatchCollector.MergePatch(sp, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}
	}

	return nil
}
