/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
)

const (
	ProjectsQueue         = "projects"
	D8MultitenancyManager = "d8-multitenancy-manager"
	ProjectsSecrets       = "projects_secrets"
)

var (
	ProjectHookKubeConfig           = projectHookConfig(filterProjects)
	ProjectWithStatusHookKubeConfig = projectHookConfig(filterProjectsWithStatus)
	ProjectHookKubeConfigOld        = go_hook.KubernetesConfig{
		Name:       ProjectsSecrets,
		ApiVersion: "v1",
		Kind:       "Secret",
		FilterFunc: filterOldValuesSecret,
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{D8MultitenancyManager},
			},
		},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"owner":  "helm",
				"status": "deployed",
			},
		},
		// only snapshot update is needed
		ExecuteHookOnEvents:          go_hook.Bool(false),
		ExecuteHookOnSynchronization: go_hook.Bool(false),
	}
)

func projectHookConfig(filterFunc go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       ProjectsQueue,
		ApiVersion: APIVersion,
		Kind:       ProjectKind,
		FilterFunc: filterFunc,
	}
}

type ProjectSnapshotWithStatus struct {
	Snapshot ProjectSnapshot
	Status   v1alpha1.ProjectStatus
}

type ProjectSnapshot struct {
	ProjectName     string                 `json:"projectName" yaml:"projectName"`
	ProjectTypeName string                 `json:"projectTypeName" yaml:"projectTypeName"`
	Template        map[string]interface{} `json:"params" yaml:"params"`
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

func filterOldValuesSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if projecName, ok := obj.GetLabels()["name"]; ok {
		return projecName, nil
	}

	return nil, nil
}

func projectSnapshotFromUnstructed(obj *unstructured.Unstructured) (*ProjectSnapshotWithStatus, error) {
	project := &v1alpha1.Project{}
	if err := sdk.FromUnstructured(obj, project); err != nil {
		return nil, err
	}

	return &ProjectSnapshotWithStatus{
		Snapshot: ProjectSnapshot{
			ProjectName:     project.Name,
			ProjectTypeName: project.Spec.ProjectTypeName,
			Template:        project.Spec.Template,
		},
		Status: project.Status,
	}, nil
}

func validateProject(project ProjectSnapshot, projectTypes map[string]ProjectTypeSnapshot) error {
	if project.ProjectTypeName == "" {
		return fmt.Errorf("ProjectType not set for Project '%s'", project.ProjectName)
	}

	ptSpecValues, ok := projectTypes[project.ProjectTypeName]
	if !ok {
		return fmt.Errorf("can't find valid ProjectType '%s' for Project", project.ProjectTypeName)
	}

	sc, err := LoadOpenAPISchema(ptSpecValues.Spec.OpenAPI)
	if err != nil {
		return fmt.Errorf("can't load '%s' ProjectType OpenAPI schema: %v", project.ProjectTypeName, err)
	}

	sc = schema.TransformSchema(sc, &schema.AdditionalPropertiesTransformer{})
	if err := validate.AgainstSchema(sc, project.Template, strfmt.Default); err != nil {
		return fmt.Errorf("template data doesn't match the OpenAPI schema for '%s' ProjectType: %v", project.ProjectTypeName, err)
	}
	return nil
}

func GetProjectSnapshots(input *go_hook.HookInput, projectTypes map[string]ProjectTypeSnapshot) map[string]ProjectSnapshot {
	projectSnapshots := make(map[string]ProjectSnapshot)

	for _, projectSnap := range input.Snapshots[ProjectsQueue] {
		project, ok := projectSnap.(ProjectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		if err := validateProject(project, projectTypes); err != nil {
			input.LogEntry.Errorf("validation project: %v, error: %v", project.ProjectName, err)
			SetProjectStatusError(input.PatchCollector, project.ProjectName, err.Error())
			continue
		}

		projectSnapshots[project.ProjectName] = project

		SetProjectStatusDeploying(input.PatchCollector, project.ProjectName)
	}

	pj, err := json.Marshal(projectSnapshots)
	input.LogEntry.Infof("projects from err: %v snap: %s", err, string(pj))
	return projectSnapshots
}

func GetProjectSnapshotsWithStatus(input *go_hook.HookInput, projectTypes map[string]ProjectTypeSnapshot) map[string]*ProjectSnapshotWithStatus {
	projectSnapshots := make(map[string]*ProjectSnapshotWithStatus)

	for _, projectSnap := range input.Snapshots[ProjectsQueue] {
		project, ok := projectSnap.(*ProjectSnapshotWithStatus)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		if err := validateProject(project.Snapshot, projectTypes); err != nil {
			input.LogEntry.Errorf("validation project: %v, error: %v", project.Snapshot.ProjectName, err)
			SetProjectStatusError(input.PatchCollector, project.Snapshot.ProjectName, err.Error())
			continue
		}

		projectSnapshots[project.Snapshot.ProjectName] = project

		SetProjectStatusDeploying(input.PatchCollector, project.Snapshot.ProjectName)
	}

	pj, _ := json.Marshal(projectSnapshots)
	input.LogEntry.Infof("projects with status from snap %s", string(pj))
	return projectSnapshots
}
