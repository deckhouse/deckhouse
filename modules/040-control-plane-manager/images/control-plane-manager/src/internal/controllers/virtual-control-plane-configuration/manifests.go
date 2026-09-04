/*
Copyright 2026 Flant JSC

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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"strings"

	"control-plane-manager/internal/constants"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// applyVCPScope stamps the VCP scope settings on a vcp-owned deployment
func applyVCPScope(dep *appsv1.Deployment, vcpName string) {
	if dep.Labels == nil {
		dep.Labels = map[string]string{}
	}
	dep.Labels[constants.VirtualControlPlaneScopeLabelKey] = vcpName

	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{}
	}
	if dep.Spec.Selector.MatchLabels == nil {
		dep.Spec.Selector.MatchLabels = map[string]string{}
	}
	dep.Spec.Selector.MatchLabels[constants.VirtualControlPlaneScopeLabelKey] = vcpName

	if dep.Spec.Template.Labels == nil {
		dep.Spec.Template.Labels = map[string]string{}
	}
	dep.Spec.Template.Labels[constants.VirtualControlPlaneScopeLabelKey] = vcpName
}

// renameImagePullSecret rewrites a Deployment pod template imagePullSecret from the embedded manifest placeholder name to the per-VCP scoped name.
func renameImagePullSecret(dep *appsv1.Deployment, from, to string) {
	for i := range dep.Spec.Template.Spec.ImagePullSecrets {
		if dep.Spec.Template.Spec.ImagePullSecrets[i].Name == from {
			dep.Spec.Template.Spec.ImagePullSecrets[i].Name = to
		}
	}
}

// parseManifestDocs splits a multi-doc YAML blob (config-Secret template) into unstructured
// objects, skipping blank docs. If defaultNamespace is non-empty it is set on objects that lack one.
func parseManifestDocs(raw []byte, defaultNamespace string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured
	for _, doc := range strings.Split(string(raw), "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			return nil, fmt.Errorf("decode manifest: %w", err)
		}
		if len(obj.Object) == 0 {
			continue
		}
		if defaultNamespace != "" && obj.GetNamespace() == "" {
			obj.SetNamespace(defaultNamespace)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

// applyObject creates target if absent, otherwise patches it. mutate builds the object to patch
// from (current, target); returning ok=false skips the patch (no change needed).
func applyObject(ctx context.Context, cl client.Client, target *unstructured.Unstructured, mutate func(current, target *unstructured.Unstructured) (client.Object, bool)) error {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(target.GroupVersionKind())

	err := cl.Get(ctx, client.ObjectKeyFromObject(target), current)
	if apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, target); err != nil {
			return fmt.Errorf("create %s %s: %w", target.GetKind(), target.GetName(), err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get %s %s: %w", target.GetKind(), target.GetName(), err)
	}

	base := current.DeepCopy()
	patched, ok := mutate(current, target)
	if !ok {
		return nil
	}
	if err := cl.Patch(ctx, patched, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch %s %s: %w", target.GetKind(), target.GetName(), err)
	}

	return nil
}
