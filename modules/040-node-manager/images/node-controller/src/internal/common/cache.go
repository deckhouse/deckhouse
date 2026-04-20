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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CacheOptions returns cache and client options for the manager.
func CacheOptions() (cache.Options, client.Options) {
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
