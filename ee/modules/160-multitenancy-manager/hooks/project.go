/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/structs"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/kube-client/manifest/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	userResourcesTemplatesPath            = "/deckhouse/modules/160-multitenancy-manager/templates/user-resources/"
	alternativeUserResourcesTemplatesPath = "/deckhouse/ee/modules/160-multitenancy-manager/templates/user-resources/"
)

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
	postRenderer := &projectTemplateHelmRenderer{
		logger: input.LogEntry,
	}
	var projectTypeValuesSnap = internal.GetProjectTypeSnapshots(input)
	var projectTemplateValuesSnap = internal.GetProjectTemplateSnapshots(input)

	//createDefaultProjectTemplate(input)

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
	var projectsSnap = internal.GetProjectSnapshots(input, projectTemplateValuesSnap)
	var existProjects = set.NewFromSnapshot(input.Snapshots[internal.ProjectsSecrets])

	helmClient, err := dc.GetHelmClient(internal.D8MultitenancyManager)
	if err != nil {
		return err
	}

	resourcesTemplate, err := readUserResourcesTemplate()
	if err != nil {
		return err
	}

	for projectName, projectValues := range projectsSnap {
		postRenderer.SetProject(projectName)
		if existProjects.Has(projectName) {
			existProjects.Delete(projectName)
		}

		projectTemplateValues := projectTemplateValuesSnap[projectValues.ProjectTemplateName]
		values := concatValues(projectValues, projectTemplateValues)
		err = helmClient.Upgrade(projectName, projectName, resourcesTemplate, values, false, postRenderer)
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

func concatValues(ps internal.ProjectSnapshot, pts internal.ProjectTemplateSnapshot) map[string]interface{} {
	structs.DefaultTagName = "yaml"

	return map[string]interface{}{
		"projectTemplate": structs.Map(pts.Spec),
		"project":         structs.Map(ps),
	}
}

var userTemplates = make(map[string]interface{})

func readUserResourcesTemplate() (map[string]interface{}, error) {
	if len(userTemplates) > 0 {
		return userTemplates, nil
	}

	dirPath := userResourcesTemplatesPath

	files, err := os.ReadDir(userResourcesTemplatesPath)
	if err != nil {
		if os.IsNotExist(err) {
			dirPath = alternativeUserResourcesTemplatesPath
			files, err = os.ReadDir(alternativeUserResourcesTemplatesPath)
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	}

	fmt.Println("FIles", files)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fmt.Println("READING FILE", file.Name())
		templateData, err := os.ReadFile(filepath.Join(dirPath, file.Name()))
		if err != nil {
			return nil, err
		}

		userTemplates[file.Name()] = templateData
	}

	//templateData, err := os.ReadFile(userResourcesTemplatePath)
	//if err != nil {
	//	if errors.Is(err, os.ErrNotExist) {
	//		templateData, err = os.ReadFile(alternativeUserResourcesTemplatePath)
	//		if err != nil {
	//			return nil, err
	//		}
	//	} else {
	//		return nil, err
	//	}
	//}

	//templates := map[string]interface{}{
	//	filepath.Base(userResourcesTemplatePath): templateData,
	//}

	return userTemplates, nil
}

//
//func createDefaultProjectTemplate(input *go_hook.HookInput) {
//	onceBody := func() {
//		defaultProjectTemplateRaw, err := readDefaultProjectTemplate(defaultProjectTemplatePath, alternativeDefaultProjectTemplatePath)
//		if err != nil {
//			klog.Errorf("error reading default ProjectTemplate: %v", err)
//			return
//		}
//
//		secureProjectTemplateRaw, err := readDefaultProjectTemplate(secureProjectTemplatePath, alternativeSecureProjectTemplatePath)
//		if err != nil {
//			klog.Errorf("error reading default ProjectTemplate: %v", err)
//			return
//		}
//
//		dedicatedNodesProjectTemplateRaw, err := readDefaultProjectTemplate(dedicatedNodesTemplatePath, alternativeDedicatedNodesTemplatePath)
//		if err != nil {
//			klog.Errorf("error reading default ProjectTemplate: %v", err)
//			return
//		}
//
//		input.PatchCollector.Create(defaultProjectTemplateRaw, object_patch.UpdateIfExists())
//		input.PatchCollector.Create(secureProjectTemplateRaw, object_patch.UpdateIfExists())
//		input.PatchCollector.Create(dedicatedNodesProjectTemplateRaw, object_patch.UpdateIfExists())
//	}
//
//	onceCreateDefaultTemplates.Do(onceBody)
//}

type projectTemplateHelmRenderer struct {
	projectName string
	logger      logger.Logger
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

	builder := strings.Builder{}

	manifests := releaseutil.SplitManifests(renderedManifests.String())

	namespaces := make([]*unstructured.Unstructured, 0)

	for _, manifest := range manifests {
		var un unstructured.Unstructured
		err = yaml.Unmarshal([]byte(manifest), &un)
		if err != nil {
			return renderedManifests, err
		}

		if un.GetAPIVersion() == "" || un.GetKind() == "" {
			// skip empty manifests
			continue
		}

		// inject multitenancy-manager labels
		labels := un.GetLabels()
		if labels == nil {
			labels = make(map[string]string, 1)
		}
		labels["heritage"] = "multitenancy-manager"
		un.SetLabels(labels)

		if un.GetAPIVersion() != "v1" || un.GetKind() != "Namespace" {
			data, _ := yaml.Marshal(un.Object)
			builder.WriteString("\n---\n" + string(data))
			continue
		}

		if un.GetName() != ptr.projectName {
			// drop Namespace from manifests if it's not a project namespace
			continue
		}

		namespaces = append(namespaces, &un)
	}

	result := bytes.NewBuffer(nil)

	for _, ns := range namespaces {
		if _, ok := ns.GetAnnotations()["multitenancy-boilerplate"]; ok && len(namespaces) > 1 {
			continue
		}

		data, _ := yaml.Marshal(ns.Object)
		result.WriteString("---\n")
		result.Write(data)
		break
	}

	result.WriteString(builder.String())

	ptr.logger.Debugf("Rendered project %q: \n%s", ptr.projectName, result.String())

	return result, nil
}
