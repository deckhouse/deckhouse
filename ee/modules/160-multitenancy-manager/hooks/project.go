/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/helm"

	"github.com/fatih/structs"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/klog"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	defaultProjectTemplatePath = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/default-project-template.yaml"
	secureProjectTemplatePath  = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/secure-project-template.yaml"
	dedicatedNodesTemplatePath = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/secure-with-dedicated-nodes-project-template.yaml"
	userResourcesTemplatePath  = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/user-resources-templates.yaml"
	// Alternative path is needed to run tests in ci\cd pipeline
	alternativeDefaultProjectTemplatePath = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/default-project-template.yaml"
	alternativeSecureProjectTemplatePath  = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/secure-project-template.yaml"
	alternativeDedicatedNodesTemplatePath = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/secure-with-dedicated-nodes-project-template.yaml"
	alternativeUserResourcesTemplatePath  = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/user-resources-templates.yaml"
)

var onceCreateDefaultTemplates sync.Once

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.ModuleQueue(internal.ProjectsQueue),
	Kubernetes: []go_hook.KubernetesConfig{
		internal.ProjectHookKubeConfig,
		internal.ProjectTemplateHookKubeConfig,
		internal.ProjectTypeHookKubeConfig,
		internal.ProjectHookKubeConfigOld,
	},
}, dependency.WithExternalDependencies(handleProjects))

func handleProjects(input *go_hook.HookInput, dc dependency.Container) error {
	var projectTypeValuesSnap = internal.GetProjectTypeSnapshots(input)
	var projectTemplateValuesSnap = internal.GetProjectTemplateSnapshots(input)

	createDefaultProjectTemplate(input)

	// map ProjectType to ProjectTemplate
	for key, val := range projectTypeValuesSnap {
		if _, ok := projectTemplateValuesSnap[key]; !ok {
			resourcesTemplate := strings.ReplaceAll(val.Spec.ResourcesTemplate, ".params.", ".parameters.")
			projectTemplateValuesSnap[key] = internal.ProjectTemplateSnapshot{
				Name: val.Name,
				Spec: v1alpha1.ProjectTemplateSpec{
					Subjects:          val.Spec.Subjects,
					NamespaceMetadata: val.Spec.NamespaceMetadata,
					ResourcesTemplate: resourcesTemplate,
					ParametersSchema: v1alpha1.ParametersSchema{
						OpenAPIV3Schema: map[string]interface{}{
							"properties": val.Spec.OpenAPI,
						},
					},
				},
			}
		}
	}
	var projectValuesSnap = internal.GetProjectSnapshots(input, projectTemplateValuesSnap)
	var existProjects = set.NewFromSnapshot(input.Snapshots[internal.ProjectsSecrets])
	projectPostRenderer := &projectTemplateHelmRenderer{}

	helmClient, err := dc.GetHelmClient(internal.D8MultitenancyManager, helm.WithPostRenderer(projectPostRenderer))
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

		projectTemplateValues := projectTemplateValuesSnap[projectValues.ProjectTemplateName]
		values := concatValues(projectValues, projectTemplateValues)
		fmt.Println("INSTALLL", values)
		projectPostRenderer.SetProject(projectName)
		err = helmClient.Upgrade(projectName, resourcesTemplate, values, false)
		if err != nil {
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

	return nil
}

type projectTemplateHelmRenderer struct {
	projectName string
}

func (f *projectTemplateHelmRenderer) SetProject(name string) {
	f.projectName = name
}

func (f *projectTemplateHelmRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	fmt.Println("RUN POST RENDERER")
	if f.projectName == "" {
		return renderedManifests, nil
	}

	fmt.Println("BEFORE", renderedManifests.String())
	manifests := releaseutil.SplitManifests(renderedManifests.String())

	renderedManifests.Reset()

	var nsExists bool

	for _, manifest := range manifests {
		var ns v1.Namespace
		_ = yaml.Unmarshal([]byte(manifest), &ns)

		fmt.Println("NS", ns)
		if ns.APIVersion != "v1" || ns.Kind != "Namespace" {
			renderedManifests.WriteString(manifest)
			continue
		}

		if ns.Name != f.projectName {
			// drop Namespace from manifests if it's not a project namespace
			continue
		}

		nsExists = true

		renderedManifests.WriteString(manifest)
	}

	if !nsExists {
		projectNS := fmt.Sprintf(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, f.projectName)

		renderedManifests.WriteString(projectNS)
	}

	fmt.Println("AFTER", renderedManifests.String())

	return renderedManifests, nil
}

func concatValues(ps internal.ProjectSnapshot, pts internal.ProjectTemplateSnapshot) map[string]interface{} {
	structs.DefaultTagName = "yaml"

	return map[string]interface{}{
		"projectTemplate": structs.Map(pts.Spec),
		"project":         structs.Map(ps),
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

func createDefaultProjectTemplate(input *go_hook.HookInput) {
	onceBody := func() {
		defaultProjectTemplateRaw, err := readDefaultProjectTemplate(defaultProjectTemplatePath, alternativeDefaultProjectTemplatePath)
		if err != nil {
			klog.Errorf("error reading default ProjectTemplate: %v", err)
			return
		}

		secureProjectTemplateRaw, err := readDefaultProjectTemplate(secureProjectTemplatePath, alternativeSecureProjectTemplatePath)
		if err != nil {
			klog.Errorf("error reading default ProjectTemplate: %v", err)
			return
		}

		dedicatedNodesProjectTemplateRaw, err := readDefaultProjectTemplate(dedicatedNodesTemplatePath, alternativeDedicatedNodesTemplatePath)
		if err != nil {
			klog.Errorf("error reading default ProjectTemplate: %v", err)
			return
		}

		input.PatchCollector.Create(defaultProjectTemplateRaw, object_patch.UpdateIfExists())
		input.PatchCollector.Create(secureProjectTemplateRaw, object_patch.UpdateIfExists())
		input.PatchCollector.Create(dedicatedNodesProjectTemplateRaw, object_patch.UpdateIfExists())
	}

	onceCreateDefaultTemplates.Do(onceBody)
}

func readDefaultProjectTemplate(defaultPath, alternativePath string) ([]byte, error) {
	projectTemplate, err := os.ReadFile(defaultPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			projectTemplate, err = os.ReadFile(alternativePath)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return projectTemplate, nil
}
