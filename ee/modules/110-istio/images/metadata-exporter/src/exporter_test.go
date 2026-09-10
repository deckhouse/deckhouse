/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func TestExtractLoadBalancerInfo_BasicCase(t *testing.T) {
	tests := []struct {
		name     string
		services *v1.ServiceList
		want     []Gateway
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
			want: []Gateway{
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
			want: []Gateway{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLoadBalancerInfo(tt.services, ingressGatewayPortName, firstIngressEntry)

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

		got := extractLoadBalancerInfo(services, ingressGatewayPortName, firstIngressEntry)
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

		got := extractLoadBalancerInfo(services, ingressGatewayPortName, firstIngressEntry)
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
		expected      []Gateway
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
			expected: []Gateway{
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
			expected:      []Gateway{},
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
			expected: []Gateway{
				{Address: "10.0.0.1", Port: 30000},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractNodePortInfo(tt.service, tt.pods, tt.nodes, ingressGatewayPortName)
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestExtractAdvertisedGatewaysFromCM(t *testing.T) {
	tests := []struct {
		name          string
		configMap     v1.ConfigMap
		expected      []Gateway
		expectedError bool
	}{
		{
			name: "Successful extraction",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					advertisedGatewaysKey: `[{"address":"192.168.1.1","port":30000}]`,
				},
			},
			expected: []Gateway{
				{Address: "192.168.1.1", Port: 30000},
			},
			expectedError: false,
		},
		{
			name: "Only the deprecated key, written by an earlier release",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					deprecatedAdvertisedGatewaysKey: `[{"address":"192.168.1.1","port":30000}]`,
				},
			},
			expected: []Gateway{
				{Address: "192.168.1.1", Port: 30000},
			},
			expectedError: false,
		},
		{
			name: "Both keys, as the template writes them",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					advertisedGatewaysKey:           `[{"address":"192.168.1.1","port":30000}]`,
					deprecatedAdvertisedGatewaysKey: `[{"address":"192.168.1.2","port":30000}]`,
				},
			},
			expected: []Gateway{
				{Address: "192.168.1.1", Port: 30000},
			},
			expectedError: false,
		},
		{
			name: "Neither key",
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
					advertisedGatewaysKey: `invalid-json`,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "Empty JSON array",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					advertisedGatewaysKey: `[]`,
				},
			},
			expected:      []Gateway{},
			expectedError: false,
		},
		{
			name: "Several entries, including IPv6",
			configMap: v1.ConfigMap{
				Data: map[string]string{
					advertisedGatewaysKey: `[{"address":"192.168.1.1","port":15008},{"address":"2001:db8::1","port":15008}]`,
				},
			},
			expected: []Gateway{
				{Address: "192.168.1.1", Port: 15008},
				{Address: "2001:db8::1", Port: 15008},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAdvertisedGatewaysFromCM(&tt.configMap)
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestIngressGatewaysSorting(t *testing.T) {
	tests := []struct {
		name     string
		input    []Gateway
		expected []Gateway
	}{
		{
			name: "Sort by address",
			input: []Gateway{
				{Address: "192.168.1.28", Port: 32010},
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.27", Port: 32010},
				{Address: "192.168.1.26", Port: 32010},
			},
			expected: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.26", Port: 32010},
				{Address: "192.168.1.27", Port: 32010},
				{Address: "192.168.1.28", Port: 32010},
			},
		},
		{
			name: "Sort by address and port",
			input: []Gateway{
				{Address: "192.168.1.26", Port: 32011},
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.26", Port: 32010},
				{Address: "192.168.1.25", Port: 32011},
			},
			expected: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.25", Port: 32011},
				{Address: "192.168.1.26", Port: 32010},
				{Address: "192.168.1.26", Port: 32011},
			},
		},
		{
			name: "Already sorted list",
			input: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.26", Port: 32010},
				{Address: "192.168.1.27", Port: 32010},
			},
			expected: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
				{Address: "192.168.1.26", Port: 32010},
				{Address: "192.168.1.27", Port: 32010},
			},
		},
		{
			name:     "Empty list",
			input:    []Gateway{},
			expected: []Gateway{},
		},
		{
			name: "Single element",
			input: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
			},
			expected: []Gateway{
				{Address: "192.168.1.25", Port: 32010},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]Gateway, len(tt.input))
			copy(got, tt.input)

			// Apply the same sorting logic as in ingressGateways()
			sort.Slice(got, func(i, j int) bool {
				if got[i].Address != got[j].Address {
					return got[i].Address < got[j].Address
				}
				return got[i].Port < got[j].Port
			})

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d gateways, got %d", len(tt.expected), len(got))
			}

			for i := range got {
				if got[i].Address != tt.expected[i].Address {
					t.Errorf("gateway %d address mismatch: want %s, got %s", i, tt.expected[i].Address, got[i].Address)
				}
				if got[i].Port != tt.expected[i].Port {
					t.Errorf("gateway %d port mismatch: want %d, got %d", i, tt.expected[i].Port, got[i].Port)
				}
			}
		})
	}
}

