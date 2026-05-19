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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointVPA(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	if !r.VPAEnabled {
		return nil
	}

	if instance.Spec.ReplicasManagement != nil && instance.Spec.ReplicasManagement.Mode == "HPA" {
		return r.deleteChildIfOwned(ctx, instance, &v1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceBaseName(instance.Name),
				Namespace: instance.Namespace,
			},
		})
	}

	if instance.Spec.ResourcesManagement != nil && instance.Spec.ResourcesManagement.Mode == "Static" {
		return r.deleteChildIfOwned(ctx, instance, &v1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceBaseName(instance.Name),
				Namespace: instance.Namespace,
			},
		})
	}

	vpa := &v1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, vpa, func() error {
		desired, err := newVPAForWaypoint(instance)
		if err != nil {
			return err
		}

		vpa.Labels = desired.Labels
		vpa.Spec = desired.Spec

		if err := controllerutil.SetControllerReference(instance, vpa, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"VPA spec set",
			"name", vpa.Name,
			"namespace", vpa.Namespace,
		)

		return nil
	})

	return err
}
