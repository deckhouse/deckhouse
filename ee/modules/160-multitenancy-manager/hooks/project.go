/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/fatih/structs"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const userResourcesTemplatePath = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/user-resources-templates.yaml"

// Alternative path is needed to run tests in ci\cd pipeline
const alternativeUserResourcesTemplatePath = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/user-resources-templates.yaml"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectsQueue),
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
		// subscribe to ProjectTypes to update Projects when ProjectType changes
		internal.ProjectTypeHookKubeConfig,
		internal.ProjectHookKubeConfigOld,
	},
}, dependency.WithExternalDependencies(handleProjects))

func handleProjects(input *go_hook.HookInput, dc dependency.Container) error {
	var projectTypeValuesSnap = internal.GetProjectTypeSnapshots(input)
	var projectValuesSnap = internal.GetProjectSnapshots(input, projectTypeValuesSnap)
	var existProjects = set.NewFromSnapshot(input.Snapshots[internal.ProjectsSecrets])
	var projectUpgradeError error

	helmClient, err := dc.GetHelmClient(internal.D8MultitenancyManager)
	if err != nil {
		return err
	}

	// TODO read template once
	resourcesTemplate, err := readUserResourcesTemplate()
	if err != nil {
		return err
	}

	for projectName, projectValues := range projectValuesSnap {
		if existProjects.Has(projectName) {
			existProjects.Delete(projectName)
		}

		values := concatValues(projectValues, projectTypeValuesSnap[projectValues.ProjectTypeName])

		err = helmClient.Upgrade(projectName, resourcesTemplate, values, false)
		if err != nil {
			projectUpgradeError = err
			internal.SetProjectStatusError(input.PatchCollector, projectName, err.Error())
			input.LogEntry.Errorf("upgrade project \"%v\" error: %v", projectName, err)
			continue
		}

		internal.SetSyncStatusProject(input.PatchCollector, projectName)
	}

	for projectName := range existProjects {
		err := helmClient.Delete(projectName)
		if err != nil {
			internal.SetProjectStatusError(input.PatchCollector, projectName, err.Error())
			input.LogEntry.Errorf("delete project \"%v\" error: %v", projectName, err)
		}
	}

	// return err if some projects couldn't get upgraded
	if projectUpgradeError != nil {
		return projectUpgradeError
	}

	return nil
}

func concatValues(pv internal.ProjectSnapshot, ptv internal.ProjectTypeSnapshot) map[string]interface{} {
	structs.DefaultTagName = "yaml"

	return map[string]interface{}{
		"projectType": structs.Map(ptv.Spec),
		"project":     structs.Map(pv),
	}
}

func readUserResourcesTemplate() (map[string]interface{}, error) {
	templateData, err := os.ReadFile(userResourcesTemplatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			templateData, err = os.ReadFile(alternativeUserResourcesTemplatePath)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	templates := map[string]interface{}{
		filepath.Base(userResourcesTemplatePath): templateData,
	}

	return templates, nil
}