func TestExtractLoadBalancerInfo_SelectsByPortName(t *testing.T) {
	services := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "ambientgateway"},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "tls", Port: 15443},
						{Name: ambientGatewayPortName, Port: 15008},
					},
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}},
					},
				},
			},
		},
	}

	got := extractLoadBalancerInfo(services, ambientGatewayPortName, anyEntryWithIP)
	if len(got) != 1 || got[0].Address != "1.2.3.4" || got[0].Port != 15008 {
		t.Fatalf("ambient port name: got %+v, want one entry 1.2.3.4:15008", got)
	}

	got = extractLoadBalancerInfo(services, ingressGatewayPortName, firstIngressEntry)
	if len(got) != 1 || got[0].Port != 15443 {
		t.Fatalf("sidecar port name: got %+v, want one entry on 15443", got)
	}

	if got := extractLoadBalancerInfo(services, "nonexistent", anyEntryWithIP); len(got) != 0 {
		t.Fatalf("unknown port name: got %+v, want nothing", got)
	}
}

func TestExtractLoadBalancerInfo_PrefersAnIPAcrossEntries(t *testing.T) {
	service := func(ingresses ...v1.LoadBalancerIngress) *v1.ServiceList {
		return &v1.ServiceList{
			Items: []v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ambientgateway"},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{{Name: ambientGatewayPortName, Port: 15008}},
					},
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{Ingress: ingresses},
					},
				},
			},
		}
	}

	cases := []struct {
		name      string
		ingresses []v1.LoadBalancerIngress
		want      string
	}{
		{
			name:      "hostname listed first does not shadow an IP",
			ingresses: []v1.LoadBalancerIngress{{Hostname: "lb.example.com"}, {IP: "1.2.3.4"}},
			want:      "1.2.3.4",
		},
		{
			name:      "IP listed first",
			ingresses: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}, {Hostname: "lb.example.com"}},
			want:      "1.2.3.4",
		},
		{
			name:      "a hostname is all there is",
			ingresses: []v1.LoadBalancerIngress{{Hostname: "lb.example.com"}},
			want:      "lb.example.com",
		},
		{
			name:      "the first of several hostnames",
			ingresses: []v1.LoadBalancerIngress{{Hostname: "a.example.com"}, {Hostname: "b.example.com"}},
			want:      "a.example.com",
		},
		{
			name:      "no address at all",
			ingresses: nil,
			want:      "",
		},
		{
			name:      "an entry carrying neither",
			ingresses: []v1.LoadBalancerIngress{{}},
			want:      "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractLoadBalancerInfo(service(tc.ingresses...), ambientGatewayPortName, anyEntryWithIP)

			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("got %+v, want no gateway", got)
				}
				return
			}

			if len(got) != 1 || got[0].Address != tc.want || got[0].Port != 15008 {
				t.Fatalf("got %+v, want one entry %s:15008", got, tc.want)
			}
		})
	}

	t.Run("the sidecar rule is unchanged", func(t *testing.T) {
		services := service(v1.LoadBalancerIngress{Hostname: "lb.example.com"}, v1.LoadBalancerIngress{IP: "1.2.3.4"})
		services.Items[0].Spec.Ports = []v1.ServicePort{{Name: ingressGatewayPortName, Port: 15443}}

		got := extractLoadBalancerInfo(services, ingressGatewayPortName, firstIngressEntry)
		if len(got) != 1 || got[0].Address != "lb.example.com" {
			t.Fatalf("got %+v, want the first entry's hostname", got)
		}
	})
}

