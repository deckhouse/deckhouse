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

package capi

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	helmManagedByLabel             = "app.kubernetes.io/managed-by"
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
	werfFailModeAnnotation         = "werf.io/fail-mode"
	werfTrackTerminationAnnotation = "werf.io/track-termination-mode"
)

func prepareClusterTemplateObject(object *unstructured.Unstructured) {
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["heritage"] = "deckhouse"
	labels["module"] = "node-manager"
	object.SetLabels(labels)

	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["helm.sh/resource-policy"] = "keep"
	object.SetAnnotations(annotations)
}

func removeLegacyHelmMetadata(object *unstructured.Unstructured) bool {
	changed := false

	labels := object.GetLabels()
	if _, ok := labels[helmManagedByLabel]; ok {
		delete(labels, helmManagedByLabel)
		object.SetLabels(labels)
		changed = true
	}

	annotations := object.GetAnnotations()
	for _, key := range []string{
		helmReleaseNameAnnotation,
		helmReleaseNamespaceAnnotation,
		werfFailModeAnnotation,
		werfTrackTerminationAnnotation,
	} {
		if _, ok := annotations[key]; ok {
			delete(annotations, key)
			changed = true
		}
	}
	if changed {
		object.SetAnnotations(annotations)
	}
	return changed
}
