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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
)

type postRenderer struct {
	project  *v1alpha2.Project
	versions map[string]struct{}
	logger   logr.Logger
}

func newPostRenderer(project *v1alpha2.Project, versions map[string]struct{}, logger logr.Logger) *postRenderer {
	return &postRenderer{
		project:  project,
		versions: versions,
		logger:   logger.WithName("post-renderer"),
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *postRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	// clear resources
	r.project.Status.Resources = make(map[string]map[string]v1alpha2.ResourceKind)

	var coreFound bool
	builder := strings.Builder{}
	for _, manifest := range releaseutil.SplitManifests(renderedManifests.String()) {
		object := new(unstructured.Unstructured)
		if err = yaml.Unmarshal([]byte(manifest), object); err != nil {
			r.logger.Info("failed to unmarshal manifest", "project", r.project.Name, "manifest", manifest, "error", err.Error())
			return renderedManifests, err
		}

		// skip empty manifests
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}

		// skip resource that not present in the cluster
		if r.versions != nil {
			version := fmt.Sprintf("%s/%s", object.GetAPIVersion(), object.GetKind())
			if _, ok := r.versions[version]; !ok {
				r.project.AddResource(object, false)
				r.logger.Info("the resource skipped during render project", "project", r.project.Name, "resource", object.GetName(), "version", version)
				continue
			}
		}

		// inject project annotations
		if len(r.project.Spec.ResourceAnnotations) != 0 {
			annotations := object.GetAnnotations()
			if len(annotations) == 0 {
				annotations = map[string]string{}
			}
			for k, v := range r.project.Spec.ResourceAnnotations {
				annotations[k] = v
			}
			object.SetAnnotations(annotations)
		}

		labels := object.GetLabels()
		if len(labels) == 0 {
			labels = make(map[string]string)
		}

		// inject project labels
		for k, v := range r.project.Spec.ResourceLabels {
			labels[k] = v
		}

		// inject multitenancy-manager
		labels[v1alpha2.ResourceLabelHeritage] = v1alpha2.ResourceHeritageMultitenancy
		labels[v1alpha2.ResourceLabelProject] = r.project.Name
		labels[v1alpha2.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

		object.SetLabels(labels)

		if object.GetKind() != "Namespace" {
			object.SetNamespace(r.project.Name)
		} else {
			// skip other namespaces
			if object.GetName() != r.project.Name {
				r.logger.Info("namespace skipped during render project", "project", r.project.Name, "namespace", object.GetName())
				continue
			}
			coreFound = true
		}

		r.project.AddResource(object, true)

		data, _ := yaml.Marshal(object.Object)
		builder.WriteString("\n---\n" + string(data))
	}

	buf := bytes.NewBuffer(nil)
	// ensure core namespace
	if !coreFound {
		core := r.newNamespace(r.project.Name)
		buf.WriteString("\n---\n" + string(core))
	}
	buf.WriteString(builder.String())

	return buf, nil
}

func (r *postRenderer) newNamespace(name string) []byte {
	obj := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}

	// inject project annotations
	if len(r.project.Spec.ResourceAnnotations) != 0 {
		obj.SetAnnotations(r.project.Spec.ResourceAnnotations)
	}

	// inject project labels
	if len(r.project.Spec.ResourceLabels) != 0 {
		obj.SetLabels(r.project.Spec.ResourceLabels)
	}

	obj.Labels[v1alpha2.ResourceLabelHeritage] = v1alpha2.ResourceHeritageMultitenancy
	obj.Labels[v1alpha2.ResourceLabelProject] = r.project.Name
	obj.Labels[v1alpha2.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

	data, _ := yaml.Marshal(obj)
	return data
}
