/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointDisruptionBudget(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	minReplicas := effectiveMinReplicas(&instance.Spec)

	if minReplicas < 2 {
		return r.deleteChildIfOwned(ctx, instance, &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceBaseName(instance.Name),
				Namespace: instance.Namespace,
			},
		})
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, pdb, func() error {
		pdb.Labels = make(map[string]string)
		for k, v := range instanceLabels(instance) {
			pdb.Labels[k] = v
		}
		pdb.Labels[WaypointComponentLabelKey] = "pdb"

		pdb.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: func() *intstr.IntOrString {
				value := intstr.FromInt32(1)
				return &value
			}(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					AppLabelKey:                              AppLabelValue,
					"gateway.networking.k8s.io/gateway-name": resourceBaseName(instance.Name),
				},
			},
		}

		if err := controllerutil.SetControllerReference(instance, pdb, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"PDB spec set",
			"name", pdb.Name,
			"namespace", pdb.Namespace,
		)

		return nil
	})

	return err
}
