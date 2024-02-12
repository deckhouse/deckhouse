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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

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
		{
			Name:       "ns",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "name",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterNsName,
		},
	},
}, dependency.WithExternalDependencies(handleProjects))

func filterNsName(unst *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return unst.GetName(), nil
}

func handleProjects(input *go_hook.HookInput, dc dependency.Container) error {
	var projectTypeValuesSnap = internal.GetProjectTypeSnapshots(input)
	var projectTemplateValuesSnap = internal.GetProjectTemplateSnapshots(input)
	var namespaces = set.NewFromSnapshot(input.Snapshots["ns"])

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

		projectTemplateValues := projectTemplateValuesSnap[projectValues.ProjectTemplateName]
		projectTemplateValues = preprocessProjectTemplate(projectName, projectTemplateValues)
		values := concatValues(projectValues, projectTemplateValues)
		err = helmClient.Upgrade(projectName, projectName, resourcesTemplate, values, false)
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
		if namespaces.Has(projectName) {
			input.PatchCollector.Delete("v1", "Namespace", "", projectName, object_patch.InBackground())
		}
	}

	return nil
}

func preprocessProjectTemplate(projectName string, pts internal.ProjectTemplateSnapshot) internal.ProjectTemplateSnapshot {
	helmTemplate := pts.Spec.ResourcesTemplate
	manifests := releaseutil.SplitManifests(helmTemplate)

	builder := strings.Builder{}

	var nsExists bool

	for _, manifest := range manifests {
		var ns v1.Namespace
		_ = yaml.Unmarshal([]byte(manifest), &ns)

		if ns.APIVersion != "v1" || ns.Kind != "Namespace" {
			builder.WriteString("\n---\n" + manifest)
			continue
		}

		if ns.Name != projectName {
			// drop Namespace from manifests if it's not a project namespace
			continue
		}

		nsExists = true

		builder.WriteString("\n---\n" + manifest)
	}

	result := builder.String()

	if !nsExists {
		ns := fmt.Sprintf(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    meta.helm.sh/release-name: %[1]s
    meta.helm.sh/release-namespace: %[1]s
  labels:
    app.kubernetes.io/managed-by: Helm
    module: multitenancy-manager
  name: %[1]s
`, projectName)
		result = ns + result
	}

	pts.Spec.ResourcesTemplate = result
	fmt.Println("RESULT", result)

	return pts
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

type projectTemplateHelmRenderer struct {
	projectName string
}

func (ptr *projectTemplateHelmRenderer) SetProject(name string) {
	ptr.projectName = name
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (ptr *projectTemplateHelmRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	if ptr.projectName == "" {
		return renderedManifests, nil
	}

	result := bytes.NewBuffer(nil)

	manifests := releaseutil.SplitManifests(renderedManifests.String())

	for _, manifest := range manifests {
		var ns v1.Namespace
		err = yaml.Unmarshal([]byte(manifest), &ns)
		if err != nil {
			return result, err
		}

		if ns.APIVersion != "v1" || ns.Kind != "Namespace" {
			result.WriteString("\n---\n" + manifest)
			continue
		}

		if ns.Name != ptr.projectName {
			// drop Namespace from manifests if it's not a project namespace
			continue
		}

		result.WriteString("\n---\n" + manifest)
	}

	return result, nil
}
