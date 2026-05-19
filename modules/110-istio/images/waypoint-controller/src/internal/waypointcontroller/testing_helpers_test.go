//go:build !integration

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
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

// newWaypointPodSpecConfig returns a baseline waypointPodSpecConfig used by
// pod-spec/env tests. Individual tests can override fields as needed.
func newWaypointPodSpecConfig() waypointPodSpecConfig {
	return waypointPodSpecConfig{
		InstanceName:       "main",
		Namespace:          "test-ns",
		ClusterDomain:      "cluster.local",
		ProxyImage:         "registry.example.com/istio/proxyv2:test",
		ServiceAccount:     "d8-waypoint-main",
		IstioRevision:      "v1x25x2",
		IstioNetworkName:   "test-network",
		IstioCloudPlatform: "none",
		IstioClusterID:     "test-cluster",
	}
}

func newInstance(name, namespace string, opts ...func(*networkv1alpha1.WaypointInstance)) *networkv1alpha1.WaypointInstance {
	inst := &networkv1alpha1.WaypointInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, o := range opts {
		o(inst)
	}
	return inst
}

func withWaypointFor(v string) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		i.Spec.WaypointFor = v
	}
}

func withStaticReplicas(n int32) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.ReplicasManagement == nil {
			i.Spec.ReplicasManagement = &networkv1alpha1.ReplicasManagement{}
		}
		i.Spec.ReplicasManagement.Mode = "Static"
		i.Spec.ReplicasManagement.Static = &networkv1alpha1.ReplicasStatic{Replicas: n}
	}
}

func withHPAMode(minReplicas, maxReplicas, cpuUtil int32) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.ReplicasManagement == nil {
			i.Spec.ReplicasManagement = &networkv1alpha1.ReplicasManagement{}
		}
		i.Spec.ReplicasManagement.Mode = "HPA"
		i.Spec.ReplicasManagement.HPA = &networkv1alpha1.ReplicasHPA{
			MinReplicas: minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []networkv1alpha1.HPAMetric{
				{Type: "CPU", TargetAverageUtilization: cpuUtil},
			},
		}
	}
}

func withReplicasManagementMode(mode string) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.ReplicasManagement == nil {
			i.Spec.ReplicasManagement = &networkv1alpha1.ReplicasManagement{}
		}
		i.Spec.ReplicasManagement.Mode = mode
	}
}

func withResourcesManagementMode(mode string) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.ResourcesManagement == nil {
			i.Spec.ResourcesManagement = &networkv1alpha1.ResourcesManagement{}
		}
		i.Spec.ResourcesManagement.Mode = mode
	}
}

func withVPAMode(mode string) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.ResourcesManagement == nil {
			i.Spec.ResourcesManagement = &networkv1alpha1.ResourcesManagement{}
		}
		i.Spec.ResourcesManagement.Mode = "VPA"
		if i.Spec.ResourcesManagement.VPA == nil {
			i.Spec.ResourcesManagement.VPA = &networkv1alpha1.ResourcesVPA{}
		}
		i.Spec.ResourcesManagement.VPA.Mode = mode
	}
}

func mustParseQ(t *testing.T, s string) resource.Quantity {
	t.Helper()
	if s == "" {
		return resource.Quantity{}
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		t.Fatalf("failed to parse quantity %q: %v", s, err)
	}
	return q
}

func limitRatioPtr(v float64) *float64 {
	return &v
}
