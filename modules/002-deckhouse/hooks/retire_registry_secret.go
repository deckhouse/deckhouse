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

// Retiring `deckhouse-registry` while the registry module serves the cluster's images, and bringing it
// back when it stops.
//
// The secret is this platform's original way of telling kubelet how to authenticate to the registry:
// dhctl writes it at bootstrap, this module renders it, and about a hundred manifests across the
// platform name it in `imagePullSecrets`. The design ADR for the registry model (decision 22) asks for
// that contour to go, and it cannot go while anything depends on it — so what this hook does is remove
// the dependency's object exactly when it has no reader, and let it return the moment it has one again.
//
// Why it has no reader while the module serves the images: credentials then live on the NODE, in the
// container runtime's per-registry configuration, written by the module's agent. A pod needs none of its
// own. Measured on an air-gapped cluster: a pod with an empty `imagePullSecrets` pulled from the
// in-cluster registry in 72 ms, and a pod naming a secret that does not exist pulled just as well —
// kubelet says `FailedToRetrieveImagePullSecret` and proceeds. That second measurement is what makes
// this safe to do before the hundred manifests are cleaned up: their references degrade to a warning
// rather than a failure, so they can be removed at leisure instead of in one campaign.
//
// # What decides it
//
// One fact, and deliberately not "is the module enabled": a module that is enabled but Unmanaged serves
// nothing, and the node's credentials are then built out of THIS secret — removing it would take the
// cluster's only way to pull. The fact is the ConfigMap `d8-system/registry-image-address`, which the
// registry module renders only while it manages the pull path AND every node's agent has applied the
// layout it was given; the module's own Helm release withdraws it when it stops managing or is disabled.
// So the ConfigMap means "the replacement is in place and proven", which is the only condition under
// which the old credentials are safe to take away.
//
// # Why a hook and not just a condition in the template
//
// Both, in fact — the template stops rendering it, or the next Helm release would put it straight back.
// But the secret carries `helm.sh/resource-policy: keep`, so Helm will not delete it when it disappears
// from the release: that annotation exists to protect a cluster from losing its pull credentials to a
// failed render, and it is worth keeping. The deletion therefore has to be explicit, which is this hook.
//
// Coming back needs no hook at all: the ConfigMap goes away, the template renders the secret again, and
// Helm creates it. Reversibility lives in the template, which is where a declarative system should keep
// it.
package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// RegistrySecretName is the contour being retired.
	RegistrySecretName = "deckhouse-registry"

	// ImageAddressConfigMapName is the registry module's record that it serves the cluster's images.
	// Spelled out rather than imported: this module deliberately shares no code with that one, and a
	// rename shows up here as a cluster that keeps its old credentials — which is the safe direction.
	ImageAddressConfigMapName = "registry-image-address"

	servedByModuleSnapName   = "registry-image-address"
	servedByModuleValuesPath = "deckhouse.internal.registry.servedByModule"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/retire-registry-secret",
	// Before the render that reads the value this publishes.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       servedByModuleSnapName,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{ImageAddressConfigMapName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				// Only that it is there. What is inside it is the registry module's business, and
				// reading it here would be this module forming an opinion about another's data.
				return obj.GetName(), nil
			},
		},
	},
}, handleRetireRegistrySecret)

func handleRetireRegistrySecret(_ context.Context, input *go_hook.HookInput) error {
	served := len(input.Snapshots.Get(servedByModuleSnapName)) > 0

	input.Values.Set(servedByModuleValuesPath, served)

	if !served {
		// Either no registry module manages the pull path, or it has stopped. The template renders the
		// secret again on this very release, so there is nothing to do here but stay out of the way.
		return nil
	}

	// Present and unconditional: a delete of something already gone costs one no-op request, while
	// remembering whether it was done would be state that can be remembered wrongly.
	input.PatchCollector.Delete("v1", "Secret", "d8-system", RegistrySecretName)
	input.Logger.Info("the registry module serves the cluster's images, so the credentials it replaces are withdrawn",
		"secret", fmt.Sprintf("d8-system/%s", RegistrySecretName))

	return nil
}
