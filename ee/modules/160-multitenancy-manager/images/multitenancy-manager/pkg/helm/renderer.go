/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helm

import (
	"bytes"
	"strings"

	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	ProjectRequireSyncAnnotation = "projects.deckhouse.io/require-sync"
	ProjectLabel                 = "projects.deckhouse.io/project"
	ProjectTemplateLabel         = "projects.deckhouse.io/project-template"
	HeritageLabel                = "heritage"
	HeritageValue                = "multitenancy-manager"
)

type PostRenderer struct {
	projectName     string
	projectTemplate string
}

func newPostRenderer(projectName, projectTemplate string) *PostRenderer {
	return &PostRenderer{
		projectName:     projectName,
		projectTemplate: projectTemplate,
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *PostRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	if r.projectName == "" {
		return renderedManifests, nil
	}
	builder := strings.Builder{}
	manifests := releaseutil.SplitManifests(renderedManifests.String())
	var namespaces []*unstructured.Unstructured
	for _, manifest := range manifests {
		var object unstructured.Unstructured
		if err = yaml.Unmarshal([]byte(manifest), &object); err != nil {
			return renderedManifests, err
		}
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			// skip empty manifests
			continue
		}
		// inject multitenancy-manager labels
		labels := object.GetLabels()
		if labels == nil {
			labels = make(map[string]string, 1)
		}
		labels[HeritageLabel] = HeritageValue
		labels[ProjectLabel] = r.projectName
		labels[ProjectTemplateLabel] = r.projectTemplate
		object.SetLabels(labels)
		if object.GetAPIVersion() != "v1" || object.GetKind() != "Namespace" {
			object.SetNamespace(r.projectName)
			data, _ := yaml.Marshal(object.Object)
			builder.WriteString("\n---\n" + string(data))
			continue
		}
		if object.GetName() != r.projectName {
			// drop Namespace from manifests if it's not a project namespace
			continue
		}
		namespaces = append(namespaces, &object)
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
	return result, nil
}
