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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
)

// postRenderer is the Helm post-renderer the project release goes through. The chart itself renders
// to nothing; the objects come from manifests, rendered natively from the structured ProjectTemplate
// (controller/internal/render). The post-renderer stamps the ownership labels, drops the kinds the
// cluster does not serve, pins every namespaced object to a namespace of the project and records what
// was rendered in the project status.
type postRenderer struct {
	project  *v1alpha3.Project
	versions map[string]struct{}
	logger   logr.Logger
	// manifests, when non-empty, is the source the post-renderer processes instead of the chart's
	// rendered output.
	manifests string
}

func newPostRenderer(project *v1alpha3.Project, versions map[string]struct{}, logger logr.Logger) *postRenderer {
	return &postRenderer{
		project:  project,
		versions: versions,
		logger:   logger.WithName("post-renderer"),
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *postRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// clear resources
	r.project.Status.Resources = make(map[string]map[string]v1alpha3.ResourceKind)

	source := renderedManifests.String()
	if r.manifests != "" {
		source = r.manifests
	}

	var core *unstructured.Unstructured
	builder := strings.Builder{}
	for _, manifest := range releaseutil.SplitManifests(source) {
		object := new(unstructured.Unstructured)
		if err := yaml.Unmarshal([]byte(manifest), object); err != nil {
			r.logger.Info("failed to unmarshal manifest", "project", r.project.Name, "manifest", manifest, "error", err.Error())
			return renderedManifests, err
		}

		// skip empty manifests
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}

		if err := r.processObject(object, &core, &builder); err != nil {
			return renderedManifests, err
		}
	}

	buf := bytes.NewBuffer(nil)

	// ensure core namespace
	if core == nil {
		buf.WriteString("\n---\n" + string(r.newNamespace(r.project.Name)))
	} else {
		data, err := yaml.Marshal(core.Object)
		if err != nil {
			return renderedManifests, fmt.Errorf("marshal core namespace: %w", err)
		}
		buf.WriteString("\n---\n" + string(data))
	}

	buf.WriteString(builder.String())

	return buf, nil
}

// processObject renders a single object into builder, applying label injection. List wrappers
// (kind: List) are expanded recursively because Helm flattens lists after post-rendering.
func (r *postRenderer) processObject(object *unstructured.Unstructured, core **unstructured.Unstructured, builder *strings.Builder) error {
	if object.IsList() {
		return object.EachListItem(func(item runtime.Object) error {
			nested, ok := item.(*unstructured.Unstructured)
			if !ok {
				return nil
			}
			return r.processObject(nested, core, builder)
		})
	}

	// skip resource that not present in the cluster
	if r.versions != nil {
		version := fmt.Sprintf("%s/%s", object.GetAPIVersion(), object.GetKind())
		if _, ok := r.versions[version]; !ok {
			r.project.AddResource(object, false)
			r.logger.Info("the resource skipped during render project", "project", r.project.Name, "resource", object.GetName(), "version", version)
			return nil
		}
	}

	labels := object.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}

	// inject multitenancy-manager labels
	labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	labels[v1alpha3.ResourceLabelProject] = r.project.Name
	labels[v1alpha3.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

	object.SetLabels(labels)

	if object.GetKind() == "Namespace" {
		// skip other namespaces
		if object.GetName() == r.project.Name {
			r.project.AddResource(object, true)
			*core = object
		}

		return nil
	}

	// Namespaced objects may target ANY namespace of the project (main + additional): the renderer
	// emits NetworkPolicy/PodLoggingConfig once per project namespace. The render and this
	// post-renderer derive the project namespace set from the same project.Status.Namespaces, so every
	// rendered target is allowed here (no duplicates). An empty namespace defaults to main, and so
	// does a namespace outside the project -- nothing the renderer emits targets one, so this is a
	// backstop, not a feature.
	if ns := object.GetNamespace(); ns == "" || !r.isProjectNamespace(ns) {
		object.SetNamespace(r.project.Name)
	}

	r.project.AddResource(object, true)

	data, err := yaml.Marshal(object.Object)
	if err != nil {
		return fmt.Errorf("marshal rendered object %s/%s: %w", object.GetKind(), object.GetName(), err)
	}
	builder.WriteString("\n---\n" + string(data))
	return nil
}

// isProjectNamespace reports whether ns is the project's main namespace or one of its additional
// namespaces (from status.namespaces).
func (r *postRenderer) isProjectNamespace(ns string) bool {
	if ns == r.project.Name {
		return true
	}
	for _, s := range r.project.Status.Namespaces {
		if s.Name == ns {
			return true
		}
	}
	return false
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

	obj.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	obj.Labels[v1alpha3.ResourceLabelProject] = r.project.Name
	obj.Labels[v1alpha3.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

	data, _ := yaml.Marshal(obj) //nolint:errcheck // marshaling a static in-memory corev1.Namespace cannot fail
	return data
}
