/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestExtractLoadBalancerInfo_BasicCase(t *testing.T) {
	tests := []struct {
		name     string
		services *v1.ServiceList
		want     []IngressGateway
		wantErr  bool
	}{
		{
			name: "service with IP and TLS port",
			services: &v1.ServiceList{
				Items: []v1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "svc-valid"},
						Status: v1.ServiceStatus{
							LoadBalancer: v1.LoadBalancerStatus{
								Ingress: []v1.LoadBalancerIngress{
									{IP: "10.0.0.1", Hostname: ""},
								},
							},
						},
						Spec: v1.ServiceSpec{
							Ports: []v1.ServicePort{
								{Name: "tls", Port: 443},
								{Name: "http", Port: 80},
							},
						},
					},
				},
			},
			want: []IngressGateway{
				{Address: "10.0.0.1", Port: 443},
			},
		},
		{
			name: "service with Hostname and no TLS port",
			services: &v1.ServiceList{
				Items: []v1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "svc-no-tls"},
						Status: v1.ServiceStatus{
							LoadBalancer: v1.LoadBalancerStatus{
								Ingress: []v1.LoadBalancerIngress{
									{Hostname: "example.com"},
								},
							},
						},
						Spec: v1.ServiceSpec{
							Ports: []v1.ServicePort{
								{Name: "http", Port: 80},
							},
						},
					},
				},
			},
			want: []IngressGateway{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractLoadBalancerInfo(tt.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractLoadBalancerInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("expected %d gateways, got %d", len(tt.want), len(got))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("gateway %d mismatch:\nwant: %+v\ngot:  %+v", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestExtractLoadBalancerInfo_EdgeCases(t *testing.T) {
	t.Run("multiple ingress entries", func(t *testing.T) {
		services := &v1.ServiceList{
			Items: []v1.Service{
				{
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								{Hostname: "backup.example.com"},
								{IP: "10.0.0.2"},
							},
						},
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{Name: "tls", Port: 8443},
						},
					},
				},
			},
		}

		got, _ := extractLoadBalancerInfo(services)
		if len(got) != 1 {
			t.Fatalf("expected 1 gateway, got %d", len(got))
		}

		if got[0].Address != "backup.example.com" {
			t.Errorf("expected first ingress hostname, got: %s", got[0].Address)
		}
	})

	t.Run("mixed valid and invalid services", func(t *testing.T) {
		services := &v1.ServiceList{
			Items: []v1.Service{
				{
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{},
						},
					},
				},
				{
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								{IP: "10.0.0.3"},
							},
						},
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{Name: "tls", Port: 443},
						},
					},
				},
			},
		}

		got, _ := extractLoadBalancerInfo(services)
		if len(got) != 1 {
			t.Errorf("expected 1 valid gateway, got %d", len(got))
		}
	})
}

func TestExtractNodePortInfo(t *testing.T) {
	tests := []struct {
		name          string
		service       *v1.Service
		pods          *v1.PodList
		nodes         *v1.NodeList
		expected      []IngressGateway
		expectedError bool
	}{
		{
			name: "Successful extraction",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "tls", NodePort: 30000},
					},
				},
			},
			pods: &v1.PodList{
				Items: []v1.Pod{
					{Spec: v1.PodSpec{NodeName: "node1"}},
				},
			},
			nodes: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: v1.NodeStatus{
							Addresses: []v1.NodeAddress{
								{Type: v1.NodeExternalIP, Address: "192.168.1.1"},
							},
							Conditions: []v1.NodeCondition{
								{Type: v1.NodeReady, Status: v1.ConditionTrue},
							},
						},
					},
				},
			},
			expected: []IngressGateway{
				{Address: "192.168.1.1", Port: 30000},
			},
			expectedError: false,
		},
		{
			name: "No TLS port found",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "http", NodePort: 30001},
					},
				},
			},
			pods:          &v1.PodList{},
			nodes:         &v1.NodeList{},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "Node not ready",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "tls", NodePort: 30000},
					},
				},
			},
			pods: &v1.PodList{
				Items: []v1.Pod{
					{Spec: v1.PodSpec{NodeName: "node1"}},
				},
			},
			nodes: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: v1.NodeStatus{
							Addresses: []v1.NodeAddress{
								{Type: v1.NodeExternalIP, Address: "192.168.1.1"},
							},
							Conditions: []v1.NodeCondition{
								{Type: v1.NodeReady, Status: v1.ConditionFalse},
							},
						},
					},
				},
			},
			expected:      []IngressGateway{},
			expectedError: false,
		},
		{
			name: "No external IP, fallback to internal IP",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "tls", NodePort: 30000},
					},
				},
			},
			pods: &v1.PodList{
				Items: []v1.Pod{
					{Spec: v1.PodSpec{NodeName: "node1"}},
				},
			},
			nodes: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: v1.NodeStatus{
							Addresses: []v1.NodeAddress{
								{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
							},
							Conditions: []v1.NodeCondition{
								{Type: v1.NodeReady, Status: v1.ConditionTrue},
							},
						},
					},
				},
			},
			expected: []IngressGateway{
				{Address: "10.0.0.1", Port: 30000},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractNodePortInfo(tt.service, tt.pods, tt.nodes)
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestExtractIngressGatewaysFromCM(t *testing.T) {
	tests := []struct {
		name          string
		configMap     v1.ConfigMap
		expected      []IngressGateway
		expectedError bool
	}{
		{
			name: "Successful extraction",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					"ingressgateways-array.json": `[{"address":"192.168.1.1","port":30000}]`,
				},
			},
			expected: []IngressGateway{
				{Address: "192.168.1.1", Port: 30000},
			},
			expectedError: false,
		},
		{
			name: "Missing ingressgateways-array.json key",
			configMap: v1.ConfigMap{
				Data: map[string]string{},
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "Invalid JSON format",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					"ingressgateways-array.json": `invalid-json`,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "Empty JSON array",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					"ingressgateways-array.json": `[]`,
				},
			},
			expected:      []IngressGateway{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractIngressGatewaysFromCM(&tt.configMap)
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}
		})
	}
}
