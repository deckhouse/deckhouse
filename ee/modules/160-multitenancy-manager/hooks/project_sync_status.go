/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

// if helm successfully renders templates - then all Projects are ready

const (
	readyStatusQueue = "ready_status"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(readyStatusQueue),
	OnAfterHelm: &go_hook.OrderedConfig{
		Order: 25,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       internal.ProjectsQueue,
			ApiVersion: internal.APIVersion,
			Kind:       internal.ProjectKind,
			FilterFunc: filterProjects,
		},
	},
}, handleReadyStatusForProjectsAndProjectTypes)

func handleReadyStatusForProjectsAndProjectTypes(input *go_hook.HookInput) error {
	projectSnapshots := input.Snapshots[internal.ProjectsQueue]
	for _, projectSnap := range projectSnapshots {
		if projectSnap == nil {
			continue
		}

		project, ok := projectSnap.(projectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		internal.SetSyncStatusProject(input.PatchCollector, project.Name, project.Conditions)
	}
	return nil
}
