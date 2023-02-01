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

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/005-external-module-manager/hooks/internal/apis/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/external-module-source/cleanup",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ExternalModuleRelease",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterDeprecatedRelease,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_deckhouse_release",
			Crontab: "13 3 * * *",
		},
	},
}, cleanupReleases)

const (
	keepReleaseCount = 3
)

func cleanupReleases(input *go_hook.HookInput) error {
	snap := input.Snapshots["releases"]

	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")

	moduleRelease := make(map[string][]deprecatedRelease, 0)

	for _, sn := range snap {
		if sn == nil {
			continue
		}
		rel := sn.(deprecatedRelease)
		moduleRelease[rel.Module] = append(moduleRelease[rel.Module], rel)
	}

	for _, releases := range moduleRelease {
		sort.Sort(sort.Reverse(byVersion[deprecatedRelease](releases)))

		if len(releases) > keepReleaseCount {
			for i := keepReleaseCount; i < len(releases); i++ {
				release := releases[i]

				modulePath := path.Join(externalModulesDir, release.Module, "v"+release.Version.String())
				err := deleteFromFS(modulePath)
				if err != nil {
					input.LogEntry.Errorf("unable to remove module: %v", err)
					continue
				}

				input.PatchCollector.Delete("deckhouse.io/v1alpha1", "ExternalModuleRelease", "", release.Name, object_patch.InBackground())
			}
		}
	}

	return nil
}

func deleteFromFS(dir string) error {
	return os.RemoveAll(dir)
}

func filterDeprecatedRelease(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.ExternalModuleRelease

	err := sdk.FromUnstructured(obj, &release)
	if err != nil {
		return nil, err
	}

	if !(release.Status.Phase == v1alpha1.PhaseOutdated || release.Status.Phase == v1alpha1.PhaseSuspended) {
		return nil, err
	}

	return deprecatedRelease{
		Name:    release.Name,
		Module:  release.Spec.ModuleName,
		Version: release.Spec.Version,
	}, nil
}

type deprecatedRelease struct {
	Name    string
	Module  string
	Version *semver.Version
}

func (dr deprecatedRelease) GetVersion() *semver.Version {
	return dr.Version
}

type versionGetter interface {
	GetVersion() *semver.Version
}

type byVersion[T versionGetter] []T

func (b byVersion[T]) Len() int {
	return len(b)
}

func (b byVersion[T]) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byVersion[T]) Less(i, j int) bool {
	return b[i].GetVersion().LessThan(b[j].GetVersion())
}
