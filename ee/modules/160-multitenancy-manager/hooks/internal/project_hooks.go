/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
)

const (
	ProjectsQueue = "projects"
)

var (
	ProjectHookKubeConfig           = projectHookConfig(filterProjects)
	ProjectWithStatusHookKubeConfig = projectHookConfig(filterProjectsWithStatus)
)

func projectHookConfig(filterFunc go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       ProjectsQueue,
		ApiVersion: APIVersion,
		Kind:       ProjectKind,
		FilterFunc: filterFunc,
		// only snapshot update is needed
		ExecuteHookOnEvents:          go_hook.Bool(false),
		ExecuteHookOnSynchronization: go_hook.Bool(false),
	}
}

type ProjectSnapshotWithStatus struct {
	Snapshot ProjectSnapshot
	Status   v1alpha1.ProjectStatus
}

type ProjectSnapshot struct {
	Name            string
	Template        map[string]interface{}
	ProjectTypeName string
}

func filterProjects(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	projectSnapWithStatus, err := projectSnapshotFromUnstructed(obj)
	if err != nil {
		return nil, err
	}
	return projectSnapWithStatus.Snapshot, nil
}

func filterProjectsWithStatus(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return projectSnapshotFromUnstructed(obj)
}

func projectSnapshotFromUnstructed(obj *unstructured.Unstructured) (*ProjectSnapshotWithStatus, error) {
	project := &v1alpha1.Project{}
	if err := sdk.FromUnstructured(obj, project); err != nil {
		return nil, err
	}

	return &ProjectSnapshotWithStatus{
		Snapshot: ProjectSnapshot{
			Name:            project.Name,
			ProjectTypeName: project.Spec.ProjectTypeName,
			Template:        project.Spec.Template,
		},
		Status: project.Status,
	}, nil
}
