/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

const (
	projectsQueue = "projects"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(projectsQueue),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 25,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       projectsQueue,
			ApiVersion: internal.APIVersion,
			Kind:       internal.ProjectKind,
			FilterFunc: filterProjects,
		},
		{
			Name:       projectTypesQueue,
			ApiVersion: internal.APIVersion,
			Kind:       internal.ProjectTypeKind,
			FilterFunc: filterProjectTypesForUpdateProjects,
		},
	},
}, handleProjects)

type projectSnapshot struct {
	Name            string
	Template        map[string]interface{}
	ProjectTypeName string
	Conditions      []v1alpha1.Condition
}

func filterProjects(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pt := &v1alpha1.Project{}
	if err := sdk.FromUnstructured(obj, pt); err != nil {
		return nil, err
	}

	return projectSnapshot{
		Name:            pt.Name,
		ProjectTypeName: pt.Spec.ProjectTypeName,
		Template:        pt.Spec.Template,
		Conditions:      pt.Status.Conditions,
	}, nil
}

func filterProjectTypesForUpdateProjects(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pt := &v1alpha1.ProjectType{}
	if err := sdk.FromUnstructured(obj, pt); err != nil {
		return nil, err
	}

	return pt.Spec, nil
}

type projectValues struct {
	Params          map[string]interface{} `json:"params"`
	ProjectTypeName string                 `json:"projectTypeName"`
	ProjectName     string                 `json:"projectName"`
}

func handleProjects(input *go_hook.HookInput) error {
	projectSnapshots := input.Snapshots[projectsQueue]

	values := make([]projectValues, 0, len(projectSnapshots))
	for _, projectSnap := range projectSnapshots {
		if projectSnap == nil {
			continue
		}

		project, ok := projectSnap.(projectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		errStatusProject := setErrorProjectWrap(input.PatchCollector, project.Name, project.Conditions)

		ptSpecValues, ok := input.Values.GetOk(internal.ModuleValuePath(internal.PTValuesPath, project.ProjectTypeName))
		if !ok {
			errMsg := fmt.Sprintf("Can't find valid ProjectType '%s' for Project", project.ProjectTypeName)
			errStatusProject(errMsg)
			continue
		}

		ptValues := ptSpecValues.Value()
		ptValuesMap, ok := ptValues.(map[string]interface{})
		if !ok {
			errMsg := fmt.Sprintf("can't convert '%s' ProjectType values to map[string]interface: %T", project.ProjectTypeName, ptValues)
			errStatusProject(errMsg)
			continue
		}

		schema, err := internal.LoadOpenAPISchema(ptValuesMap["openAPI"])
		if err != nil {
			errMsg := fmt.Sprintf("can't load '%s' ProjectType OpenAPI schema: %v", project.ProjectTypeName, err)
			errStatusProject(errMsg)
			continue
		}

		if schema.AdditionalProperties == nil {
			schema.AdditionalProperties = &spec.SchemaOrBool{Allows: false}
		}
		if err := validate.AgainstSchema(schema, project.Template, strfmt.Default); err != nil {
			errMsg := fmt.Sprintf("template data doesn't match the OpenAPI schema for '%s' ProjectType: %v", project.ProjectTypeName, err)
			errStatusProject(errMsg)
			continue
		}

		values = append(values, projectValues{
			ProjectTypeName: project.ProjectTypeName,
			ProjectName:     project.Name,
			Params:          project.Template,
		})

		setSyncStatusProject(input.PatchCollector, project.Name, project.Conditions)
	}

	input.Values.Set(internal.ModuleValuePath(internal.ProjectValuesPath), values)
	return nil
}

func setErrorProjectWrap(patcher *object_patch.PatchCollector, projectName string, conditions []v1alpha1.Condition) func(errMsg string) {
	return func(errMsg string) {
		setErrorStatusProject(patcher, projectName, errMsg, conditions)
	}
}

func setErrorStatusProject(patcher *object_patch.PatchCollector, projectName, errMsg string, conditions []v1alpha1.Condition) {
	conditions = append(conditions, v1alpha1.Condition{
		Name:    "Error",
		Message: errMsg,
		Status:  false,
	})

	internal.SetProjectStatus(patcher, projectName, false, errMsg, conditions)
}

func setSyncStatusProject(patcher *object_patch.PatchCollector, projectName string, conditions []v1alpha1.Condition) {
	conditions = append(conditions, v1alpha1.Condition{
		Name:   "Sync",
		Status: true,
	})
	internal.SetProjectStatus(patcher, projectName, true, "", conditions)
}
