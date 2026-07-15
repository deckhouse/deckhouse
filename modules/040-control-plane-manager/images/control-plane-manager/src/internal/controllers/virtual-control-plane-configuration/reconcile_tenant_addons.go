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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"maps"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/bootstraptoken"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// tenantAddonManifestKeys are config-Secret template keys applied into the tenant cluster, in order.
// Each is idempotent (create if not found).
var tenantAddonManifestKeys = []string{
	"tenant-rbac.yaml.tpl",        // bootstrap CRBs + kube-apiserver-kubelet-client -> system:kubelet-api-admin
	"konnectivity-agent.yaml.tpl", // SA + DaemonSet dialing konn.<vcp>:443
	"cilium-vcp.yaml.tpl",         // CNI: agent DaemonSet + operator RBAC; operator Deployment runs in the parent cluster
}

// reconcileTenantAddons ensures all tenant-side resources exist:
// - node bootstrap-token (returned for join.sh),
// - node-bootstrapper RBAC, the apiserver->kubelet RBAC
// - konnectivity-agent
// - Cilium
// The tenant clients are built once and shared across the sub-steps.
func (r *reconciler) reconcileTenantAddons(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (string, reconcile.Result, error) {
	ts, tc, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return "", reconcile.Result{}, fmt.Errorf("build tenant clients: %w", err)
	}

	scopeLabels := map[string]string{
		constants.HeritageLabelKey: constants.HeritageLabelValue,
		"module":                   constants.ControlPlaneManagerName,
		constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
	}
	selector := fmt.Sprintf("%s=%s", constants.VirtualControlPlaneScopeLabelKey, vcp.Name)

	token, err := bootstraptoken.EnsureValid(
		ctx, ts, selector,
		[]string{constants.VirtualBootstrapTokenGroup},
		constants.VirtualBootstrapTokenTTL,
		constants.VirtualBootstrapTokenRegenBelow,
		scopeLabels,
	)
	if err != nil {
		return "", reconcile.Result{}, fmt.Errorf("ensure bootstrap token: %w", err)
	}

	for _, key := range tenantAddonManifestKeys {
		if err := applyTenantManifests(ctx, tc, configSecret, key); err != nil {
			return "", reconcile.Result{}, fmt.Errorf("apply tenant %s: %w", key, err)
		}
	}

	return token, reconcile.Result{}, nil
}

// applyTenantManifests renders a multi-doc template from the config Secret into the tenant cluster.
// Objects carry their own (tenant) namespaces.
func applyTenantManifests(ctx context.Context, tc client.Client, configSecret *corev1.Secret, key string) error {
	raw, ok := configSecret.Data[key]
	if !ok {
		return fmt.Errorf("config Secret missing %q", key)
	}

	objects, err := parseManifestDocs(raw, "")
	if err != nil {
		return err
	}

	for _, target := range objects {
		if err := applyObject(ctx, tc, target, patchTenantObject); err != nil {
			return err
		}
	}

	return nil
}

func patchTenantObject(current, target *unstructured.Unstructured) (client.Object, bool) {
	current.SetLabels(mergeMetadata(current.GetLabels(), target.GetLabels()))
	current.SetAnnotations(mergeMetadata(current.GetAnnotations(), target.GetAnnotations()))

	for _, field := range []string{"data", "spec", "rules", "roleRef", "subjects"} {
		if value, ok := target.Object[field]; ok {
			current.Object[field] = value
		}
	}

	return current, true
}

func mergeMetadata(current, target map[string]string) map[string]string {
	if len(target) == 0 {
		return current
	}
	out := make(map[string]string, len(current)+len(target))
	maps.Copy(out, current)
	maps.Copy(out, target)
	return out
}
