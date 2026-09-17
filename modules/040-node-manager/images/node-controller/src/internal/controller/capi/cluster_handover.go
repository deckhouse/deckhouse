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

// Helm's ownership metadata is left on the objects taken over from the chart: addon-operator
// upgrades with TakeOwnership, so Helm adopts a live object by kind and name and never reads that
// metadata, and stripping it would cost a read plus a patch on every apply.
//
// TODO(1.81): drop the keep annotation together with hooks/set_keep_policy_on_capi_resources.go.
// Prune only considers the previous release manifest, so past the migration release it protects
// nothing — but it still blocks deletion on uninstall, capi-user-credentials included.
const (
	helmManagedByLabel        = "app.kubernetes.io/managed-by"
	helmReleaseNameAnnotation = "meta.helm.sh/release-name"
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
