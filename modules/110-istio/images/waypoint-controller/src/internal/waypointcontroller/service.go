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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointService(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, svc, func() error {
		svc.Labels = make(map[string]string)
		for k, v := range instanceLabels(instance) {
			svc.Labels[k] = v
		}
		for k, v := range istioLabels(instance, r.istioRevision, r.istioNetworkName) {
			svc.Labels[k] = v
		}
		svc.Labels["gateway.networking.k8s.io/gateway-name"] = resourceBaseName(instance.Name)
		svc.Labels[WaypointComponentLabelKey] = "service"

		svc.Annotations = map[string]string{
			"networking.istio.io/traffic-distribution": "PreferClose",
		}

		// Preserve server-assigned and defaulted fields before overwriting Spec.
		existingClusterIP := svc.Spec.ClusterIP
		existingClusterIPs := svc.Spec.ClusterIPs
		existingIPFamilies := svc.Spec.IPFamilies
		existingIPFamilyPolicy := svc.Spec.IPFamilyPolicy

		svc.Spec = corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				AppLabelKey:                              AppLabelValue,
				"gateway.networking.k8s.io/gateway-name": resourceBaseName(instance.Name),
			},
			Ports: []corev1.ServicePort{
				{
					Name:        "status-port",
					Protocol:    corev1.ProtocolTCP,
					Port:        15021,
					TargetPort:  intstr.FromInt(15021),
					AppProtocol: ptr.To("tcp"),
				},
				{
					Name:        "mesh",
					Protocol:    corev1.ProtocolTCP,
					Port:        15008,
					TargetPort:  intstr.FromInt(15008),
					AppProtocol: ptr.To("hbone"),
				},
			},
		}

		svc.Spec.ClusterIP = existingClusterIP
		svc.Spec.ClusterIPs = existingClusterIPs
		svc.Spec.IPFamilies = existingIPFamilies
		svc.Spec.IPFamilyPolicy = existingIPFamilyPolicy

		if err := controllerutil.SetControllerReference(instance, svc, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"Service spec set",
			"name", svc.Name,
			"namespace", svc.Namespace,
		)

		return nil
	})

	return err
}