func TestExtractNodePortInfo_SelectsByPortName(t *testing.T) {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "ambientgateway"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{Name: "tls", Port: 15443, NodePort: 30001},
				{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002},
			},
		},
	}
	pods := &v1.PodList{
		Items: []v1.Pod{{Spec: v1.PodSpec{NodeName: "node-1"}}},
	}
	nodes := &v1.NodeList{
		Items: []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: v1.NodeStatus{
					Addresses:  []v1.NodeAddress{{Type: v1.NodeExternalIP, Address: "5.6.7.8"}},
					Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
				},
			},
		},
	}

	got, err := extractNodePortInfo(service, pods, nodes, ambientGatewayPortName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Address != "5.6.7.8" || got[0].Port != 30002 {
		t.Fatalf("ambient port name: got %+v, want one entry 5.6.7.8:30002", got)
	}

	if _, err := extractNodePortInfo(service, pods, nodes, "nonexistent"); err == nil {
		t.Fatal("unknown port name: expected an error, got none")
	}
}

func TestKeepIPAddresses(t *testing.T) {
	got, dropped := keepIPAddresses([]Gateway{
		{Address: "1.2.3.4", Port: 15008},
		{Address: "lb.example.com", Port: 15008},
		{Address: "2001:db8::1", Port: 15008},
		{Address: "::ffff:5.6.7.8", Port: 15008},
	})

	want := []Gateway{
		{Address: "1.2.3.4", Port: 15008},
		{Address: "2001:db8::1", Port: 15008},
		{Address: "5.6.7.8", Port: 15008},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kept %+v, want %+v", got, want)
	}

	if wantDropped := []Gateway{{Address: "lb.example.com", Port: 15008}}; !reflect.DeepEqual(dropped, wantDropped) {
		t.Fatalf("dropped %+v, want %+v", dropped, wantDropped)
	}
}

func storeInformer(exampleObject runtime.Object, objects ...interface{}) cache.SharedInformer {
	informer := cache.NewSharedInformer(&cache.ListWatch{}, exampleObject, 0)
	for _, object := range objects {
		if err := informer.GetStore().Add(object); err != nil {
			panic(err)
		}
	}

	return informer
}

func ambientExporter(ambientGatewayInlet string, ambientService *v1.Service) *Exporter {
	exp := &Exporter{
		ingressGatewayInlet:                      "LoadBalancer",
		ambientGatewayInlet:                      ambientGatewayInlet,
		multiclusterClusterID:                    "my-domain-199688871",
		multiclusterNetworkName:                  "network-my-domain-199688871",
		multiclusterAPIHost:                      "istio-api.example.com",
		ingressGatewayServiceInformer:            storeInformer(&v1.Service{}),
		ingressGatewayAdvertiseConfigMapInformer: storeInformer(&v1.ConfigMap{}),
		ambientGatewayAdvertiseConfigMapInformer: storeInformer(&v1.ConfigMap{}),
		nodeInformer:                             storeInformer(&v1.Node{}),
		ingressGatewayPodInformer:                storeInformer(&v1.Pod{}),
		ambientGatewayPodInformer:                storeInformer(&v1.Pod{}),
	}
	if ambientService == nil {
		exp.ambientGatewayServiceInformer = storeInformer(&v1.Service{})
	} else {
		exp.ambientGatewayServiceInformer = storeInformer(&v1.Service{}, ambientService)
	}

	return exp
}

func ambientService(ports ...v1.ServicePort) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "ambientgateway", Namespace: "d8-istio"},
		Spec:       v1.ServiceSpec{Ports: ports},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}}},
		},
	}
}

func advertiseCM(t *testing.T, name string, entries ...Gateway) cache.SharedInformer {
	t.Helper()

	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("cannot build the advertise ConfigMap: %v", err)
	}

	return storeInformer(&v1.ConfigMap{}, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "d8-istio"},
		Data:       map[string]string{advertisedGatewaysKey: string(data)},
	})
}

