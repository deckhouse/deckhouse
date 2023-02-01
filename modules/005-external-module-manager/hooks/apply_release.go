/*
Copyright 2023 Flant JSC

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
	"os"
	"path"
	"sort"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/005-external-module-manager/hooks/internal/apis/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/external-module-source/apply-release",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ExternalModuleRelease",
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			FilterFunc:                   filterRelease,
		},
	},
}, applyModuleRelease)

func applyModuleRelease(input *go_hook.HookInput) error {
	var modulesChanged bool

	snap := input.Snapshots["releases"]

	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")

	moduleReleases := make(map[string][]enqueueRelease, 0)

	for _, sn := range snap {
		if sn == nil {
			continue
		}
		rel := sn.(enqueueRelease)
		if rel.Status == "" {
			rel.Status = v1alpha1.PhasePending
			status := map[string]v1alpha1.ExternalModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhasePending,
					TransitionTime: time.Now().UTC(),
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ExternalModuleRelease", "", rel.Name, object_patch.WithSubresource("/status"))
		}

		moduleReleases[rel.Module] = append(moduleReleases[rel.Module], rel)
	}

	for module, releases := range moduleReleases {
		sort.Sort(byVersion[enqueueRelease](releases))

		pred := NewReleasePredictor(releases)

		pred.calculateRelease()

		if pred.currentReleaseIndex == len(pred.releases)-1 {
			// latest release deployed
			continue
		}

		if len(pred.skippedPatchesIndexes) > 0 {
			for _, index := range pred.skippedPatchesIndexes {
				release := pred.releases[index]
				status := map[string]v1alpha1.ExternalModuleReleaseStatus{
					"status": {
						Phase:          v1alpha1.PhaseOutdated,
						TransitionTime: pred.ts,
						Message:        "",
					},
				}
				input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ExternalModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
			}
		}

		if pred.currentReleaseIndex >= 0 {
			release := pred.releases[pred.currentReleaseIndex]
			status := map[string]v1alpha1.ExternalModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhaseOutdated,
					TransitionTime: pred.ts,
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ExternalModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}

		if pred.desiredReleaseIndex >= 0 {
			release := pred.releases[pred.desiredReleaseIndex]

			symlinkName := path.Join(externalModulesDir, "modules", module)
			modulePath := path.Join(externalModulesDir, module, "v"+release.Version.String())
			err := enableModule(symlinkName, modulePath)
			if err != nil {
				input.LogEntry.Errorf("Module release failed: %v", err)
				continue
			}
			modulesChanged = true

			status := map[string]v1alpha1.ExternalModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhaseDeployed,
					TransitionTime: pred.ts,
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ExternalModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}
	}

	if modulesChanged {
		err := syscall.Kill(1, syscall.SIGUSR2)
		if err != nil {
			input.LogEntry.Errorf("Send SIGUSR2 signal failed: %s", err)
			return nil
		}
	}

	return nil
}

func enableModule(symlinkPath, modulePath string) error {
	if _, err := os.Lstat(symlinkPath); err == nil {
		err = os.Remove(symlinkPath)
		if err != nil {
			return err
		}
	}

	return os.Symlink(modulePath, symlinkPath)
}

func filterRelease(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.ExternalModuleRelease

	err := sdk.FromUnstructured(obj, &release)
	if err != nil {
		return nil, err
	}

	var releaseApproved bool
	if v, ok := release.Annotations["release.deckhouse.io/approved"]; ok {
		if v == "true" {
			releaseApproved = true
		}
	}

	return enqueueRelease{
		Name:     release.Name,
		Version:  release.Spec.Version,
		Module:   release.Spec.ModuleName,
		Status:   release.Status.Phase,
		Approved: releaseApproved,
	}, nil
}

type enqueueRelease struct {
	Name     string
	Version  *semver.Version
	Module   string
	Status   string
	Approved bool
}

func (er enqueueRelease) GetVersion() *semver.Version {
	return er.Version
}

type releasePredictor struct {
	ts time.Time

	releases              []enqueueRelease
	currentReleaseIndex   int
	desiredReleaseIndex   int
	skippedPatchesIndexes []int
}

func NewReleasePredictor(releases []enqueueRelease) *releasePredictor {
	return &releasePredictor{
		ts:       time.Now().UTC(),
		releases: releases,

		currentReleaseIndex:   -1,
		desiredReleaseIndex:   -1,
		skippedPatchesIndexes: make([]int, 0),
	}
}

func (rp *releasePredictor) calculateRelease() {
	for index, rl := range rp.releases {
		switch rl.Status {
		case v1alpha1.PhaseDeployed:
			rp.currentReleaseIndex = index

		case v1alpha1.PhasePending:
			if rp.desiredReleaseIndex >= 0 {
				previousPredictedRelease := rp.releases[rp.desiredReleaseIndex]
				if previousPredictedRelease.Version.Major() != rl.Version.Major() {
					continue
				}

				if previousPredictedRelease.Version.Minor() != rl.Version.Minor() {
					continue
				}
				// it's a patch for predicted release, continue
				rp.skippedPatchesIndexes = append(rp.skippedPatchesIndexes, rp.desiredReleaseIndex)
			}

			// release is predicted to be Deployed
			rp.desiredReleaseIndex = index
		}
	}
}
