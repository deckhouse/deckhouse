/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

// if helm successfully renders templates - then all Projects from values are ready

const (
	readyStatusQueue = "ready_status"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(readyStatusQueue),
	OnAfterHelm: &go_hook.OrderedConfig{
		Order: 25,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
	},
}, handleReadyStatusForProjectsAndProjectTypes)

func handleReadyStatusForProjectsAndProjectTypes(input *go_hook.HookInput) error {
	projectSnapshots := make(map[string]internal.ProjectSnapshot)
	for _, projectSnap := range input.Snapshots[internal.ProjectsQueue] {
		project, ok := projectSnap.(internal.ProjectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}
		projectSnapshots[project.Name] = project
	}

	values, ok := input.Values.GetOk(internal.ModuleValuePath(internal.ProjectValuesPath))
	if !ok {
		return fmt.Errorf("can't find project values path: %s", internal.ModuleValuePath(internal.ProjectValuesPath))
	}

	for _, value := range values.Array() {
		projectValue := value.Value().(map[string]interface{})
		if !ok {
			return errors.New("can't convert Project values to map[string]interface")
		}
		projectName, ok := projectValue["projectName"].(string)
		if !ok {
			return errors.New("can't get Project name from values")
		}

		projectSnap, ok := projectSnapshots[projectName]
		if !ok {
			return errors.New("can't find Project in cluster from values by name")
		}

		internal.SetSyncStatusProject(input.PatchCollector, projectSnap.Name, projectSnap.Conditions)
	}
	return nil
}
