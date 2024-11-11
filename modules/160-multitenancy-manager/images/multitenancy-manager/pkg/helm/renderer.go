/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"bytes"
	"strings"

	"github.com/go-logr/logr"

	"controller/pkg/consts"

	"helm.sh/helm/v3/pkg/releaseutil"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

type postRenderer struct {
	projectName     string
	projectTemplate string
	log             logr.Logger
}

func newPostRenderer(projectName, projectTemplate string, log logr.Logger) *postRenderer {
	return &postRenderer{
		projectName:     projectName,
		projectTemplate: projectTemplate,
		log:             log.WithName("post-renderer"),
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
			r.log.Info("failed to unmarshal manifest", "project", r.projectName, "manifest", manifest, "error", err.Error())
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
		labels[consts.HeritageLabel] = consts.MultitenancyHeritage
		labels[consts.ProjectLabel] = r.projectName
		labels[consts.ProjectTemplateLabel] = r.projectTemplate
		object.SetLabels(labels)

		if object.GetKind() == "Namespace" {
			// skip other namespaces
			if object.GetName() != r.projectName {
				r.log.Info("namespace is skipped during render project", "project", r.projectName, "namespace", object.GetName())
				continue
			}
			coreFound = true
		} else {
			object.SetNamespace(r.projectName)
		}

		data, _ := yaml.Marshal(object.Object)
		builder.WriteString("\n---\n" + string(data))
	}

	buf := bytes.NewBuffer(nil)
	// ensure core namespace
	if !coreFound {
		core := r.makeNamespace(r.projectName)
		buf.WriteString("\n---\n" + string(core))
	}
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
				consts.ProjectLabel:         r.projectName,
				consts.ProjectTemplateLabel: r.projectTemplate,
				consts.HeritageLabel:        consts.MultitenancyHeritage,
			},
		},
	}
	data, _ := yaml.Marshal(obj)
	return data
}
