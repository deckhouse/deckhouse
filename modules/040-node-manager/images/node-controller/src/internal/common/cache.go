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
	"context"

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WebhookCacheOptions returns cache and client options for the webhook manager.
// Webhooks only need a small set of Secrets from kube-system for validation and conversion.
// Node and Endpoints are read rarely and go directly to the API server.
func WebhookCacheOptions() (cache.Options, client.Options) {
	configSecretsReq, _ := labels.NewRequirement("name", selection.In, []string{
		"d8-cluster-configuration",
		"d8-provider-cluster-configuration",
	})

	cacheOpts := cache.Options{
		DefaultTransform: cache.TransformStripManagedFields(),
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
				Namespaces: map[string]cache.Config{
					"kube-system": {
						LabelSelector: labels.NewSelector().Add(*configSecretsReq),
					},
				},
			},
		},
	}

	clientOpts := client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.Node{},
				&corev1.Endpoints{},
			},
		},
	}

	return cacheOpts, clientOpts
}

// controllerSecretCacheConfig returns per-namespace cache config for Secrets.
// Only specific secrets needed by controllers are cached:
//   - d8-cloud-instance-manager/configuration-checksums (nodegroup-status, update-approval)
//   - kube-system/d8-node-manager-cloud-provider (nodegroup-status)
func controllerSecretCacheConfig() cache.ByObject {
	return cache.ByObject{
		Namespaces: map[string]cache.Config{
			MachineNamespace: {
				FieldSelector: fields.SelectorFromSet(fields.Set{
					"metadata.name": ConfigurationChecksumsSecretName,
				}),
			},
			"kube-system": {
				FieldSelector: fields.SelectorFromSet(fields.Set{
					"metadata.name": "d8-node-manager-cloud-provider",
				}),
			},
		},
	}
}

// machineNamespaceOnly returns a cache.ByObject that restricts an informer
// to the d8-cloud-instance-manager namespace (where all MCM/CAPI machines live).
func machineNamespaceOnly() cache.ByObject {
	return cache.ByObject{
		Namespaces: map[string]cache.Config{
			MachineNamespace: {},
		},
	}
}

// newUnstructured creates an Unstructured object with the given GVK.
func newUnstructured(group, version, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return u
}

// ControllerCacheOptions returns cache options for the controller manager.
func ControllerCacheOptions(ctx context.Context, logger logr.Logger) cache.Options {
	machineNS := machineNamespaceOnly()
	return cache.Options{
		DefaultTransform: CacheTransformWithLogging(ctx, logger),
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: controllerSecretCacheConfig(),
			// Machine/MachineDeployment only exist in d8-cloud-instance-manager.
			newUnstructured("machine.sapcloud.io", "v1alpha1", "Machine"):           machineNS,
			newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment"): machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "Machine"):              machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment"):    machineNS,
		},
	}
}

// ControllerClientOptions returns client options for the controller manager.
// Pod and Lease are excluded from cache — they are only needed by the fencing
// controller which reads them rarely via direct API calls.
func ControllerClientOptions() client.Options {
	return client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.Pod{},
				&coordinationv1.Lease{},
			},
		},
	}
}

// ControllerCacheOptionsWithTransform is like ControllerCacheOptions but allows
// overriding the default transform function (e.g. for testing without logging).
func ControllerCacheOptionsWithTransform(transform toolscache.TransformFunc) cache.Options {
	machineNS := machineNamespaceOnly()
	return cache.Options{
		DefaultTransform: transform,
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: controllerSecretCacheConfig(),
			newUnstructured("machine.sapcloud.io", "v1alpha1", "Machine"):           machineNS,
			newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment"): machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "Machine"):              machineNS,
			newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment"):    machineNS,
		},
	}
}
