/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
)

const (
	oldValuesSecretQueue = "old_values_secret"
	oldValuesSecretKey   = "projectValues"
	oldValuesSecretName  = "deckhouse-multitenancy-manager"
	d8SystemNS           = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectsQueue),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 25,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
		// subscribe to ProjectTypes to update Projects when ProjectType changes
		internal.ProjectTypeHookKubeConfig,
		{
			Name:       oldValuesSecretQueue,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: filterOldValuesSecret,
			NameSelector: &types.NameSelector{
				MatchNames: []string{oldValuesSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{d8SystemNS},
				},
			},
		},
	},
}, handleProjects)

func filterOldValuesSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert old_values Secret to v1.Secret struct: %w", err)
	}

	return secret.Data, nil
}

type projectValues struct {
	Params          map[string]interface{} `json:"params"`
	ProjectTypeName string                 `json:"projectTypeName"`
	ProjectName     string                 `json:"projectName"`
}

func handleProjects(input *go_hook.HookInput) error {
	projectSnapshots := input.Snapshots[internal.ProjectsQueue]

	allProjectsFromCluster := make(map[string]bool, len(projectSnapshots))
	newValues := make(map[string]projectValues)
	for _, projectSnap := range projectSnapshots {
		project, ok := projectSnap.(internal.ProjectSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		allProjectsFromCluster[project.Name] = true

		if err := validateProject(input, project); err != nil {
			internal.SetErrorStatusProject(input.PatchCollector, project.Name, err.Error())
			continue
		}

		newValues[project.Name] = projectValues{
			ProjectTypeName: project.ProjectTypeName,
			ProjectName:     project.Name,
			Params:          project.Template,
		}

		internal.SetDeployingStatusProject(input.PatchCollector, project.Name)
	}

	values, err := projectValuesCompare(input.Snapshots[oldValuesSecretQueue], newValues, allProjectsFromCluster)
	if err != nil {
		return err
	}

	valuesSecret, err := newSecretFromProjectValues(values)
	if err != nil {
		return err
	}

	input.PatchCollector.Create(valuesSecret, object_patch.UpdateIfExists())
	// input.Values.Set(internal.ModuleValuePath(internal.ProjectValuesPath), projectValuesmapToSlice(values))

	return nil
}

func validateProject(input *go_hook.HookInput, project internal.ProjectSnapshot) error {
	if project.ProjectTypeName == "" {
		return fmt.Errorf("ProjectType not set for Project '%s'", project.Name)
	}

	ptSpecValues, ok := input.Values.GetOk(internal.ModuleValuePath(internal.PTValuesPath, project.ProjectTypeName))
	if !ok {
		return fmt.Errorf("can't find valid ProjectType '%s' for Project", project.ProjectTypeName)
	}

	ptValues := ptSpecValues.Value()
	ptValuesMap, ok := ptValues.(map[string]interface{})
	if !ok {
		return fmt.Errorf("can't convert '%s' ProjectType values to map[string]interface: %T", project.ProjectTypeName, ptValues)
	}

	sc, err := internal.LoadOpenAPISchema(ptValuesMap["openAPI"])
	if err != nil {
		return fmt.Errorf("can't load '%s' ProjectType OpenAPI schema: %v", project.ProjectTypeName, err)
	}

	sc = schema.TransformSchema(sc, &schema.AdditionalPropertiesTransformer{})
	if err := validate.AgainstSchema(sc, project.Template, strfmt.Default); err != nil {
		return fmt.Errorf("template data doesn't match the OpenAPI schema for '%s' ProjectType: %v", project.ProjectTypeName, err)
	}
	return nil
}

func projectValuesCompare(oldValuesSecrets []go_hook.FilterResult, newValues map[string]projectValues, allProjectsFromCluster map[string]bool) (map[string]projectValues, error) {
	if len(oldValuesSecrets) < 1 {
		return newValues, nil
	}

	oldValues, err := oldProjectValuesFromFilteredSecret(oldValuesSecrets[0])
	if err != nil {
		return nil, err
	}

	values := make(map[string]projectValues, len(oldValues)+len(newValues))
	for projectName := range allProjectsFromCluster {
		newValue, newExists := newValues[projectName]
		oldValue, oldExists := oldValues[projectName]

		switch {
		case newExists:
			values[projectName] = newValue
		case oldExists:
			values[projectName] = oldValue
		}
	}

	return values, nil
}

func newSecretFromProjectValues(values map[string]projectValues) (*v1.Secret, error) {
	marshalledValues, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldValuesSecretName,
			Namespace: d8SystemNS,
		},
		Data: map[string][]byte{
			oldValuesSecretKey: marshalledValues,
		},
	}, nil
}

func oldProjectValuesFromFilteredSecret(filter go_hook.FilterResult) (map[string]projectValues, error) {
	oldValuesSecret, ok := filter.(map[string][]byte)
	if !ok {
		return nil, errors.New("can't convert old values secret data snapshot to *v1.Secret")
	}

	oldValuesData, ok := oldValuesSecret[oldValuesSecretKey]
	if !ok {
		return nil, fmt.Errorf(`can't find "%s" key from old values secret`, oldValuesSecretKey)
	}

	var oldValues map[string]projectValues
	if err := json.Unmarshal(oldValuesData, &oldValues); err != nil {
		return nil, err
	}

	return oldValues, nil
}

func projectValuesmapToSlice(values map[string]projectValues) []projectValues {
	valuesList := make([]projectValues, 0, len(values))
	for _, value := range values {
		valuesList = append(valuesList, value)
	}
	sort.Slice(valuesList, func(i, j int) bool {
		return strings.Compare(valuesList[i].ProjectName, valuesList[j].ProjectName) < 0
	})

	return valuesList
}
