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

package waypointcontroller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointServiceAccount(ctx context.Context, instance *networkv1alpha1.WaypointInstance) (string, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, sa, func() error {
		sa.Labels = make(map[string]string)
		for k, v := range instanceLabels(instance) {
			sa.Labels[k] = v
		}
		for k, v := range istioLabels(instance, r.istioRevision, r.istioNetworkName) {
			sa.Labels[k] = v
		}
		sa.Labels["gateway.networking.k8s.io/gateway-name"] = resourceBaseName(instance.Name)
		sa.Labels[WaypointComponentLabelKey] = "serviceaccount"

		if err := controllerutil.SetControllerReference(instance, sa, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"ServiceAccount spec set",
			"name", sa.Name,
			"namespace", sa.Namespace,
		)

		return nil
	})

	return sa.Name, err
}