func TestRenderMulticlusterPrivateMetadataJSON_AmbientGateways(t *testing.T) {
	hbone := v1.ServicePort{Name: ambientGatewayPortName, Port: 15008}

	t.Run("published when the gateway has a usable address", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(
			v1.ServicePort{Name: ingressGatewayPortName, Port: 15443},
			hbone,
		))

		var pm MulticlusterPrivateMetadata
		if err := json.Unmarshal([]byte(exp.RenderMulticlusterPrivateMetadataJSON()), &pm); err != nil {
			t.Fatalf("cannot parse the rendered document: %v", err)
		}
		if pm.AmbientGateways == nil {
			t.Fatal("ambientGateways missing")
		}
		if want := (Gateway{Address: "1.2.3.4", Port: 15008}); (*pm.AmbientGateways)[0] != want {
			t.Fatalf("got %+v, want %+v", (*pm.AmbientGateways)[0], want)
		}
	})

	for _, tt := range []struct {
		name    string
		service *v1.Service
	}{
		{"not deployed", nil},
		{"deployed with no usable address", ambientService(v1.ServicePort{Name: ingressGatewayPortName, Port: 15443})},
	} {
		t.Run("absent when "+tt.name, func(t *testing.T) {
			exp := ambientExporter("LoadBalancer", tt.service)

			rendered := exp.RenderMulticlusterPrivateMetadataJSON()
			if strings.Contains(rendered, "ambientGateways") {
				t.Fatalf("ambientGateways present: %s", rendered)
			}

			assertSidecarHalfIntact(t, rendered)
		})
	}
}

func TestAmbientGateways_Unusable(t *testing.T) {
	hbone := v1.ServicePort{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}

	tests := []struct {
		name                string
		ambientGatewayInlet string
		setUp               func(*Exporter)
	}{
		{
			name:                "the Service has no hbone port",
			ambientGatewayInlet: "LoadBalancer",
			setUp: func(exp *Exporter) {
				exp.ambientGatewayServiceInformer = storeInformer(&v1.Service{},
					ambientService(v1.ServicePort{Name: ingressGatewayPortName, Port: 15443}))
			},
		},
		{
			name:                "the node port is not assigned yet",
			ambientGatewayInlet: "NodePort",
			setUp: func(exp *Exporter) {
				exp.ambientGatewayServiceInformer = storeInformer(&v1.Service{},
					ambientService(v1.ServicePort{Name: ambientGatewayPortName, Port: 15008}))
			},
		},
		{
			name:                "the load balancer has not assigned an address yet",
			ambientGatewayInlet: "LoadBalancer",
			setUp: func(exp *Exporter) {
				exp.ambientGatewayServiceInformer.GetStore().List()[0].(*v1.Service).Status.LoadBalancer.Ingress = nil
			},
		},
		{
			name:                "no node is running a gateway pod",
			ambientGatewayInlet: "NodePort",
			setUp:               func(*Exporter) {},
		},
		{
			name:                "every address is a hostname",
			ambientGatewayInlet: "LoadBalancer",
			setUp: func(exp *Exporter) {
				exp.ambientGatewayServiceInformer.GetStore().List()[0].(*v1.Service).Status.LoadBalancer.Ingress =
					[]v1.LoadBalancerIngress{{Hostname: "lb.example.com"}}
			},
		},
		{
			name:                "the inlet is not one we know",
			ambientGatewayInlet: "",
			setUp:               func(*Exporter) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ambientGatewayAddressUnusable.Set(0)

			exp := ambientExporter(tt.ambientGatewayInlet, ambientService(hbone))
			tt.setUp(exp)

			gateways, err := exp.ambientGateways()
			if err == nil {
				t.Fatalf("got %+v and no error, want a reason there is no address", gateways)
			}
			if gateways != nil {
				t.Fatalf("got %+v alongside the error, want nothing", gateways)
			}

			if got := gaugeValue(t, ambientGatewayAddressUnusable); got != 0 {
				t.Fatalf("metric is %v before reporting, want 0: the request path must not write it", got)
			}
			if reportErr := exp.reportAmbientGatewayState(); reportErr == nil {
				t.Fatal("reportAmbientGatewayState returned no error, want the same reason")
			}
			if got := gaugeValue(t, ambientGatewayAddressUnusable); got != 1 {
				t.Fatalf("metric is %v, want 1 so the alert can fire", got)
			}

			rendered := exp.RenderMulticlusterPrivateMetadataJSON()
			if strings.Contains(rendered, "ambientGateways") {
				t.Fatalf("ambientGateways present: %s", rendered)
			}
			assertSidecarHalfIntact(t, rendered)
		})
	}
}

