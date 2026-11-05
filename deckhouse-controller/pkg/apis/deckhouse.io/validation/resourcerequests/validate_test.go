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

package resourcerequests

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func quantity(value string) *resource.Quantity {
	q := resource.MustParse(value)
	return &q
}

// bound returns the bound of cpu and memory, an empty value leaving the resource out.
func bound(cpu, memory string) *v1alpha1.ApplicationContainerResourceBounds {
	b := new(v1alpha1.ApplicationContainerResourceBounds)

	if cpu != "" {
		b.CPU = quantity(cpu)
	}

	if memory != "" {
		b.Memory = quantity(memory)
	}

	return b
}

// backend requests the Deployment d8a-shop-backend with the container backend of the resources.
func backend(resources v1alpha1.ApplicationContainerResources) v1alpha1.ApplicationResourceRequest {
	return v1alpha1.ApplicationResourceRequest{
		Kind:       "Deployment",
		Name:       "d8a-shop-backend",
		Containers: []v1alpha1.ApplicationContainerResourceRequest{{Name: "backend", Resources: resources}},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		requests []v1alpha1.ApplicationResourceRequest
		want     []string
	}{
		{
			name: "ranges",
			requests: []v1alpha1.ApplicationResourceRequest{{
				Kind:        "Deployment",
				Name:        "d8a-shop-backend",
				MinReplicas: new(int32(2)),
				MaxReplicas: new(int32(10)),
				Containers: []v1alpha1.ApplicationContainerResourceRequest{{
					Name:      "backend",
					Resources: v1alpha1.ApplicationContainerResources{Min: bound("250m", "256Mi"), Max: bound("2", "2Gi")},
				}},
			}},
		},
		{
			name: "fixed values",
			requests: []v1alpha1.ApplicationResourceRequest{{
				Kind:     "StatefulSet",
				Name:     "d8a-shop-db",
				Replicas: new(int32(3)),
				Containers: []v1alpha1.ApplicationContainerResourceRequest{{
					Name: "postgres",
					Resources: v1alpha1.ApplicationContainerResources{ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
					}},
				}},
			}},
		},
		{
			name: "equal bounds and ephemeral storage next to a range",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceEphemeralStorage: resource.MustParse("1Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceEphemeralStorage: resource.MustParse("2Gi")},
				},
				Min: bound("500m", ""),
				Max: bound("0.5", ""),
			})},
		},
		{
			name: "replicas next to a replicas range",
			requests: []v1alpha1.ApplicationResourceRequest{{
				Kind:        "Deployment",
				Name:        "d8a-shop-backend",
				Replicas:    new(int32(3)),
				MinReplicas: new(int32(2)),
				MaxReplicas: new(int32(10)),
			}},
			want: []string{"Deployment 'd8a-shop-backend': replicas cannot be set together with minReplicas and maxReplicas. Set replicas to fix the number of replicas, or minReplicas and maxReplicas to let HorizontalPodAutoscaler choose it within the range"},
		},
		{
			name:     "upper bound of the replicas alone",
			requests: []v1alpha1.ApplicationResourceRequest{{Kind: "StatefulSet", Name: "d8a-shop-db", MaxReplicas: new(int32(5))}},
			want:     []string{"StatefulSet 'd8a-shop-db': set both minReplicas and maxReplicas: a range needs both bounds, but only maxReplicas is set"},
		},
		{
			name: "inverted replicas range",
			requests: []v1alpha1.ApplicationResourceRequest{{
				Kind:        "Deployment",
				Name:        "d8a-shop-backend",
				MinReplicas: new(int32(5)),
				MaxReplicas: new(int32(2)),
			}},
			want: []string{"Deployment 'd8a-shop-backend': minReplicas (5) is greater than maxReplicas (2)"},
		},
		{
			name: "replicas range of a DaemonSet",
			requests: []v1alpha1.ApplicationResourceRequest{{
				Kind:        "DaemonSet",
				Name:        "d8a-shop-agent",
				MinReplicas: new(int32(1)),
				MaxReplicas: new(int32(3)),
			}},
			want: []string{"DaemonSet 'd8a-shop-agent': minReplicas and maxReplicas apply only to Deployment, StatefulSet, ReplicaSet, as only those kinds have replicas"},
		},
		{
			name: "fixed resources next to a range",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
				Min: bound("", "256Mi"),
				Max: bound("", "2Gi"),
			})},
			want: []string{"Deployment 'd8a-shop-backend', container 'backend': cpu request and memory limit cannot be set together with min and max, as VerticalPodAutoscaler sets the requests within the range. Remove them, or replace min and max with requests and limits to fix the resources"},
		},
		{
			name:     "lower bound alone",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{Min: bound("250m", "")})},
			want:     []string{"Deployment 'd8a-shop-backend', container 'backend': set both min and max: a range needs both bounds, but only min is set"},
		},
		{
			name:     "bounds of different resources",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{Min: bound("250m", ""), Max: bound("", "1Gi")})},
			want: []string{
				"Deployment 'd8a-shop-backend', container 'backend': set cpu in both min and max: a range needs both bounds, but cpu is only in min",
				"Deployment 'd8a-shop-backend', container 'backend': set memory in both min and max: a range needs both bounds, but memory is only in max",
			},
		},
		{
			name:     "empty bounds",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{Min: bound("", ""), Max: bound("", "")})},
			want:     []string{"Deployment 'd8a-shop-backend', container 'backend': min and max must set cpu, memory or both"},
		},
		{
			name:     "inverted cpu range across units",
			requests: []v1alpha1.ApplicationResourceRequest{backend(v1alpha1.ApplicationContainerResources{Min: bound("1500m", "1Gi"), Max: bound("1", "1Gi")})},
			want:     []string{"Deployment 'd8a-shop-backend', container 'backend': min cpu (1500m) is greater than max cpu (1)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Validate(tt.requests))
		})
	}
}

// Every problem of every workload and container is reported, in the order of the requests.
func TestValidateReportsEveryProblem(t *testing.T) {
	problems := Validate([]v1alpha1.ApplicationResourceRequest{
		{Kind: "DaemonSet", Name: "d8a-shop-agent", Replicas: new(int32(1)), MinReplicas: new(int32(3))},
		backend(v1alpha1.ApplicationContainerResources{Max: bound("1", "")}),
	})

	require.Equal(t, []string{
		"DaemonSet 'd8a-shop-agent': minReplicas and maxReplicas apply only to Deployment, StatefulSet, ReplicaSet, as only those kinds have replicas",
		"DaemonSet 'd8a-shop-agent': replicas cannot be set together with minReplicas and maxReplicas. Set replicas to fix the number of replicas, or minReplicas and maxReplicas to let HorizontalPodAutoscaler choose it within the range",
		"DaemonSet 'd8a-shop-agent': set both minReplicas and maxReplicas: a range needs both bounds, but only minReplicas is set",
		"Deployment 'd8a-shop-backend', container 'backend': set both min and max: a range needs both bounds, but only max is set",
	}, problems)
}
