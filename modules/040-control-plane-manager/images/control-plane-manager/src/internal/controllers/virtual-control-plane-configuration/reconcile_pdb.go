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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const pdbManifestKey = "pdb.yaml.tpl"

// reconcilePDB keeps the per-component PodDisruptionBudgets in step with HA: a PDB over a single
// replica blocks node drains instead of protecting anything.
func (r *reconciler) reconcilePDB(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	if vcp.Spec.HighAvailability {
		return reconcile.Result{}, r.applyParentManifests(ctx, vcp, configSecret, pdbManifestKey)
	}

	// One collection delete by label for a single request per reconcile
	err := r.client.DeleteAllOf(ctx, &policyv1.PodDisruptionBudget{},
		client.InNamespace(vcp.Namespace),
		client.MatchingLabels{constants.VirtualControlPlaneScopeLabelKey: vcp.Name},
	)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("delete PodDisruptionBudgets: %w", err)
	}

	return reconcile.Result{}, nil
}
