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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	capiCRDSelector := labels.SelectorFromSet(labels.Set{
		"cluster.x-k8s.io/provider": "cluster-api",
	})

	machineNSSecretReq, _ := labels.NewRequirement(
		"app",
		selection.In,
		[]string{"bashible-apiserver", "node-controller", "capi-controller-manager"},
	)
	machineNSSecretSelector := labels.NewSelector().Add(*machineNSSecretReq)

	dnsServiceReq, _ := labels.NewRequirement("k8s-app", selection.In, []string{"kube-dns", "coredns"})
	dnsServiceSelector := labels.NewSelector().Add(*dnsServiceReq)

	apiserverPodSelector := labels.SelectorFromSet(labels.Set{
		"component": "kube-apiserver",
		"tier":      "control-plane",
	})

	cacheOpts := cache.Options{
		DefaultTransform: func(obj interface{}) (interface{}, error) {
			stripNodeHeavyFields(obj)
			return stripManagedFields(obj)
		},
		ByObject: map[client.Object]cache.ByObject{
			&apiextensionsv1.CustomResourceDefinition{}: {
				Label: capiCRDSelector,
			},
			&corev1.Secret{}: {
				Namespaces: map[string]cache.Config{
					MachineNamespace: {
						LabelSelector: machineNSSecretSelector,
					},
					// Unfiltered on purpose: the NodeGroup webhook and the derived-status
					// service read d8-cluster-configuration (and the provider configs) on
					// every NodeGroup write, and a name FieldSelector can list only one
					// secret — the rest became live GETs on the apiserver hot path (and the
					// webhook's cached reads silently missed). All kube-system secrets are
					// ~140KiB total, so caching them all is cheaper than one live GET.
					"kube-system": {},
				},
			},
			&corev1.Pod{}: {
				Namespaces: map[string]cache.Config{
					"kube-system": {LabelSelector: apiserverPodSelector},
				},
			},
			&corev1.Service{}: {
				Namespaces: map[string]cache.Config{
					"kube-system": {LabelSelector: dnsServiceSelector},
				},
			},
			&corev1.ConfigMap{}: {
				Namespaces: map[string]cache.Config{
					"kube-system": {
						FieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": "d8-cluster-uuid"}),
					},
					"d8-system": {
						FieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": "d8-deckhouse-version-info"}),
					},
				},
			},
			&mcmv1alpha1.Machine{}: machineNS,
			&capiv1beta2.Machine{}: machineNS,
			// NOTE: ByObject keys are mapped by GVK, so a typed and an unstructured key of
			// the same kind (e.g. corev1.Secret and an unstructured v1/Secret) COLLIDE: map
			// iteration order decides which scope wins and the loser's reads break
			// non-deterministically. Never add per-representation Secret entries here.
			newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment"):                 machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment"):                     machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "Cluster"):                               machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineHealthCheck"):                    machineNS,
			newUnstructured("infrastructure.cluster.x-k8s.io", "v1alpha1", "DeckhouseControlPlane"): machineNS,
			// The NodeGroup webhook reads only ModuleConfig "global"; without this scope the
			// lazily-created informer would watch and cache every ModuleConfig cluster-wide.
			newUnstructured("deckhouse.io", "v1alpha1", "ModuleConfig"): {
				Field: fields.SelectorFromSet(fields.Set{"metadata.name": "global"}),
			},
		},
	}

	clientOpts := client.Options{
		Cache: &client.CacheOptions{
			// Serve unstructured reads from informers too: InstanceClass objects and the
			// InstanceTypesCatalog are read as unstructured on every derived-status pass,
			// and the default (uncached) client turns each of them into a live apiserver
			// GET/List — hundreds of requests during a NodeGroup burst. Informers keep the
			// data watch-fresh; wide unstructured kinds (MachineDeployment, Cluster, ...)
			// are already namespace/label-scoped via ByObject above.
			Unstructured: true,
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
	node.Status.NodeInfo = corev1.NodeSystemInfo{KubeletVersion: node.Status.NodeInfo.KubeletVersion}
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
