/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helm

import (
	"bytes"
	"strings"

	"helm.sh/helm/v3/pkg/releaseutil"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

const (
	ProjectRequireSyncAnnotation = "projects.deckhouse.io/require-sync"
	ProjectLabel                 = "projects.deckhouse.io/project"
)

const ProjectTemplateLabel = "projects.deckhouse.io/project-template"

const (
	HeritageLabel = "heritage"
	HeritageValue = "multitenancy-manager"
)

type postRenderer struct {
	projectName     string
	projectTemplate string
}

func newPostRenderer(projectName, projectTemplate string) *postRenderer {
	return &postRenderer{
		projectName:     projectName,
		projectTemplate: projectTemplate,
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *postRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	var coreFound bool
	builder := strings.Builder{}
	for _, manifest := range releaseutil.SplitManifests(renderedManifests.String()) {
		var object unstructured.Unstructured
		if err = yaml.Unmarshal([]byte(manifest), &object); err != nil {
			return renderedManifests, err
		}

		// skip empty manifests
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
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

		if object.GetKind() == "Namespace" {
			// skip other namespaces
			if object.GetName() != r.projectName {
				continue
			}
			coreFound = true
		} else {
			object.SetNamespace(r.projectName)
		}

		data, _ := yaml.Marshal(object.Object)
		builder.WriteString("\n---\n" + string(data))
	}

	// ensure core namespace
	if !coreFound {
		core := r.makeNamespace(r.projectName)
		builder.WriteString("\n---\n" + string(core))
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString(builder.String())

	return buf, nil
}

func (r *postRenderer) makeNamespace(name string) []byte {
	obj := v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				ProjectLabel:         r.projectName,
				ProjectTemplateLabel: r.projectTemplate,
				HeritageLabel:        HeritageValue,
			},
		},
	}
	data, _ := yaml.Marshal(obj)
	return data
}
