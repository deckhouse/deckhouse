/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oldValuesSecretQueue = "old_values_secret"
	oldValuesSecretKey   = "projectValues"
	oldValuesSecretName  = "deckhouse-multitenancy-manager"
	// d8SystemNS            = "d8-system" // deprecated
	d8MultitenancyManager     = "d8-multitenancy-manager"
	userResourcesTemplatePath = "/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal/templates/user-resources-templates.yaml"
)

type projectTypeValues struct {
	Params          map[string]interface{} `json:"params"`
	ProjectTypeName string                 `json:"projectTypeName"`
	ProjectName     string                 `json:"projectName"`
}

type projectValues struct {
	Params          map[string]interface{} `json:"params"`
	ProjectTypeName string                 `json:"projectTypeName"`
	ProjectName     string                 `json:"projectName"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectsQueue),
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
		// subscribe to ProjectTypes to update Projects when ProjectType changes
		internal.ProjectTypeHookKubeConfig,
	},
}, dependency.WithExternalDependencies(handleProjects))

func handleProjects(input *go_hook.HookInput, dc dependency.Container) error {
	var projectTypeValuesSnap map[string]v1alpha1.ProjectTypeSpec
	var projectValuesSnapOld map[string]projectValues
	var projectValuesSnap map[string]projectValues

	projectTypeValuesSnap = getProjectTypeValues(input)

	projectValuesSnap = getProjectValues(input)

	if len(input.Snapshots[oldValuesSecretQueue]) > 0 {
		values, err := oldProjectValuesFromFilteredSecret(input.Snapshots[oldValuesSecretQueue][0])
		if err != nil {
			return err
		}

		projectValuesSnapOld = values
	}

	templateData, err := os.ReadFile(userResourcesTemplatePath)
	if err != nil {
		return err
	}
	templates := map[string]interface{}{
		filepath.Base(userResourcesTemplatePath): templateData,
	}

	toDelete, toUpdate := projectValuesCompareNew(projectValuesSnapOld, projectValuesSnap)

	values, err := concatValues(toUpdate, projectTypeValuesSnap)
	if err != nil {
		return err
	}

	helmClient, err := dc.GetHelmClient()
	if err != nil {
		return err
	}
	// for name, project := range toInstall {
	// 	helmClient.Upgrade(name, d8MultitenancyManager, templates, values, false)
	// }
	for projectName := range toUpdate {
		err := helmClient.Upgrade(projectName, d8MultitenancyManager, templates, values, false)
		if err != nil {
			input.LogEntry.Errorf("upgrade project \"%v\" error: %v", projectName, err)
		}
		// if internal.ProjectStatusIsDeploying(proect.Status) {
		// 	internal.SetSyncStatusProject(input.PatchCollector, projectSnap.Snapshot.Name)
		// }
	}
	for projectName := range toDelete {
		helmClient.Delete(projectName)
	}

	valuesSecret, err := newSecretFromProjectValues(toUpdate)
	if err != nil {
		return err
	}

	input.PatchCollector.Create(valuesSecret, object_patch.UpdateIfExists()) // <--

	return nil
}

func getProjectTypeValues(input *go_hook.HookInput) map[string]v1alpha1.ProjectTypeSpec {
	ptSnapshots := input.Snapshots[internal.ProjectTypesQueue]

	projectTypesValues := make(map[string]v1alpha1.ProjectTypeSpec)

	for _, ptSnapshot := range ptSnapshots {
		pt, ok := ptSnapshot.(internal.ProjectTypeSnapshot)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectTypeSnapshot': %v", ptSnapshot)
			continue
		}

		if err := validateProjectType(pt); err != nil {
			internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, false, err.Error())
			continue
		}

		projectTypesValues[pt.Name] = pt.Spec // TODO replace

		internal.SetProjectTypeStatus(input.PatchCollector, pt.Name, true, "")
	}

	// input.Values.Set(internal.ModuleValuePath(internal.PTValuesPath), projectTypesValues)
	return projectTypesValues
}

func getProjectValues(input *go_hook.HookInput) map[string]projectValues {
	projectSnapshots := input.Snapshots[internal.ProjectsQueue]

	// allProjectsFromCluster := make(map[string]bool, len(projectSnapshots))
	newValues := make(map[string]projectValues)

	for _, projectSnap := range projectSnapshots {
		project, ok := projectSnap.(internal.ProjectSnapshotWithStatus)
		if !ok {
			input.LogEntry.Errorf("can't convert snapshot to 'projectSnapshot': %v", project)
			continue
		}

		// allProjectsFromCluster[project.Name] = true

		if err := validateProject(input, project.Snapshot); err != nil {
			internal.SetErrorStatusProject(input.PatchCollector, project.Snapshot.Name, err.Error()) // <--
			continue
		}

		newValues[project.Snapshot.Name] = projectValues{
			ProjectTypeName: project.Snapshot.ProjectTypeName,
			ProjectName:     project.Snapshot.Name,
			Params:          project.Snapshot.Template,
		}

		internal.SetDeployingStatusProject(input.PatchCollector, project.Snapshot.Name) // <--
	}

	return newValues
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
			Namespace: d8MultitenancyManager, // d8SystemNS
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

func validateProjectType(projectType internal.ProjectTypeSnapshot) error {
	// TODO (alex123012): Add open-api spec validation
	if _, err := internal.LoadOpenAPISchema(projectType.Spec.OpenAPI); err != nil {
		return fmt.Errorf("can't load open api schema from '%s' ProjectType spec: %s", projectType.Name, err)
	}
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

func projectValuesCompareNew(oldValues, newValues map[string]projectValues) ( /*toInstall,*/ toDelete, toUpdate map[string]projectValues) {
	// toInstall = make(map[string]projectValues)
	toDelete = make(map[string]projectValues)
	toUpdate = make(map[string]projectValues)

	if len(oldValues) < 1 {
		toUpdate = newValues
		return
	}

	projectNames := sumMapKeys(oldValues, newValues)
	for projectName := range projectNames {
		newValue, newValueExist := newValues[projectName]
		oldValue, oldValueExist := oldValues[projectName]

		if oldValueExist && !newValueExist {
			toDelete[projectName] = oldValue
			continue
		}

		toUpdate[projectName] = newValue

		/*switch {
		case !oldValueExist && newValueExist:
			toUpdate[projectName] = newValue

		case newValueExist && !reflect.DeepEqual(oldValue, newValue):
			toUpdate[projectName] = newValue

		case oldValueExist && !newValueExist:
			toDelete[projectName] = oldValue
		}*/
	}

	return
}

func sumMapKeys(map1, map2 map[string]projectValues) map[string]struct{} {
	var resultMap = make(map[string]struct{})

	for key := range map1 {
		resultMap[key] = struct{}{}
	}

	for key := range map2 {
		resultMap[key] = struct{}{}
	}

	return resultMap
}

func concatValues(pv map[string]projectValues, ptv map[string]v1alpha1.ProjectTypeSpec) (map[string]interface{}, error) {
	pvRaw, err := json.Marshal(projectValuesmapToSlice(pv))
	if err != nil {
		return nil, err
	}

	pvtRaw, _ := json.Marshal(ptv)
	if err != nil {
		return nil, err
	}

	str := fmt.Sprintf(`{"multitenancyManager":{"internal":{"projectTypes":{%s}, "projects":[{%s}]}},"global":{}}`, string(pvtRaw), string(pvRaw))

	resp := make(map[string]interface{})
	err = json.Unmarshal([]byte(str), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
