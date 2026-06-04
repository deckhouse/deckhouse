/*
Copyright 2025 Flant JSC

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

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
)

func CacheOptions() (cache.Options, client.Options) {
	stripManagedFields := cache.TransformStripManagedFields()

	machineNS := cache.ByObject{
		Namespaces: map[string]cache.Config{
			MachineNamespace: {},
		},
	}

	kubeSystemSecrets, _ := labels.NewRequirement("name", selection.In, []string{
		"d8-node-manager-cloud-provider",
		"d8-cluster-configuration",
		"d8-provider-cluster-configuration",
	})

	cacheOpts := cache.Options{
		DefaultTransform: func(obj interface{}) (interface{}, error) {
			stripNodeHeavyFields(obj)
			return stripManagedFields(obj)
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
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
			},
			&mcmv1alpha1.Machine{}: machineNS,
			&capiv1beta2.Machine{}: machineNS,
			newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment"): machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment"):     machineNS,
		},
	}

	clientOpts := client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.Pod{},
				&coordinationv1.Lease{},
			},
		},
	}

	return cacheOpts, clientOpts
}

func stripNodeHeavyFields(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return
	}
	node.Status.Images = nil
	node.Status.NodeInfo = corev1.NodeSystemInfo{}
	node.Status.Addresses = nil
	node.Status.Capacity = nil
	node.Status.Allocatable = nil
	node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{}
	node.Status.VolumesAttached = nil
	node.Status.VolumesInUse = nil
	node.Spec.PodCIDR = ""
	node.Spec.PodCIDRs = nil
}

func newUnstructured(group, version, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return u
}
