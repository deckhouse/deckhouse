/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package virtualcontrolplaneconfiguration

import (
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func setVCPControllerReference(vcp metav1.Object, obj client.Object, scheme *runtime.Scheme) error {
	return ctrl.SetControllerReference(vcp, obj, scheme)
}

func ownerReferencesDiffer(current, target client.Object) bool {
	return !equality.Semantic.DeepEqual(current.GetOwnerReferences(), target.GetOwnerReferences())
}

func syncOwnerReferences(current, target client.Object) {
	current.SetOwnerReferences(target.GetOwnerReferences())
}

// patchSpecDataAndOwnerRefs reconciles spec/data and heals OwnerReferences without touching status.
func patchSpecDataAndOwnerRefs(current, target *unstructured.Unstructured) (client.Object, bool) {
	changed := false

	if !equality.Semantic.DeepEqual(current.Object["spec"], target.Object["spec"]) ||
		!equality.Semantic.DeepEqual(current.Object["data"], target.Object["data"]) {
		if spec, ok := target.Object["spec"]; ok {
			current.Object["spec"] = spec
		}
		if data, ok := target.Object["data"]; ok {
			current.Object["data"] = data
		}
		changed = true
	}

	if ownerReferencesDiffer(current, target) {
		syncOwnerReferences(current, target)
		changed = true
	}

	if !changed {
		return nil, false
	}
	return current, true
}
