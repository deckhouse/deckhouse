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

package common

import (
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CacheByObject() map[client.Object]cache.ByObject {
	machineNS := cache.ByObject{
		Namespaces: map[string]cache.Config{
			MachineNamespace: {},
		},
	}

	return map[client.Object]cache.ByObject{
		&corev1.Secret{}: secretCacheConfig(),
		newUnstructured("machine.sapcloud.io", "v1alpha1", "Machine"):           machineNS,
		newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment"): machineNS,
		newUnstructured("cluster.x-k8s.io", "v1beta2", "Machine"):              machineNS,
		newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment"):    machineNS,
	}
}

func ControllerDisableFor() []client.Object {
	return []client.Object{
		&corev1.Pod{},
		&coordinationv1.Lease{},
	}
}

func secretCacheConfig() cache.ByObject {
	kubeSystemSecrets, _ := labels.NewRequirement("name", selection.In, []string{
		"d8-node-manager-cloud-provider",
		"d8-cluster-configuration",
		"d8-provider-cluster-configuration",
	})

	return cache.ByObject{
		Namespaces: map[string]cache.Config{
			MachineNamespace: {
				FieldSelector: fields.SelectorFromSet(fields.Set{
					"metadata.name": ConfigurationChecksumsSecretName,
				}),
			},
			"kube-system": {
				LabelSelector: labels.NewSelector().Add(*kubeSystemSecrets),
			},
		},
	}
}

func newUnstructured(group, version, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return u
}