func TestAmbientGateways_MetricClears(t *testing.T) {
	hbone := v1.ServicePort{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}

	for _, tt := range []struct {
		name    string
		service *v1.Service
	}{
		{"an address is usable again", ambientService(hbone)},
		{"the gateway is undeployed", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ambientGatewayAddressUnusable.Set(1)

			exp := ambientExporter("LoadBalancer", tt.service)
			if err := exp.reportAmbientGatewayState(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := gaugeValue(t, ambientGatewayAddressUnusable); got != 0 {
				t.Fatalf("metric is %v, want 0 so the alert stops firing", got)
			}
		})
	}
}

func TestAmbientGateways_Advertised(t *testing.T) {
	hbone := v1.ServicePort{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}

	tests := []struct {
		name                string
		ambientGatewayInlet string
		entry               Gateway
		want                Gateway
	}{
		{
			name:                "the advertised address and port are used as given",
			ambientGatewayInlet: "LoadBalancer",
			entry:               Gateway{Address: "172.16.0.5", Port: 16008},
			want:                Gateway{Address: "172.16.0.5", Port: 16008},
		},
		{
			name:                "and under the NodePort inlet too",
			ambientGatewayInlet: "NodePort",
			entry:               Gateway{Address: "172.16.0.5", Port: 16008},
			want:                Gateway{Address: "172.16.0.5", Port: 16008},
		},
		{
			name:                "an IPv6 address survives",
			ambientGatewayInlet: "LoadBalancer",
			entry:               Gateway{Address: "2001:db8::1", Port: 15008},
			want:                Gateway{Address: "2001:db8::1", Port: 15008},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := ambientExporter(tt.ambientGatewayInlet, ambientService(hbone))
			exp.ambientGatewayAdvertiseConfigMapInformer = advertiseCM(t, ambientGatewayAdvertiseConfigMapName, tt.entry)

			gateways, err := exp.ambientGateways()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(gateways) != 1 {
				t.Fatalf("got %+v, want exactly one gateway", gateways)
			}
			if gateways[0] != tt.want {
				t.Fatalf("got %+v, want %+v", gateways[0], tt.want)
			}
		})
	}

	t.Run("a malformed advertise ConfigMap is a reason, not a panic", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(hbone))
		exp.ambientGatewayAdvertiseConfigMapInformer = storeInformer(&v1.ConfigMap{}, &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ambientGatewayAdvertiseConfigMapName, Namespace: "d8-istio"},
			Data:       map[string]string{advertisedGatewaysKey: "not-json"},
		})

		if gateways, err := exp.ambientGateways(); err == nil {
			t.Fatalf("got %+v and no error, want the parse failure reported", gateways)
		}
		assertSidecarHalfIntact(t, exp.RenderMulticlusterPrivateMetadataJSON())
	})

	t.Run("an advertised hostname is still rejected", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(hbone))
		exp.ambientGatewayAdvertiseConfigMapInformer = advertiseCM(t, ambientGatewayAdvertiseConfigMapName,
			Gateway{Address: "lb.example.com", Port: 15008})

		if gateways, err := exp.ambientGateways(); err == nil {
			t.Fatalf("got %+v and no error, want the address rejected", gateways)
		}
	})
}

func TestAmbientGateways_IndependentFromTheSidecarGateway(t *testing.T) {
	hbone := v1.ServicePort{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}

	t.Run("a hostname advertised for the sidecar gateway does not suppress ours", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(hbone))
		exp.ingressGatewayAdvertiseConfigMapInformer = advertiseCM(t, ingressGatewayAdvertiseConfigMapName,
			Gateway{Address: "istio-ew.example.com", Port: 15443})

		gateways, err := exp.ambientGateways()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := (Gateway{Address: "1.2.3.4", Port: 15008}); len(gateways) != 1 || gateways[0] != want {
			t.Fatalf("got %+v, want the observed %+v", gateways, want)
		}
	})

	t.Run("each gateway advertises its own address", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(hbone))
		exp.ingressGatewayAdvertiseConfigMapInformer = advertiseCM(t, ingressGatewayAdvertiseConfigMapName,
			Gateway{Address: "172.16.0.5", Port: 15443})
		exp.ambientGatewayAdvertiseConfigMapInformer = advertiseCM(t, ambientGatewayAdvertiseConfigMapName,
			Gateway{Address: "172.16.0.6", Port: 15008})

		gateways, err := exp.ambientGateways()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := (Gateway{Address: "172.16.0.6", Port: 15008}); len(gateways) != 1 || gateways[0] != want {
			t.Fatalf("got %+v, want %+v", gateways, want)
		}
	})

	t.Run("each gateway has its own inlet", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", ambientService(hbone))
		exp.ingressGatewayInlet = "NodePort"

		gateways, err := exp.ambientGateways()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := (Gateway{Address: "1.2.3.4", Port: 15008}); len(gateways) != 1 || gateways[0] != want {
			t.Fatalf("got %+v, want %+v: the ambient inlet must not be read off INLET", gateways, want)
		}
	})
}

