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
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/bootstraptoken"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

// tenantAddonManifestKeys are config-Secret template keys applied into the tenant cluster, in order.
// Each is idempotent (create if not found).
var tenantAddonManifestKeys = []string{
	"tenant-rbac.yaml.tpl",        // bootstrap CRBs + kube-apiserver-kubelet-client -> system:kubelet-api-admin
	"konnectivity-agent.yaml.tpl", // SA + DaemonSet dialing konn.<vcp>:443
	"cilium.yaml.tpl",             // CNI: agent DaemonSet + operator
}

// reconcileTenantAddons ensures all tenant-side resources exist:
// - node bootstrap-token (returned for join.sh),
// - node-bootstrapper RBAC, the apiserver->kubelet RBAC
// - konnectivity-agent
// - Cilium
// The tenant clients are built once and shared across the sub-steps.
func (r *reconciler) reconcileTenantAddons(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (string, reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ts, tc, err := r.tenantClients(ctx, vcp)
	if err != nil {
		log.Info("tenant not reachable yet, requeuing", "error", err.Error())
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
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
		log.Info("bootstrap-token ensure failed, requeuing", "error", err.Error())
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	for _, key := range tenantAddonManifestKeys {
		if err := applyTenantManifests(ctx, tc, configSecret, key); err != nil {
			return "", reconcile.Result{}, fmt.Errorf("apply tenant %s: %w", key, err)
		}
	}

	return token, reconcile.Result{}, nil
}

// applyTenantManifests renders a multi-doc template from the config Secret into the tenant cluster.
// Objects carry their own (tenant) namespaces
func applyTenantManifests(ctx context.Context, tc client.Client, configSecret *corev1.Secret, key string) error {
	raw, ok := configSecret.Data[key]
	if !ok {
		return fmt.Errorf("config Secret missing %q", key)
	}

	for _, doc := range strings.Split(string(raw), "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		target := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), target); err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}
		if len(target.Object) == 0 {
			continue
		}

		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(target.GroupVersionKind())
		err := tc.Get(ctx, client.ObjectKeyFromObject(target), current)
		if apierrors.IsNotFound(err) {
			if err := tc.Create(ctx, target); err != nil {
				return fmt.Errorf("create %s %s: %w", target.GetKind(), target.GetName(), err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("get %s %s: %w", target.GetKind(), target.GetName(), err)
		}
	}

	return nil
}
