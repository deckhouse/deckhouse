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

package cloudprovider

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

// IsInputSecret reports whether a Secret can change provider template rendering.
func IsInputSecret(object client.Object) bool {
	if object.GetNamespace() != common.KubeSystemNamespace {
		return false
	}
	name := object.GetName()
	return name == common.CloudProviderSecretName ||
		name == common.ClusterConfigSecretName ||
		(strings.HasPrefix(name, "d8-cloud-provider-") &&
			(strings.HasSuffix(name, "-capi") || strings.HasSuffix(name, "-mcm")))
}

// WatchInputs subscribes a controller to the mutable inputs read by Source.
func WatchInputs(w register.Watcher, enqueue handler.EventHandler) {
	w.Watches(&corev1.Secret{}, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(IsInputSecret),
		predicate.ResourceVersionChangedPredicate{},
	))

	moduleConfig := &unstructured.Unstructured{}
	moduleConfig.SetGroupVersionKind(clusterprefix.ModuleConfigGVK())
	w.Watches(moduleConfig, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == clusterprefix.GlobalModuleConfigName
		}),
		predicate.ResourceVersionChangedPredicate{},
	))
}