func assertSidecarHalfIntact(t *testing.T, rendered string) {
	t.Helper()

	var pm MulticlusterPrivateMetadata
	if err := json.Unmarshal([]byte(rendered), &pm); err != nil {
		t.Fatalf("cannot parse the rendered document: %v", err)
	}
	if pm.IngressGateways == nil || pm.APIHost == "" || pm.ClusterID == "" || pm.NetworkName == "" {
		t.Fatalf("the sidecar half of the document went missing with it: %s", rendered)
	}
}

func gaugeValue(t *testing.T, gauge prometheus.Gauge) float64 {
	t.Helper()

	var metric dto.Metric
	if err := gauge.Write(&metric); err != nil {
		t.Fatalf("cannot read the gauge: %v", err)
	}

	return metric.GetGauge().GetValue()
}

func TestIngressGateways_DeployedButNoPort(t *testing.T) {
	exp := ambientExporter("LoadBalancer", nil)
	exp.ingressGatewayInlet = "NodePort"
	exp.ingressGatewayServiceInformer = storeInformer(&v1.Service{}, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "ingressgateway", Namespace: "d8-istio"},
		Spec: v1.ServiceSpec{
			Type:  v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}},
		},
	})

	if _, err := exp.ingressGateways(); err == nil {
		t.Fatal("service without the tls port: expected an error, got none")
	}
}

func TestPrivateMetadata_OmitsGatewaysItCouldNotLookUp(t *testing.T) {
	t.Run("a failed lookup leaves the field out", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", nil)
		exp.ingressGatewayInlet = "NodePort"
		exp.ingressGatewayServiceInformer = storeInformer(&v1.Service{}, &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "ingressgateway", Namespace: "d8-istio"},
			Spec: v1.ServiceSpec{
				Type:  v1.ServiceTypeNodePort,
				Ports: []v1.ServicePort{{Name: ambientGatewayPortName, Port: 15008, NodePort: 30002}},
			},
		})

		if _, err := exp.ingressGateways(); err == nil {
			t.Fatal("the fixture stopped failing, so the documents below prove nothing")
		}

		for name, rendered := range map[string]string{
			"multicluster": exp.RenderMulticlusterPrivateMetadataJSON(),
			"federation":   exp.RenderFederationPrivateMetadataJSON(),
		} {
			if strings.Contains(rendered, "ingressGateways") {
				t.Fatalf("%s document published ingressGateways after a failed lookup: %s", name, rendered)
			}
		}
	})

	t.Run("a successful lookup with no gateways still publishes an empty list", func(t *testing.T) {
		exp := ambientExporter("LoadBalancer", nil)

		gateways, err := exp.ingressGateways()
		if err != nil || len(gateways) != 0 {
			t.Fatalf("got %+v and %v, want no gateways and no error", gateways, err)
		}

		var pm MulticlusterPrivateMetadata
		if err := json.Unmarshal([]byte(exp.RenderMulticlusterPrivateMetadataJSON()), &pm); err != nil {
			t.Fatalf("cannot parse the document: %v", err)
		}
		if pm.IngressGateways == nil {
			t.Fatal("ingressGateways absent, want an empty list: a peer must tell this apart from a failed lookup")
		}
		if len(*pm.IngressGateways) != 0 {
			t.Fatalf("got %+v, want an empty list", *pm.IngressGateways)
		}
	})
}
