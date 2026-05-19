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

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointHPA(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	if instance.Spec.ReplicasManagement == nil || instance.Spec.ReplicasManagement.Mode != "HPA" {
		return r.deleteChildIfOwned(ctx, instance, &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceBaseName(instance.Name),
				Namespace: instance.Namespace,
			},
		})
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, hpa, func() error {
		hpa.Labels = make(map[string]string)
		for k, v := range instanceLabels(instance) {
			hpa.Labels[k] = v
		}
		hpa.Labels[WaypointComponentLabelKey] = "hpa"

		minReplicas := int32(1)
		maxReplicas := int32(3)
		metrics := []autoscalingv2.MetricSpec{}

		if hpaCfg := instance.Spec.ReplicasManagement.HPA; hpaCfg != nil {
			if hpaCfg.MinReplicas > 0 {
				minReplicas = hpaCfg.MinReplicas
			}
			if hpaCfg.MaxReplicas > 0 {
				maxReplicas = hpaCfg.MaxReplicas
			}
			for _, m := range hpaCfg.Metrics {
				if m.Type == "CPU" {
					metrics = append(metrics, autoscalingv2.MetricSpec{
						Type: "Resource",
						Resource: &autoscalingv2.ResourceMetricSource{
							Name: "cpu",
							Target: autoscalingv2.MetricTarget{
								Type:               autoscalingv2.UtilizationMetricType,
								AverageUtilization: &m.TargetAverageUtilization,
							},
						},
					})
				}
			}
		}

		if len(metrics) == 0 {
			metrics = []autoscalingv2.MetricSpec{
				{
					Type: "Resource",
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(int32(80)),
						},
					},
				},
			}
		}

		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       resourceBaseName(instance.Name),
			},
			Metrics: metrics,
		}

		if err := controllerutil.SetControllerReference(instance, hpa, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"HPA spec set",
			"name", hpa.Name,
			"namespace", hpa.Namespace,
		)

		return nil
	})

	return err
}
