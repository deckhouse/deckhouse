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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Transitional: in a virtual control plane tenant bashible-apiserver-context was created by the
// parent's control-plane-manager, which stamped no Helm metadata on it. Now that the Secret is
// rendered by this module, Helm refuses to adopt it ("invalid ownership metadata") and the whole
// release fails, so the metadata is written here before the chart runs. Delete this hook once
// every tenant has reconciled at least once past the handover.
const (
	bashibleContextSecretNamespace = "d8-cloud-instance-manager"
	bashibleContextSecretName      = "bashible-apiserver-context"
	// The key templates/bashible-apiserver/context-secret.yaml renders the context under.
	bashibleContextSecretKey = "input.yaml"

	// The release the chart runs under: addon-operator names the release after the module and
	// stores it in ADDON_OPERATOR_NAMESPACE, which the tenant Deployment sets to d8-system so
	// that leases and releases land in the tenant rather than in the parent's vcp-<name>.
	bashibleContextReleaseName      = "node-manager"
	bashibleContextReleaseNamespace = "d8-system"
)

// bashibleContextHelmOwnership is the exact set Helm's ownership check requires, and nothing
// else: the Secret holds the live bashible context and the patch must not reach its data.
var bashibleContextHelmOwnership = map[string]interface{}{
	"metadata": map[string]interface{}{
		"labels": map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
		},
		"annotations": map[string]string{
			"meta.helm.sh/release-name":      bashibleContextReleaseName,
			"meta.helm.sh/release-namespace": bashibleContextReleaseNamespace,
		},
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/bashible_context_vcp",
	// After handleBashibleContextVCP (order 20): it decides whether the Secret is rendered at
	// all, and there is no point adopting a Secret the chart is not about to claim.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "context",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{bashibleContextSecretNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{bashibleContextSecretName}},
			FilterFunc:   filterBashibleContextOwnership,
		},
	},
}, handleBashibleContextAdoptVCP)

// bashibleContextOwnership is all the snapshot carries. The Secret's data is the context itself
// and has no business being copied into hook memory.
type bashibleContextOwnership struct {
	Adopted bool `json:"adopted"`
}

func filterBashibleContextOwnership(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	return bashibleContextOwnership{Adopted: helmOwns(secret)}, nil
}

func helmOwns(secret *corev1.Secret) bool {
	return secret.Labels["app.kubernetes.io/managed-by"] == "Helm" &&
		secret.Annotations["meta.helm.sh/release-name"] == bashibleContextReleaseName &&
		secret.Annotations["meta.helm.sh/release-namespace"] == bashibleContextReleaseNamespace
}

// handleBashibleContextAdoptVCP hands a control-plane-manager-created bashible-apiserver-context
// over to this module's Helm release. It runs in the beforeHelm phase, whose patches are applied
// before the chart is rendered, so the release never sees the unowned Secret.
func handleBashibleContextAdoptVCP(_ context.Context, input *go_hook.HookInput) error {
	if !nestedControlPlane(input) {
		return nil
	}

	// The same condition templates/bashible-apiserver/context-secret.yaml renders under, set by
	// handleBashibleContextVCP earlier in this phase. Without it Helm claims nothing.
	if input.Values.Get("nodeManager.internal.bashibleContext").String() == "" {
		return nil
	}

	snaps, err := sdkobjectpatch.UnmarshalToStruct[bashibleContextOwnership](input.Snapshots, "context")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'context' snapshot: %w", err)
	}
	// No Secret: a tenant that never had one, so Helm creates it outright.
	if len(snaps) == 0 || snaps[0].Adopted {
		return nil
	}

	input.PatchCollector.PatchWithMerge(bashibleContextHelmOwnership,
		"v1", "Secret", bashibleContextSecretNamespace, bashibleContextSecretName,
		object_patch.WithIgnoreMissingObject())

	return nil
}
