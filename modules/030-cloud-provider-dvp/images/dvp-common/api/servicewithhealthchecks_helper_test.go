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

package api

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSetServiceWithHealthchecksSpecSkipsUDPProbes(t *testing.T) {
	tests := []struct {
		name       string
		ports      []corev1.ServicePort
		wantProbes int
	}{
		{
			name: "mixed TCP and UDP ports",
			ports: []corev1.ServicePort{
				{Name: "dns-udp", Port: 53, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromInt(5353)},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8443)},
			},
			wantProbes: 1,
		},
		{
			name: "UDP-only ports leave probes empty",
			ports: []corev1.ServicePort{
				{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromInt(5353)},
			},
			wantProbes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newServiceWithHealthchecksUnstructured(testNamespace, "test")
			if err := setServiceWithHealthchecksSpec(u, tt.ports, corev1.ServiceExternalTrafficPolicyCluster, nil, nil, nil, ""); err != nil {
				t.Fatalf("setServiceWithHealthchecksSpec() error = %v", err)
			}

			probes, found, err := unstructured.NestedSlice(u.Object, "spec", "healthcheck", "probes")
			if err != nil {
				t.Fatalf("get healthcheck probes: %v", err)
			}
			if !found {
				t.Fatal("healthcheck probes are missing")
			}
			if len(probes) != tt.wantProbes {
				t.Errorf("number of healthcheck probes = %d, want %d", len(probes), tt.wantProbes)
			}
		})
	}
}
