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
	ProjectHookKubeConfig = go_hook.KubernetesConfig{
		Name:       ProjectsQueue,
		ApiVersion: APIVersion,
		Kind:       ProjectKind,
		FilterFunc: filterProjects,
	}
)

type ProjectSnapshot struct {
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

	return ProjectSnapshot{
		Name:            pt.Name,
		ProjectTypeName: pt.Spec.ProjectTypeName,
		Template:        pt.Spec.Template,
		Conditions:      pt.Status.Conditions,
	}, nil
}
