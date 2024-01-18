/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha2"
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
		FilterFunc: filterProjectReleaseSecret,
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
		ApiVersion: "deckhouse.io/v1alpha2",
		Kind:       ProjectKind,
		FilterFunc: filterFunc,
	}
}

type ProjectSnapshotWithStatus struct {
	Snapshot ProjectSnapshot
	Status   v1alpha2.ProjectStatus
}

type ProjectSnapshot struct {
	ProjectName        string                       `json:"projectName" yaml:"projectName"`
	TemplateName       string                       `json:"templateName" yaml:"templateName"`
	TemplateValues     map[string]interface{}       `json:"templateValues" yaml:"templateValues"`
	AuthorizationRules []v1alpha2.AuthorizationRule `json:"authorizationRules" yaml:"authorizationRules"`
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

func filterProjectReleaseSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if projecName, ok := obj.GetLabels()["name"]; ok {
		return projecName, nil
	}

	return nil, nil
}

func projectSnapshotFromUnstructed(obj *unstructured.Unstructured) (*ProjectSnapshotWithStatus, error) {
	project := &v1alpha2.Project{}
	if err := sdk.FromUnstructured(obj, project); err != nil {
		return nil, err
	}

	return &ProjectSnapshotWithStatus{
		Snapshot: ProjectSnapshot{
			ProjectName:        project.Name,
			TemplateName:       project.Spec.TemplateName,
			TemplateValues:     project.Spec.TemplateValues,
			AuthorizationRules: project.Spec.AuthorizationRules,
		},
		Status: project.Status,
	}, nil
}

func validateProject(project ProjectSnapshot, projectTemplates map[string]ProjectTemplateSnapshot) error {
	if project.TemplateName == "" {
		return fmt.Errorf("ProjectTemplate not set for Project '%s'", project.ProjectName)
	}

	ptSpecValues, ok := projectTemplates[project.TemplateName]
	if !ok {
		return fmt.Errorf("can't find valid ProjectTemplates '%s' for Project", project.TemplateName)
	}

	sc, err := LoadOpenAPISchema(ptSpecValues.Spec.ValuesSchema)
	if err != nil {
		return fmt.Errorf("can't load '%s' ProjectType OpenAPI schema: %v", project.TemplateName, err)
	}

	sc = schema.TransformSchema(sc, &schema.AdditionalPropertiesTransformer{})
	if err := validate.AgainstSchema(sc, project.TemplateValues, strfmt.Default); err != nil {
		return fmt.Errorf("template data doesn't match the OpenAPI schema for '%s' ProjectType: %v", project.TemplateName, err)
	}
	return nil
}

func GetProjectSnapshots(input *go_hook.HookInput, projectTemplates map[string]ProjectTemplateSnapshot) map[string]ProjectSnapshot {
	projectSnapshots := make(map[string]ProjectSnapshot)

	for _, projectSnap := range input.Snapshots[ProjectsQueue] {
		project, ok := projectSnap.(ProjectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		if err := validateProject(project, projectTemplates); err != nil {
			input.LogEntry.Errorf("validation project: %v, error: %v", project.ProjectName, err)
			SetProjectStatusError(input.PatchCollector, project.ProjectName, err.Error())
			continue
		}

		projectSnapshots[project.ProjectName] = project

		SetProjectStatusDeploying(input.PatchCollector, project.ProjectName)
	}

	return projectSnapshots
}
