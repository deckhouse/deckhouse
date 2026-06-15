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
	"errors"
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

var (
	ErrNamespaceOverride = errors.New("objects that defined in different namespaces will still be deployed to project namespace")
)

// filteredKinds lists the resource kinds that are no longer allowed in project templates: they are
// now managed by the controller from the Project spec (quota -> ResourceQuota, administrators ->
// ProjectRoleBinding -> AuthorizationRule). Such objects are dropped during rendering.
var filteredKinds = map[string]struct{}{
	"ResourceQuota":     {},
	"AuthorizationRule": {},
}

type postRenderer struct {
	project        *v1alpha3.Project
	versions       map[string]struct{}
	logger         logr.Logger
	warning        error
	isFirstInstall bool
	// filtered is set when at least one ResourceQuota/AuthorizationRule object was dropped.
	filtered bool
}

func newPostRenderer(project *v1alpha3.Project, versions map[string]struct{}, logger logr.Logger, isFirstInstall bool) *postRenderer {
	return &postRenderer{
		project:        project,
		versions:       versions,
		logger:         logger.WithName("post-renderer"),
		isFirstInstall: isFirstInstall,
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *postRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// clear resources
	r.project.Status.Resources = make(map[string]map[string]v1alpha3.ResourceKind)

	var core *unstructured.Unstructured
	builder := strings.Builder{}
	for _, manifest := range releaseutil.SplitManifests(renderedManifests.String()) {
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

// processObject renders a single object into builder, applying the managed-by filter and label
// injection. List wrappers (kind: List) are expanded recursively because Helm flattens lists after
// post-rendering: without this, a filtered kind smuggled inside a List would slip past the filter.
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

	// drop resources that are now managed by the controller from the Project spec
	// (ResourceQuota via spec.quota, AuthorizationRule via spec.administrators).
	if _, ok := filteredKinds[object.GetKind()]; ok {
		r.filtered = true
		r.logger.Info("the resource is managed by the project spec and was filtered out", "project", r.project.Name, "resource", object.GetName(), "kind", object.GetKind())
		return nil
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

	// check if resource should be excluded from management
	isUnmanaged := false
	if _, ok := labels[v1alpha3.ResourceLabelUnmanaged]; ok {
		isUnmanaged = true
		// Include unmanaged resources only on first install to create them once
		// On subsequent upgrades, skip them so they won't be updated
		if !r.isFirstInstall {
			r.logger.Info("the resource is unmanaged and will be skipped (not first install)", "project", r.project.Name, "resource", object.GetName(), "kind", object.GetKind())
			return nil
		}
		// On first install, include unmanaged resources but mark them to not be tracked
		r.logger.Info("the resource is unmanaged but will be created on first install", "project", r.project.Name, "resource", object.GetName(), "kind", object.GetKind())
		// Add Helm resource-policy annotation to prevent deletion on release uninstall
		annotations := object.GetAnnotations()
		if len(annotations) == 0 {
			annotations = make(map[string]string)
		}
		annotations["helm.sh/resource-policy"] = "keep"
		object.SetAnnotations(annotations)
	}

	// inject multitenancy-manager labels
	// For unmanaged resources, only add project and template labels, not heritage
	// For other resources, skip heritage label if ResourceLabelSkipHeritage is set
	if !isUnmanaged {
		if _, skipHeritage := labels[v1alpha3.ResourceLabelSkipHeritage]; !skipHeritage {
			labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		}
	}
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

	if object.GetNamespace() != "" && object.GetNamespace() != r.project.Name {
		r.warning = ErrNamespaceOverride
	}

	object.SetNamespace(r.project.Name)

	// Track resource in project status only if it's not unmanaged
	// Unmanaged resources are created once but not tracked/updated
	if _, isUnmanaged := labels[v1alpha3.ResourceLabelUnmanaged]; !isUnmanaged {
		r.project.AddResource(object, true)
	}

	data, err := yaml.Marshal(object.Object)
	if err != nil {
		return fmt.Errorf("marshal rendered object %s/%s: %w", object.GetKind(), object.GetName(), err)
	}
	builder.WriteString("\n---\n" + string(data))
	return nil
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

	data, _ := yaml.Marshal(obj)
	return data
}
